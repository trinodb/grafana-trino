import { of } from 'rxjs';
import { TestScheduler } from 'rxjs/testing';

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

import { DataSource } from '../datasource';
import { TrinoDataSourceOptions } from '../types';

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

  // https://rxjs-dev.firebaseapp.com/guide/testing/marble-testing
  const runMarbleTest = (args: {
    options: any;
    values: { [marble: string]: FetchResponse };
    marble: string;
    expectedValues: { [marble: string]: any };
    expectedMarble: string;
  }) => {
    const { expectedValues, expectedMarble, options, values, marble } = args;
    const scheduler: TestScheduler = new TestScheduler((actual, expected) => {
      expect(actual).toEqual(expected);
    });

    const { ds } = setupTestContext({});

    scheduler.run(({ cold, expectObservable }) => {
      const source = cold(marble, values);
      jest.clearAllMocks();
      fetchMock.mockImplementation(() => source);

      const result = ds.query(options);
      expectObservable(result).toBe(expectedMarble, expectedValues);
    });
  };

  describe('When performing a time series query', () => {
    it('should transform response correctly', () => {
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

      const values = { a: createFetchResponse(response) };
      const marble = '-a|';
      const expectedMarble = '-a|';
      const expectedValues = {
        a: {
          data: [
            {
              fields: [
                {
                  config: {},
                  entities: {},
                  name: 'time',
                  type: 'time',
                  values: {
                    buffer: [1599643351085],
                  },
                },
                {
                  config: {},
                  entities: {},
                  labels: {
                    metric: 'America',
                  },
                  name: 'metric',
                  type: 'number',
                  values: {
                    buffer: [30.226249741223704],
                  },
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
      };

      runMarbleTest({ options, marble, values, expectedMarble, expectedValues });
    });
  });

  describe('When performing a table query', () => {
    it('should transform response correctly', () => {
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

      const values = { a: createFetchResponse(response) };
      const marble = '-a|';
      const expectedMarble = '-a|';
      const expectedValues = {
        a: {
          data: [
            {
              fields: [
                {
                  config: {},
                  entities: {},
                  name: 'time',
                  type: 'time',
                  values: {
                    buffer: [1599643351085],
                  },
                },
                {
                  config: {},
                  entities: {},
                  name: 'metric',
                  type: 'string',
                  values: {
                    buffer: ['America'],
                  },
                },
                {
                  config: {},
                  entities: {},
                  name: 'value',
                  type: 'number',
                  values: {
                    buffer: [30.226249741223704],
                  },
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
      };

      runMarbleTest({ options, marble, values, expectedMarble, expectedValues });
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
