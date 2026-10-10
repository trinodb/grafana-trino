package trino

import (
	"context"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/sqlds/v4"
	trinoClient "github.com/trinodb/grafana-trino/pkg/trino/client"
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
		user, err := impersonatedUser(req.PluginContext.User, settings.ImpersonationIdentity)
		if err != nil {
			return errorForEachQuery(req, err), nil
		}

		if user == "" {
			log.DefaultLogger.FromContext(ctx).Info("Not impersonating anonymous Grafana user, query runs as the data source's Trino user")
		} else {
			ctx = context.WithValue(ctx, trinoUserHeader, user)
			ctx = trinoClient.WithSessionUser(ctx)
		}
	}

	if settings.ClientTags != "" {
		ctx = context.WithValue(ctx, trinoClientTagsKey, settings.ClientTags)
	}

	response, err := ds.SQLDatasource.QueryData(ctx, req)
	if response == nil {
		return response, err
	}
	trimResponseEdges(req.Queries, response)
	return response, err
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

func clientTagsFromContext(ctx context.Context) string {
	tags, _ := ctx.Value(trinoClientTagsKey).(string)
	return tags
}

func impersonatedUser(user *backend.User, identity string) (string, error) {
	if user == nil {
		return "", fmt.Errorf("user can't be nil if impersonation is enabled")
	}
	if user.Login == "" && user.Email == "" {
		return "", nil
	}
	if identity == models.ImpersonationIdentityEmail {
		if user.Email == "" {
			return "", fmt.Errorf("impersonation is configured to use the user's email, but Grafana user %q has no email", user.Login)
		}
		return user.Email, nil
	}
	if user.Login == "" {
		return "", fmt.Errorf("impersonation is configured to use the user's login, but Grafana user %q has no login", user.Email)
	}
	return user.Login, nil
}

func errorForEachQuery(req *backend.QueryDataRequest, err error) *backend.QueryDataResponse {
	resp := backend.NewQueryDataResponse()
	for _, q := range req.Queries {
		resp.Responses[q.RefID] = backend.ErrDataResponseWithSource(backend.StatusBadRequest, backend.ErrorSourceDownstream, err.Error())
	}
	return resp
}
