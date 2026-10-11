package async

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// waitForStatus polls until the query leaves the running states, which is how
// a caller observes the background goroutine finishing.
func waitForStatus(t *testing.T, r *Registry, id string) Status {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		status, err := r.Status(id)
		if err != nil {
			t.Fatalf("Status(%s) returned %v", id, err)
		}
		if !status.Running() {
			return status
		}
		if time.Now().After(deadline) {
			t.Fatalf("query %s still %s after 5s", id, status)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestStartRunsQueryAndCollectsResultOnce(t *testing.T) {
	r := newRegistry()
	defer r.Close()

	frames := data.Frames{data.NewFrame("A", data.NewField("n", nil, []int64{1}))}
	id, err := r.Start(context.Background(), func(context.Context) (data.Frames, error) {
		return frames, nil
	})
	if err != nil {
		t.Fatalf("Start returned %v", err)
	}

	if status := waitForStatus(t, r, id); status != StatusFinished {
		t.Errorf("status = %s, want %s", status, StatusFinished)
	}

	got, err := r.Collect(id)
	if err != nil {
		t.Fatalf("Collect returned %v", err)
	}
	if len(got) != 1 || got[0].Name != "A" {
		t.Errorf("Collect returned %v, want the frames the query produced", got)
	}

	// The result is handed over once; a repeat poll must not find it again.
	if _, err := r.Collect(id); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Collect returned %v, want %v", err, ErrNotFound)
	}
}

func TestCollectWhileRunningFails(t *testing.T) {
	r := newRegistry()
	defer r.Close()

	release := make(chan struct{})
	id, err := r.Start(context.Background(), func(context.Context) (data.Frames, error) {
		<-release
		return nil, nil
	})
	if err != nil {
		t.Fatalf("Start returned %v", err)
	}
	defer close(release)

	if _, err := r.Collect(id); err == nil {
		t.Error("Collect on a running query returned no error")
	}
}

func TestFailedQueryReportsItsError(t *testing.T) {
	r := newRegistry()
	defer r.Close()

	queryErr := errors.New("line 1:8: column x does not exist")
	id, err := r.Start(context.Background(), func(context.Context) (data.Frames, error) {
		return nil, queryErr
	})
	if err != nil {
		t.Fatalf("Start returned %v", err)
	}

	if status := waitForStatus(t, r, id); status != StatusFailed {
		t.Errorf("status = %s, want %s", status, StatusFailed)
	}
	if _, err := r.Collect(id); !errors.Is(err, queryErr) {
		t.Errorf("Collect returned %v, want the query error", err)
	}
}

func TestStartDetachesFromTheRequestContext(t *testing.T) {
	r := newRegistry()
	defer r.Close()

	type key struct{}
	reqCtx, cancelRequest := context.WithCancel(context.WithValue(context.Background(), key{}, "token"))

	started := make(chan struct{})
	observed := make(chan error, 1)
	value := make(chan any, 1)
	id, err := r.Start(reqCtx, func(runCtx context.Context) (data.Frames, error) {
		value <- runCtx.Value(key{})
		close(started)
		// Give the cancelled request context time to propagate, if it were
		// going to.
		time.Sleep(50 * time.Millisecond)
		observed <- runCtx.Err()
		return nil, nil
	})
	if err != nil {
		t.Fatalf("Start returned %v", err)
	}

	<-started
	cancelRequest()

	if got := <-value; got != "token" {
		t.Errorf("run context value = %v, want the request context value to survive", got)
	}
	if err := <-observed; err != nil {
		t.Errorf("run context error = %v, want the query to outlive the request", err)
	}
	if status := waitForStatus(t, r, id); status != StatusFinished {
		t.Errorf("status = %s, want %s", status, StatusFinished)
	}
}

func TestCancelStopsTheRunContext(t *testing.T) {
	r := newRegistry()
	defer r.Close()

	cancelled := make(chan struct{})
	id, err := r.Start(context.Background(), func(runCtx context.Context) (data.Frames, error) {
		<-runCtx.Done()
		close(cancelled)
		return nil, runCtx.Err()
	})
	if err != nil {
		t.Fatalf("Start returned %v", err)
	}

	if err := r.Cancel(id); err != nil {
		t.Fatalf("Cancel returned %v", err)
	}

	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelling the query did not cancel the run context")
	}

	if status := waitForStatus(t, r, id); status != StatusCanceled {
		t.Errorf("status = %s, want %s", status, StatusCanceled)
	}
}

func TestUnknownHandleFromAnotherProcessIsReportedSeparately(t *testing.T) {
	r := newRegistry()
	defer r.Close()

	if _, err := r.Status("some-other-grafana:d3adbeef"); !errors.Is(err, ErrOtherInstance) {
		t.Errorf("Status of a handle from another process returned %v, want %v", err, ErrOtherInstance)
	}
	if _, err := r.Status(r.processID + ":d3adbeef"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Status of an expired handle returned %v, want %v", err, ErrNotFound)
	}
}

func TestMaxConcurrentRejectsFurtherStarts(t *testing.T) {
	r := newRegistry()
	defer r.Close()
	r.MaxConcurrent = 1

	release := make(chan struct{})
	block := func(context.Context) (data.Frames, error) {
		<-release
		return nil, nil
	}
	if _, err := r.Start(context.Background(), block); err != nil {
		t.Fatalf("first Start returned %v", err)
	}
	defer close(release)

	if _, err := r.Start(context.Background(), block); !errors.Is(err, ErrTooManyQueries) {
		t.Errorf("second Start returned %v, want %v", err, ErrTooManyQueries)
	}
}

func TestReapCancelsQueriesNobodyIsPolling(t *testing.T) {
	r := newRegistry()
	defer r.Close()

	now := time.Now()
	r.now = func() time.Time { return now }

	cancelled := make(chan struct{})
	id, err := r.Start(context.Background(), func(runCtx context.Context) (data.Frames, error) {
		<-runCtx.Done()
		close(cancelled)
		return nil, runCtx.Err()
	})
	if err != nil {
		t.Fatalf("Start returned %v", err)
	}

	// Still within the window: a client that is polling must be left alone.
	now = now.Add(r.UnpolledTimeout / 2)
	r.reap()
	if _, err := r.Status(id); err != nil {
		t.Fatalf("query was reaped while still being polled: %v", err)
	}

	now = now.Add(r.UnpolledTimeout * 2)
	r.reap()

	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("reaping did not cancel the query")
	}
	if _, err := r.Status(id); !errors.Is(err, ErrNotFound) {
		t.Errorf("Status after reaping returned %v, want %v", err, ErrNotFound)
	}
}

func TestReapDropsResultsNobodyCollects(t *testing.T) {
	r := newRegistry()
	defer r.Close()

	now := time.Now()
	r.now = func() time.Time { return now }

	id, err := r.Start(context.Background(), func(context.Context) (data.Frames, error) {
		return data.Frames{data.NewFrame("A")}, nil
	})
	if err != nil {
		t.Fatalf("Start returned %v", err)
	}
	waitForStatus(t, r, id)

	now = now.Add(r.ResultTTL * 2)
	r.reap()

	if _, err := r.Collect(id); !errors.Is(err, ErrNotFound) {
		t.Errorf("Collect after the TTL returned %v, want %v", err, ErrNotFound)
	}
}
