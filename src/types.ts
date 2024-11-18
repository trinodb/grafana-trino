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
  $__timeGroup(time_column, '1h'),
  value_column as value,
  series_column as metric
FROM
  catalog_name.schema_name.table_name
WHERE
  $__timeFilter(time_column)
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
}

export interface TrinoDataSourceOptions extends DataSourceJsonData {
  enableImpersonation?: boolean;
}
/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
