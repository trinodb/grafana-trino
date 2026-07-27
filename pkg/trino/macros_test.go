package trino

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
)

func TestMacroTimeFilter(t *testing.T) {
	got, err := macroTimeFilter(testQuery(), []string{"orderdate"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "orderdate BETWEEN TIMESTAMP '2023-01-01 00:00:00' AND TIMESTAMP '2023-01-02 00:00:00'"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMacroTimeFilter_MissingArgs(t *testing.T) {
	if _, err := macroTimeFilter(testQuery(), []string{}); err == nil {
		t.Error("expected error for missing arguments, got nil")
	}
}

func TestMacroDateFilter(t *testing.T) {
	got, err := macroDateFilter(testQuery(), []string{"orderdate"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "orderdate BETWEEN date '2023-01-01' AND date '2023-01-02'"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMacroDateFilter_WrongArgCount(t *testing.T) {
	if _, err := macroDateFilter(testQuery(), []string{"a", "b"}); err == nil {
		t.Error("expected error for wrong argument count, got nil")
	}
}

func TestMacroUnixEpochFilter(t *testing.T) {
	got, err := macroUnixEpochFilter(testQuery(), []string{"orderdate"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "orderdate BETWEEN 1672531200 AND 1672617600"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMacroTimeGroup(t *testing.T) {
	got, err := macroTimeGroup(testQuery(), []string{"orderdate", "'1w'"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "FROM_UNIXTIME(FLOOR(TO_UNIXTIME(orderdate)/604800)*604800)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMacroTimeGroup_MissingArgs(t *testing.T) {
	if _, err := macroTimeGroup(testQuery(), []string{"orderdate"}); err == nil {
		t.Error("expected error for missing interval argument, got nil")
	}
}

func TestMacroParseTime(t *testing.T) {
	got, err := macroParseTime(testQuery(), []string{"orderdate"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "TIMESTAMP orderdate"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMacroTimeFromTo(t *testing.T) {
	from, err := macroTimeFrom(testQuery(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if from != "TIMESTAMP '2023-01-01 00:00:00'" {
		t.Errorf("unexpected timeFrom: %q", from)
	}

	to, err := macroTimeTo(testQuery(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if to != "TIMESTAMP '2023-01-02 00:00:00'" {
		t.Errorf("unexpected timeTo: %q", to)
	}
}

func testQuery() *sqlutil.Query {
	return &sqlutil.Query{
		TimeRange: backend.TimeRange{
			From: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
		},
	}
}
