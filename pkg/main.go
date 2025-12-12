package main

import (
	"context"
	"os"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/trinodb/grafana-trino/pkg/trino"
)

func main() {
	// Start listening to requests sent from Grafana. This call is blocking so
	// it won't finish until Grafana shuts down the process or the plugin choose
	// to exit by itself using os.Exit. Manage automatically manages life cycle
	// of datasource instances. It accepts datasource instance factory as first
	// argument. This factory will be automatically called on incoming request
	// from Grafana to create different instances of TrinoDatasource (per datasource
	// ID). When datasource configuration changed Dispose method will be called and
	// new datasource instance created using New factory.
	s := &trino.TrinoDatasource{}
	ds := trino.NewDatasource(s)
	ds.Completable = s
	dsInstanceFactory := func(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		return ds.NewDatasource(ctx, settings)
	}
	if err := datasource.Manage("trino-datasource", dsInstanceFactory, datasource.ManageOpts{}); err != nil {
		log.DefaultLogger.Error(err.Error())
		os.Exit(1)
	}
}
