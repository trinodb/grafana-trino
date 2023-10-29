import React from 'react';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from './datasource';
import { TrinoDataSourceOptions, TrinoQuery, defaultQuery, SelectableFormatOptions } from './types';
import { FormatSelect, QueryCodeEditor } from '@grafana/aws-sdk';

type Props = QueryEditorProps<DataSource, TrinoQuery, TrinoDataSourceOptions>;

export function QueryEditor(props: Props) {
  const queryWithDefaults = {
    ...defaultQuery,
    ...props.query,
  };

  return (
    <>
        <div className="gf-form-group">
          <h6>Frames</h6>
          <FormatSelect
            query={props.query}
            options={SelectableFormatOptions}
            onChange={() => []}
            onRunQuery={props.onRunQuery}
          />
        </div>
        <div style={{ minWidth: '400px', marginLeft: '10px', flex: 1 }}>
          <QueryCodeEditor
            language="sql"
            query={queryWithDefaults}
            onChange={() => []}
            onRunQuery={props.onRunQuery}
            getSuggestions={() => []}
          />
        </div>
    </>
  );
}
