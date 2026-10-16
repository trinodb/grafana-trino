package driver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/trinodb/grafana-trino/pkg/trino/models"
)

// TestOpenExplainsRejectedPassword checks that a 401 names the credential the
// data source sent, next to the reason Trino gives, through the error
// trino-go-client reports for a query and for the health check's ping.
func TestOpenExplainsRejectedPassword(t *testing.T) {
	trinoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Trino"`)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Password not allowed for insecure authentication\n"))
	}))
	t.Cleanup(trinoServer.Close)

	u, err := url.Parse(trinoServer.URL)
	if err != nil {
		t.Fatalf("parse trino URL: %v", err)
	}
	u.User = url.User("alice")
	db, err := Open(models.TrinoDatasourceSettings{
		URL:  u,
		Opts: httpclient.Options{BasicAuth: &httpclient.BasicAuthOptions{User: "alice", Password: "s3cret"}},
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	want := `trino: query failed (401 Unauthorized): "Password not allowed for insecure authentication. Trino rejected the basic auth password of the data source: correct it, or remove it if Trino does not authenticate with passwords"`
	_, err = db.QueryContext(context.Background(), "SELECT 1")
	if err == nil || err.Error() != want {
		t.Errorf("query error\n got: %v\nwant: %s", err, want)
	}
	err = db.PingContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("ping error\n got: %v\nwant: %s", err, want)
	}
}

func TestExplainUnauthorized(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		reason        string
		want          string
	}{
		{
			name:          "password with Trino's reason",
			authorization: "Basic YWxpY2U6czNjcmV0",
			reason:        "Password not allowed for insecure authentication\n",
			want:          "Password not allowed for insecure authentication. Trino rejected the basic auth password of the data source: correct it, or remove it if Trino does not authenticate with passwords",
		},
		{
			name:          "password without a reason",
			authorization: "Basic YWxpY2U6czNjcmV0",
			want:          "Trino rejected the basic auth password of the data source: correct it, or remove it if Trino does not authenticate with passwords",
		},
		{
			name:          "bearer token",
			authorization: "Bearer eyJ",
			reason:        "Unauthorized.",
			want:          "Unauthorized. Trino rejected the bearer token: the access token of the data source, the token from its OAuth client credentials, or the forwarded OAuth identity",
		},
		{
			name:          "Kerberos",
			authorization: "Negotiate YIIB",
			want:          "Trino rejected the Kerberos credentials of the data source",
		},
		{
			name:   "no credentials",
			reason: "Basic authentication or X-Trino-Original-User or X-Trino-User must be sent",
			want:   "Basic authentication or X-Trino-Original-User or X-Trino-User must be sent. Trino requires authentication, but the data source sent no credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := explainUnauthorized(tt.authorization, tt.reason); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
