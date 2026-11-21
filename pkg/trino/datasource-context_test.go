package trino

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/trinodb/grafana-trino/pkg/trino/models"
)

func TestQueryDataStripsViewerSuffixWithoutChangingCredentials(t *testing.T) {
	requests := make(chan [3]string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, _ := r.BasicAuth()
		requests <- [3]string{r.Header.Get(trinoUserHeader), user, password}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"q1","stats":{"state":"FINISHED"},"columns":[{"name":"n","type":"bigint","typeSignature":{"rawType":"bigint","arguments":[]}}],"data":[[1]]}`))
	}))
	t.Cleanup(server.Close)
	settings := &backend.DataSourceInstanceSettings{
		Name:                    "suffix-test",
		URL:                     server.URL,
		BasicAuthEnabled:        true,
		BasicAuthUser:           "service@example.com",
		DecryptedSecureJSONData: map[string]string{"basicAuthPassword": "test-password"},
		JSONData:                []byte(`{"enableImpersonation":true,"impersonationIdentity":"login","impersonationStripSuffix":"@example.com"}`),
	}
	ds := NewDatasource(New())
	if _, err := ds.NewDatasource(context.Background(), *settings); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ds.Dispose)
	for _, name := range []string{"alice", "bob"} {
		resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				User:                       &backend.User{Login: name + "@example.com"},
				DataSourceInstanceSettings: settings,
			},
			Queries: []backend.DataQuery{{RefID: "A", JSON: []byte(`{"rawSQL":"SELECT 1","format":1}`)}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := resp.Responses["A"].Error; err != nil {
			t.Fatal(err)
		}
		select {
		case got := <-requests:
			want := [3]string{name, "service@example.com", "test-password"}
			if got != want {
				t.Errorf("request headers = %q, want %q", got, want)
			}
		default:
			t.Fatal("Trino received no request")
		}
	}
}

func TestImpersonatedUser(t *testing.T) {
	alice := &backend.User{Login: "alice", Email: "alice@example.com"}
	noEmail := &backend.User{Login: "bob"}

	tests := []struct {
		name     string
		user     *backend.User
		identity string
		suffix   string
		want     string
		wantErr  bool
	}{
		{name: "login", user: alice, identity: models.ImpersonationIdentityLogin, want: "alice"},
		{name: "email", user: alice, identity: models.ImpersonationIdentityEmail, want: "alice@example.com"},
		{name: "login when user has no email", user: noEmail, identity: models.ImpersonationIdentityLogin, want: "bob"},
		{name: "email when user has no email", user: noEmail, identity: models.ImpersonationIdentityEmail, wantErr: true},
		{name: "anonymous user is not impersonated by login", user: &backend.User{}, identity: models.ImpersonationIdentityLogin, want: ""},
		{name: "anonymous user is not impersonated by email", user: &backend.User{}, identity: models.ImpersonationIdentityEmail, want: ""},
		{name: "email for user without login", user: &backend.User{Email: "carol@example.com"}, identity: models.ImpersonationIdentityEmail, want: "carol@example.com"},
		{name: "login for user without login", user: &backend.User{Email: "carol@example.com"}, identity: models.ImpersonationIdentityLogin, wantErr: true},
		{name: "no user", user: nil, identity: models.ImpersonationIdentityLogin, wantErr: true},
		{name: "email login unchanged by default", user: &backend.User{Login: "alice@example.com"}, want: "alice@example.com"},
		{name: "strip login suffix", user: &backend.User{Login: "alice@example.com"}, suffix: "@example.com", want: "alice"},
		{name: "strip email suffix", user: alice, identity: models.ImpersonationIdentityEmail, suffix: "@example.com", want: "alice"},
		{name: "short login unchanged", user: alice, suffix: "@example.com", want: "alice"},
		{name: "other domain unchanged", user: &backend.User{Login: "alice@elsewhere.com"}, suffix: "@example.com", want: "alice@elsewhere.com"},
		{name: "suffix matched literally", user: &backend.User{Login: "alice@exampleXcom"}, suffix: "@example.com", want: "alice@exampleXcom"},
		{name: "suffix must be trailing", user: &backend.User{Login: "alice@example.com.evil"}, suffix: "@example.com", want: "alice@example.com.evil"},
		{name: "suffix is case sensitive", user: &backend.User{Login: "alice@EXAMPLE.COM"}, suffix: "@example.com", want: "alice@EXAMPLE.COM"},
		{name: "only one suffix removed", user: &backend.User{Login: "alice@example.com@example.com"}, suffix: "@example.com", want: "alice@example.com"},
		{name: "empty mapped login is rejected", user: &backend.User{Login: "@example.com"}, suffix: "@example.com", wantErr: true},
		{name: "empty mapped email is rejected", user: &backend.User{Email: "@example.com"}, identity: models.ImpersonationIdentityEmail, suffix: "@example.com", wantErr: true},
		{name: "anonymous user with suffix is not impersonated", user: &backend.User{}, suffix: "@example.com", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := impersonatedUser(tt.user, tt.identity, tt.suffix)
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

func TestQueryDataRejectsEmptyMappedUser(t *testing.T) {
	ds := NewDatasource(New())
	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{
			User: &backend.User{Login: "@example.com"},
			DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
				URL:      "http://localhost:8080",
				JSONData: []byte(`{"enableImpersonation":true,"impersonationStripSuffix":"@example.com"}`),
			},
		},
		Queries: []backend.DataQuery{{RefID: "A"}, {RefID: "B"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, refID := range []string{"A", "B"} {
		response := resp.Responses[refID]
		if response.Error == nil || !strings.Contains(response.Error.Error(), "leaves an empty user") {
			t.Errorf("query %s: expected empty-user error, got %v", refID, response.Error)
		}
		if response.Status != backend.StatusBadRequest {
			t.Errorf("query %s: expected bad request, got %v", refID, response.Status)
		}
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
