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
  $__timeEpoch(<time_column>),
  <value column> as value,
  <series name column> as metric
FROM
  <catalog name>.<schema name>.<table name>
WHERE
  $__timeFilter(time_column)
ORDER BY
  <time_column> ASC`,
  format: FormatOptions.Table,
};

/**
 * These are options configured for each DataSource instance.
 */

export interface TrinoDataSourceOptions extends DataSourceJsonData {}
/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
