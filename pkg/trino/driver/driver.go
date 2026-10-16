package driver

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/keytab"
	trinoClient "github.com/trinodb/grafana-trino/pkg/trino/client"

	"github.com/trinodb/grafana-trino/pkg/trino/models"
	"github.com/trinodb/trino-go-client/trino"
)

// trino-go-client retries a request that failed with a connection error or a
// 429, 502, 503 or 504 response for two minutes by default. A dashboard
// pointed at an unreachable or misconfigured coordinator would show a
// spinner for that long before reporting the error, so the retries are cut
// short while still covering a brief blip or a coordinator restart.
const requestRetryTimeout = 10 * time.Second

// just compile time assertion
var _ http.RoundTripper = &customTransport{}

type customTransport struct {
	client *trinoClient.Client
}

func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.client.Do(req)
}

// Open connects to Trino through the HTTP client built from the data source
// settings. The client is handed to trino-go-client directly instead of
// through its process-global custom client registry, so nothing is shared
// between data sources and nothing has to be unregistered when one is closed.
func Open(settings models.TrinoDatasourceSettings) (*sql.DB, error) {
	if err := validateKerberos(settings); err != nil {
		return nil, err
	}
	httpClient, err := newHTTPClient(settings)
	if err != nil {
		return nil, err
	}
	kerberosClient, err := newKerberosClient(settings)
	if err != nil {
		return nil, err
	}
	config, err := newConfig(settings, httpClient, kerberosClient)
	if err != nil {
		return nil, err
	}
	connector, err := trino.NewConnector(&config)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}

// newHTTPClient builds the client the Grafana plugin SDK builds for any data
// source from its HTTP options: TLS, the secure SOCKS proxy of Private Data
// Source Connect, the proxy environment variables, and the default middlewares
// that add the basic auth credentials, enforce Grafana's response size limit
// and report metrics and traces.
//
// The basic auth middleware sends the password wherever Trino is configured to
// accept it, including over HTTP, and only when nothing else set the
// Authorization header: an access token, a forwarded OAuth identity, the OAuth
// client token added below and Kerberos all take precedence. The middleware
// explaining a 401 runs after it, so it sees the credential that was sent.
func newHTTPClient(settings models.TrinoDatasourceSettings) (*http.Client, error) {
	if err := validateTLS(settings.Opts.TLS); err != nil {
		return nil, err
	}
	settings.Opts.Middlewares = append(httpclient.DefaultMiddlewares(), unauthorizedMiddleware())
	client, err := httpclient.New(settings.Opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create the HTTP client: %w", err)
	}
	if settings.TokenUrl == "" && settings.ClientId == "" && settings.ClientSecret == "" {
		return client, nil
	}
	if err := validateOAuth(settings); err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: &customTransport{
			client: &trinoClient.Client{
				Client:            client,
				ClientId:          settings.ClientId,
				ClientSecret:      settings.ClientSecret,
				Url:               settings.TokenUrl,
				ImpersonationUser: settings.ImpersonationUser,
			},
		},
	}, nil
}

// newKerberosClient logs in with the keytab, so that an unreachable KDC or
// a keytab without the principal's key is reported right away, instead of by
// every connection. trino-go-client reuses the client for all connections and
// gokrb5 renews or replaces the ticket from the keytab when it expires.
func newKerberosClient(settings models.TrinoDatasourceSettings) (*client.Client, error) {
	if !settings.KerberosEnabled {
		return nil, nil
	}
	krb5Config, err := config.NewFromString(settings.KerberosConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid Kerberos config: %w", err)
	}
	kt := keytab.New()
	if err := kt.Unmarshal(settings.KerberosKeytab); err != nil {
		return nil, fmt.Errorf("invalid Kerberos keytab: %w", err)
	}
	kerberosClient := client.NewWithKeytab(settings.KerberosPrincipal, settings.KerberosRealm, kt, krb5Config)
	if err := kerberosClient.Login(); err != nil {
		return nil, fmt.Errorf("Kerberos login as %s@%s failed: %w", settings.KerberosPrincipal, settings.KerberosRealm, err)
	}
	return kerberosClient, nil
}

// newConfig maps the data source settings to the trino-go-client
// configuration, which sends every request through httpClient and
// authenticates them with kerberosClient when Kerberos is enabled.
func newConfig(settings models.TrinoDatasourceSettings, httpClient *http.Client, kerberosClient *client.Client) (trino.Config, error) {
	roles, err := parseRoles(settings.Roles)
	if err != nil {
		return trino.Config{}, err
	}

	retryTimeout := requestRetryTimeout
	config := trino.Config{
		ServerURI:                  settings.URL.String(),
		Source:                     "grafana",
		HTTPClient:                 httpClient,
		ForwardAuthorizationHeader: true,
		AccessToken:                settings.AccessToken,
		Roles:                      roles,
		RequestRetryTimeout:        &retryTimeout,
	}
	if !settings.KerberosEnabled {
		return config, nil
	}
	config.KerberosEnabled = true
	config.KerberosClient = kerberosClient
	config.KerberosPrincipal = settings.KerberosPrincipal
	config.KerberosRealm = settings.KerberosRealm
	config.KerberosRemoteServiceName = settings.KerberosRemoteServiceName
	config.KerberosServicePrincipalPattern = settings.KerberosServicePrincipalPattern
	config.KerberosDisableCanonicalHostname = settings.KerberosDisableCanonicalHostname
	return config, nil
}

// validateKerberos rejects settings trino-go-client would only report once a
// query runs, or would silently let override the Kerberos authorization
// header: a password, an access token and an OAuth token all replace it.
// Kerberos itself works with the HTTP client built here, since trino-go-client
// adds the header to each request before handing it to the transport.
func validateKerberos(settings models.TrinoDatasourceSettings) error {
	if !settings.KerberosEnabled {
		return nil
	}
	if settings.URL.Scheme != "https" {
		return errors.New("'Kerberos Authentication' requires an HTTPS Trino URL")
	}
	var conflicts []string
	if settings.Opts.BasicAuth != nil {
		conflicts = append(conflicts, "basic auth password")
	}
	if settings.AccessToken != "" {
		conflicts = append(conflicts, "access token")
	}
	if settings.TokenUrl != "" || settings.ClientId != "" || settings.ClientSecret != "" {
		conflicts = append(conflicts, "'OAuth Trino Authentication'")
	}
	if settings.OAuthPassThru {
		conflicts = append(conflicts, "forwarding the OAuth identity")
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("'Kerberos Authentication' cannot be combined with: %s", strings.Join(conflicts, ", "))
	}
	var missingParams []string
	if settings.KerberosPrincipal == "" {
		missingParams = append(missingParams, "Principal")
	}
	if settings.KerberosRealm == "" {
		missingParams = append(missingParams, "Realm")
	}
	if settings.KerberosConfig == "" {
		missingParams = append(missingParams, "Config")
	}
	if len(settings.KerberosKeytab) == 0 {
		missingParams = append(missingParams, "Keytab")
	}
	if len(missingParams) > 0 {
		return fmt.Errorf("missing parameters in 'Kerberos Authentication': %v", strings.Join(missingParams, ", "))
	}
	return nil
}

// validateTLS rejects a client certificate without its key, which the SDK
// would silently leave out of the TLS config.
func validateTLS(opts *httpclient.TLSOptions) error {
	if opts != nil && opts.ClientCertificate != "" && opts.ClientKey == "" {
		return errors.New("client certificate was configured without a client key")
	}
	return nil
}

// validateOAuth also rejects a basic auth password, which the HTTP client
// would send to the token URL along with the client credentials, while Trino
// itself never sees it behind the bearer token.
func validateOAuth(settings models.TrinoDatasourceSettings) error {
	if settings.AccessToken != "" {
		return errors.New("access token must not be set within 'OAuth Trino Authentication' settings")
	}
	if settings.Opts.BasicAuth != nil {
		return errors.New("'OAuth Trino Authentication' cannot be combined with a basic auth password, which would be sent to the token URL")
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
		return fmt.Errorf("missing parameters for 'OAuth Trino Authentication': %v", strings.Join(missingParams, ", "))
	}
	return nil
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
