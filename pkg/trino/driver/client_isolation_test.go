package driver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/trinodb/grafana-trino/pkg/trino/models"
)

// TestOpenIsolatesClientsBetweenDatasources guards against every data source
// sharing one entry in trino-go-client's process-global custom client registry.
// When that happened, a query on one data source authenticated with another
// data source's OAuth client, and picked up its TLS settings and dialer too.
//
// Each data source gets its own token endpoint here, so the endpoint that
// receives the token request identifies which client was actually used.
func TestOpenIsolatesClientsBetweenDatasources(t *testing.T) {
	recorder := newTokenRecorder()

	tokenA := recorder.server(t, "A")
	tokenB := recorder.server(t, "B")
	trinoServer := unreachableTrino(t)

	settings := func(uid, tokenURL, clientID string) models.TrinoDatasourceSettings {
		u, err := url.Parse(trinoServer.URL)
		if err != nil {
			t.Fatalf("parse trino URL: %v", err)
		}
		u.User = url.User("grafana")
		return models.TrinoDatasourceSettings{
			UID:          uid,
			URL:          u,
			TokenUrl:     tokenURL,
			ClientId:     clientID,
			ClientSecret: "secret-" + clientID,
		}
	}

	dbA, err := Open(settings("uid-a", tokenA.URL, "client-a"))
	if err != nil {
		t.Fatalf("open data source A: %v", err)
	}
	defer dbA.Close()

	// Opening B second is what used to overwrite A's registry entry.
	dbB, err := Open(settings("uid-b", tokenB.URL, "client-b"))
	if err != nil {
		t.Fatalf("open data source B: %v", err)
	}
	defer dbB.Close()

	// The queries fail (the Trino stub always errors), but only after the
	// client has been resolved and has fetched its token, which is what's
	// being asserted.
	_, _ = dbA.QueryContext(context.Background(), "SELECT 1")
	recorder.assertOnly(t, "A", "client-a")

	_, _ = dbB.QueryContext(context.Background(), "SELECT 1")
	recorder.assertOnly(t, "B", "client-b")
}

// TestOpenIsolatesTransportBetweenDatasources covers the other half of what the
// shared registry entry leaked: the transport, and with it the TLS config and
// the dialer. A data source that verifies certificates must not inherit another
// data source's skip-verify.
func TestOpenIsolatesTransportBetweenDatasources(t *testing.T) {
	trinoServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(trinoServer.Close)

	settings := func(uid string, skipVerify bool) models.TrinoDatasourceSettings {
		u, err := url.Parse(trinoServer.URL)
		if err != nil {
			t.Fatalf("parse trino URL: %v", err)
		}
		u.User = url.User("grafana")
		return models.TrinoDatasourceSettings{
			UID:  uid,
			URL:  u,
			Opts: httpclient.Options{TLS: &httpclient.TLSOptions{InsecureSkipVerify: skipVerify}},
		}
	}

	// A verifies certificates and must reject the server's self-signed cert.
	dbA, err := Open(settings("uid-verifying", false))
	if err != nil {
		t.Fatalf("open verifying data source: %v", err)
	}
	defer dbA.Close()

	// B skips verification. Opening it second used to overwrite A's transport.
	dbB, err := Open(settings("uid-skip-verify", true))
	if err != nil {
		t.Fatalf("open skip-verify data source: %v", err)
	}
	defer dbB.Close()

	_, err = dbA.QueryContext(context.Background(), "SELECT 1")
	if err == nil {
		t.Fatal("verifying data source accepted a self-signed certificate, so it used the skip-verify transport")
	}
	if !strings.Contains(err.Error(), "certificate") {
		t.Errorf("expected a certificate verification error, got: %v", err)
	}
}

// TestOpenRejectsMissingUID checks the invariant fails closed: without a UID
// there is no way to key the client per data source, and falling back to a
// shared key would silently reintroduce the bug above.
func TestOpenRejectsMissingUID(t *testing.T) {
	u, err := url.Parse("http://trino.invalid:8080")
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	u.User = url.User("grafana")

	_, err = Open(models.TrinoDatasourceSettings{URL: u})
	if err == nil {
		t.Fatal("expected an error when the data source UID is missing")
	}
}

// tokenRecorder records which client_id each OAuth token endpoint received.
type tokenRecorder struct {
	mu       sync.Mutex
	requests map[string][]string
}

func newTokenRecorder() *tokenRecorder {
	return &tokenRecorder{requests: map[string][]string{}}
}

func (r *tokenRecorder) server(t *testing.T, name string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		r.mu.Lock()
		r.requests[name] = append(r.requests[name], req.Form.Get("client_id"))
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"token-` + name + `","expires_in":3600}`))
	}))
	t.Cleanup(server.Close)
	return server
}

// assertOnly asserts that the named endpoint received exactly the given
// client_id and that no other endpoint was contacted, then resets the record.
func (r *tokenRecorder) assertOnly(t *testing.T, name, clientID string) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()

	for other, got := range r.requests {
		if other != name && len(got) > 0 {
			t.Errorf("data source %s used data source %s's OAuth client %v", name, other, got)
		}
	}
	got := r.requests[name]
	if len(got) == 0 {
		t.Errorf("data source %s never reached its own token endpoint", name)
	}
	for _, id := range got {
		if id != clientID {
			t.Errorf("data source %s authenticated as %q, want %q", name, id, clientID)
		}
	}

	r.requests = map[string][]string{}
}

// unreachableTrino stands in for a Trino coordinator that always rejects the
// query, so the test never depends on the query protocol itself.
func unreachableTrino(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	return server
}
