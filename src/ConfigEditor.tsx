import React, { ChangeEvent, PureComponent } from 'react';
import { DataSourceHttpSettings, InlineField, InlineSwitch, SecretInput } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import {TrinoDataSourceOptions, TrinoSecureJsonData} from './types';

interface Props extends DataSourcePluginOptionsEditorProps<TrinoDataSourceOptions, TrinoSecureJsonData> {}

interface State {}

export class ConfigEditor extends PureComponent<Props, State> {
  render() {
    const { options, onOptionsChange } = this.props;
    const onEnableImpersonationChange = (event: ChangeEvent<HTMLInputElement>) => {
      onOptionsChange({...options, jsonData: {...options.jsonData, enableImpersonation: event.target.checked}})
    }
    const onTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
      onOptionsChange({...options, secureJsonData: {...options.secureJsonData, accessToken: event.target.value}})
    }
    const onResetToken = () => {
      onOptionsChange({...options, secureJsonFields: {...options.secureJsonFields, accessToken: false }, secureJsonData: {...options.secureJsonData, accessToken: '' }});
    };
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
              labelWidth={26}
            >
              <InlineSwitch
                id="trino-settings-enable-impersonation"
                value={options.jsonData?.enableImpersonation ?? false}
                onChange={onEnableImpersonationChange}
              />
            </InlineField>
          </div>
          <div className="gf-form-inline">
            <InlineField
                label="Access token"
                tooltip="If set, use the access token for authentication to Trino"
                labelWidth={26}
              >
                <SecretInput
                  value={options.secureJsonData?.accessToken ?? ''}
                  isConfigured={options.secureJsonFields?.accessToken}
                  onChange={onTokenChange}
                  width={40}
                  onReset={onResetToken}
                />
            </InlineField>
          </div>
        </div>
      </div>
    );
  }
}
