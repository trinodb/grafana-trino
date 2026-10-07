package trino

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/sqlds/v4"
	"github.com/trinodb/grafana-trino/pkg/trino/async"
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

	async *async.Registry
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

	if settings.EnableAsyncQueryData && async.IsAsyncRequest(req) {
		return ds.queryDataAsync(ctx, req), nil
	}

	return ds.SQLDatasource.QueryData(ctx, req)
}

// queryDataAsync answers one round of the polling flow: it either starts each
// query and hands back a handle, or reports on a query already running. Both
// are cheap enough to do inline - unlike the synchronous path, there is
// nothing here worth fanning out across goroutines.
func (ds *SQLDatasourceWithTrinoUserContext) queryDataAsync(ctx context.Context, req *backend.QueryDataRequest) *backend.QueryDataResponse {
	response := backend.NewQueryDataResponse()
	for _, query := range req.Queries {
		response.Responses[query.RefID] = ds.handleAsyncQuery(ctx, req, query)
	}
	return response
}

func (ds *SQLDatasourceWithTrinoUserContext) handleAsyncQuery(ctx context.Context, req *backend.QueryDataRequest, query backend.DataQuery) backend.DataResponse {
	q, err := async.ParseQuery(query)
	if err != nil {
		return backend.ErrorResponseWithErrorSource(backend.PluginError(err))
	}

	if q.QueryID == "" {
		queryID, err := ds.async.Start(ctx, func(runCtx context.Context) (data.Frames, error) {
			return ds.runSingleQuery(runCtx, req, query)
		})
		if err != nil {
			return backend.ErrorResponseWithErrorSource(backend.PluginError(err))
		}
		return backend.DataResponse{Frames: async.StatusFrames(query.RefID, queryID, async.StatusSubmitted)}
	}

	status, err := ds.async.Status(q.QueryID)
	if err != nil {
		return backend.ErrorResponseWithErrorSource(backend.DownstreamError(err))
	}
	if status.Running() {
		return backend.DataResponse{Frames: async.StatusFrames(query.RefID, q.QueryID, status)}
	}

	frames, err := ds.async.Collect(q.QueryID)
	if err != nil {
		return backend.ErrorResponseWithErrorSource(err)
	}
	return backend.DataResponse{Frames: async.WithMeta(frames, q.QueryID, status)}
}

// runSingleQuery re-enters the synchronous path with a request holding just
// this query, so macros, converters, fill mode, row limits and retries behave
// identically in both flows.
func (ds *SQLDatasourceWithTrinoUserContext) runSingleQuery(ctx context.Context, req *backend.QueryDataRequest, query backend.DataQuery) (data.Frames, error) {
	res, err := ds.SQLDatasource.QueryData(ctx, &backend.QueryDataRequest{
		PluginContext: req.PluginContext,
		Headers:       req.Headers,
		Queries:       []backend.DataQuery{query},
	})
	if err != nil {
		return nil, err
	}
	response, ok := res.Responses[query.RefID]
	if !ok {
		return nil, fmt.Errorf("no response for query %s", query.RefID)
	}
	return response.Frames, response.Error
}

// handleCancel backs the resource the frontend calls when a panel is closed or
// re-queried while a query is still running.
func (ds *SQLDatasourceWithTrinoUserContext) handleCancel(rw http.ResponseWriter, req *http.Request) {
	// The body shape is set by the frontend library: {"queryId": "..."}.
	body := struct {
		QueryID string `json:"queryId"`
	}{}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	if body.QueryID == "" {
		http.Error(rw, "queryId is required", http.StatusBadRequest)
		return
	}
	if err := ds.async.Cancel(body.QueryID); err != nil {
		// A query that already finished or was reaped is not worth an error
		// at the client: there is nothing left to cancel either way.
		backend.Logger.Debug("could not cancel query", "queryID", body.QueryID, "error", err)
	}
	rw.WriteHeader(http.StatusOK)
}

// Dispose stops the async queries this instance is running. sqlds calls it
// when the data source is updated or removed.
func (ds *SQLDatasourceWithTrinoUserContext) Dispose() {
	ds.async.Close()
	ds.SQLDatasource.Dispose()
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
	ds := &SQLDatasourceWithTrinoUserContext{SQLDatasource: *base, async: async.NewRegistry()}
	// sqlds reads CustomRoutes while building the resource handler in
	// NewDatasource, so the route has to be registered before then.
	ds.CustomRoutes = map[string]func(http.ResponseWriter, *http.Request){
		"/cancel": ds.handleCancel,
	}
	return ds
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
