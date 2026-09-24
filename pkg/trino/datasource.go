package trino

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v4"
	"github.com/trinodb/grafana-trino/pkg/trino/driver"
	"github.com/trinodb/grafana-trino/pkg/trino/models"
)

type TrinoDatasource struct {
	db *sql.DB
}

var (
	_ sqlds.Driver          = (*TrinoDatasource)(nil)
	_ sqlds.QueryMutator    = (*TrinoDatasource)(nil)
	_ sqlds.QueryArgSetter  = (*TrinoDatasource)(nil)
	_ sqlds.Completable     = (*TrinoDatasource)(nil)
	_ sqlds.ResponseMutator = (*TrinoDatasource)(nil)
)

func New() *TrinoDatasource {
	return &TrinoDatasource{}
}

func (s *TrinoDatasource) Settings(ctx context.Context, config backend.DataSourceInstanceSettings) sqlds.DriverSettings {
	return sqlds.DriverSettings{
		FillMode: &data.FillMissing{
			Mode: data.FillModeNull,
		},
	}
}

// Connect opens a sql.DB connection using datasource settings
func (s *TrinoDatasource) Connect(ctx context.Context, config backend.DataSourceInstanceSettings, queryArgs json.RawMessage) (*sql.DB, error) {
	settings := models.TrinoDatasourceSettings{}
	err := settings.Load(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("error reading settings: %w", err)
	}

	db, err := driver.Open(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database. Is the hostname and port correct?: %w", err)
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
		newComplexTypeConverter(),
		nullStringConverter,
		nullDecimalConverter,
		nullInt64Converter,
		nullTimeConverter,
		nullBoolConverter,
	}
}

// MutateQuery combines the client tags set on the query with the ones
// configured on the data source. sqlds calls this once per query, in that
// query's own goroutine, and passes the returned context to SetQueryArgs, so
// tags set on one query never reach the other queries of the same panel.
//
// The data source tags are always kept: they are set by an administrator,
// while queries carry whatever the user running them sends.
func (s *TrinoDatasource) MutateQuery(ctx context.Context, req backend.DataQuery) (context.Context, backend.DataQuery) {
	var query struct {
		ClientTags string `json:"clientTags"`
	}
	if err := json.Unmarshal(req.JSON, &query); err != nil {
		return ctx, req
	}

	tags := mergeClientTags(clientTagsFromContext(ctx), query.ClientTags)
	if tags == "" {
		return ctx, req
	}

	return context.WithValue(ctx, trinoClientTagsKey, tags), req
}

func (s *TrinoDatasource) MutateResponse(ctx context.Context, frames data.Frames) (data.Frames, error) {
	enableJSONCellInspect(frames)
	return frames, nil
}

func (s *TrinoDatasource) SetQueryArgs(ctx context.Context, headers http.Header) []interface{} {
	var args []interface{}

	user := ctx.Value(trinoUserHeader)
	accessToken := ctx.Value(accessTokenKey)
	clientTags := clientTagsFromContext(ctx)

	if user != nil {
		args = append(args, sql.Named(trinoUserHeader, user.(string)))
	}

	if accessToken != nil {
		args = append(args, sql.Named(accessTokenKey, accessToken.(string)))
	}

	if clientTags != "" {
		args = append(args, sql.Named(trinoClientTagsKey, clientTags))
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

// mergeClientTags parses comma-separated client tag lists and joins them back
// into one, dropping blanks and duplicates but keeping the order the tags were
// given in.
func mergeClientTags(tagLists ...string) string {
	tags := []string{}
	seen := map[string]struct{}{}
	for _, tagList := range tagLists {
		for _, tag := range strings.Split(tagList, ",") {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			if _, duplicate := seen[tag]; duplicate {
				continue
			}
			seen[tag] = struct{}{}
			tags = append(tags, tag)
		}
	}
	return strings.Join(tags, ",")
}
