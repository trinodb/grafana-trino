package trino

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/sqlds/v4/test"
	"github.com/trinodb/grafana-trino/pkg/trino/async"
)

func newAsyncTestDatasource(t *testing.T, asyncEnabled bool) (*SQLDatasourceWithTrinoUserContext, backend.DataSourceInstanceSettings) {
	t.Helper()

	rows := test.Data{
		Cols: []test.Column{{Name: "n", Kind: int64(0), DataType: "BIGINT"}},
		Rows: [][]any{{int64(1)}, {int64(2)}},
	}
	// The driver name doubles as the registration key in the test package's
	// process-global registry, so it has to be unique per test.
	driver, _ := test.NewDriver(t.Name(), rows, nil, test.DriverOpts{}, nil)

	settings := backend.DataSourceInstanceSettings{
		UID:      "async-test",
		URL:      "http://localhost:8080",
		JSONData: []byte(fmt.Sprintf(`{"enableAsyncQueryData":%t}`, asyncEnabled)),
	}

	ds := NewDatasource(driver)
	if _, err := ds.NewDatasource(context.Background(), settings); err != nil {
		t.Fatalf("creating the datasource: %v", err)
	}
	t.Cleanup(ds.Dispose)

	return ds, settings
}

func asyncRequest(settings backend.DataSourceInstanceSettings, queryID string) *backend.QueryDataRequest {
	query := fmt.Sprintf(`{"rawSQL":"SELECT 1","meta":{"queryFlow":"async"},"queryID":%q}`, queryID)
	return &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{DataSourceInstanceSettings: &settings},
		Queries:       []backend.DataQuery{{RefID: "A", JSON: []byte(query)}},
	}
}

func metaOf(t *testing.T, res backend.DataResponse) async.QueryMeta {
	t.Helper()
	if res.Error != nil {
		t.Fatalf("response carried an error: %v", res.Error)
	}
	if len(res.Frames) == 0 {
		t.Fatal("response carried no frames")
	}
	if res.Frames[0].Meta == nil {
		t.Fatal("first frame has no meta")
	}
	meta, ok := res.Frames[0].Meta.Custom.(async.QueryMeta)
	if !ok {
		t.Fatalf("first frame meta custom is %T, want async.QueryMeta", res.Frames[0].Meta.Custom)
	}
	return meta
}

// TestQueryDataAsyncFlow walks the whole contract the frontend relies on: the
// first request starts the query and only hands back a handle, later requests
// carrying that handle report progress, and the data arrives once the query is
// finished.
func TestQueryDataAsyncFlow(t *testing.T) {
	ds, settings := newAsyncTestDatasource(t, true)
	ctx := context.Background()

	res, err := ds.QueryData(ctx, asyncRequest(settings, ""))
	if err != nil {
		t.Fatalf("starting the query: %v", err)
	}
	started := metaOf(t, res.Responses["A"])
	if started.Status != string(async.StatusSubmitted) {
		t.Errorf("status = %q, want %q", started.Status, async.StatusSubmitted)
	}
	if started.QueryID == "" {
		t.Fatal("the response that starts a query must carry a handle to poll with")
	}
	if len(res.Responses["A"].Frames[0].Fields) != 0 {
		t.Error("the response that starts a query must not carry data")
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		res, err = ds.QueryData(ctx, asyncRequest(settings, started.QueryID))
		if err != nil {
			t.Fatalf("polling the query: %v", err)
		}
		meta := metaOf(t, res.Responses["A"])
		if meta.QueryID != started.QueryID {
			t.Fatalf("poll returned handle %q, want %q", meta.QueryID, started.QueryID)
		}
		if meta.Status == string(async.StatusFinished) {
			break
		}
		if meta.Status != string(async.StatusSubmitted) && meta.Status != string(async.StatusRunning) {
			t.Fatalf("unexpected status %q", meta.Status)
		}
		if time.Now().After(deadline) {
			t.Fatal("query never finished")
		}
		time.Sleep(time.Millisecond)
	}

	frames := res.Responses["A"].Frames
	if len(frames) == 0 || len(frames[0].Fields) == 0 {
		t.Fatalf("finished response carried no data: %+v", frames)
	}
	if got := frames[0].Fields[0].Len(); got != 2 {
		t.Errorf("got %d rows, want the 2 the driver returned", got)
	}

	// The result is handed over once, so a stray repeat poll must not silently
	// start the query again.
	res, err = ds.QueryData(ctx, asyncRequest(settings, started.QueryID))
	if err != nil {
		t.Fatalf("polling a collected query: %v", err)
	}
	if res.Responses["A"].Error == nil {
		t.Error("polling an already collected query should be an error, not a fresh start")
	}
}

func TestQueryDataStaysSynchronousWhenAsyncIsDisabled(t *testing.T) {
	ds, settings := newAsyncTestDatasource(t, false)

	// The query still asks for the async flow; the data source setting is what
	// decides, so the data must come back on the first request.
	res, err := ds.QueryData(context.Background(), asyncRequest(settings, ""))
	if err != nil {
		t.Fatalf("QueryData returned %v", err)
	}
	frames := res.Responses["A"].Frames
	if len(frames) == 0 || len(frames[0].Fields) == 0 {
		t.Fatalf("synchronous response carried no data: %+v", frames)
	}
}

func TestQueryDataStaysSynchronousForAlerts(t *testing.T) {
	ds, settings := newAsyncTestDatasource(t, true)

	req := asyncRequest(settings, "")
	req.Headers = map[string]string{"FromAlert": "true"}

	res, err := ds.QueryData(context.Background(), req)
	if err != nil {
		t.Fatalf("QueryData returned %v", err)
	}
	frames := res.Responses["A"].Frames
	if len(frames) == 0 || len(frames[0].Fields) == 0 {
		t.Fatalf("alerting response carried no data: %+v", frames)
	}
}

func TestQueryDataRejectsAHandleFromAnotherInstance(t *testing.T) {
	ds, settings := newAsyncTestDatasource(t, true)

	res, err := ds.QueryData(context.Background(), asyncRequest(settings, "another-grafana:d3adbeef"))
	if err != nil {
		t.Fatalf("QueryData returned %v", err)
	}
	// Starting a new query here would turn every misrouted poll into another
	// query against the cluster.
	if res.Responses["A"].Error == nil {
		t.Fatal("a handle from another instance must be an error, not a fresh start")
	}
}

func TestCancelResourceStopsTheQuery(t *testing.T) {
	ds, settings := newAsyncTestDatasource(t, true)

	res, err := ds.QueryData(context.Background(), asyncRequest(settings, ""))
	if err != nil {
		t.Fatalf("starting the query: %v", err)
	}
	queryID := metaOf(t, res.Responses["A"]).QueryID

	body, err := json.Marshal(map[string]string{"queryId": queryID})
	if err != nil {
		t.Fatalf("marshalling the cancel body: %v", err)
	}
	rw := httptest.NewRecorder()
	ds.handleCancel(rw, httptest.NewRequest(http.MethodPost, "/cancel", bytes.NewReader(body)))

	if rw.Code != http.StatusOK {
		t.Fatalf("cancel returned %d, want %d", rw.Code, http.StatusOK)
	}
	// The query is either gone, if it had already finished, or marked
	// cancelled. What must not happen is it still being reported as running.
	if status, err := ds.async.Status(queryID); err == nil && status.Running() {
		t.Errorf("the query is still %s after being cancelled", status)
	}
}

type recordingSender struct {
	res *backend.CallResourceResponse
}

func (s *recordingSender) Send(res *backend.CallResourceResponse) error {
	s.res = res
	return nil
}

// TestCancelRouteIsReachable guards the registration order: sqlds builds its
// resource mux inside NewDatasource, so a CustomRoute added afterwards is
// silently never served and cancellation would quietly do nothing.
func TestCancelRouteIsReachable(t *testing.T) {
	ds, settings := newAsyncTestDatasource(t, true)

	res, err := ds.QueryData(context.Background(), asyncRequest(settings, ""))
	if err != nil {
		t.Fatalf("starting the query: %v", err)
	}
	queryID := metaOf(t, res.Responses["A"]).QueryID

	body, err := json.Marshal(map[string]string{"queryId": queryID})
	if err != nil {
		t.Fatalf("marshalling the cancel body: %v", err)
	}

	sender := &recordingSender{}
	// "cancel" without a leading slash is what the frontend's postResource
	// sends; the SDK's HTTP adapter adds the slash before routing.
	callErr := ds.CallResource(context.Background(), &backend.CallResourceRequest{
		PluginContext: backend.PluginContext{DataSourceInstanceSettings: &settings},
		Path:          "cancel",
		Method:        http.MethodPost,
		Body:          body,
	}, sender)
	if callErr != nil {
		t.Fatalf("CallResource returned %v", callErr)
	}
	if sender.res == nil {
		t.Fatal("CallResource sent no response")
	}
	if sender.res.Status != http.StatusOK {
		t.Fatalf("cancel route returned %d, want %d: the route is not registered", sender.res.Status, http.StatusOK)
	}
}

func TestCancelResourceRejectsAnEmptyHandle(t *testing.T) {
	ds, _ := newAsyncTestDatasource(t, true)

	rw := httptest.NewRecorder()
	ds.handleCancel(rw, httptest.NewRequest(http.MethodPost, "/cancel", bytes.NewReader([]byte(`{}`))))

	if rw.Code != http.StatusBadRequest {
		t.Errorf("cancel returned %d, want %d", rw.Code, http.StatusBadRequest)
	}
}
