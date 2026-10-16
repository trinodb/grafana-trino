package driver

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/iana/etypeID"
	"github.com/jcmturner/gokrb5/v8/keytab"
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
			URL:               u,
			KerberosEnabled:   true,
			KerberosPrincipal: "grafana",
			KerberosRealm:     "EXAMPLE.COM",
			KerberosConfig:    unreachableKDCConfig,
			KerberosKeytab:    newKeytab(t, "grafana", "EXAMPLE.COM"),
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
				s.URL.User = url.User("alice")
				s.Opts.BasicAuth = &httpclient.BasicAuthOptions{User: "alice", Password: "s3cret"}
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
			name: "nothing but the switch",
			settings: kerberos("https://trino.example.com:8443", func(s *models.TrinoDatasourceSettings) {
				s.KerberosPrincipal = ""
				s.KerberosRealm = ""
				s.KerberosConfig = ""
				s.KerberosKeytab = nil
			}),
			wantErr: "missing parameters in 'Kerberos Authentication': Principal, Realm, Config, Keytab",
		},
		{
			name: "keytab without principal and realm",
			settings: kerberos("https://trino.example.com:8443", func(s *models.TrinoDatasourceSettings) {
				s.KerberosPrincipal = ""
				s.KerberosRealm = ""
			}),
			wantErr: "missing parameters in 'Kerberos Authentication': Principal, Realm",
		},
		{
			name: "config with an invalid value",
			settings: kerberos("https://trino.example.com:8443", func(s *models.TrinoDatasourceSettings) {
				s.KerberosConfig = "[libdefaults]\n  dns_lookup_kdc = maybe\n"
			}),
			wantErr: "invalid Kerberos config: error processing libdefaults section: invalid krb5 config libdefaults section line (dns_lookup_kdc = maybe): invalid boolean value",
		},
		{
			name: "keytab that is not a keytab",
			settings: kerberos("https://trino.example.com:8443", func(s *models.TrinoDatasourceSettings) {
				s.KerberosKeytab = []byte("not a keytab")
			}),
			wantErr: "invalid Kerberos keytab: invalid keytab data. First byte does not equal 5",
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

// TestOpenLogsInWithTheKeytabBeforeContactingTrino checks that the keytab and
// config settings build the Kerberos client the data source authenticates
// with, without a KDC: logging in is the first thing Open does with them, so
// an unreachable KDC fails Open before Trino is contacted.
func TestOpenLogsInWithTheKeytabBeforeContactingTrino(t *testing.T) {
	var requests atomic.Int32
	trinoServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(trinoServer.Close)

	u, err := url.Parse(trinoServer.URL)
	if err != nil {
		t.Fatalf("parse trino URL: %v", err)
	}
	db, err := Open(models.TrinoDatasourceSettings{
		URL:               u,
		Opts:              httpclient.Options{TLS: &httpclient.TLSOptions{InsecureSkipVerify: true}},
		KerberosEnabled:   true,
		KerberosPrincipal: "grafana",
		KerberosRealm:     "EXAMPLE.COM",
		KerberosConfig:    unreachableKDCConfig,
		KerberosKeytab:    newKeytab(t, "grafana", "EXAMPLE.COM"),
	})
	if err == nil {
		_ = db.Close()
		t.Fatal("expected the login to fail against the unreachable KDC")
	}
	if !strings.HasPrefix(err.Error(), "Kerberos login as grafana@EXAMPLE.COM failed: ") {
		t.Errorf("expected a login error, got: %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("Trino received %d requests before Kerberos logged in", got)
	}
}

// unreachableKDCConfig names a KDC on a port nothing listens on, so a login
// fails with a connection error instead of waiting for a DNS lookup.
const unreachableKDCConfig = "[libdefaults]\n" +
	"  default_realm = EXAMPLE.COM\n" +
	"  dns_lookup_kdc = false\n" +
	"[realms]\n" +
	"  EXAMPLE.COM = {\n" +
	"    kdc = 127.0.0.1:1\n" +
	"  }\n"

// newKeytab returns a keytab holding one AES key for the principal, as a KDC
// administrator would export it.
func newKeytab(t *testing.T, principal, realm string) []byte {
	t.Helper()
	kt := keytab.New()
	if err := kt.AddEntry(principal, realm, "password", time.Now(), 1, etypeID.AES256_CTS_HMAC_SHA1_96); err != nil {
		t.Fatalf("add keytab entry: %v", err)
	}
	data, err := kt.Marshal()
	if err != nil {
		t.Fatalf("marshal keytab: %v", err)
	}
	return data
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
		KerberosRemoteServiceName:        "HTTP",
		KerberosServicePrincipalPattern:  "${SERVICE}@trino.example.com",
		KerberosDisableCanonicalHostname: true,
	}

	httpClient := &http.Client{}
	kerberosClient := &client.Client{}
	t.Run("disabled", func(t *testing.T) {
		config, err := newConfig(settings, httpClient, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.KerberosEnabled || config.KerberosPrincipal != "" || config.KerberosClient != nil {
			t.Errorf("expected no Kerberos settings while Kerberos is disabled, got %+v", config)
		}
	})

	t.Run("enabled", func(t *testing.T) {
		enabled := settings
		enabled.KerberosEnabled = true
		config, err := newConfig(enabled, httpClient, kerberosClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		retryTimeout := requestRetryTimeout
		want := trino.Config{
			ServerURI:                        "https://trino.example.com:8443",
			Source:                           "grafana",
			HTTPClient:                       httpClient,
			ForwardAuthorizationHeader:       true,
			Roles:                            map[string]string{},
			RequestRetryTimeout:              &retryTimeout,
			KerberosEnabled:                  true,
			KerberosClient:                   kerberosClient,
			KerberosPrincipal:                "grafana",
			KerberosRealm:                    "EXAMPLE.COM",
			KerberosRemoteServiceName:        "HTTP",
			KerberosServicePrincipalPattern:  "${SERVICE}@trino.example.com",
			KerberosDisableCanonicalHostname: true,
		}
		if !reflect.DeepEqual(config, want) {
			t.Errorf("got %+v, want %+v", config, want)
		}
	})
}
