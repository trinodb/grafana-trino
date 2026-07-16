package models

import (
	"context"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestLoad_BasicAuth(t *testing.T) {
	tests := []struct {
		name             string
		instanceSettings backend.DataSourceInstanceSettings
		wantUserinfo     string
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
			wantUserinfo: "alice:s3cret",
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
