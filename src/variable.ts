import {
  CustomVariableSupport,
  DataFrame,
  DataQueryRequest,
  DataQueryResponse,
  Field,
  FieldType,
  MetricFindValue,
} from '@grafana/data';
import { uniqBy } from 'lodash';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { DataSource } from './datasource';
import { FormatOptions, StoredVariableQuery, TrinoQuery, toVariableTrinoQuery } from './types';
import { VariableQueryEditor } from './VariableQueryEditor';

export class TrinoDataVariableSupport extends CustomVariableSupport<DataSource, TrinoQuery> {
  editor = VariableQueryEditor;

  constructor(private readonly datasource: DataSource) {
    super();
  }

  query(request: DataQueryRequest<TrinoQuery>): Observable<DataQueryResponse> {
    const targets = request.targets.map((target: StoredVariableQuery) => ({
      ...toVariableTrinoQuery(target),
      format: FormatOptions.Table,
    }));
    return this.datasource.query({ ...request, targets }).pipe(
      map((response) => ({
        ...response,
        data: response.data.map((frame: DataFrame) => toMetricFindFrame(frame)),
      }))
    );
  }
}

// Grafana only accepts string fields as variable values, so every value is
// converted to a string and exposed through the `text` and `value` fields it
// looks for first.
export function toMetricFindFrame(frame: DataFrame): DataFrame {
  const values = toMetricFindValues(frame);
  return {
    ...frame,
    length: values.length,
    fields: [
      { name: 'text', type: FieldType.string, config: {}, values: values.map((value) => value.text) },
      { name: 'value', type: FieldType.string, config: {}, values: values.map((value) => String(value.value)) },
    ],
  };
}

// Follows the convention of Grafana's own SQL data sources: `__text` and
// `__value` columns provide separate labels and values, otherwise the first
// column provides both.
export function toMetricFindValues(frame: DataFrame): MetricFindValue[] {
  const textField = frame.fields.find((field) => field.name === '__text');
  const valueField = frame.fields.find((field) => field.name === '__value');
  if (textField && valueField) {
    return textField.values.map((text, index) => ({
      text: valueToString(textField, text),
      value: valueToString(valueField, valueField.values[index]),
    }));
  }

  const firstField = frame.fields[0];
  if (!firstField) {
    return [];
  }
  const values: string[] = firstField.values
    .filter((value) => value !== null && value !== undefined)
    .map((value) => valueToString(firstField, value));
  return uniqBy(
    values.map((value) => ({ text: value, value })),
    'value'
  );
}

function valueToString(field: Field, value: unknown): string {
  if (value === null || value === undefined) {
    return '';
  }
  if (field.type === FieldType.time) {
    return new Date(value as number | string | Date).toISOString();
  }
  return String(value);
}
