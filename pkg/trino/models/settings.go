package models

import (
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

type TrinoDatasourceSettings struct {
	URL  string
	User string
	Opts httpclient.Options
}

func (s *TrinoDatasourceSettings) Load(config backend.DataSourceInstanceSettings) error {
	opts, err := config.HTTPClientOptions()
	if err != nil {
		return err
	}
	log.DefaultLogger.Info("Loading Trino data source settings")
	s.URL = config.URL
	s.User = opts.BasicAuth.User
	if opts.BasicAuth.Password != "" {
		s.User += ":" + opts.BasicAuth.Password
	}
	s.Opts = opts
	return nil
}
