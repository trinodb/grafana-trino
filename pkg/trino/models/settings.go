package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

const (
	ImpersonationIdentityLogin = "login"
	ImpersonationIdentityEmail = "email"
	DefaultKerberosConfigPath  = "/etc/krb5.conf"
)

type TrinoDatasourceSettings struct {
	// URL names the Trino session user, without the basic auth password: the
	// HTTP client built from Opts sends the credentials, so they are accepted
	// wherever Trino is configured to take them, including over HTTP.
	URL                   *url.URL           `json:"-"`
	Opts                  httpclient.Options `json:"-"`
	EnableImpersonation   bool               `json:"enableImpersonation"`
	ImpersonationIdentity string             `json:"impersonationIdentity"`
	AccessToken           string             `json:"accessToken"`
	TokenUrl              string             `json:"tokenUrl"`
	ClientId              string             `json:"clientId"`
	ClientSecret          string             `json:"clientSecret"`
	ImpersonationUser     string             `json:"impersonationUser"`
	Roles                 string             `json:"roles"`
	ClientTags            string             `json:"clientTags"`
	// OAuthPassThru is Grafana's "Forward OAuth Identity" setting.
	OAuthPassThru                    bool   `json:"oauthPassThru"`
	KerberosEnabled                  bool   `json:"kerberosEnabled"`
	KerberosPrincipal                string `json:"kerberosPrincipal"`
	KerberosRealm                    string `json:"kerberosRealm"`
	KerberosConfigPath               string `json:"kerberosConfigPath"`
	KerberosKeytabPath               string `json:"kerberosKeytabPath"`
	KerberosCredentialCachePath      string `json:"kerberosCredentialCachePath"`
	KerberosRemoteServiceName        string `json:"kerberosRemoteServiceName"`
	KerberosServicePrincipalPattern  string `json:"kerberosServicePrincipalPattern"`
	KerberosDisableCanonicalHostname bool   `json:"kerberosDisableCanonicalHostname"`
}

func (s *TrinoDatasourceSettings) Load(ctx context.Context, config backend.DataSourceInstanceSettings) error {
	opts, err := config.HTTPClientOptions(ctx)
	if err != nil {
		return err
	}
	if len(opts.Header) != 0 {
		return errors.New("Custom headers are not supported and must be not set")
	}
	log.DefaultLogger.Debug("Loading Trino data source settings")
	s.URL, err = parseHTTPURL(config.URL, "Trino URL")
	if err != nil {
		return err
	}
	s.Opts = opts
	err = json.Unmarshal(config.JSONData, &s)
	if err != nil {
		return err
	}
	switch {
	case opts.BasicAuth != nil:
		s.URL.User = url.User(opts.BasicAuth.User)
	case s.KerberosEnabled:
		// Without a user, Trino runs queries as the user the Kerberos
		// principal maps to, instead of requiring the principal to be allowed
		// to impersonate "grafana".
		s.URL.User = nil
	default:
		s.URL.User = url.User("grafana")
	}
	// A basic auth user without a password only names the session user, so no
	// Authorization header is sent for it.
	if opts.BasicAuth != nil && opts.BasicAuth.Password == "" {
		s.Opts.BasicAuth = nil
	}
	if s.KerberosEnabled && s.KerberosConfigPath == "" {
		s.KerberosConfigPath = DefaultKerberosConfigPath
	}
	switch s.ImpersonationIdentity {
	case "":
		s.ImpersonationIdentity = ImpersonationIdentityLogin
	case ImpersonationIdentityLogin, ImpersonationIdentityEmail:
	default:
		return fmt.Errorf("invalid impersonation identity %q, expected %q or %q", s.ImpersonationIdentity, ImpersonationIdentityLogin, ImpersonationIdentityEmail)
	}
	if s.TokenUrl != "" {
		tokenURL, err := parseHTTPURL(s.TokenUrl, "OAuth token URL")
		if err != nil {
			return err
		}
		s.TokenUrl = tokenURL.String()
	}
	if token, ok := config.DecryptedSecureJSONData["accessToken"]; ok {
		s.AccessToken = token
	}
	if clientSecret, ok := config.DecryptedSecureJSONData["clientSecret"]; ok {
		s.ClientSecret = clientSecret
	}
	return nil
}

func parseHTTPURL(value string, name string) (*url.URL, error) {
	parsedURL, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", name, err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("%s must use HTTP or HTTPS", name)
	}
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("%s must include a host", name)
	}
	return parsedURL, nil
}
