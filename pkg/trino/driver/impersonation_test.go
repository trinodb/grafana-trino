package driver

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	trinoClient "github.com/trinodb/grafana-trino/pkg/trino/client"
	"github.com/trinodb/grafana-trino/pkg/trino/models"
)

func TestOAuthImpersonationUserYieldsToSessionUser(t *testing.T) {
	var mu sync.Mutex
	var users []string
	trinoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		users = append(users, r.Header.Get("X-Trino-User"))
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(trinoServer.Close)
	tokenServer := newTokenRecorder().server(t, "A")

	u, err := url.Parse(trinoServer.URL)
	if err != nil {
		t.Fatalf("parse trino URL: %v", err)
	}
	u.User = url.User("grafana")
	db, err := Open(models.TrinoDatasourceSettings{
		UID:               "uid-impersonation",
		URL:               u,
		TokenUrl:          tokenServer.URL,
		ClientId:          "client-a",
		ClientSecret:      "secret",
		ImpersonationUser: "service-account",
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	tests := []struct {
		name string
		ctx  context.Context
		args []interface{}
		want string
	}{
		{name: "no session user", ctx: context.Background(), want: "service-account"},
		{
			name: "session user",
			ctx:  trinoClient.WithSessionUser(context.Background()),
			args: []interface{}{sql.Named("X-Trino-User", "alice@example.com")},
			want: "alice@example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mu.Lock()
			users = nil
			mu.Unlock()
			_, _ = db.QueryContext(tt.ctx, "SELECT 1", tt.args...)
			mu.Lock()
			defer mu.Unlock()
			if len(users) == 0 {
				t.Fatal("Trino never received a request")
			}
			for _, got := range users {
				if got != tt.want {
					t.Errorf("Trino saw user %q, want %q", got, tt.want)
				}
			}
		})
	}
}
