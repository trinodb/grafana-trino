package trino

import (
	"context"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/sqlds/v2"
)

type SQLDatasourceWithPluginContext struct {
	sqlds.SQLDatasource
}

func (ds *SQLDatasourceWithPluginContext) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	ctx = context.WithValue(ctx, "plugin-context", req.PluginContext)
	return ds.SQLDatasource.QueryData(ctx, req)
}

func (ds *SQLDatasourceWithPluginContext) NewDatasource(settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	_, err := ds.SQLDatasource.NewDatasource(settings)
	if err != nil {
		return nil, err
	}
	return ds, nil
}

func NewDatasource(c sqlds.Driver) *SQLDatasourceWithPluginContext {
	base := sqlds.NewDatasource(c)
	return &SQLDatasourceWithPluginContext{*base}
}
