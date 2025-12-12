package models

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

type TrinoDatasourceSettings struct {
	URL                 *url.URL           `json:"-"`
	Opts                httpclient.Options `json:"-"`
	EnableImpersonation bool               `json:"enableImpersonation"`
	AccessToken         string             `json:"accessToken"`
	TokenUrl            string             `json:"tokenUrl"`
	ClientId            string             `json:"clientId"`
	ClientSecret        string             `json:"clientSecret"`
	ImpersonationUser   string             `json:"impersonationUser"`
	ClientTags          string             `json:"clientTags"`
}

func (s *TrinoDatasourceSettings) Load(ctx context.Context,config backend.DataSourceInstanceSettings) error {
	opts, err := config.HTTPClientOptions(ctx)
	if err != nil {
		return err
	}
	if len(opts.Header) != 0 {
		return errors.New("Custom headers are not supported and must be not set")
	}
	log.DefaultLogger.Info("Loading Trino data source settings")
	s.URL, err = url.Parse(config.URL)
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
	if token, ok := config.DecryptedSecureJSONData["accessToken"]; ok {
		s.AccessToken = token
	}
	if clientSecret, ok := config.DecryptedSecureJSONData["clientSecret"]; ok {
		s.ClientSecret = clientSecret
	}
	return nil
}
