package trino

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/trinodb/trino-go-client/trino"
)

func TestIsComplexType(t *testing.T) {
	for dbType, want := range map[string]bool{
		"ARRAY(BIGINT)":               true,
		"MAP(VARCHAR, BIGINT)":        true,
		"ROW(A BIGINT)":               true,
		"array(bigint)":               true,
		"VARCHAR":                     false,
		"JSON":                        false,
		"TIMESTAMP(3) WITH TIME ZONE": false,
		"":                            false,
	} {
		if got := isComplexType(dbType); got != want {
			t.Errorf("isComplexType(%q) = %v, want %v", dbType, got, want)
		}
	}
}

func TestComplexToJSON(t *testing.T) {
	tests := []struct {
		name   string
		dbType string
		value  string
		want   string
	}{
		{
			name:   "array of named rows",
			dbType: `ARRAY(ROW(SCORE DOUBLE, WORD_START DOUBLE, WORD VARCHAR))`,
			value:  `[[0.95, 8.64, "Bonsoir"], [0.98, 8.97, "à"]]`,
			want:   `[{"score":0.95,"word_start":8.64,"word":"Bonsoir"},{"score":0.98,"word_start":8.97,"word":"à"}]`,
		},
		{
			name:   "row field order is preserved",
			dbType: `ROW(Z BIGINT, A BIGINT)`,
			value:  `[1, 2]`,
			want:   `{"z":1,"a":2}`,
		},
		{
			name:   "quoted field names and parameterized types",
			dbType: `ROW("WORD START" DECIMAL(10, 2), "A,""B" VARCHAR(3), TS TIMESTAMP(3) WITH TIME ZONE)`,
			value:  `["8.60", "x", "2024-01-01 00:00:00.123 UTC"]`,
			want:   `{"word start":"8.60","a,\"b":"x","ts":"2024-01-01 00:00:00.123 UTC"}`,
		},
		{
			name:   "anonymous row fields stay positional",
			dbType: `ROW(INTEGER, INTERVAL DAY TO SECOND)`,
			value:  `[1, "2 00:00:00.000"]`,
			want:   `[1,"2 00:00:00.000"]`,
		},
		{
			name:   "field names colliding after lower-casing stay positional",
			dbType: `ROW("A" BIGINT, "a" BIGINT)`,
			value:  `[1, 2]`,
			want:   `[1,2]`,
		},
		{
			name:   "map of rows",
			dbType: `MAP(VARCHAR(1), ROW(ID INTEGER, AT TIMESTAMP WITH TIME ZONE))`,
			value:  `{"k": [1, "2024-01-01 00:00:00.123 UTC"], "a": null}`,
			want:   `{"a":null,"k":{"id":1,"at":"2024-01-01 00:00:00.123 UTC"}}`,
		},
		{
			name:   "nested arrays and maps inside rows",
			dbType: `ROW(TAGS ARRAY(VARCHAR), ATTRS MAP(VARCHAR, ARRAY(ROW(K VARCHAR))), ROW ROW(ARRAY BIGINT))`,
			value:  `[["a", null], {"x": [["v"]]}, [7]]`,
			want:   `{"tags":["a",null],"attrs":{"x":[{"k":"v"}]},"row":{"array":7}}`,
		},
		{
			name:   "large integers keep their precision",
			dbType: `ARRAY(BIGINT)`,
			value:  `[9223372036854775807]`,
			want:   `[9223372036854775807]`,
		},
		{
			name:   "unparseable type falls back to raw structure",
			dbType: `ROW(A BIGINT`,
			value:  `[1]`,
			want:   `[1]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ, _ := parseTrinoType(tt.dbType)
			var buf bytes.Buffer
			if err := writeJSON(&buf, decodeJSON(t, tt.value), typ); err != nil {
				t.Fatalf("writeJSON: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestParseTrinoTypeErrors(t *testing.T) {
	for _, dbType := range []string{"", "ROW(", "MAP(VARCHAR)", `ROW("A BIGINT)`, "ARRAY(BIGINT))"} {
		if _, err := parseTrinoType(dbType); err == nil {
			t.Errorf("parseTrinoType(%q): expected error", dbType)
		}
	}
}

func TestComplexTypesFromTrinoResponse(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"q1","nextUri":"` + server.URL + `/v1/statement/q1/1","stats":{"state":"QUEUED"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"q1","stats":{"state":"FINISHED"},` + complexResponseBody + `}`))
	}))
	t.Cleanup(server.Close)

	dsn, err := (&trino.Config{ServerURI: "http://grafana@" + server.Listener.Addr().String()}).FormatDSN()
	if err != nil {
		t.Fatalf("format DSN: %v", err)
	}
	db, err := sql.Open("trino", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	frame, err := sqlutil.FrameFromRows(rows, -1, New().Converters()...)
	if err != nil {
		t.Fatalf("FrameFromRows: %v", err)
	}

	if len(frame.Fields) != 3 || frame.Rows() != 1 {
		t.Fatalf("got %d fields and %d rows, want 3 fields and 1 row", len(frame.Fields), frame.Rows())
	}
	want := map[string]*string{
		"a": ptr(`[{"score":0.95,"word start":"8.60","word":"Bonsoir"}]`),
		"m": ptr(`{"k":[1,"2024-01-01 00:00:00.123 UTC"]}`),
		"n": nil,
	}
	for _, field := range frame.Fields {
		if field.Type() != data.FieldTypeNullableJSON {
			t.Errorf("field %s: type %s, want %s", field.Name, field.Type(), data.FieldTypeNullableJSON)
			continue
		}
		got := field.At(0).(*json.RawMessage)
		switch w := want[field.Name]; {
		case w == nil && got != nil:
			t.Errorf("field %s: got %s, want null", field.Name, *got)
		case w != nil && (got == nil || string(*got) != *w):
			t.Errorf("field %s: got %v, want %s", field.Name, got, *w)
		}
	}
}

const complexResponseBody = `"columns":[{"name":"a","type":"array(row(score double, \"Word Start\" decimal(10, 2), word varchar))","typeSignature":{"rawType":"array","arguments":[{"kind":"TYPE","value":{"rawType":"row","arguments":[{"kind":"NAMED_TYPE","value":{"fieldName":{"name":"score"},"typeSignature":{"rawType":"double","arguments":[]}}},{"kind":"NAMED_TYPE","value":{"fieldName":{"name":"Word Start"},"typeSignature":{"rawType":"decimal","arguments":[{"kind":"LONG","value":10},{"kind":"LONG","value":2}]}}},{"kind":"NAMED_TYPE","value":{"fieldName":{"name":"word"},"typeSignature":{"rawType":"varchar","arguments":[{"kind":"LONG","value":2147483647}]}}}]}}]}},{"name":"m","type":"map(varchar(1), row(integer, timestamp with time zone))","typeSignature":{"rawType":"map","arguments":[{"kind":"TYPE","value":{"rawType":"varchar","arguments":[{"kind":"LONG","value":1}]}},{"kind":"TYPE","value":{"rawType":"row","arguments":[{"kind":"NAMED_TYPE","value":{"typeSignature":{"rawType":"integer","arguments":[]}}},{"kind":"NAMED_TYPE","value":{"typeSignature":{"rawType":"timestamp with time zone","arguments":[]}}}]}}]}},{"name":"n","type":"array(bigint)","typeSignature":{"rawType":"array","arguments":[{"kind":"TYPE","value":{"rawType":"bigint","arguments":[]}}]}}],"data":[[[[0.95,"8.60","Bonsoir"]],{"k":[1,"2024-01-01 00:00:00.123 UTC"]},null]]`

func decodeJSON(t *testing.T, s string) interface{} {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader([]byte(s)))
	d.UseNumber()
	var v interface{}
	if err := d.Decode(&v); err != nil {
		t.Fatalf("decode %s: %v", s, err)
	}
	return v
}

func ptr(s string) *string { return &s }

func TestEnableJSONCellInspect(t *testing.T) {
	jsonField := data.NewField("j", nil, []*json.RawMessage{nil})
	optedOut := data.NewField("o", nil, []*json.RawMessage{nil}).SetConfig(&data.FieldConfig{Custom: map[string]interface{}{"inspect": false}})
	scalar := data.NewField("s", nil, []*string{nil})

	enableJSONCellInspect(data.Frames{data.NewFrame("", jsonField, optedOut, scalar)})

	if jsonField.Config == nil || jsonField.Config.Custom["inspect"] != true {
		t.Errorf("JSON field: expected inspect enabled, got %+v", jsonField.Config)
	}
	if optedOut.Config.Custom["inspect"] != false {
		t.Errorf("JSON field with explicit inspect setting was overridden: %+v", optedOut.Config)
	}
	if scalar.Config != nil {
		t.Errorf("non-JSON field should be untouched, got %+v", scalar.Config)
	}
}
