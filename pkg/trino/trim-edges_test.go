package trino

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func TestTrimEdges(t *testing.T) {
	for _, tc := range []struct {
		name      string
		trimEdges int
		frame     *data.Frame
		want      []string
	}{
		{
			name:  "not set",
			frame: timeSeriesFrame(1, 2, 3, 4, 5),
			want:  []string{"1:1", "2:2", "3:3", "4:4", "5:5"},
		},
		{
			name:      "one point from each end",
			trimEdges: 1,
			frame:     timeSeriesFrame(1, 2, 3, 4, 5),
			want:      []string{"2:2", "3:3", "4:4"},
		},
		{
			name:      "rows are ordered by time before trimming",
			trimEdges: 1,
			frame:     timeSeriesFrame(5, 3, 1, 4, 2),
			want:      []string{"2:2", "3:3", "4:4"},
		},
		{
			name:      "exactly twice as many rows as trimmed points",
			trimEdges: 2,
			frame:     timeSeriesFrame(1, 2, 3, 4),
			want:      []string{},
		},
		{
			name:      "more trimmed points than rows",
			trimEdges: 10,
			frame:     timeSeriesFrame(1, 2, 3),
			want:      []string{},
		},
		{
			name:      "negative value is ignored",
			trimEdges: -1,
			frame:     timeSeriesFrame(1, 2, 3),
			want:      []string{"1:1", "2:2", "3:3"},
		},
		{
			name:      "rows without a time are kept and not counted",
			trimEdges: 1,
			frame:     nullableTimeSeriesFrame(new(int64(3)), nil, new(int64(1)), new(int64(2))),
			want:      []string{"null:null", "2:2"},
		},
		{
			name:      "frame without a time field",
			trimEdges: 1,
			frame:     data.NewFrame("A", data.NewField("value", nil, []int64{1, 2, 3})),
			want:      []string{"1", "2", "3"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := responseWithFrames("A", tc.frame)

			trimResponseEdges([]backend.DataQuery{queryWithTrimEdges("A", tc.trimEdges)}, response)

			if got := frameRows(response.Responses["A"].Frames[0]); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got rows %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTrimEdgesOfEveryFrameOfAQuery(t *testing.T) {
	response := responseWithFrames("A", timeSeriesFrame(1, 2, 3), timeSeriesFrame(4, 5, 6, 7))

	trimResponseEdges([]backend.DataQuery{queryWithTrimEdges("A", 1)}, response)

	frames := response.Responses["A"].Frames
	if got, want := frameRows(frames[0]), []string{"2:2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("first frame: got rows %v, want %v", got, want)
	}
	if got, want := frameRows(frames[1]), []string{"5:5", "6:6"}; !reflect.DeepEqual(got, want) {
		t.Errorf("second frame: got rows %v, want %v", got, want)
	}
}

func TestTrimEdgesOnlyAppliesToTheQueryItIsSetOn(t *testing.T) {
	response := responseWithFrames("A", timeSeriesFrame(1, 2, 3))
	response.Responses["B"] = backend.DataResponse{Frames: data.Frames{timeSeriesFrame(1, 2, 3)}}

	trimResponseEdges([]backend.DataQuery{queryWithTrimEdges("A", 1), queryWithTrimEdges("B", 0)}, response)

	if got, want := frameRows(response.Responses["A"].Frames[0]), []string{"2:2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("query A: got rows %v, want %v", got, want)
	}
	if got, want := frameRows(response.Responses["B"].Frames[0]), []string{"1:1", "2:2", "3:3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("query B: got rows %v, want %v", got, want)
	}
}

func TestTrimEdgesKeepsFrameMetadataAndFieldConfig(t *testing.T) {
	frame := timeSeriesFrame(1, 2, 3)
	frame.Meta = &data.FrameMeta{ExecutedQueryString: "SELECT 1"}
	frame.Fields[1].Config = &data.FieldConfig{Unit: "bytes"}
	response := responseWithFrames("A", frame)

	trimResponseEdges([]backend.DataQuery{queryWithTrimEdges("A", 1)}, response)

	trimmed := response.Responses["A"].Frames[0]
	if trimmed.Name != "A" {
		t.Errorf("got frame name %q, want %q", trimmed.Name, "A")
	}
	if trimmed.Meta == nil || trimmed.Meta.ExecutedQueryString != "SELECT 1" {
		t.Errorf("got frame meta %+v, want the executed query to be kept", trimmed.Meta)
	}
	if trimmed.Fields[1].Config == nil || trimmed.Fields[1].Config.Unit != "bytes" {
		t.Errorf("got field config %+v, want the unit to be kept", trimmed.Fields[1].Config)
	}
}

func TestTrimEdgesLeavesFailedQueriesAlone(t *testing.T) {
	response := responseWithFrames("A", timeSeriesFrame(1, 2, 3))
	failed := response.Responses["A"]
	failed.Error = errors.New("query failed")
	response.Responses["A"] = failed

	trimResponseEdges([]backend.DataQuery{queryWithTrimEdges("A", 1)}, response)

	if got, want := frameRows(response.Responses["A"].Frames[0]), []string{"1:1", "2:2", "3:3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got rows %v, want %v", got, want)
	}
}

func TestTrimEdgesIgnoresUnparseableQuery(t *testing.T) {
	response := responseWithFrames("A", timeSeriesFrame(1, 2, 3))

	trimResponseEdges([]backend.DataQuery{{RefID: "A", JSON: json.RawMessage(`{"trimEdges": "1"}`)}}, response)

	if got, want := frameRows(response.Responses["A"].Frames[0]), []string{"1:1", "2:2", "3:3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got rows %v, want %v", got, want)
	}
}

func queryWithTrimEdges(refID string, trimEdges int) backend.DataQuery {
	raw, err := json.Marshal(map[string]int{"trimEdges": trimEdges})
	if err != nil {
		panic(err)
	}
	return backend.DataQuery{RefID: refID, JSON: raw}
}

func responseWithFrames(refID string, frames ...*data.Frame) *backend.QueryDataResponse {
	response := backend.NewQueryDataResponse()
	response.Responses[refID] = backend.DataResponse{Frames: frames}
	return response
}

// timeSeriesFrame returns a frame with one row per given hour, valued with
// that hour, so rows read the same before and after trimming.
func timeSeriesFrame(hours ...int64) *data.Frame {
	times := make([]time.Time, len(hours))
	for i, hour := range hours {
		times[i] = hourTime(hour)
	}
	return data.NewFrame("A", data.NewField("time", nil, times), data.NewField("value", nil, hours))
}

func nullableTimeSeriesFrame(hours ...*int64) *data.Frame {
	times := make([]*time.Time, len(hours))
	for i, hour := range hours {
		if hour != nil {
			times[i] = new(hourTime(*hour))
		}
	}
	return data.NewFrame("A", data.NewField("time", nil, times), data.NewField("value", nil, hours))
}

func hourTime(hour int64) time.Time {
	return time.Date(2024, 1, 1, int(hour), 0, 0, 0, time.UTC)
}

// frameRows renders each row as its values joined with colons, with times
// shown as the hour of the day.
func frameRows(frame *data.Frame) []string {
	rows := []string{}
	for row := range frame.Rows() {
		var rendered string
		for i := range frame.Fields {
			if i > 0 {
				rendered += ":"
			}
			value, ok := frame.ConcreteAt(i, row)
			switch {
			case !ok:
				rendered += "null"
			case frame.Fields[i].Type().Time():
				rendered += fmt.Sprint(value.(time.Time).Hour())
			default:
				rendered += fmt.Sprint(value)
			}
		}
		rows = append(rows, rendered)
	}
	return rows
}
