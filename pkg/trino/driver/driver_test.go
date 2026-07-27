package driver

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
)

func TestBuildTLSConfig(t *testing.T) {
	certPEM, keyPEM := selfSignedCert(t)

	t.Run("nil options", func(t *testing.T) {
		cfg, err := buildTLSConfig(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.RootCAs != nil || cfg.Certificates != nil || cfg.InsecureSkipVerify {
			t.Errorf("expected an empty config, got %+v", cfg)
		}
	})

	t.Run("skip verify", func(t *testing.T) {
		cfg, err := buildTLSConfig(&httpclient.TLSOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.InsecureSkipVerify {
			t.Error("expected InsecureSkipVerify to be true")
		}
	})

	t.Run("CA certificate is applied to RootCAs", func(t *testing.T) {
		// Regression test: this previously silently no-op'd because the CA
		// certificate pool was built into a shadowed local variable instead
		// of the one actually used to construct tls.Config.
		cfg, err := buildTLSConfig(&httpclient.TLSOptions{CACertificate: certPEM})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.RootCAs == nil {
			t.Fatal("expected RootCAs to be populated from the configured CA certificate")
		}
		if len(cfg.RootCAs.Subjects()) != 1 { //nolint:staticcheck // Subjects() is deprecated but fine for a test assertion
			t.Errorf("expected exactly one CA subject in the pool, got %d", len(cfg.RootCAs.Subjects()))
		}
	})

	t.Run("client certificate and key are applied", func(t *testing.T) {
		cfg, err := buildTLSConfig(&httpclient.TLSOptions{
			ClientCertificate: certPEM,
			ClientKey:         keyPEM,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Certificates) != 1 {
			t.Errorf("expected exactly one client certificate, got %d", len(cfg.Certificates))
		}
	})

	t.Run("client certificate without key is an error", func(t *testing.T) {
		_, err := buildTLSConfig(&httpclient.TLSOptions{ClientCertificate: certPEM})
		if err == nil {
			t.Fatal("expected an error when client certificate is set without a client key")
		}
	})

	t.Run("invalid client certificate/key pair is an error", func(t *testing.T) {
		_, err := buildTLSConfig(&httpclient.TLSOptions{ClientCertificate: certPEM, ClientKey: "not a key"})
		if err == nil {
			t.Fatal("expected an error for a client certificate/key that don't match")
		}
	})
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
