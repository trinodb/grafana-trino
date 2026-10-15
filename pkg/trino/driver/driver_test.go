package driver

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/trinodb/grafana-trino/pkg/trino/models"
)

// TestOpenRejectsBrokenClientCertificates covers the two ways a client
// certificate can be misconfigured: the SDK reports a key that does not match
// the certificate, but silently leaves out a certificate without any key.
func TestOpenRejectsBrokenClientCertificates(t *testing.T) {
	certPEM, _ := selfSignedCert(t)

	tests := []struct {
		name string
		tls  *httpclient.TLSOptions
	}{
		{name: "client certificate without key", tls: &httpclient.TLSOptions{ClientCertificate: certPEM}},
		{name: "mismatched client certificate and key", tls: &httpclient.TLSOptions{ClientCertificate: certPEM, ClientKey: "not a key"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse("https://trino.example.com:8443")
			if err != nil {
				t.Fatalf("parse trino URL: %v", err)
			}
			db, err := Open(models.TrinoDatasourceSettings{
				UID:  "uid-broken-tls",
				URL:  u,
				Opts: httpclient.Options{TLS: tt.tls},
			})
			if err == nil {
				_ = db.Close()
				t.Fatal("expected an error")
			}
		})
	}
}

// TestOpenSendsBasicAuthThroughTheHTTPClient checks that the basic auth
// password reaches Trino over plain HTTP, where trino-go-client itself refuses
// to send one, and that a credential set on the request keeps precedence over
// the data source password.
func TestOpenSendsBasicAuthThroughTheHTTPClient(t *testing.T) {
	var mu sync.Mutex
	var authorizations, users []string
	trinoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authorizations = append(authorizations, r.Header.Get("Authorization"))
		users = append(users, r.Header.Get("X-Trino-User"))
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(trinoServer.Close)

	u, err := url.Parse(trinoServer.URL)
	if err != nil {
		t.Fatalf("parse trino URL: %v", err)
	}
	u.User = url.User("alice")
	db, err := Open(models.TrinoDatasourceSettings{
		UID:  "uid-basic-auth",
		URL:  u,
		Opts: httpclient.Options{BasicAuth: &httpclient.BasicAuthOptions{User: "alice", Password: "s3cret"}},
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	tests := []struct {
		name              string
		args              []interface{}
		wantAuthorization string
	}{
		{name: "password", wantAuthorization: "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:s3cret"))},
		{name: "forwarded access token", args: []interface{}{sql.Named("accessToken", "forwarded")}, wantAuthorization: "Bearer forwarded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mu.Lock()
			authorizations, users = nil, nil
			mu.Unlock()
			_, _ = db.QueryContext(context.Background(), "SELECT 1", tt.args...)
			mu.Lock()
			defer mu.Unlock()
			if len(authorizations) == 0 {
				t.Fatal("Trino never received a request")
			}
			for i, got := range authorizations {
				if got != tt.wantAuthorization {
					t.Errorf("Trino saw Authorization %q, want %q", got, tt.wantAuthorization)
				}
				if users[i] != "alice" {
					t.Errorf("Trino saw user %q, want %q", users[i], "alice")
				}
			}
		})
	}
}

// TestNewHTTPClientLimitsResponseSize checks that the SDK's default
// middlewares are in place, through the one Grafana administrators configure:
// the response size limit.
func TestNewHTTPClientLimitsResponseSize(t *testing.T) {
	t.Setenv("GF_DATAPROXY_RESPONSE_LIMIT", "16")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), 1024))
	}))
	t.Cleanup(server.Close)

	client, err := newHTTPClient(models.TrinoDatasourceSettings{UID: "uid-response-limit"})
	if err != nil {
		t.Fatalf("new HTTP client: %v", err)
	}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer response.Body.Close()
	_, err = io.ReadAll(response.Body)
	if !errors.Is(err, httpclient.ErrResponseBodyTooLarge) {
		t.Errorf("expected the response limit middleware to cut the body short, got: %v", err)
	}
}

// selfSignedCert generates a throwaway self-signed cert/key pair PEM for TLS tests.
func selfSignedCert(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	var certBuf bytes.Buffer
	if err := pem.Encode(&certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("failed to encode certificate: %v", err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal key: %v", err)
	}
	var keyBuf bytes.Buffer
	if err := pem.Encode(&keyBuf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		t.Fatalf("failed to encode key: %v", err)
	}

	return certBuf.String(), keyBuf.String()
}

func TestParseRoles(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{name: "empty", input: "", want: map[string]string{}},
		{name: "blank", input: "   ", want: map[string]string{}},
		{
			name:  "single",
			input: "system:admin",
			want:  map[string]string{"system": "admin"},
		},
		{
			name:  "multiple",
			input: "system:admin;catalog1:roleA;catalog2:roleB",
			want:  map[string]string{"system": "admin", "catalog1": "roleA", "catalog2": "roleB"},
		},
		{
			name:  "trims whitespace",
			input: " system : admin ; catalog1 : roleA ",
			want:  map[string]string{"system": "admin", "catalog1": "roleA"},
		},
		{
			name:    "missing colon",
			input:   "system-admin",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRoles(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
