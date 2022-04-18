package trino

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v2"
	"github.com/pkg/errors"
	"github.com/starburstdata/grafana-trino/pkg/trino/driver"
	"github.com/starburstdata/grafana-trino/pkg/trino/models"
)

type TrinoDatasource struct {
	db *sql.DB
}

var (
	_ sqlds.Driver      = (*TrinoDatasource)(nil)
	_ sqlds.Completable = (*TrinoDatasource)(nil)
)

func New() *TrinoDatasource {
	return &TrinoDatasource{}
}

func (s *TrinoDatasource) FillMode() *data.FillMissing {
	return &data.FillMissing{
		Mode: data.FillModeNull,
	}
}

func (s *TrinoDatasource) Settings(config backend.DataSourceInstanceSettings) sqlds.DriverSettings {
	return sqlds.DriverSettings{}
}

// Connect opens a sql.DB connection using datasource settings
func (s *TrinoDatasource) Connect(config backend.DataSourceInstanceSettings, queryArgs json.RawMessage) (*sql.DB, error) {
	settings := models.TrinoDatasourceSettings{}
	err := settings.Load(config)
	if err != nil {
		return nil, fmt.Errorf("error reading settings: %s", err.Error())
	}

	db, err := driver.Open(settings)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to connect to database. Is the hostname and port correct?")
	}
	s.db = db

	return db, nil
}

func (s *TrinoDatasource) Converters() (sc []sqlutil.Converter) {
	return []sqlutil.Converter{
		NullStringConverter,
		NullDecimalConverter,
		NullInt64Converter,
		NullInt32Converter,
		NullTimeConverter,
		NullBoolConverter,
		NullDateConverter,
	}
}

func (s *TrinoDatasource) Schemas(ctx context.Context, options sqlds.Options) ([]string, error) {
	// TBD
	return []string{}, nil
}

func (s *TrinoDatasource) Tables(ctx context.Context, options sqlds.Options) ([]string, error) {
	// TBD
	return []string{}, nil
}

func (s *TrinoDatasource) Columns(ctx context.Context, options sqlds.Options) ([]string, error) {
	// TBD
	return []string{}, nil
}
