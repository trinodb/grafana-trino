import { lastValueFrom, of } from 'rxjs';

import { dataFrameToJSON, DataSourceInstanceSettings, dateTime, MutableDataFrame } from '@grafana/data';
import {
  BackendSrv,
  DataSourceSrv,
  FetchResponse,
  setBackendSrv,
  setDataSourceSrv,
  TemplateSrv,
  setTemplateSrv,
} from '@grafana/runtime';

import { DataSource } from './datasource';
import { TrinoDataSourceOptions } from './types';

const mockBackend = { fetch: () => {} };
setBackendSrv(mockBackend as unknown as BackendSrv);
const mockTemplate = {
  replace: (target: any) => {
    return target;
  },
};
setTemplateSrv(mockTemplate as unknown as TemplateSrv);
const mockDataSource = {
  getInstanceSettings: () => ({ id: 8674 }),
};
setDataSourceSrv(mockDataSource as unknown as DataSourceSrv);

jest.mock('@grafana/runtime', () => ({
  ...(jest.requireActual('@grafana/runtime') as unknown as object),
  getBackendSrv: () => mockBackend,
  getTemplateSrv: () => mockTemplate,
  getDataSourceSrv: () => mockDataSource,
}));

describe('DataSource', () => {
  const fetchMock = jest.spyOn(mockBackend, 'fetch');
  const setupTestContext = (data: any) => {
    jest.clearAllMocks();
    fetchMock.mockImplementation(() => of(createFetchResponse(data)));
    const instanceSettings = {
      jsonData: {
        defaultProject: 'testproject',
      },
    } as unknown as DataSourceInstanceSettings<TrinoDataSourceOptions>;
    const ds = new DataSource(instanceSettings);

    return { ds };
  };

  // DataSourceWithBackend.query resolves the target data sources before it
  // calls the backend, so the response is observed by awaiting it instead of
  // with a marble test, whose virtual clock cannot drive that promise.
  const runQueryTest = async (args: { options: any; response: unknown; expected: unknown }) => {
    const { ds } = setupTestContext(args.response);
    expect(await lastValueFrom(ds.query(args.options))).toEqual(args.expected);
  };

  describe('When performing a time series query', () => {
    it('should transform response correctly', async () => {
      const options = {
        range: {
          from: dateTime(1432288354),
          to: dateTime(1432288401),
        },
        targets: [
          {
            format: 'time_series',
            rawQuery: true,
            rawSql: 'select time, metric from grafana_metric',
            refId: 'A',
            datasource: 'gdev-ds',
          },
        ],
      };
      const response = {
        results: {
          A: {
            refId: 'A',
            frames: [
              dataFrameToJSON(
                new MutableDataFrame({
                  fields: [
                    { name: 'time', values: [1599643351085] },
                    { name: 'metric', values: [30.226249741223704], labels: { metric: 'America' } },
                  ],
                  meta: {
                    executedQueryString: 'select time, metric from grafana_metric',
                  },
                })
              ),
            ],
          },
        },
      };

      await runQueryTest({
        options,
        response,
        expected: {
        data: [
          {
            fields: [
              {
                config: {},
                entities: {},
                name: 'time',
                type: 'time',
                values: [1599643351085],
              },
              {
                config: {},
                entities: {},
                labels: {
                  metric: 'America',
                },
                name: 'metric',
                type: 'number',
                values: [30.226249741223704],
              },
            ],
            length: 1,
            meta: {
              executedQueryString: 'select time, metric from grafana_metric',
            },
            name: undefined,
            refId: 'A',
          },
        ],
        state: 'Done',
        },
      });
    });
  });

  describe('When performing a table query', () => {
    it('should transform response correctly', async () => {
      const options = {
        range: {
          from: dateTime(1432288354),
          to: dateTime(1432288401),
        },
        targets: [
          {
            format: 'table',
            rawQuery: true,
            rawSql: 'select time, metric, value from grafana_metric',
            refId: 'A',
            datasource: 'gdev-ds',
          },
        ],
      };
      const response = {
        results: {
          A: {
            refId: 'A',
            frames: [
              dataFrameToJSON(
                new MutableDataFrame({
                  fields: [
                    { name: 'time', values: [1599643351085] },
                    { name: 'metric', values: ['America'] },
                    { name: 'value', values: [30.226249741223704] },
                  ],
                  meta: {
                    executedQueryString: 'select time, metric, value from grafana_metric',
                  },
                })
              ),
            ],
          },
        },
      };

      await runQueryTest({
        options,
        response,
        expected: {
        data: [
          {
            fields: [
              {
                config: {},
                entities: {},
                name: 'time',
                type: 'time',
                values: [1599643351085],
              },
              {
                config: {},
                entities: {},
                name: 'metric',
                type: 'string',
                values: ['America'],
              },
              {
                config: {},
                entities: {},
                name: 'value',
                type: 'number',
                values: [30.226249741223704],
              },
            ],
            length: 1,
            meta: {
              executedQueryString: 'select time, metric, value from grafana_metric',
            },
            name: undefined,
            refId: 'A',
          },
        ],
        state: 'Done',
        },
      });
    });
  });

  describe('When applying template variables', () => {
    it('should interpolate client tags as a plain comma-separated list', () => {
      const { ds } = setupTestContext({});
      const replaceSpy = jest.spyOn(mockTemplate, 'replace');

      const query = ds.applyTemplateVariables({ refId: 'A', rawSQL: 'SELECT 1', clientTags: '$cluster,adhoc' }, {});

      expect(query.clientTags).toBe('$cluster,adhoc');
      // 'csv' and no formatting function - client tags go into an HTTP header,
      // so they must not be quoted or escaped the way SQL literals are
      expect(replaceSpy).toHaveBeenCalledWith('$cluster,adhoc', {}, 'csv');
    });
  });
});

const createFetchResponse = <T>(data: T): FetchResponse<T> => ({
  data,
  status: 200,
  url: 'http://localhost:3000/api/query',
  config: { url: 'http://localhost:3000/api/query' },
  type: 'basic',
  statusText: 'Ok',
  redirected: false,
  headers: {} as unknown as Headers,
  ok: true,
});
