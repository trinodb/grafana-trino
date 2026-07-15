import React from 'react';
import { QueryEditorProps } from '@grafana/data';
import { CodeEditor, InlineField, Select } from '@grafana/ui';
import { DataSource } from './datasource';
import { TrinoDataSourceOptions, TrinoQuery, defaultQuery, SelectableFormatOptions } from './types';

type Props = QueryEditorProps<DataSource, TrinoQuery, TrinoDataSourceOptions>;

export function QueryEditor(props: Props) {
  const { query, onChange, onRunQuery } = props;
  const queryWithDefaults = {
    ...defaultQuery,
    ...query,
  };

  const onFormatChange = (format: (typeof SelectableFormatOptions)[number]) => {
    onChange({ ...query, format: format.value });
    onRunQuery();
  };

  const onSqlChange = (rawSQL: string) => {
    onChange({ ...query, rawSQL });
    onRunQuery();
  };

  return (
    <>
      <div className="gf-form-group">
        <InlineField label="Format as" labelWidth={16}>
          <Select
            options={SelectableFormatOptions}
            value={queryWithDefaults.format}
            onChange={onFormatChange}
            width={30}
          />
        </InlineField>
      </div>
      <div style={{ minWidth: '400px', marginLeft: '10px', flex: 1 }}>
        <CodeEditor
          language="sql"
          value={queryWithDefaults.rawSQL ?? ''}
          onBlur={onSqlChange}
          onSave={onSqlChange}
          showMiniMap={false}
          showLineNumbers={true}
          height="200px"
        />
      </div>
    </>
  );
}
