import { DataSourceInstanceSettings, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';
import { TrinoDataSourceOptions, TrinoQuery } from './types';
import { TrinoDataVariableSupport } from './variable';
import { map } from 'lodash';

export class DataSource extends DataSourceWithBackend<TrinoQuery, TrinoDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<TrinoDataSourceOptions>) {
    super(instanceSettings);
    this.variables = new TrinoDataVariableSupport();
    this.annotations={};
    // give interpolateQueryStr access to this
    this.interpolateQueryStr = this.interpolateQueryStr.bind(this);
  }

  applyTemplateVariables(query: TrinoQuery, scopedVars: ScopedVars) {
    return {
      ...query,
      rawSQL: getTemplateSrv().replace(query.rawSQL, scopedVars, this.interpolateQueryStr),
    };
  }

  interpolateQueryStr(value: any, variable: { multi: any; includeAll: any }, defaultFormatFn: any) {
    // if no multi or include all do not regexEscape
    if (!variable.multi && !variable.includeAll) {
      return this.escapeLiteral(value);
    }

    if (typeof value === 'string') {
      return this.quoteLiteral(value);
    }

    const escapedValues = map(value, this.quoteLiteral);
    return escapedValues.join(',');
  }

  quoteIdentifier(value: any) {
    return '"' + String(value).replace(/"/g, '""') + '"';
  }

  quoteLiteral(value: any) {
    return "'" + String(value).replace(/'/g, "''") + "'";
  }

  escapeLiteral(value: any) {
    return String(value).replace(/'/g, "''");
  }
}
