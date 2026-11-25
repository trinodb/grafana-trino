package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

func newTokenServer(t *testing.T, accessToken string) (*httptest.Server, *int32) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": accessToken,
			"expires_in":   3600,
		})
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func TestClient_TokenIsCachedPerInstance(t *testing.T) {
	server, calls := newTokenServer(t, "token-a")
	c := &Client{Client: http.DefaultClient, ClientId: "id-a", ClientSecret: "secret-a", Url: server.URL}

	token1, err := c.getToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	token2, err := c.getToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token1.AccessToken != "token-a" || token2.AccessToken != "token-a" {
		t.Fatalf("unexpected tokens: %q, %q", token1.AccessToken, token2.AccessToken)
	}
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Errorf("expected token endpoint to be called once (cached on second call), got %d calls", got)
	}
}

func TestClient_TokensAreNotSharedAcrossInstances(t *testing.T) {
	serverA, _ := newTokenServer(t, "token-a")
	serverB, _ := newTokenServer(t, "token-b")

	clientA := &Client{Client: http.DefaultClient, ClientId: "id-a", ClientSecret: "secret-a", Url: serverA.URL}
	clientB := &Client{Client: http.DefaultClient, ClientId: "id-b", ClientSecret: "secret-b", Url: serverB.URL}

	tokenA, err := clientA.getToken()
	if err != nil {
		t.Fatalf("unexpected error from clientA: %v", err)
	}
	tokenB, err := clientB.getToken()
	if err != nil {
		t.Fatalf("unexpected error from clientB: %v", err)
	}

	if tokenA.AccessToken != "token-a" {
		t.Errorf("clientA got wrong token: %q", tokenA.AccessToken)
	}
	if tokenB.AccessToken != "token-b" {
		t.Errorf("clientB got wrong token: %q (expected it to not leak clientA's cached token)", tokenB.AccessToken)
	}

	// Re-fetching from clientA must still return its own cached token, not clientB's.
	tokenAAgain, err := clientA.getToken()
	if err != nil {
		t.Fatalf("unexpected error from clientA: %v", err)
	}
	if tokenAAgain.AccessToken != "token-a" {
		t.Errorf("clientA's cached token was clobbered by clientB: got %q", tokenAAgain.AccessToken)
	}
}

func TestClient_AuthenticatesRequestsToTrino(t *testing.T) {
	tokenServer, _ := newTokenServer(t, "token-a")
	var authorization, user string
	trino := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		user = r.Header.Get("X-Trino-User")
	}))
	t.Cleanup(trino.Close)
	trinoURL, err := url.Parse(trino.URL)
	if err != nil {
		t.Fatalf("parse trino URL: %v", err)
	}
	c := &Client{Client: http.DefaultClient, ClientId: "id-a", ClientSecret: "secret-a", Url: tokenServer.URL, ImpersonationUser: "alice", TrinoHost: trinoURL.Host}

	req, err := http.NewRequest(http.MethodGet, trino.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	response, err := (&http.Client{Transport: c}).Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = response.Body.Close()

	if authorization != "Bearer token-a" {
		t.Errorf("got Authorization %q, want the OAuth token", authorization)
	}
	if user != "alice" {
		t.Errorf("got X-Trino-User %q, want the impersonation user", user)
	}
	if req.Header.Get("Authorization") != "" {
		t.Error("the caller's request was modified")
	}
}

func TestClient_DoesNotSendTheTokenToAnotherHost(t *testing.T) {
	tokenServer, _ := newTokenServer(t, "token-a")
	var authorization, user string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		user = r.Header.Get("X-Trino-User")
	}))
	t.Cleanup(other.Close)
	trino := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusFound)
	}))
	t.Cleanup(trino.Close)
	trinoURL, err := url.Parse(trino.URL)
	if err != nil {
		t.Fatalf("parse trino URL: %v", err)
	}
	c := &Client{Client: http.DefaultClient, ClientId: "id-a", ClientSecret: "secret-a", Url: tokenServer.URL, ImpersonationUser: "alice", TrinoHost: trinoURL.Host}

	response, err := (&http.Client{Transport: c}).Get(trino.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = response.Body.Close()

	if authorization != "" {
		t.Errorf("the redirect target got Authorization %q", authorization)
	}
	if user != "" {
		t.Errorf("the redirect target got X-Trino-User %q", user)
	}
}

func TestClient_RejectsInvalidRequestURLs(t *testing.T) {
	tests := []struct {
		name string
		url  *url.URL
	}{
		{name: "unsupported scheme", url: &url.URL{Scheme: "file", Path: "/tmp/trino"}},
		{name: "missing host", url: &url.URL{Scheme: "https", Path: "/trino"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := Client{}
			_, err := client.RoundTrip(&http.Request{URL: tt.url})
			if err == nil {
				t.Fatal("expected an invalid URL error")
			}
		})
	}
}
