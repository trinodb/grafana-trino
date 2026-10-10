import { firstValueFrom, of } from 'rxjs';

import {
  DataFrame,
  dataFrameToJSON,
  DataQueryRequest,
  DataSourceInstanceSettings,
  FieldType,
  toDataFrame,
} from '@grafana/data';
import { BackendSrv, FetchResponse, setBackendSrv, setTemplateSrv, TemplateSrv } from '@grafana/runtime';

import { DataSource } from './datasource';
import { FormatOptions, TrinoDataSourceOptions, TrinoQuery, toVariableTrinoQuery } from './types';
import { toMetricFindFrame, toMetricFindValues, TrinoDataVariableSupport } from './variable';

const mockBackend = { fetch: (_options: any) => {} };
setBackendSrv(mockBackend as unknown as BackendSrv);
const mockTemplate = {
  replace: (target: any) => {
    return target;
  },
};
setTemplateSrv(mockTemplate as unknown as TemplateSrv);

jest.mock('@grafana/runtime', () => ({
  ...(jest.requireActual('@grafana/runtime') as unknown as object),
  getBackendSrv: () => mockBackend,
  getTemplateSrv: () => mockTemplate,
  getDataSourceSrv: () => ({ getInstanceSettings: () => ({ id: 8674 }) }),
}));

describe('toMetricFindValues', () => {
  it('converts non-string values of the first field to strings', () => {
    const frame = toDataFrame({
      fields: [
        { name: '_col0', type: FieldType.number, values: [1, 2, 2, null] },
        { name: 'ignored', type: FieldType.string, values: ['a', 'b', 'c', 'd'] },
      ],
    });

    expect(toMetricFindValues(frame)).toEqual([
      { text: '1', value: '1' },
      { text: '2', value: '2' },
    ]);
  });

  it('formats time values as ISO 8601 timestamps', () => {
    const frame = toDataFrame({
      fields: [{ name: '_col0', type: FieldType.time, values: [Date.UTC(2024, 0, 2, 3, 4, 5, 6)] }],
    });

    expect(toMetricFindValues(frame)).toEqual([
      { text: '2024-01-02T03:04:05.006Z', value: '2024-01-02T03:04:05.006Z' },
    ]);
  });

  it('converts boolean values to strings', () => {
    const frame = toDataFrame({
      fields: [{ name: '_col0', type: FieldType.boolean, values: [true, false] }],
    });

    expect(toMetricFindValues(frame)).toEqual([
      { text: 'true', value: 'true' },
      { text: 'false', value: 'false' },
    ]);
  });

  it('uses __text and __value fields when both are present', () => {
    const frame = toDataFrame({
      fields: [
        { name: '__value', type: FieldType.number, values: [1, 2] },
        { name: '__text', type: FieldType.string, values: ['one', 'two'] },
      ],
    });

    expect(toMetricFindValues(frame)).toEqual([
      { text: 'one', value: '1' },
      { text: 'two', value: '2' },
    ]);
  });

  it('returns no values for a frame without fields', () => {
    expect(toMetricFindValues(toDataFrame({ fields: [] }))).toEqual([]);
  });
});

describe('toMetricFindFrame', () => {
  it('exposes the values as string text and value fields', () => {
    const frame = toDataFrame({
      fields: [
        { name: '__text', type: FieldType.string, values: ['one'] },
        { name: '__value', type: FieldType.number, values: [1] },
      ],
    });

    const result = toMetricFindFrame(frame);

    expect(result.length).toBe(1);
    expect(result.fields.map(({ name, type, values }) => ({ name, type, values }))).toEqual([
      { name: 'text', type: FieldType.string, values: ['one'] },
      { name: 'value', type: FieldType.string, values: ['1'] },
    ]);
  });
});

describe('toVariableTrinoQuery', () => {
  it('converts a plain SQL string', () => {
    expect(toVariableTrinoQuery('SELECT 1')).toEqual({ refId: 'TrinoDataSource-QueryVariable', rawSQL: 'SELECT 1' });
  });

  it('converts a query saved by the standard variable support', () => {
    expect(toVariableTrinoQuery({ refId: 'A', query: 'SELECT 1' })).toEqual({ refId: 'A', rawSQL: 'SELECT 1' });
  });

  it('keeps a Trino query', () => {
    const query = { refId: 'A', rawSQL: 'SELECT 1', clientTags: 'tag' };

    expect(toVariableTrinoQuery(query)).toEqual(query);
  });
});

describe('TrinoDataVariableSupport', () => {
  it('runs the variable query as a table and returns string values', async () => {
    const response = {
      results: {
        A: {
          refId: 'A',
          frames: [
            dataFrameToJSON(
              toDataFrame({
                refId: 'A',
                fields: [{ name: '_col0', type: FieldType.number, values: [1] }],
              })
            ),
          ],
        },
      },
    };
    const fetchMock = jest.spyOn(mockBackend, 'fetch').mockImplementation(() => of(createFetchResponse(response)));
    const ds = new DataSource({ jsonData: {} } as unknown as DataSourceInstanceSettings<TrinoDataSourceOptions>);
    const request = {
      targets: [{ refId: 'A', query: 'SELECT 1' }],
    } as unknown as DataQueryRequest<TrinoQuery>;

    const result = await firstValueFrom(new TrinoDataVariableSupport(ds).query(request));

    expect(fetchMock.mock.calls[0][0].data.queries[0]).toMatchObject({
      refId: 'A',
      rawSQL: 'SELECT 1',
      format: FormatOptions.Table,
    });
    const frame: DataFrame = result.data[0];
    expect(frame.fields.map(({ name, values }) => ({ name, values }))).toEqual([
      { name: 'text', values: ['1'] },
      { name: 'value', values: ['1'] },
    ]);
  });
});

const createFetchResponse = <T>(data: T): FetchResponse<T> => ({
  data,
  status: 200,
  url: 'http://localhost:3000/api/ds/query',
  config: { url: 'http://localhost:3000/api/ds/query' },
  type: 'basic',
  statusText: 'Ok',
  redirected: false,
  headers: {} as unknown as Headers,
  ok: true,
});
