package trino

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v2"
	"github.com/pkg/errors"
	"github.com/trinodb/grafana-trino/pkg/trino/driver"
	"github.com/trinodb/grafana-trino/pkg/trino/models"
)

type TrinoDatasource struct {
	db *sql.DB
}

var (
	_ sqlds.Driver         = (*TrinoDatasource)(nil)
	_ sqlds.QueryArgSetter = (*TrinoDatasource)(nil)
	_ sqlds.Completable    = (*TrinoDatasource)(nil)
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

func (s *TrinoDatasource) SetQueryArgs(ctx context.Context, headers http.Header) []interface{} {
	var args []interface{}

	user := ctx.Value(trinoUserHeader)
	accessToken := ctx.Value(accessTokenKey)
	clientTags := ctx.Value(trinoClientTagsKey)

	if user != nil {
		args = append(args, sql.Named(trinoUserHeader, string(user.(*backend.User).Login)))
	}

	if accessToken != nil {
		args = append(args, sql.Named(accessTokenKey, accessToken.(string)))
	}

	if clientTags != nil {
		args = append(args, sql.Named(trinoClientTagsKey, clientTags.(string)))
	}

	return args
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
