package trino

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

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
		user, err := impersonatedUser(req.PluginContext.User, settings.ImpersonationIdentity, settings.ImpersonationStripSuffix)
		if err != nil {
			return errorForEachQuery(req, err), nil
		}

		if user == "" {
			log.DefaultLogger.FromContext(ctx).Debug("Not impersonating anonymous Grafana user, query runs as the data source's Trino user")
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

	response, err := ds.SQLDatasource.QueryData(ctx, req)
	if response == nil {
		return response, err
	}
	trimResponseEdges(req.Queries, response)
	return response, err
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

	user := req.PluginContext.User
	if q.QueryID == "" {
		queryID, err := ds.async.Start(ctx, user, func(runCtx context.Context) (data.Frames, error) {
			return ds.runSingleQuery(runCtx, req, query)
		})
		if err != nil {
			return backend.ErrorResponseWithErrorSource(backend.PluginError(err))
		}
		return backend.DataResponse{Frames: async.StatusFrames(query.RefID, queryID, async.StatusSubmitted)}
	}

	status, err := ds.async.Status(q.QueryID, user)
	if err != nil {
		return asyncCallError(err)
	}
	if status.Running() {
		return backend.DataResponse{Frames: async.StatusFrames(query.RefID, q.QueryID, status)}
	}

	frames, err := ds.async.Collect(q.QueryID, user)
	if err != nil {
		if errors.Is(err, async.ErrWrongUser) {
			return backend.ErrorResponseWithErrorSource(backend.PluginError(err))
		}
		// The query already ran. Its error is returned as the registry stored
		// it, including a failure that came back from Trino.
		return backend.ErrorResponseWithErrorSource(err)
	}
	return backend.DataResponse{Frames: async.WithMeta(frames, q.QueryID, status)}
}

// runSingleQuery re-enters the synchronous path with a request holding just
// this query, so macros, converters, fill mode, row limits and retries behave
// identically in both flows. Edge trimming is applied here for the same
// reason: it lives in the wrapper rather than in sqlds, so re-entering
// sqlds alone would skip it and leave async results untrimmed.
func (ds *SQLDatasourceWithTrinoUserContext) runSingleQuery(ctx context.Context, req *backend.QueryDataRequest, query backend.DataQuery) (data.Frames, error) {
	queries := []backend.DataQuery{query}
	res, err := ds.SQLDatasource.QueryData(ctx, &backend.QueryDataRequest{
		PluginContext: req.PluginContext,
		Headers:       req.Headers,
		Queries:       queries,
	})
	if err != nil {
		return nil, err
	}
	trimResponseEdges(queries, res)
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
	if err := ds.async.Cancel(body.QueryID, backend.UserFromContext(req.Context())); err != nil {
		if errors.Is(err, async.ErrWrongUser) {
			http.Error(rw, err.Error(), http.StatusForbidden)
			return
		}
		// A query that already finished or was reaped is not worth an error
		// at the client: there is nothing left to cancel either way.
		backend.Logger.Debug("could not cancel query", "queryID", body.QueryID, "error", err)
	}
	rw.WriteHeader(http.StatusOK)
}

// asyncCallError reports a registry failure. A handle presented by someone
// other than the user who started the query is the plugin refusing the call.
// Anything else is a problem talking to the query that is already running.
func asyncCallError(err error) backend.DataResponse {
	if errors.Is(err, async.ErrWrongUser) {
		return backend.ErrorResponseWithErrorSource(backend.PluginError(err))
	}
	return backend.ErrorResponseWithErrorSource(backend.DownstreamError(err))
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
	// Makes sqlds apply the row limit returned by TrinoDatasource.Settings.
	base.EnableRowLimit = true
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
		return trinoClient.WithForwardedToken(ctx, token)
	}

	return ctx
}

func clientTagsFromContext(ctx context.Context) string {
	tags, _ := ctx.Value(trinoClientTagsKey).(string)
	return tags
}

func impersonatedUser(user *backend.User, identity, stripSuffix string) (string, error) {
	if user == nil {
		return "", fmt.Errorf("user can't be nil if impersonation is enabled")
	}
	if user.Login == "" && user.Email == "" {
		return "", nil
	}
	name := user.Login
	if identity == models.ImpersonationIdentityEmail {
		if user.Email == "" {
			return "", fmt.Errorf("impersonation is configured to use the user's email, but Grafana user %q has no email", user.Login)
		}
		name = user.Email
	} else if user.Login == "" {
		return "", fmt.Errorf("impersonation is configured to use the user's login, but Grafana user %q has no login", user.Email)
	}
	name = trimSuffixFold(name, stripSuffix)
	if name == "" {
		return "", fmt.Errorf("removing the impersonation username suffix leaves an empty user; change Remove username suffix in the data source settings")
	}
	return name, nil
}

func trimSuffixFold(name, suffix string) string {
	// Case-equivalent runes can have different UTF-8 widths. Find the suffix
	// boundary by rune count so the retained prefix keeps its original spelling.
	start := len(name)
	for range suffix {
		if start == 0 {
			return name
		}
		_, size := utf8.DecodeLastRuneInString(name[:start])
		start -= size
	}
	if strings.EqualFold(name[start:], suffix) {
		return name[:start]
	}
	return name
}

func errorForEachQuery(req *backend.QueryDataRequest, err error) *backend.QueryDataResponse {
	resp := backend.NewQueryDataResponse()
	for _, q := range req.Queries {
		resp.Responses[q.RefID] = backend.ErrDataResponseWithSource(backend.StatusBadRequest, backend.ErrorSourceDownstream, err.Error())
	}
	return resp
}
