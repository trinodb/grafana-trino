import { DataQueryRequest, DataQueryResponse, DataSourceInstanceSettings, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';
import { DatasourceWithAsyncBackend } from '@grafana/async-query-data';
import { Observable } from 'rxjs';
import { TrinoDataSourceOptions, TrinoQuery } from './types';
import { TrinoDataVariableSupport } from './variable';
import { map } from 'lodash';

export class DataSource extends DatasourceWithAsyncBackend<TrinoQuery, TrinoDataSourceOptions> {
  private asyncQueryDataSupport: boolean;

  constructor(instanceSettings: DataSourceInstanceSettings<TrinoDataSourceOptions>) {
    super(instanceSettings);
    this.asyncQueryDataSupport = instanceSettings.jsonData?.enableAsyncQueryData ?? false;
    this.variables = new TrinoDataVariableSupport();
    this.annotations={};
    // give interpolateQueryStr access to this
    this.interpolateQueryStr = this.interpolateQueryStr.bind(this);
  }

  query(request: DataQueryRequest<TrinoQuery>): Observable<DataQueryResponse> {
    if (!this.asyncQueryDataSupport) {
      // Asynchronous polling is opt-in per data source. Going straight to
      // DataSourceWithBackend keeps the synchronous flow on a single batched
      // request per panel, instead of the one-request-per-target shape the
      // polling base class needs.
      return DataSourceWithBackend.prototype.query.call(this, request);
    }

    // A handle is single-use, so replaying a cached response leaves the panel
    // polling a query the backend has already collected. The base class only
    // skips the cache once a query is known to be running, which still leaves
    // the request that starts one cacheable.
    return super.query({ ...request, skipQueryCache: true });
  }

  applyTemplateVariables(query: TrinoQuery, scopedVars: ScopedVars) {
    return {
      ...query,
      rawSQL: getTemplateSrv().replace(query.rawSQL, scopedVars, this.interpolateQueryStr),
      // client tags end up in an HTTP header, not in SQL, so they must not be
      // quoted or escaped like SQL literals; 'csv' renders a multi-value
      // variable as the comma-separated list the tags are already written as
      clientTags: getTemplateSrv().replace(query.clientTags, scopedVars, 'csv'),
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
