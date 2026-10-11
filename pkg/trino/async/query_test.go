package async

import (
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func asyncQuery(refID string) backend.DataQuery {
	return backend.DataQuery{RefID: refID, JSON: []byte(`{"rawSQL":"SELECT 1","meta":{"queryFlow":"async"}}`)}
}

func syncQuery(refID string) backend.DataQuery {
	return backend.DataQuery{RefID: refID, JSON: []byte(`{"rawSQL":"SELECT 1"}`)}
}

func TestIsAsyncRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *backend.QueryDataRequest
		want bool
	}{
		{
			name: "query marked async",
			req:  &backend.QueryDataRequest{Queries: []backend.DataQuery{asyncQuery("A")}},
			want: true,
		},
		{
			name: "query without the marker",
			req:  &backend.QueryDataRequest{Queries: []backend.DataQuery{syncQuery("A")}},
			want: false,
		},
		{
			name: "one query in the request is synchronous",
			req:  &backend.QueryDataRequest{Queries: []backend.DataQuery{asyncQuery("A"), syncQuery("B")}},
			want: false,
		},
		{
			// There is no browser to run the polling loop for an alert.
			name: "alerting request",
			req: &backend.QueryDataRequest{
				Headers: map[string]string{fromAlertHeader: "true"},
				Queries: []backend.DataQuery{asyncQuery("A")},
			},
			want: false,
		},
		{
			name: "expression request",
			req: &backend.QueryDataRequest{
				Headers: map[string]string{fromExpressionHeader: "true"},
				Queries: []backend.DataQuery{asyncQuery("A")},
			},
			want: false,
		},
		{
			name: "unparseable query",
			req: &backend.QueryDataRequest{
				Queries: []backend.DataQuery{{RefID: "A", JSON: []byte(`not json`)}},
			},
			want: false,
		},
		{
			name: "no queries",
			req:  &backend.QueryDataRequest{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAsyncRequest(tt.req); got != tt.want {
				t.Errorf("IsAsyncRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseQueryReadsTheHandle(t *testing.T) {
	query := backend.DataQuery{RefID: "A", JSON: []byte(`{"rawSQL":"SELECT 1","queryID":"proc:abc"}`)}
	q, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery returned %v", err)
	}
	if q.QueryID != "proc:abc" {
		t.Errorf("QueryID = %q, want %q", q.QueryID, "proc:abc")
	}
}

func TestStatusFramesCarryTheHandleWithoutData(t *testing.T) {
	frames := StatusFrames("A", "proc:abc", StatusRunning)
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
	// The frontend only keeps frames that have fields, so a status response
	// must not add an empty series to the panel.
	if len(frames[0].Fields) != 0 {
		t.Errorf("status frame has %d fields, want none", len(frames[0].Fields))
	}
	meta, ok := frames[0].Meta.Custom.(QueryMeta)
	if !ok {
		t.Fatalf("frame meta custom is %T, want QueryMeta", frames[0].Meta.Custom)
	}
	if meta.QueryID != "proc:abc" || meta.Status != string(StatusRunning) {
		t.Errorf("meta = %+v, want the handle and a running status", meta)
	}
}

func TestWithMetaTagsTheFirstFrame(t *testing.T) {
	frames := data.Frames{
		data.NewFrame("A", data.NewField("n", nil, []int64{1})),
		data.NewFrame("B"),
	}
	got := WithMeta(frames, "proc:abc", StatusFinished)

	meta, ok := got[0].Meta.Custom.(QueryMeta)
	if !ok {
		t.Fatalf("first frame meta custom is %T, want QueryMeta", got[0].Meta.Custom)
	}
	if meta.Status != string(StatusFinished) {
		t.Errorf("status = %q, want %q", meta.Status, StatusFinished)
	}
	if len(got[0].Fields) != 1 {
		t.Error("WithMeta dropped the data from the frame")
	}
}

func TestWithMetaHandlesAnEmptyResult(t *testing.T) {
	// A query returning no rows still has to carry the handle, otherwise the
	// frontend cannot tell the response apart from one it should keep polling.
	got := WithMeta(nil, "proc:abc", StatusFinished)
	if len(got) != 1 {
		t.Fatalf("got %d frames, want 1", len(got))
	}
	if _, ok := got[0].Meta.Custom.(QueryMeta); !ok {
		t.Errorf("frame meta custom is %T, want QueryMeta", got[0].Meta.Custom)
	}
}
