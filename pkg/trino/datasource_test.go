package trino

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestClientTags(t *testing.T) {
	for _, tc := range []struct {
		name           string
		dataSourceTags string
		queryTags      string
		want           string
	}{
		{name: "no tags at all"},
		{name: "data source tags only", dataSourceTags: "dsTag", want: "dsTag"},
		{name: "query tags only", queryTags: "queryTag", want: "queryTag"},
		{name: "query tags are added to data source tags", dataSourceTags: "dsTag", queryTags: "queryTag", want: "dsTag,queryTag"},
		{name: "tags set on both sides are sent once", dataSourceTags: "shared,dsTag", queryTags: "queryTag,shared", want: "shared,dsTag,queryTag"},
		{name: "blanks and surrounding spaces are ignored", dataSourceTags: " dsTag , ", queryTags: ",, queryTag ,", want: "dsTag,queryTag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := contextWithDataSourceClientTags(tc.dataSourceTags)
			ctx, _ = New().MutateQuery(ctx, queryWithClientTags(tc.queryTags))

			if got := clientTagsQueryArg(t, ctx); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClientTagsDontLeakBetweenQueriesOfOnePanel(t *testing.T) {
	ds := New()
	requestCtx := contextWithDataSourceClientTags("dsTag")

	firstCtx, _ := ds.MutateQuery(requestCtx, queryWithClientTags("firstTag"))
	secondCtx, _ := ds.MutateQuery(requestCtx, queryWithClientTags("secondTag"))

	if got, want := clientTagsQueryArg(t, firstCtx), "dsTag,firstTag"; got != want {
		t.Errorf("first query: got %q, want %q", got, want)
	}
	if got, want := clientTagsQueryArg(t, secondCtx), "dsTag,secondTag"; got != want {
		t.Errorf("second query: got %q, want %q", got, want)
	}
}

func TestMutateQueryIgnoresUnparseableQuery(t *testing.T) {
	ctx := contextWithDataSourceClientTags("dsTag")
	ctx, _ = New().MutateQuery(ctx, backend.DataQuery{JSON: json.RawMessage("not json")})

	if got, want := clientTagsQueryArg(t, ctx), "dsTag"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func contextWithDataSourceClientTags(tags string) context.Context {
	ctx := context.Background()
	if tags == "" {
		return ctx
	}
	return context.WithValue(ctx, trinoClientTagsKey, tags)
}

func queryWithClientTags(tags string) backend.DataQuery {
	raw, err := json.Marshal(map[string]string{"clientTags": tags})
	if err != nil {
		panic(err)
	}
	return backend.DataQuery{JSON: raw}
}

// clientTagsQueryArg returns the value the client tags header would be sent
// with for a query run in the given context, or an empty string when the
// header wouldn't be sent at all.
func clientTagsQueryArg(t *testing.T, ctx context.Context) string {
	t.Helper()
	for _, arg := range New().SetQueryArgs(ctx, nil) {
		named, ok := arg.(sql.NamedArg)
		if !ok {
			t.Fatalf("query arg %v is not a named arg", arg)
		}
		if named.Name == trinoClientTagsKey {
			return named.Value.(string)
		}
	}
	return ""
}
