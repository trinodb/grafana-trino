package async

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

const (
	// queryFlowAsync is the marker the frontend adds to a query when it is
	// driving the polling loop.
	queryFlowAsync = "async"

	// Alerting and expression requests have no browser to poll for them, so
	// they always take the synchronous path. Grafana identifies them with
	// these headers.
	fromAlertHeader      = "FromAlert"
	fromExpressionHeader = "http_X-Grafana-From-Expr"
)

// Query is the subset of the query model the async flow cares about. Every
// other field is left to sqlds, which parses the same JSON again on the
// synchronous path.
type Query struct {
	// QueryID is empty on the request that starts a query and set on every
	// poll after that.
	QueryID string `json:"queryID,omitempty"`
	Meta    struct {
		QueryFlow string `json:"queryFlow,omitempty"`
	} `json:"meta,omitempty"`
}

// ParseQuery reads the async fields out of a data query.
func ParseQuery(query backend.DataQuery) (Query, error) {
	q := Query{}
	if err := json.Unmarshal(query.JSON, &q); err != nil {
		return q, fmt.Errorf("could not read query %s: %w", query.RefID, err)
	}
	return q, nil
}

// IsAsyncRequest reports whether a request should take the async path. Every
// query in the request has to opt in: a request mixing flows is run
// synchronously, because a single response cannot be half polled.
func IsAsyncRequest(req *backend.QueryDataRequest) bool {
	if _, ok := req.Headers[fromAlertHeader]; ok {
		return false
	}
	if _, ok := req.Headers[fromExpressionHeader]; ok {
		return false
	}
	if len(req.Queries) == 0 {
		return false
	}
	for _, query := range req.Queries {
		q, err := ParseQuery(query)
		if err != nil || q.Meta.QueryFlow != queryFlowAsync {
			return false
		}
	}
	return true
}

// QueryMeta is what the frontend reads off the first frame to decide whether
// to poll again. The field names are part of that contract.
type QueryMeta struct {
	QueryID string `json:"queryID"`
	Status  string `json:"status"`
}

// StatusFrames is the response for a query that has not finished yet: a single
// frame carrying no data, only the handle to poll with and the status. The
// frontend discards frames without fields, so this adds nothing to the panel.
func StatusFrames(refID, queryID string, status Status) data.Frames {
	frame := data.NewFrame(refID)
	frame.Meta = &data.FrameMeta{Custom: QueryMeta{QueryID: queryID, Status: string(status)}}
	return data.Frames{frame}
}

// WithMeta tags a finished result so the frontend can match it to the query it
// has been polling. Only the first frame is read, but it must exist.
func WithMeta(frames data.Frames, queryID string, status Status) data.Frames {
	if len(frames) == 0 {
		frames = data.Frames{data.NewFrame("")}
	}
	if frames[0].Meta == nil {
		frames[0].Meta = &data.FrameMeta{}
	}
	frames[0].Meta.Custom = QueryMeta{QueryID: queryID, Status: string(status)}
	return frames
}
