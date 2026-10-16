package models

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/url"
	"reflect"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
)

func TestLoad_BasicAuth(t *testing.T) {
	tests := []struct {
		name             string
		instanceSettings backend.DataSourceInstanceSettings
		wantUserinfo     string
		wantBasicAuth    *httpclient.BasicAuthOptions
	}{
		{
			name: "no basic auth configured defaults to the grafana user",
			instanceSettings: backend.DataSourceInstanceSettings{
				URL:      "http://localhost:8080",
				JSONData: []byte(`{}`),
			},
			wantUserinfo: "grafana",
		},
		{
			name: "basic auth with user and password",
			instanceSettings: backend.DataSourceInstanceSettings{
				URL:              "http://localhost:8080",
				BasicAuthEnabled: true,
				BasicAuthUser:    "alice",
				DecryptedSecureJSONData: map[string]string{
					"basicAuthPassword": "s3cret",
				},
				JSONData: []byte(`{}`),
			},
			wantUserinfo:  "alice",
			wantBasicAuth: &httpclient.BasicAuthOptions{User: "alice", Password: "s3cret"},
		},
		{
			name: "basic auth with user only",
			instanceSettings: backend.DataSourceInstanceSettings{
				URL:              "http://localhost:8080",
				BasicAuthEnabled: true,
				BasicAuthUser:    "bob",
				JSONData:         []byte(`{}`),
			},
			wantUserinfo: "bob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := TrinoDatasourceSettings{}
			if err := settings.Load(context.Background(), tt.instanceSettings); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := settings.URL.User.String(); got != tt.wantUserinfo {
				t.Errorf("got URL userinfo %q, want %q", got, tt.wantUserinfo)
			}
			if !reflect.DeepEqual(settings.Opts.BasicAuth, tt.wantBasicAuth) {
				t.Errorf("got basic auth %+v, want %+v", settings.Opts.BasicAuth, tt.wantBasicAuth)
			}
		})
	}
}

func TestLoad_RejectsCustomHeaders(t *testing.T) {
	settings := TrinoDatasourceSettings{}
	err := settings.Load(context.Background(), backend.DataSourceInstanceSettings{
		URL:      "http://localhost:8080",
		JSONData: []byte(`{"httpHeaderName1": "X-Custom-Header"}`),
		DecryptedSecureJSONData: map[string]string{
			"httpHeaderValue1": "some-value",
		},
	})
	if err == nil {
		t.Fatal("expected an error when custom headers are configured, got nil")
	}
}

func TestLoad_RejectsInvalidURLs(t *testing.T) {
	tests := []struct {
		name     string
		trinoURL string
		jsonData string
	}{
		{name: "Trino URL scheme", trinoURL: "file:///tmp/trino", jsonData: `{}`},
		{name: "Trino URL host", trinoURL: "https:///trino", jsonData: `{}`},
		{name: "OAuth token URL scheme", trinoURL: "https://trino.example", jsonData: `{"tokenUrl":"file:///tmp/token"}`},
		{name: "OAuth token URL host", trinoURL: "https://trino.example", jsonData: `{"tokenUrl":"https:///token"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := TrinoDatasourceSettings{}
			err := settings.Load(context.Background(), backend.DataSourceInstanceSettings{
				URL:      tt.trinoURL,
				JSONData: []byte(tt.jsonData),
			})
			if err == nil {
				t.Fatal("expected an invalid URL error")
			}
		})
	}
}

func TestLoad_ImpersonationIdentity(t *testing.T) {
	tests := []struct {
		jsonData string
		want     string
		wantErr  bool
	}{
		{jsonData: `{}`, want: ImpersonationIdentityLogin},
		{jsonData: `{"impersonationIdentity":""}`, want: ImpersonationIdentityLogin},
		{jsonData: `{"impersonationIdentity":"login"}`, want: ImpersonationIdentityLogin},
		{jsonData: `{"impersonationIdentity":"email"}`, want: ImpersonationIdentityEmail},
		{jsonData: `{"impersonationIdentity":"name"}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.jsonData, func(t *testing.T) {
			settings := TrinoDatasourceSettings{}
			err := settings.Load(context.Background(), backend.DataSourceInstanceSettings{
				URL:      "http://localhost:8080",
				JSONData: []byte(tt.jsonData),
			})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if settings.ImpersonationIdentity != tt.want {
				t.Errorf("got %q, want %q", settings.ImpersonationIdentity, tt.want)
			}
		})
	}
}

func TestLoad_Kerberos(t *testing.T) {
	settings := TrinoDatasourceSettings{}
	err := settings.Load(context.Background(), backend.DataSourceInstanceSettings{
		URL: "https://trino.example.com:8443",
		JSONData: []byte(`{
			"oauthPassThru": true,
			"kerberosEnabled": true,
			"kerberosPrincipal": "grafana",
			"kerberosRealm": "EXAMPLE.COM",
			"kerberosConfig": "[libdefaults]\n  default_realm = EXAMPLE.COM\n",
			"kerberosRemoteServiceName": "HTTP",
			"kerberosServicePrincipalPattern": "${SERVICE}@trino.example.com",
			"kerberosDisableCanonicalHostname": true
		}`),
		DecryptedSecureJSONData: map[string]string{"kerberosKeytab": base64.StdEncoding.EncodeToString([]byte("keytab bytes"))},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settings.URL = nil
	settings.Opts = httpclient.Options{}
	want := TrinoDatasourceSettings{
		ImpersonationIdentity:            ImpersonationIdentityLogin,
		OAuthPassThru:                    true,
		KerberosEnabled:                  true,
		KerberosPrincipal:                "grafana",
		KerberosRealm:                    "EXAMPLE.COM",
		KerberosConfig:                   "[libdefaults]\n  default_realm = EXAMPLE.COM\n",
		KerberosKeytab:                   []byte("keytab bytes"),
		KerberosRemoteServiceName:        "HTTP",
		KerberosServicePrincipalPattern:  "${SERVICE}@trino.example.com",
		KerberosDisableCanonicalHostname: true,
	}
	if !reflect.DeepEqual(settings, want) {
		t.Errorf("got %+v, want %+v", settings, want)
	}
}

func TestLoad_KerberosKeytab(t *testing.T) {
	tests := []struct {
		name    string
		keytab  string
		want    []byte
		wantErr string
	}{
		{name: "absent", want: nil},
		{name: "base64", keytab: base64.StdEncoding.EncodeToString([]byte{5, 2, 0}), want: []byte{5, 2, 0}},
		{name: "raw keytab", keytab: "\x05\x02", wantErr: "invalid Kerberos keytab, expected the keytab file encoded in base64: illegal base64 data at input byte 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := TrinoDatasourceSettings{}
			err := settings.Load(context.Background(), backend.DataSourceInstanceSettings{
				URL:                     "https://trino.example.com:8443",
				JSONData:                []byte(`{"kerberosEnabled": true}`),
				DecryptedSecureJSONData: map[string]string{"kerberosKeytab": tt.keytab},
			})
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("got error %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.Equal(settings.KerberosKeytab, tt.want) {
				t.Errorf("got keytab %v, want %v", settings.KerberosKeytab, tt.want)
			}
		})
	}
}

func TestLoad_KerberosUser(t *testing.T) {
	tests := []struct {
		name             string
		instanceSettings backend.DataSourceInstanceSettings
		wantUser         *url.Userinfo
	}{
		{
			name: "no basic auth leaves the user to the Kerberos principal",
			instanceSettings: backend.DataSourceInstanceSettings{
				URL:      "https://trino.example.com:8443",
				JSONData: []byte(`{"kerberosEnabled": true}`),
			},
			wantUser: nil,
		},
		{
			name: "basic auth user is kept as the session user",
			instanceSettings: backend.DataSourceInstanceSettings{
				URL:              "https://trino.example.com:8443",
				BasicAuthEnabled: true,
				BasicAuthUser:    "alice",
				JSONData:         []byte(`{"kerberosEnabled": true}`),
			},
			wantUser: url.User("alice"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := TrinoDatasourceSettings{}
			if err := settings.Load(context.Background(), tt.instanceSettings); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(settings.URL.User, tt.wantUser) {
				t.Errorf("got URL user %v, want %v", settings.URL.User, tt.wantUser)
			}
		})
	}
}
