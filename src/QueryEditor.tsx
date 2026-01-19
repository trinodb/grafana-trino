import React from 'react';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from './datasource';
import { TrinoDataSourceOptions, TrinoQuery, defaultQuery, SelectableFormatOptions } from './types';
import { FormatSelect, QueryCodeEditor } from '@grafana/aws-sdk';
import { Input } from '@grafana/ui';

type Props = QueryEditorProps<DataSource, TrinoQuery, TrinoDataSourceOptions>;

export function QueryEditor(props: Props) {
  const queryWithDefaults = {
    ...defaultQuery,
    ...props.query,
  };

  const onClientTagsChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    props.onChange({
      ...props.query,
      clientTags: event.target.value,
    });
  };

  return (
    <>
      <div className="gf-form-group">
        <h6>Frames</h6>
        <FormatSelect
          query={props.query}
          options={SelectableFormatOptions}
          onChange={props.onChange}
          onRunQuery={props.onRunQuery}
        />
      </div>

      <div className="gf-form-group">
        <h6>Client Tags</h6>
        <Input
          value={queryWithDefaults.clientTags || ''}
          placeholder="e.g. tag1,tag2,tag3 (Note tags from all queries in this panel are combined)"
          onChange={onClientTagsChange}
        />
      </div>

      <div style={{ minWidth: '400px', marginLeft: '10px', flex: 1 }}>
        <QueryCodeEditor
          language="sql"
          query={queryWithDefaults}
          onChange={props.onChange}
          onRunQuery={props.onRunQuery}
          getSuggestions={() => []}
        />
      </div>
    </>
  );
}
