package driver

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/starburstdata/grafana-trino/pkg/trino/models"
	"github.com/trinodb/trino-go-client/trino"
	_ "github.com/trinodb/trino-go-client/trino"
)

const DriverName string = "trino"

// Open registers a new driver with a unique name
func Open(settings models.TrinoDatasourceSettings) (*sql.DB, error) {
	serverURL, err := url.Parse(settings.URL)
	if err != nil {
		return nil, err
	}

	grafana := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: settings.Opts.TLS.InsecureSkipVerify,
			},
		},
	}
	trino.RegisterCustomClient("grafana", grafana)
	config := trino.Config{
		ServerURI:        fmt.Sprintf("%s://%s@%s", serverURL.Scheme, settings.User, serverURL.Host),
		Source:           "trino-grafana",
		CustomClientName: "grafana",
	}

	dsn, err := config.FormatDSN()
	log.DefaultLogger.Info("Connecting to Trino", "dsn", dsn)
	if err != nil {
		return nil, err
	}
	return sql.Open(DriverName, dsn)
}
