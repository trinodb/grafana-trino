package driver

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/proxy"
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
	skipVerify := false
	var certPool *x509.CertPool
	var clientCert []tls.Certificate
	if settings.Opts.TLS != nil {
		skipVerify = settings.Opts.TLS.InsecureSkipVerify
		if settings.Opts.TLS.CACertificate != "" {
			certPool := x509.NewCertPool()
			certPool.AppendCertsFromPEM([]byte(settings.Opts.TLS.CACertificate))
		}
		if settings.Opts.TLS.ClientCertificate != "" {
			if settings.Opts.TLS.ClientKey == "" {
				return nil, errors.New("client certificate was configured without a client key")
			}
			cert, err := tls.X509KeyPair(
				[]byte(settings.Opts.TLS.ClientCertificate),
				[]byte(settings.Opts.TLS.ClientKey))
			if err != nil {
				return nil, fmt.Errorf("failed to load client certificate: %w", err)
			}
			clientCert = append(clientCert, cert)
		}
	}
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipVerify,
				Certificates:       clientCert,
				RootCAs:            certPool,
			},
		},
	}
	if isOAuthConfigured(settings) {
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

	client, sockErr := applyecureSocksProxy(&settings, client)
	if sockErr != nil {
		return nil, fmt.Errorf("failed to configure secure SOCKS proxy: %w", sockErr)
	}

	err := trino.RegisterCustomClient("grafana", client)
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
		CustomClientName:           "grafana",
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

func isOAuthConfigured(setting models.TrinoDatasourceSettings) bool {
	return setting.TokenUrl != "" || setting.ClientId != "" || setting.ClientSecret != ""
}

func applyecureSocksProxy(settings *models.TrinoDatasourceSettings, httpClient *http.Client) (*http.Client, error) {
	if isOAuthConfigured(*settings) {
		if customTransport, ok := httpClient.Transport.(*customTransport); ok {
			if transport, ok := customTransport.client.Client.Transport.(*http.Transport); ok {
				err := proxy.New(settings.Opts.ProxyOptions).ConfigureSecureSocksHTTPProxy(transport)
				if err != nil {
					return nil, fmt.Errorf("error configurin secure SOCKS proxy for OAuth: %w", err)
				}
			}
		}
	} else {
		if transport, ok := httpClient.Transport.(*http.Transport); ok {
			err := proxy.New(settings.Opts.ProxyOptions).ConfigureSecureSocksHTTPProxy(transport)
			if err != nil {
				return nil, fmt.Errorf("error configuring secure SOCKS proxy:%w", err)
			}
		}
	}

	return httpClient, nil
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
