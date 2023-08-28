import { StandardVariableQuery, StandardVariableSupport } from '@grafana/data';
import { DataSource } from './datasource';
import { TrinoQuery } from 'types';

export class TrinoDataVariableSupport extends StandardVariableSupport<DataSource> {
  toDataQuery(query: StandardVariableQuery): TrinoQuery {
    return {
      refId: 'TrinoDataSource-QueryVariable',
      rawSQL: query.query,
    };
  }
}
