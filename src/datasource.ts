import { DataSourceInstanceSettings } from '@grafana/data';
import { DataSourceWithBackend } from '@grafana/runtime';
import { TrinoDataSourceOptions, TrinoQuery } from './types';

export class DataSource extends DataSourceWithBackend<TrinoQuery, TrinoDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<TrinoDataSourceOptions>) {
    super(instanceSettings);
  }
}
