package trino

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"

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
	nullStringConverter := sqlutil.NullStringConverter
	nullStringConverter.InputTypeRegex = regexp.MustCompile("char|varchar|varbinary|json|interval year to month|interval day to second|decimal|ipaddress|unknown")
	nullDecimalConverter := sqlutil.NullDecimalConverter
	nullDecimalConverter.InputTypeRegex = regexp.MustCompile("real|double")
	nullInt64Converter := sqlutil.NullInt64Converter
	nullInt64Converter.InputTypeRegex = regexp.MustCompile("tinyint|smallint|integer|bigint")
	nullTimeConverter := sqlutil.NullTimeConverter
	nullTimeConverter.InputTypeRegex = regexp.MustCompile("date|time|time with time zone|timestamp|timestamp with time zone")
	nullBoolConverter := sqlutil.NullBoolConverter
	nullBoolConverter.InputTypeName = "boolean"
	return []sqlutil.Converter{
		nullStringConverter,
		nullDecimalConverter,
		nullInt64Converter,
		nullTimeConverter,
		nullBoolConverter,
	}
}

func (s *TrinoDatasource) Schemas(ctx context.Context, options sqlds.Options) ([]string, error) {
	query := "SHOW SCHEMAS"
	args := []string{}
	if options["database"] != "" {
		query += " FOR ?"
		args = append(args, options["database"])
	}
	rows, err := s.db.Query(query, args)
	if err != nil {
		return nil, err
	}
	return getNames(rows)
}

func (s *TrinoDatasource) Tables(ctx context.Context, options sqlds.Options) ([]string, error) {
	query := "SHOW TABLES"
	args := []string{}
	if options["schema"] != "" {
		query += " FOR ?"
		args = append(args, options["schema"])
	}
	rows, err := s.db.Query(query, args)
	if err != nil {
		return nil, err
	}
	return getNames(rows)
}

func (s *TrinoDatasource) Columns(ctx context.Context, options sqlds.Options) ([]string, error) {
	query := "SHOW COLUMNS"
	args := []string{}
	if options["table"] != "" {
		query += " FOR ?"
		args = append(args, options["table"])
	}
	rows, err := s.db.Query(query, args)
	if err != nil {
		return nil, err
	}
	return getNames(rows)
}

func getNames(rows *sql.Rows) ([]string, error) {
	results := []string{}
	name := ""
	for rows.Next() {
		err := rows.Scan(&name)
		if err != nil {
			return nil, err
		}
		results = append(results, name)
	}
	return results, nil
}
