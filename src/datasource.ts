import { DataSourceInstanceSettings } from '@grafana/data';
import { DataSourceWithBackend, getBackendSrv, toDataQueryError } from '@grafana/runtime';
import { TrinoDataSourceOptions, TrinoQuery } from './types';
import { lastValueFrom, of } from 'rxjs';
import { catchError, mapTo } from 'rxjs/operators';

export class DataSource extends DataSourceWithBackend<TrinoQuery, TrinoDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<TrinoDataSourceOptions>) {
    super(instanceSettings);
  }

  testDatasource(): Promise<any> {
    return lastValueFrom(
      getBackendSrv()
        .fetch({
          url: '/api/ds/query',
          method: 'POST',
          requestId: 'A',
          data: {
            from: '5m',
            to: 'now',
            queries: [
              {
                refId: 'A',
                key: 'A',
                intervalMs: 1,
                maxDataPoints: 1,
                datasource: this.getRef(),
                rawSQL: 'SELECT 1',
                format: 0,
              },
            ],
          },
        })
        .pipe(
          mapTo({ status: 'success', message: 'Database Connection OK' }),
          catchError((err) => {
            return of(toDataQueryError(err));
          })
        )
    );
  }
}
