import React, { ChangeEvent, PureComponent } from 'react';
import { DataSourceHttpSettings, InlineField, InlineSwitch, LegacyForms } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { TrinoDataSourceOptions } from './types';
const { FormField } = LegacyForms;

interface Props extends DataSourcePluginOptionsEditorProps<TrinoDataSourceOptions> {}

interface State {}

export class ConfigEditor extends PureComponent<Props, State> {
  render() {
    const { options, onOptionsChange } = this.props;
    const onEnableImpersonationChange = (event: ChangeEvent<HTMLInputElement>) => {
      onOptionsChange({...options, jsonData: {...options.jsonData, enableImpersonation: event.target.checked}})
    }
    const onTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
      onOptionsChange({...options, jsonData: {...options.jsonData, jwtAccessToken: event.target.value}})
    }
    return (
      <div className="gf-form-group">
        <DataSourceHttpSettings
          defaultUrl="http://localhost:8080"
          dataSourceConfig={options}
          onChange={onOptionsChange}
        />

        <h3 className="page-heading">Trino</h3>
        <div className="gf-form-group">
          <div className="gf-form-inline">
            <InlineField
              label="Impersonate logged in user"
              tooltip="If enabled, set the Trino session user to the current Grafana user"
            >
              <InlineSwitch
                id="trino-settings-enable-impersonation"
                value={options.jsonData?.enableImpersonation ?? false}
                onChange={onEnableImpersonationChange}
              />
            </InlineField>
          </div>
          <div className="gf-form-inline">
            <FormField
              label="Test"
              tooltip="If set, use the JWT Access Token for authentication to Trino"
              value={options.jsonData?.jwtAccessToken || ''}
              inputWidth={18}
              labelWidth={10}
              onChange={onTokenChange}
            />
          </div>
        </div>
      </div>
    );
  }
}
