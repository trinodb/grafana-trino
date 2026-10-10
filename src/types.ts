import { DataSourceJsonData, SelectableValue, StandardVariableQuery } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export enum FormatOptions {
  TimeSeries,
  Table,
  Logs,
}
export interface TrinoQuery extends DataQuery {
  rawSQL?: string;
  format?: FormatOptions;
  clientTags?: string;
  trimEdges?: number;
}

export const SelectableFormatOptions: Array<SelectableValue<FormatOptions>> = [
  {
    label: 'Time Series',
    value: FormatOptions.TimeSeries,
  },
  {
    label: 'Table',
    value: FormatOptions.Table,
  },
  {
    label: 'Logs',
    value: FormatOptions.Logs,
  },
];

export const defaultQuery: Partial<TrinoQuery> = {
  rawSQL: `SELECT
  $__timeGroup(orderdate, '1w'),
  sum(totalprice) as value,
  orderstatus as metric
FROM
  tpch.tiny.orders
WHERE
  $__timeFilter(orderdate)
GROUP BY
  1, 3
ORDER BY
  1
`,
  format: FormatOptions.TimeSeries,
};

// Variables saved while this plugin used StandardVariableSupport store the SQL
// as a plain string or as a StandardVariableQuery, not as a TrinoQuery.
export type StoredVariableQuery = TrinoQuery | StandardVariableQuery | string;

const variableRefId = 'TrinoDataSource-QueryVariable';

export function toVariableTrinoQuery(query: StoredVariableQuery): TrinoQuery {
  if (typeof query === 'string') {
    return { refId: variableRefId, rawSQL: query };
  }
  if (!('query' in query)) {
    return query;
  }
  const { query: rawSQL, ...rest } = query;
  return { ...rest, refId: rest.refId || variableRefId, rawSQL };
}

/**
 * These are options configured for each DataSource instance.
 */

export interface TrinoSecureJsonData {
  accessToken?: string;
  clientSecret?: string;
  // set by Grafana's HTTP settings
  basicAuthPassword?: string;
}

export type ImpersonationIdentity = 'login' | 'email';

export const SelectableImpersonationIdentities: Array<SelectableValue<ImpersonationIdentity>> = [
  { label: 'Login', value: 'login' },
  { label: 'Email', value: 'email' },
];

export interface TrinoDataSourceOptions extends DataSourceJsonData {
  enableImpersonation?: boolean;
  impersonationIdentity?: ImpersonationIdentity;
  tokenUrl?: string;
  clientId?: string;
  impersonationUser?: string;
  roles?: string;
  clientTags?: string;
  enableSecureSocksProxy?: boolean;
  // "Forward OAuth Identity", set by Grafana's HTTP settings
  oauthPassThru?: boolean;
  kerberosEnabled?: boolean;
  kerberosPrincipal?: string;
  kerberosRealm?: string;
  kerberosConfigPath?: string;
  kerberosKeytabPath?: string;
  kerberosCredentialCachePath?: string;
  kerberosRemoteServiceName?: string;
  kerberosServicePrincipalPattern?: string;
  kerberosDisableCanonicalHostname?: boolean;
}

export type KerberosPathOrName =
  | 'kerberosPrincipal'
  | 'kerberosRealm'
  | 'kerberosConfigPath'
  | 'kerberosKeytabPath'
  | 'kerberosCredentialCachePath'
  | 'kerberosRemoteServiceName'
  | 'kerberosServicePrincipalPattern';
/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
