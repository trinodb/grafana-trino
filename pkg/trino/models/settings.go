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

type TrinoDatasourceSettings struct {
	// UID identifies the data source instance these settings belong to. It is
	// used to keep per-instance state (such as the registered HTTP client)
	// isolated between data sources, so it must never be populated from
	// jsonData.
	UID                 string             `json:"-"`
	URL                 *url.URL           `json:"-"`
	Opts                httpclient.Options `json:"-"`
	EnableImpersonation bool               `json:"enableImpersonation"`
	AccessToken         string             `json:"accessToken"`
	TokenUrl            string             `json:"tokenUrl"`
	ClientId            string             `json:"clientId"`
	ClientSecret        string             `json:"clientSecret"`
	ImpersonationUser   string             `json:"impersonationUser"`
	Roles               string             `json:"roles"`
	ClientTags          string             `json:"clientTags"`
}

func (s *TrinoDatasourceSettings) Load(ctx context.Context, config backend.DataSourceInstanceSettings) error {
	opts, err := config.HTTPClientOptions(ctx)
	if err != nil {
		return err
	}
	if len(opts.Header) != 0 {
		return errors.New("Custom headers are not supported and must be not set")
	}
	log.DefaultLogger.Info("Loading Trino data source settings")
	s.UID = config.UID
	s.URL, err = parseHTTPURL(config.URL, "Trino URL")
	if err != nil {
		return err
	}
	if opts.BasicAuth != nil {
		if opts.BasicAuth.Password != "" {
			s.URL.User = url.UserPassword(opts.BasicAuth.User, opts.BasicAuth.Password)
		} else {
			s.URL.User = url.User(opts.BasicAuth.User)
		}
	} else {
		s.URL.User = url.User("grafana")
	}
	s.Opts = opts
	err = json.Unmarshal(config.JSONData, &s)
	if err != nil {
		return err
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
