package trino

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// trimResponseEdges drops the first and last trimEdges rows of every frame
// with a time field, because the oldest and newest time buckets usually only
// cover part of their interval and would show up as artificial drops.
//
// This runs on the whole response rather than in sqlds's MutateResponse,
// because sqlds calls MutateResponse with the request's context instead of
// the one MutateQuery returns, and without the query, so it can't tell which
// query's setting applies to the frames it gets.
func trimResponseEdges(queries []backend.DataQuery, response *backend.QueryDataResponse) {
	for _, query := range queries {
		edges := trimEdgesOf(query)
		if edges <= 0 {
			continue
		}
		queryResponse, ok := response.Responses[query.RefID]
		if !ok || queryResponse.Error != nil {
			continue
		}
		for i, frame := range queryResponse.Frames {
			queryResponse.Frames[i] = trimFrameEdges(frame, edges)
		}
	}
}

func trimEdgesOf(query backend.DataQuery) int {
	var settings struct {
		TrimEdges int `json:"trimEdges"`
	}
	if err := json.Unmarshal(query.JSON, &settings); err != nil {
		return 0
	}
	return settings.TrimEdges
}

func trimFrameEdges(frame *data.Frame, edges int) *data.Frame {
	timeIndices := frame.TypeIndices(data.FieldTypeTime, data.FieldTypeNullableTime)
	if len(timeIndices) == 0 {
		return frame
	}
	timeField := frame.Fields[timeIndices[0]]

	// Rows without a time are on neither edge of the series, so they are
	// kept, ahead of the rest, and don't count towards the trimmed rows.
	var untimedRows, timedRows []int
	for row := range timeField.Len() {
		if _, ok := timeField.ConcreteAt(row); !ok {
			untimedRows = append(untimedRows, row)
			continue
		}
		timedRows = append(timedRows, row)
	}
	// Only frames converted from the long format are guaranteed to be ordered
	// by time; wide frames keep the order the query returned the rows in.
	slices.SortStableFunc(timedRows, func(a, b int) int {
		return timeAt(timeField, a).Compare(timeAt(timeField, b))
	})

	if len(timedRows) <= 2*edges {
		return frameWithRows(frame, untimedRows)
	}
	return frameWithRows(frame, append(untimedRows, timedRows[edges:len(timedRows)-edges]...))
}

func timeAt(field *data.Field, row int) time.Time {
	value, _ := field.ConcreteAt(row)
	return value.(time.Time)
}

func frameWithRows(frame *data.Frame, rows []int) *data.Frame {
	trimmed := frame.EmptyCopy()
	// EmptyCopy leaves out the frame metadata, like the executed query, and
	// the field configs, like the JSON cell inspection set in MutateResponse
	trimmed.Meta = frame.Meta
	for i, field := range frame.Fields {
		trimmed.Fields[i].Config = field.Config
	}
	for _, row := range rows {
		trimmed.AppendRow(frame.RowCopy(row)...)
	}
	return trimmed
}
