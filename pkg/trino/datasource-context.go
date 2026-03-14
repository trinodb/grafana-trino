package trino

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/sqlds/v2"
	"github.com/trinodb/grafana-trino/pkg/trino/models"
)

const (
	accessTokenKey     = "accessToken"
	trinoUserHeader    = "X-Trino-User"
	trinoClientTagsKey = "X-Trino-Client-Tags"
	bearerPrefix       = "Bearer "
)

type activeQuery struct {
	id     uint64
	cancel context.CancelFunc
}

type SQLDatasourceWithTrinoUserContext struct {
	sqlds.SQLDatasource

	mu            sync.Mutex
	activeQueries map[string]activeQuery
	queryCounter  atomic.Uint64
}

func (ds *SQLDatasourceWithTrinoUserContext) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	config := req.PluginContext.DataSourceInstanceSettings
	settings := models.TrinoDatasourceSettings{}
	err := settings.Load(*config)
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

	// Create a cancellable context so we can cancel running queries
	// when a new request arrives for the same panel (e.g. user changes
	// filters or presses cancel). The cancel propagates through
	// database/sql to the Trino driver, which sends a DELETE request
	// to terminate the query on the Trino server.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	queryID := ds.queryCounter.Add(1)

	// Cancel any in-flight query for the same panel/refID.
	for _, q := range req.Queries {
		key := panelKey(req, q.RefID)
		ds.mu.Lock()
		if prev, ok := ds.activeQueries[key]; ok {
			log.DefaultLogger.Debug("Cancelling previous query", "refId", q.RefID)
			prev.cancel()
		}
		ds.activeQueries[key] = activeQuery{id: queryID, cancel: cancel}
		ds.mu.Unlock()
	}

	defer func() {
		for _, q := range req.Queries {
			key := panelKey(req, q.RefID)
			ds.mu.Lock()
			if aq, ok := ds.activeQueries[key]; ok && aq.id == queryID {
				delete(ds.activeQueries, key)
			}
			ds.mu.Unlock()
		}
	}()

	return ds.SQLDatasource.QueryData(ctx, req)
}

// panelKey builds a unique key for a query within a datasource.
func panelKey(req *backend.QueryDataRequest, refID string) string {
	dsUID := ""
	if req.PluginContext.DataSourceInstanceSettings != nil {
		dsUID = req.PluginContext.DataSourceInstanceSettings.UID
	}
	return dsUID + "/" + refID
}

func (ds *SQLDatasourceWithTrinoUserContext) NewDatasource(settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	_, err := ds.SQLDatasource.NewDatasource(settings)
	if err != nil {
		return nil, err
	}
	return ds, nil
}

func NewDatasource(c sqlds.Driver) *SQLDatasourceWithTrinoUserContext {
	base := sqlds.NewDatasource(c)
	return &SQLDatasourceWithTrinoUserContext{
		SQLDatasource: *base,
		activeQueries: make(map[string]activeQuery),
	}
}

func injectAccessToken(ctx context.Context, req *backend.QueryDataRequest) context.Context {
	header := req.GetHTTPHeader(backend.OAuthIdentityTokenHeaderName)

	if strings.HasPrefix(header, bearerPrefix) {
		token := strings.TrimPrefix(header, bearerPrefix)
		return context.WithValue(ctx, accessTokenKey, token)
	}

	return ctx
}
