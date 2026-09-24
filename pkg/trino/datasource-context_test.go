package trino

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/trinodb/grafana-trino/pkg/trino/models"
)

func TestImpersonatedUser(t *testing.T) {
	alice := &backend.User{Login: "alice", Email: "alice@example.com"}
	noEmail := &backend.User{Login: "bob"}

	tests := []struct {
		name     string
		user     *backend.User
		identity string
		want     string
		wantErr  bool
	}{
		{name: "login", user: alice, identity: models.ImpersonationIdentityLogin, want: "alice"},
		{name: "email", user: alice, identity: models.ImpersonationIdentityEmail, want: "alice@example.com"},
		{name: "login when user has no email", user: noEmail, identity: models.ImpersonationIdentityLogin, want: "bob"},
		{name: "email when user has no email", user: noEmail, identity: models.ImpersonationIdentityEmail, wantErr: true},
		{name: "anonymous user is not impersonated by login", user: &backend.User{}, identity: models.ImpersonationIdentityLogin, want: ""},
		{name: "anonymous user is not impersonated by email", user: &backend.User{}, identity: models.ImpersonationIdentityEmail, want: ""},
		{name: "no user", user: nil, identity: models.ImpersonationIdentityLogin, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := impersonatedUser(tt.user, tt.identity)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetQueryArgsSendsImpersonatedUser(t *testing.T) {
	ctx := context.WithValue(context.Background(), trinoUserHeader, "alice@example.com")
	args := New().SetQueryArgs(ctx, nil)
	if len(args) != 1 {
		t.Fatalf("got %d args, want 1", len(args))
	}
	arg, ok := args[0].(sql.NamedArg)
	if !ok || arg.Name != trinoUserHeader || arg.Value != "alice@example.com" {
		t.Errorf("got %#v, want %s=alice@example.com", args[0], trinoUserHeader)
	}
}

func TestQueryDataReportsImpersonationErrorPerQuery(t *testing.T) {
	ds := NewDatasource(New())
	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{
			User: &backend.User{Login: "bob"},
			DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
				URL:      "http://localhost:8080",
				JSONData: []byte(`{"enableImpersonation":true,"impersonationIdentity":"email"}`),
			},
		},
		Queries: []backend.DataQuery{{RefID: "A"}, {RefID: "B"}},
	})
	if err != nil {
		t.Fatalf("expected per-query errors, got request error: %v", err)
	}
	for _, refID := range []string{"A", "B"} {
		r := resp.Responses[refID]
		if r.Error == nil || !strings.Contains(r.Error.Error(), `"bob" has no email`) {
			t.Errorf("query %s: got error %v", refID, r.Error)
		}
		if r.Status != backend.StatusBadRequest {
			t.Errorf("query %s: got status %v, want %v", refID, r.Status, backend.StatusBadRequest)
		}
	}
}
