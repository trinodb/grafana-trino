import { of, throwError } from 'rxjs';
import { BackendSrv, FetchResponse, setBackendSrv } from '@grafana/runtime';

import { DataSource } from './datasource';

const mockBackend = { fetch: () => {} };
setBackendSrv(mockBackend as unknown as BackendSrv);

jest.mock('@grafana/runtime', () => ({
  ...(jest.requireActual('@grafana/runtime') as any),
  getBackendSrv: () => mockBackend,
  getTemplateSrv: () => ({
    replace: (val: string): string => {
      return val;
    },
  }),
}));

describe('Trino datasource', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('when performing testDatasource call', () => {
    it('should return the error from the server', async () => {
      setupFetchMock(
        undefined,
        throwError(() => ({
          status: 400,
          statusText: 'Bad Request',
          data: {
            results: {
              meta: {
                error: 'db query error: aaaa',
                frames: [
                  {
                    schema: {
                      refId: 'meta',
                      meta: {
                        executedQueryString: 'SELECT 1',
                      },
                      fields: [],
                    },
                    data: {
                      values: [],
                    },
                  },
                ],
              },
            },
          },
        }))
      );

      const ds = new DataSource({ name: '', id: 0, jsonData: {} } as any);
      const result = await ds.testDatasource();
      expect(result.status).toEqual("error");
      expect(result.message).toEqual('Query error: Bad Request');
    });
  });
});

function setupFetchMock(response: any, mock?: any) {
  const defaultMock = () => mock ?? of(createFetchResponse(response));

  const fetchMock = jest.spyOn(mockBackend, 'fetch');
  fetchMock.mockImplementation(defaultMock);
  return fetchMock;
}

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
