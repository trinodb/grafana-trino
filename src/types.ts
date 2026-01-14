import { DataQuery, DataSourceJsonData, SelectableValue } from '@grafana/data';

export enum FormatOptions {
  TimeSeries,
  Table,
  Logs,
}
export interface TrinoQuery extends DataQuery {
  rawSQL?: string;
  format?: FormatOptions;
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

/**
 * These are options configured for each DataSource instance.
 */

export interface TrinoSecureJsonData {
  accessToken?: string;
  clientSecret?: string;
}

export interface TrinoDataSourceOptions extends DataSourceJsonData {
  enableImpersonation?: boolean;
  tokenUrl?: string;
  clientId?: string;
  impersonationUser?: string;
  roles?: string;
  clientTags?: string;
  enableSecureSocksProxy?: boolean;
}
/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
