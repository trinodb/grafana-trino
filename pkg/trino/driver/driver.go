package driver

import (
	"crypto/tls"
	"database/sql"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/trinodb/grafana-trino/pkg/trino/models"
	"github.com/trinodb/trino-go-client/trino"
	_ "github.com/trinodb/trino-go-client/trino"
)

const DriverName string = "trino"

// Open registers a new driver with a unique name
func Open(settings models.TrinoDatasourceSettings) (*sql.DB, error) {
	skipVerify := false
	sslCert := ""
	if settings.Opts.TLS != nil {
		skipVerify = settings.Opts.TLS.InsecureSkipVerify
		sslCert = settings.Opts.TLS.CACertificate
	}
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipVerify,
			},
		},
	}
	err := trino.RegisterCustomClient("grafana", client)
	if err != nil {
		return nil, err
	}
	config := trino.Config{
		ServerURI:        settings.URL.String(),
		Source:           "grafana",
		CustomClientName: "grafana",
		SSLCert:          sslCert,
	}

	dsn, err := config.FormatDSN()
	log.DefaultLogger.Info("Connecting to Trino", "dsn", dsn)
	if err != nil {
		return nil, err
	}
	return sql.Open(DriverName, dsn)
}
