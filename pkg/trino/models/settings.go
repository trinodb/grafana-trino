package models

import (
	"errors"
	"net/url"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

type TrinoDatasourceSettings struct {
	URL  *url.URL
	Opts httpclient.Options
}

func (s *TrinoDatasourceSettings) Load(config backend.DataSourceInstanceSettings) error {
	opts, err := config.HTTPClientOptions()
	if err != nil {
		return err
	}
	if len(opts.Headers) != 0 {
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
	return nil
}
