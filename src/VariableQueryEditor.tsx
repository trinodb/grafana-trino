import React from 'react';
import { QueryEditorProps } from '@grafana/data';
import { CodeEditor } from '@grafana/ui';
import { DataSource } from './datasource';
import { TrinoDataSourceOptions, TrinoQuery, toVariableTrinoQuery } from './types';

type Props = QueryEditorProps<DataSource, TrinoQuery, TrinoDataSourceOptions>;

export function VariableQueryEditor({ query, onChange }: Props) {
  const trinoQuery = toVariableTrinoQuery(query);

  const onSqlChange = (rawSQL: string) => {
    onChange({ ...trinoQuery, rawSQL });
  };

  return (
    <div style={{ minWidth: '400px', flex: 1 }}>
      <CodeEditor
        language="sql"
        value={trinoQuery.rawSQL ?? ''}
        onBlur={onSqlChange}
        onSave={onSqlChange}
        showMiniMap={false}
        showLineNumbers={true}
        height="200px"
      />
    </div>
  );
}
