import React, { ChangeEvent, PureComponent } from 'react';
import { DataSourceHttpSettings, InlineField, InlineSwitch, SecretInput, Input } from '@grafana/ui';
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
    const onTokenUrlChange = (event: ChangeEvent<HTMLInputElement>) => {
      onOptionsChange({...options, jsonData: {...options.jsonData, tokenUrl: event.target.value}})
    };
    const onClientIdChange = (event: ChangeEvent<HTMLInputElement>) => {
      onOptionsChange({...options, jsonData: {...options.jsonData, clientId: event.target.value}})
    };
    const onClientSecretChange = (event: ChangeEvent<HTMLInputElement>) => {
      onOptionsChange({...options, secureJsonData: {...options.secureJsonData, clientSecret: event.target.value}})
    };
    const onResetClientSecret = () => {
      onOptionsChange({...options, secureJsonFields: {...options.secureJsonFields, clientSecret: false}, secureJsonData: {...options.secureJsonData, clientSecret: ''}});
    };
    const onImpersonationUserChange = (event: ChangeEvent<HTMLInputElement>) => {
      onOptionsChange({...options, jsonData: {...options.jsonData, impersonationUser: event.target.value}})
    };
    const onRolesChange = (event: ChangeEvent<HTMLInputElement>) => {
      onOptionsChange({...options, jsonData: {...options.jsonData, roles: event.target.value}})
    };
    const onClientTagsChange = (event: ChangeEvent<HTMLInputElement>) => {
      onOptionsChange({...options, jsonData: {...options.jsonData, clientTags: event.target.value}})
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
          <div className="gf-form-inline">
            <InlineField
              label="Roles"
              tooltip="Authorization roles to use for catalogs, specified as a list of key-value pairs for the catalog and role. For example, system:roleS;catalog1:roleA;catalog2:roleB"
              labelWidth={26}
            >
              <Input
                value={options.jsonData?.roles ?? ''}
                onChange={onRolesChange}
                width={40}
              />
            </InlineField>
          </div>
          <div className="gf-form-inline">
            <InlineField
              label="Client Tags"
              tooltip="A comma-separated list of strings, used to identify Trino resource groups."
              labelWidth={26}
            >
              <Input
                value={options.jsonData?.clientTags ?? ''}
                onChange={onClientTagsChange}
                width={60}
                placeholder="tag1,tag2,tag3"
              />
            </InlineField>
          </div>
        </div>

        <h3 className="page-heading">OAuth Trino Authentication</h3>
        <div className="gf-form-group">
          <div className="gf-form-inline">
            <InlineField
              label="Token URL"
              tooltip="If set, token is retrieved by client credentials flow before request to Trino is sent"
              labelWidth={26}
            >
              <Input
                value={options.jsonData?.tokenUrl ?? ''}
                onChange={onTokenUrlChange}
                width={60}
              />
            </InlineField>
          </div>
          <div className="gf-form-inline">
            <InlineField
              label="Client id"
              tooltip="Required if Token URL is set"
              labelWidth={26}
            >
              <Input
                value={options.jsonData?.clientId ?? ''}
                onChange={onClientIdChange}
                width={60}
              />
            </InlineField>
          </div>
          <div className="gf-form-inline">
            <InlineField
              label="Client secret"
              tooltip="Required if Token URL is set"
              labelWidth={26}
            >
              <SecretInput
                value={options.secureJsonData?.clientSecret ?? ''}
                isConfigured={options.secureJsonFields?.clientSecret}
                onChange={onClientSecretChange}
                width={60}
                onReset={onResetClientSecret}
              />
            </InlineField>
          </div>
          <div className="gf-form-inline">
            <InlineField
              label="Impersonation user"
              tooltip="If set, this user will be used for impersonation in Trino"
              labelWidth={26}
            >
              <Input
                value={options.jsonData?.impersonationUser ?? ''}
                onChange={onImpersonationUserChange}
                width={60}
              />
            </InlineField>
          </div>
        </div>
      </div>
    );
  }
}
