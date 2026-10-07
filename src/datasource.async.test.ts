import { of } from 'rxjs';
import { DataQueryRequest, DataSourceInstanceSettings, LoadingState } from '@grafana/data';

import { TrinoDataSourceOptions, TrinoQuery } from './types';

// Mocked without requireActual so the suite does not pull in @grafana/ui, and
// so the base class query can be observed directly.
const baseQuery = jest.fn((_request: DataQueryRequest<TrinoQuery>) => of({ data: [] }));
const postResource = jest.fn((_path: string, _body: unknown) => Promise.resolve({}));

jest.mock('@grafana/runtime', () => ({
  // Methods live on the prototype, not as class fields: a field would be
  // assigned onto the instance during construction and shadow the subclass
  // override, so the async path would never run.
  DataSourceWithBackend: class {
    id = 1;
    constructor(public instanceSettings: unknown) {}
    query(request: unknown) {
      return baseQuery(request as never);
    }
    postResource(path: string, body: unknown) {
      return postResource(path as never, body as never);
    }
    getRef() {
      return { type: 'trino-datasource', uid: 'uid' };
    }
    applyTemplateVariables(query: unknown) {
      return query;
    }
  },
  getTemplateSrv: () => ({ replace: (target: string) => target }),
  // @grafana/async-query-data reads both of these while deciding how to skip
  // the query cache, so they have to be present even though the values only
  // need to be plausible.
  config: { featureToggles: {}, buildInfo: { version: '10.4.0' } },
}));

// Imported after the mock is declared for readability only; jest hoists
// jest.mock above the imports either way.
import { DataSource } from './datasource';

const settings = (enableAsyncQueryData?: boolean) =>
  ({
    uid: 'trino-uid',
    id: 1,
    jsonData: enableAsyncQueryData === undefined ? {} : { enableAsyncQueryData },
  }) as unknown as DataSourceInstanceSettings<TrinoDataSourceOptions>;

const request = () =>
  ({
    targets: [{ refId: 'A', rawSQL: 'SELECT 1' }],
    scopedVars: {},
  }) as unknown as DataQueryRequest<TrinoQuery>;

describe('DataSource async query flow', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('marks queries as async when the data source opts in', () => {
    new DataSource(settings(true)).query(request()).subscribe();

    expect(baseQuery).toHaveBeenCalled();
    const sent = baseQuery.mock.calls[0][0] as unknown as DataQueryRequest<TrinoQuery>;
    expect(sent.targets[0]).toMatchObject({ meta: { queryFlow: 'async' } });
  });

  it('leaves the query untouched when the setting is off', () => {
    new DataSource(settings(false)).query(request()).subscribe();

    expect(baseQuery).toHaveBeenCalled();
    const sent = baseQuery.mock.calls[0][0] as unknown as DataQueryRequest<TrinoQuery>;
    expect(sent.targets[0]).not.toHaveProperty('meta.queryFlow');
  });

  it('defaults to the synchronous flow when the setting is absent', () => {
    new DataSource(settings()).query(request()).subscribe();

    const sent = baseQuery.mock.calls[0][0] as unknown as DataQueryRequest<TrinoQuery>;
    expect(sent.targets[0]).not.toHaveProperty('meta.queryFlow');
  });

  it('keeps polling while the backend reports the query as running', () => {
    jest.useFakeTimers();
    const running = {
      data: [{ fields: [], meta: { custom: { queryID: 'proc:abc', status: 'running' } } }],
    };
    const finished = {
      data: [
        {
          fields: [{ name: 'n', values: [1] }],
          length: 1,
          meta: { custom: { queryID: 'proc:abc', status: 'finished' } },
        },
      ],
    };
    baseQuery.mockReturnValueOnce(of(running) as never).mockReturnValueOnce(of(finished) as never);

    const next = jest.fn();
    new DataSource(settings(true)).query(request()).subscribe({ next });

    // The second request only goes out after the looper's backoff elapses.
    expect(baseQuery).toHaveBeenCalledTimes(1);
    jest.runOnlyPendingTimers();
    expect(baseQuery).toHaveBeenCalledTimes(2);

    // The poll carries the handle the first response returned, which is what
    // lets the backend find the running query instead of starting a new one.
    const polled = baseQuery.mock.calls[1][0] as unknown as DataQueryRequest<TrinoQuery>;
    expect(polled.targets[0]).toMatchObject({ queryID: 'proc:abc' });

    jest.useRealTimers();
  });

  it('bypasses the query cache, since a handle is single-use', () => {
    new DataSource(settings(true)).query(request()).subscribe();

    const sent = baseQuery.mock.calls[0][0] as unknown as DataQueryRequest<TrinoQuery> & {
      skipQueryCache?: boolean;
    };
    expect(sent.skipQueryCache).toBe(true);
  });

  it('cancels the running query when the panel unsubscribes mid-query', () => {
    jest.useFakeTimers();
    const running = {
      data: [{ fields: [], meta: { custom: { queryID: 'proc:abc', status: 'running' } } }],
    };
    baseQuery.mockReturnValue(of(running) as never);

    const subscription = new DataSource(settings(true)).query(request()).subscribe();
    expect(baseQuery).toHaveBeenCalledTimes(1);

    subscription.unsubscribe();

    expect(postResource).toHaveBeenCalledWith('cancel', { queryId: 'proc:abc' });

    jest.useRealTimers();
  });

  it('stops polling once the backend stops recognising the handle', () => {
    // Tearing a panel down leaves polls queued: @grafana/async-query-data
    // completes its observer from the cleanup path, which schedules the next
    // request before the cancel goes out. What bounds them is the backend
    // answering a handle it has already released with an error instead of
    // starting a fresh query, and an error response ends the loop.
    jest.useFakeTimers();
    const running = {
      data: [{ fields: [], meta: { custom: { queryID: 'proc:abc', status: 'running' } } }],
    };
    const released = { data: [], state: LoadingState.Error };
    baseQuery.mockReturnValueOnce(of(running) as never).mockReturnValue(of(released) as never);

    const subscription = new DataSource(settings(true)).query(request()).subscribe();
    subscription.unsubscribe();

    jest.runOnlyPendingTimers();
    const afterTeardown = baseQuery.mock.calls.length;
    jest.runOnlyPendingTimers();
    expect(baseQuery).toHaveBeenCalledTimes(afterTeardown);

    jest.useRealTimers();
  });

  it('stops polling once the query is finished', () => {
    jest.useFakeTimers();
    const finished = {
      data: [
        {
          fields: [{ name: 'n', values: [1] }],
          length: 1,
          meta: { custom: { queryID: 'proc:abc', status: 'finished' } },
        },
      ],
    };
    baseQuery.mockReturnValueOnce(of(finished) as never);

    new DataSource(settings(true)).query(request()).subscribe();

    jest.runOnlyPendingTimers();
    expect(baseQuery).toHaveBeenCalledTimes(1);

    jest.useRealTimers();
  });
});
