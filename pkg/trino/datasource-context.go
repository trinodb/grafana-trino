package trino

import (
	"context"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/sqlds/v3"
	"github.com/trinodb/grafana-trino/pkg/trino/models"
)

const (
	accessTokenKey     = "accessToken"
	trinoUserHeader    = "X-Trino-User"
	trinoClientTagsKey = "X-Trino-Client-Tags"
	bearerPrefix       = "Bearer "
)

type SQLDatasourceWithTrinoUserContext struct {
	sqlds.SQLDatasource
}

func (ds *SQLDatasourceWithTrinoUserContext) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	config := req.PluginContext.DataSourceInstanceSettings
	settings := models.TrinoDatasourceSettings{}
	err := settings.Load(ctx, *config)
	if err != nil {
		return nil, fmt.Errorf("error reading settings: %s", err.Error())
	}

	ctx = injectAccessToken(ctx, req)

	if settings.EnableImpersonation {
		user := req.PluginContext.User
		if user == nil {
			return nil, fmt.Errorf("user can't be nil if impersonation is enabled")
		}

		ctx = context.WithValue(ctx, trinoUserHeader, user)
	}

	if settings.ClientTags != "" {
		ctx = context.WithValue(ctx, trinoClientTagsKey, settings.ClientTags)
	}

	return ds.SQLDatasource.QueryData(ctx, req)
}

func (ds *SQLDatasourceWithTrinoUserContext) NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	_, err := ds.SQLDatasource.NewDatasource(ctx, settings)
	if err != nil {
		return nil, err
	}
	return ds, nil
}

func NewDatasource(c sqlds.Driver) *SQLDatasourceWithTrinoUserContext {
	base := sqlds.NewDatasource(c)
	return &SQLDatasourceWithTrinoUserContext{*base}
}

func injectAccessToken(ctx context.Context, req *backend.QueryDataRequest) context.Context {
	header := req.GetHTTPHeader(backend.OAuthIdentityTokenHeaderName)

	if strings.HasPrefix(header, bearerPrefix) {
		token := strings.TrimPrefix(header, bearerPrefix)
		return context.WithValue(ctx, accessTokenKey, token)
	}

	return ctx
}
