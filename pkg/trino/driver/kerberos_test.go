package driver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/trinodb/grafana-trino/pkg/trino/models"
	"github.com/trinodb/trino-go-client/trino"
)

func TestOpenRejectsInvalidKerberosSettings(t *testing.T) {
	kerberos := func(trinoURL string, update func(*models.TrinoDatasourceSettings)) models.TrinoDatasourceSettings {
		u, err := url.Parse(trinoURL)
		if err != nil {
			t.Fatalf("parse trino URL: %v", err)
		}
		settings := models.TrinoDatasourceSettings{
			UID:                "uid-kerberos-invalid",
			URL:                u,
			KerberosEnabled:    true,
			KerberosConfigPath: models.DefaultKerberosConfigPath,
		}
		update(&settings)
		return settings
	}

	tests := []struct {
		name     string
		settings models.TrinoDatasourceSettings
		wantErr  string
	}{
		{
			name:     "HTTP URL",
			settings: kerberos("http://trino.example.com:8080", func(*models.TrinoDatasourceSettings) {}),
			wantErr:  "'Kerberos Authentication' requires an HTTPS Trino URL",
		},
		{
			name: "every conflicting authentication method",
			settings: kerberos("https://trino.example.com:8443", func(s *models.TrinoDatasourceSettings) {
				s.URL.User = url.UserPassword("alice", "s3cret")
				s.AccessToken = "token"
				s.TokenUrl = "https://idp.example.com/token"
				s.OAuthPassThru = true
			}),
			wantErr: "'Kerberos Authentication' cannot be combined with: basic auth password, access token, 'OAuth Trino Authentication', forwarding the OAuth identity",
		},
		{
			name: "OAuth client secret only",
			settings: kerberos("https://trino.example.com:8443", func(s *models.TrinoDatasourceSettings) {
				s.ClientSecret = "secret"
			}),
			wantErr: "'Kerberos Authentication' cannot be combined with: 'OAuth Trino Authentication'",
		},
		{
			name: "keytab and credential cache",
			settings: kerberos("https://trino.example.com:8443", func(s *models.TrinoDatasourceSettings) {
				s.KerberosKeytabPath = "/etc/grafana/grafana.keytab"
				s.KerberosCredentialCachePath = "/tmp/krb5cc_grafana"
			}),
			wantErr: "'Kerberos Authentication' takes either a keytab path or a credential cache path, not both",
		},
		{
			name: "keytab without principal and realm",
			settings: kerberos("https://trino.example.com:8443", func(s *models.TrinoDatasourceSettings) {
				s.KerberosKeytabPath = "/etc/grafana/grafana.keytab"
			}),
			wantErr: "missing parameters for logging in with a keytab in 'Kerberos Authentication': Principal, Realm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := Open(tt.settings)
			if err == nil {
				_ = db.Close()
				t.Fatal("expected an error")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("got error %q, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestOpenAcceptsKerberosWithSessionUser(t *testing.T) {
	u, err := url.Parse("https://trino.example.com:8443")
	if err != nil {
		t.Fatalf("parse trino URL: %v", err)
	}
	u.User = url.User("alice")
	db, err := Open(models.TrinoDatasourceSettings{
		UID:                         "uid-kerberos-session-user",
		URL:                         u,
		KerberosEnabled:             true,
		KerberosConfigPath:          models.DefaultKerberosConfigPath,
		KerberosCredentialCachePath: "/tmp/krb5cc_grafana",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = db.Close()
}

func TestNewConfigMapsKerberosSettings(t *testing.T) {
	u, err := url.Parse("https://trino.example.com:8443")
	if err != nil {
		t.Fatalf("parse trino URL: %v", err)
	}
	settings := models.TrinoDatasourceSettings{
		URL:                              u,
		KerberosPrincipal:                "grafana",
		KerberosRealm:                    "EXAMPLE.COM",
		KerberosConfigPath:               "/etc/grafana/krb5.conf",
		KerberosKeytabPath:               "/etc/grafana/grafana.keytab",
		KerberosCredentialCachePath:      "/tmp/krb5cc_grafana",
		KerberosRemoteServiceName:        "HTTP",
		KerberosServicePrincipalPattern:  "${SERVICE}@trino.example.com",
		KerberosDisableCanonicalHostname: true,
	}

	t.Run("disabled", func(t *testing.T) {
		config, err := newConfig(settings, "grafana-uid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.KerberosEnabled || config.KerberosPrincipal != "" || config.KerberosKeytabPath != "" {
			t.Errorf("expected no Kerberos settings while Kerberos is disabled, got %+v", config)
		}
	})

	t.Run("enabled", func(t *testing.T) {
		enabled := settings
		enabled.KerberosEnabled = true
		config, err := newConfig(enabled, "grafana-uid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		retryTimeout := requestRetryTimeout
		want := trino.Config{
			ServerURI:                        "https://trino.example.com:8443",
			Source:                           "grafana",
			CustomClientName:                 "grafana-uid",
			ForwardAuthorizationHeader:       true,
			Roles:                            map[string]string{},
			RequestRetryTimeout:              &retryTimeout,
			KerberosEnabled:                  true,
			KerberosPrincipal:                "grafana",
			KerberosRealm:                    "EXAMPLE.COM",
			KerberosConfigPath:               "/etc/grafana/krb5.conf",
			KerberosKeytabPath:               "/etc/grafana/grafana.keytab",
			KerberosCredentialCachePath:      "/tmp/krb5cc_grafana",
			KerberosRemoteServiceName:        "HTTP",
			KerberosServicePrincipalPattern:  "${SERVICE}@trino.example.com",
			KerberosDisableCanonicalHostname: true,
		}
		if !reflect.DeepEqual(config, want) {
			t.Errorf("got %+v, want %+v", config, want)
		}
	})
}

// TestOpenPassesKerberosSettingsToClient checks that the Kerberos settings
// reach trino-go-client together with the custom HTTP client, without a KDC:
// the client loads the krb5 config and the keytab before it sends anything, so
// a missing keytab fails the query before the coordinator is contacted.
func TestOpenPassesKerberosSettingsToClient(t *testing.T) {
	var requests atomic.Int32
	trinoServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(trinoServer.Close)

	dir := t.TempDir()
	krb5ConfigPath := filepath.Join(dir, "krb5.conf")
	krb5Config := "[libdefaults]\n  default_realm = EXAMPLE.COM\n"
	if err := os.WriteFile(krb5ConfigPath, []byte(krb5Config), 0o600); err != nil {
		t.Fatalf("write krb5 config: %v", err)
	}
	keytabPath := filepath.Join(dir, "missing.keytab")

	u, err := url.Parse(trinoServer.URL)
	if err != nil {
		t.Fatalf("parse trino URL: %v", err)
	}
	db, err := Open(models.TrinoDatasourceSettings{
		UID:                "uid-kerberos",
		URL:                u,
		Opts:               httpclient.Options{TLS: &httpclient.TLSOptions{InsecureSkipVerify: true}},
		KerberosEnabled:    true,
		KerberosPrincipal:  "grafana",
		KerberosRealm:      "EXAMPLE.COM",
		KerberosConfigPath: krb5ConfigPath,
		KerberosKeytabPath: keytabPath,
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.QueryContext(context.Background(), "SELECT 1")
	if err == nil {
		t.Fatal("expected the query to fail loading the keytab")
	}
	if !strings.Contains(err.Error(), "Error loading Keytab") || !strings.Contains(err.Error(), keytabPath) {
		t.Errorf("expected an error loading the keytab %s, got: %v", keytabPath, err)
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("Trino received %d requests before Kerberos logged in", got)
	}
}
