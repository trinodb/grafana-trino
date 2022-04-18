import { DataSourcePlugin } from '@grafana/data';
import { DataSource } from './datasource';
import { ConfigEditor } from './ConfigEditor';
import { QueryEditor } from './QueryEditor';
import { TrinoQuery, TrinoDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, TrinoQuery, TrinoDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);
