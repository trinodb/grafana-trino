// Package async implements the backend half of Grafana's asynchronous query
// flow for Trino.
//
// Trino's client protocol is destructive streaming: every GET of a nextUri
// consumes its page, and the coordinator abandons a query once no client has
// polled it for query.client.timeout. There is therefore no way to start a
// query, hang up, and collect the results later - whoever detaches from the
// Grafana request must keep polling Trino and keep the rows. The Registry is
// that "whoever": it runs the query in a goroutine on a context detached from
// the HTTP request, and parks the resulting frames until a later poll picks
// them up.
package async

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Status is the lifecycle of an async query. The strings must stay in sync
// with the frontend, which keeps polling while it sees submitted or running
// and stops on anything else.
type Status string

const (
	StatusSubmitted Status = "submitted"
	StatusRunning   Status = "running"
	StatusFinished  Status = "finished"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
)

// Running reports whether the frontend should poll again.
func (s Status) Running() bool {
	return s == StatusSubmitted || s == StatusRunning
}

const (
	// DefaultMaxDuration bounds how long a detached query may run. Without a
	// ceiling a query nobody polls could outlive the reaper's notice.
	DefaultMaxDuration = 6 * time.Hour

	// DefaultUnpolledTimeout is how long a query may go unpolled before it is
	// cancelled. The frontend backs off to at most 10s between polls, but
	// browsers throttle timers in background tabs to roughly once a minute,
	// and leaving a long query running in another tab is exactly what this
	// feature is for. The timeout has to clear that throttled rate by a wide
	// margin or backgrounded dashboards would have their queries reaped.
	DefaultUnpolledTimeout = 5 * time.Minute

	// DefaultResultTTL is how long a finished result waits to be collected
	// before it is dropped, for the case where the browser disappears in the
	// window between the query finishing and the next poll.
	DefaultResultTTL = 5 * time.Minute

	// DefaultMaxConcurrent caps in-flight queries per registry. Each one pins
	// a database/sql connection for its whole duration, and unlike the
	// synchronous path nothing reclaims it when the browser goes away.
	DefaultMaxConcurrent = 50

	reaperInterval = 10 * time.Second
)

var (
	// ErrNotFound is returned for a handle this process has no record of,
	// either because it was reaped or because it was never ours.
	ErrNotFound = errors.New("query not found")

	// ErrOtherInstance is returned for a well-formed handle minted by a
	// different plugin process, which means the poll was routed to another
	// Grafana instance.
	ErrOtherInstance = errors.New("query belongs to another Grafana instance")

	// ErrTooManyQueries is returned once MaxConcurrent in-flight queries are
	// already running.
	ErrTooManyQueries = errors.New("too many concurrent asynchronous queries")
)

// RunFunc executes the query. It receives a context detached from the Grafana
// request that started it.
type RunFunc func(ctx context.Context) (data.Frames, error)

type entry struct {
	status     Status
	frames     data.Frames
	err        error
	cancel     context.CancelFunc
	lastPolled time.Time
	doneAt     time.Time
}

// Registry tracks the queries this plugin process is running on behalf of
// polling clients. The zero value is not usable; call NewRegistry.
type Registry struct {
	// processID distinguishes handles minted by this process from handles
	// minted by another Grafana instance, so that a poll routed to the wrong
	// instance reports that rather than looking like an expired query.
	processID string

	MaxDuration     time.Duration
	UnpolledTimeout time.Duration
	ResultTTL       time.Duration
	MaxConcurrent   int

	// now is overridable so tests can drive the reaper without sleeping.
	now func() time.Time

	mu      sync.Mutex
	queries map[string]*entry

	stop     chan struct{}
	stopOnce sync.Once
}

// NewRegistry returns a registry with a reaper already running. Call Close to
// stop the reaper and cancel everything still in flight.
func NewRegistry() *Registry {
	r := newRegistry()
	go r.reapLoop()
	return r
}

// newRegistry builds a registry without starting the reaper, so that tests can
// drive reap directly instead of waiting on a ticker.
func newRegistry() *Registry {
	return &Registry{
		processID:       uuid.NewString(),
		MaxDuration:     DefaultMaxDuration,
		UnpolledTimeout: DefaultUnpolledTimeout,
		ResultTTL:       DefaultResultTTL,
		MaxConcurrent:   DefaultMaxConcurrent,
		now:             time.Now,
		queries:         map[string]*entry{},
		stop:            make(chan struct{}),
	}
}

// Start runs fn in the background and returns the handle the client polls
// with. The handle is minted before fn is scheduled because the first response
// must already carry it: at that point nothing has been sent to Trino yet, so
// Trino's own query ID does not exist.
func (r *Registry) Start(ctx context.Context, fn RunFunc) (string, error) {
	// Keep the request context's values - the OAuth access token and the
	// impersonated user live there - while severing its cancellation, so the
	// query outlives the HTTP request that started it.
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.MaxDuration)

	id := r.processID + ":" + uuid.NewString()
	now := r.now()

	r.mu.Lock()
	if r.MaxConcurrent > 0 && r.inFlightLocked() >= r.MaxConcurrent {
		r.mu.Unlock()
		cancel()
		return "", ErrTooManyQueries
	}
	r.queries[id] = &entry{status: StatusSubmitted, cancel: cancel, lastPolled: now}
	r.mu.Unlock()

	go func() {
		defer cancel()
		frames, err := fn(runCtx)
		r.finish(id, frames, err)
	}()

	return id, nil
}

// Status reports on a query without consuming it, and records the poll so the
// reaper can tell a live client from an abandoned one.
func (r *Registry) Status(id string) (Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.queries[id]
	if !ok {
		return "", r.missLocked(id)
	}
	e.lastPolled = r.now()
	return e.status, nil
}

// Collect returns the result of a finished query and forgets it. Calling it
// for a query that is still running returns ErrNotFound, so callers should
// check Status first.
func (r *Registry) Collect(id string) (data.Frames, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.queries[id]
	if !ok {
		return nil, r.missLocked(id)
	}
	if e.status.Running() {
		return nil, fmt.Errorf("query %s is still %s", id, e.status)
	}
	delete(r.queries, id)
	return e.frames, e.err
}

// Cancel stops a running query. Cancelling the run context makes the Trino
// driver send DELETE /v1/query/{id} on its way out, so the cancellation
// reaches the cluster rather than just abandoning the goroutine.
func (r *Registry) Cancel(id string) error {
	r.mu.Lock()
	e, ok := r.queries[id]
	if !ok {
		err := r.missLocked(id)
		r.mu.Unlock()
		return err
	}
	if !e.status.Running() {
		// Already finished; just drop the buffered result.
		delete(r.queries, id)
		r.mu.Unlock()
		return nil
	}
	e.status = StatusCanceled
	cancel := e.cancel
	r.mu.Unlock()

	cancel()
	return nil
}

// Close stops the reaper and cancels every query still in flight.
func (r *Registry) Close() {
	r.stopOnce.Do(func() { close(r.stop) })

	r.mu.Lock()
	defer r.mu.Unlock()
	for id, e := range r.queries {
		e.cancel()
		delete(r.queries, id)
	}
}

func (r *Registry) finish(id string, frames data.Frames, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.queries[id]
	if !ok {
		// Reaped or cancelled while running; nothing wants the result.
		return
	}
	e.frames, e.err, e.doneAt = frames, err, r.now()
	switch {
	case e.status == StatusCanceled:
		// Keep the cancelled status: the error is just the cancellation.
	case err != nil:
		e.status = StatusFailed
	default:
		e.status = StatusFinished
	}
}

// missLocked explains an unknown handle. A handle minted elsewhere means the
// poll was load balanced to another Grafana instance, which is a different
// problem from a query this process expired, and worth distinguishing because
// the fix is a load balancer setting rather than a timeout.
//
// An unknown handle must never start a new query. Under round-robin routing
// most polls would miss, and every miss would launch another Trino query.
func (r *Registry) missLocked(id string) error {
	if prefix, _, found := strings.Cut(id, ":"); found && prefix != r.processID {
		return ErrOtherInstance
	}
	return ErrNotFound
}

func (r *Registry) inFlightLocked() int {
	n := 0
	for _, e := range r.queries {
		if e.status.Running() {
			n++
		}
	}
	return n
}

func (r *Registry) reapLoop() {
	ticker := time.NewTicker(reaperInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.reap()
		case <-r.stop:
			return
		}
	}
}

// reap cancels queries nobody is polling any more and drops results nobody
// collected. Without it an abandoned dashboard would leave a Trino query
// running and its frames resident until the process exits.
func (r *Registry) reap() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	for id, e := range r.queries {
		switch {
		case e.status.Running() && now.Sub(e.lastPolled) > r.UnpolledTimeout:
			backend.Logger.Info("cancelling asynchronous query nobody is polling", "queryID", id)
			e.cancel()
			delete(r.queries, id)
		case !e.status.Running() && now.Sub(e.doneAt) > r.ResultTTL:
			backend.Logger.Debug("dropping uncollected asynchronous query result", "queryID", id)
			delete(r.queries, id)
		}
	}
}
