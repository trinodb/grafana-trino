package driver

import (
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	trinoClient "github.com/trinodb/grafana-trino/pkg/trino/client"

	"github.com/trinodb/grafana-trino/pkg/trino/models"
	"github.com/trinodb/trino-go-client/trino"
	_ "github.com/trinodb/trino-go-client/trino"
)

const DriverName string = "trino"

// just compile time assertion
var _ http.RoundTripper = &customTransport{}

type customTransport struct {
	client *trinoClient.Client
}

func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.client.Do(req)
}

// Open registers a new driver with a unique name
func Open(settings models.TrinoDatasourceSettings) (*sql.DB, error) {
	tlsConfig, err := buildTLSConfig(settings.Opts.TLS)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
	if settings.TokenUrl != "" || settings.ClientId != "" || settings.ClientSecret != "" {
		if settings.AccessToken != "" {
			return nil, errors.New("access token must not be set within 'OAuth Trino Authentication' settings")
		}
		var missingParams []string
		if settings.TokenUrl == "" {
			missingParams = append(missingParams, "Token URL")
		}
		if settings.ClientId == "" {
			missingParams = append(missingParams, "Client id")
		}
		if settings.ClientSecret == "" {
			missingParams = append(missingParams, "Client secret")
		}
		if len(missingParams) > 0 {
			return nil, fmt.Errorf("missing parameters for 'OAuth Trino Authentication': %v", strings.Join(missingParams, ", "))
		}
		client = &http.Client{
			Transport: &customTransport{
				client: &trinoClient.Client{
					Client:            client,
					ClientId:          settings.ClientId,
					ClientSecret:      settings.ClientSecret,
					Url:               settings.TokenUrl,
					ImpersonationUser: settings.ImpersonationUser,
				},
			},
		}
	}
	clientName, err := customClientName(settings.UID)
	if err != nil {
		return nil, err
	}
	err = trino.RegisterCustomClient(clientName, client)
	if err != nil {
		return nil, err
	}

	roles, err := parseRoles(settings.Roles)
	if err != nil {
		return nil, err
	}

	config := trino.Config{
		ServerURI:                  settings.URL.String(),
		Source:                     "grafana",
		CustomClientName:           clientName,
		ForwardAuthorizationHeader: true,
		AccessToken:                settings.AccessToken,
		Roles:                      roles,
	}

	dsn, err := config.FormatDSN()
	if err != nil {
		return nil, err
	}
	return sql.Open(DriverName, dsn)
}

// customClientName returns the key this data source's HTTP client is registered
// under in trino-go-client's custom client registry. That registry is
// process-global and shared by every data source this plugin serves, and
// trino-go-client resolves the key on every new connection, so a key that isn't
// unique per data source lets one data source's credentials, TLS settings and
// dialer silently replace another's.
//
// The prefix keeps the key clear of the keys RegisterCustomClient reserves
// (anything parsing as a bool).
func customClientName(uid string) (string, error) {
	if uid == "" {
		return "", errors.New("data source UID is missing, refusing to share one Trino client between data sources")
	}
	return "grafana-" + uid, nil
}

// buildTLSConfig builds the tls.Config used for connections to Trino from
// the datasource's TLS settings (CA certificate, client certificate/key,
// skip-verify).
func buildTLSConfig(opts *httpclient.TLSOptions) (*tls.Config, error) {
	if opts != nil && opts.ClientCertificate != "" && opts.ClientKey == "" {
		return nil, errors.New("client certificate was configured without a client key")
	}

	return httpclient.GetTLSConfig(httpclient.Options{TLS: opts})
}

func parseRoles(roleStr string) (map[string]string, error) {
	roles := make(map[string]string)
	if strings.TrimSpace(roleStr) == "" {
		return roles, nil
	}
	pairs := strings.Split(roleStr, ";")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("Invalid role format. expected catalog:role, got '%s'", pair)
		}
		catalog := strings.TrimSpace(parts[0])
		role := strings.TrimSpace(parts[1])
		if catalog != "" && role != "" {
			roles[catalog] = role
		}
	}
	return roles, nil
}
