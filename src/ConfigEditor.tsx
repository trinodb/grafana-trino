import React, { ChangeEvent, SyntheticEvent } from 'react';
import {
  Alert,
  Button,
  DataSourceHttpSettings,
  FileUpload,
  InlineField,
  InlineSwitch,
  SecretInput,
  Input,
  RadioButtonGroup,
  TextArea,
} from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { config } from '@grafana/runtime';
import {
  ImpersonationIdentity,
  KerberosTextSetting,
  SelectableImpersonationIdentities,
  TrinoDataSourceOptions,
  TrinoSecureJsonData,
} from './types';

interface Props extends DataSourcePluginOptionsEditorProps<TrinoDataSourceOptions, TrinoSecureJsonData> {}

export function ConfigEditor(props: Props) {
  const { options, onOptionsChange } = props;

  const onEnableImpersonationChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...options.jsonData, enableImpersonation: event.target.checked } });
  };
  const onImpersonationIdentityChange = (value: ImpersonationIdentity) => {
    onOptionsChange({ ...options, jsonData: { ...options.jsonData, impersonationIdentity: value } });
  };
  const onTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, secureJsonData: { ...options.secureJsonData, accessToken: event.target.value } });
  };
  const onResetToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...options.secureJsonFields, accessToken: false },
      secureJsonData: { ...options.secureJsonData, accessToken: '' },
    });
  };
  const onTokenUrlChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...options.jsonData, tokenUrl: event.target.value } });
  };
  const onClientIdChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...options.jsonData, clientId: event.target.value } });
  };
  const onClientSecretChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, secureJsonData: { ...options.secureJsonData, clientSecret: event.target.value } });
  };
  const onResetClientSecret = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...options.secureJsonFields, clientSecret: false },
      secureJsonData: { ...options.secureJsonData, clientSecret: '' },
    });
  };
  const onImpersonationUserChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...options.jsonData, impersonationUser: event.target.value } });
  };
  const onRolesChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...options.jsonData, roles: event.target.value } });
  };
  const onClientTagsChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...options.jsonData, clientTags: event.target.value } });
  };
  const onEnableSecureSocksProxyChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...options.jsonData, enableSecureSocksProxy: event.target.checked } });
  };
  const onKerberosEnabledChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...options.jsonData, kerberosEnabled: event.target.checked } });
  };
  const onKerberosSettingChange =
    (setting: KerberosTextSetting) => (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      onOptionsChange({ ...options, jsonData: { ...options.jsonData, [setting]: event.target.value } });
    };
  const onKerberosKeytabUpload = async (event: SyntheticEvent<HTMLInputElement>) => {
    const file = event.currentTarget.files?.[0];
    if (!file) {
      return;
    }
    const keytab = base64(new Uint8Array(await file.arrayBuffer()));
    onOptionsChange({ ...options, secureJsonData: { ...options.secureJsonData, kerberosKeytab: keytab } });
  };
  const onResetKerberosKeytab = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...options.secureJsonFields, kerberosKeytab: false },
      secureJsonData: { ...options.secureJsonData, kerberosKeytab: '' },
    });
  };
  const onKerberosUseCanonicalHostnameChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...options.jsonData, kerberosDisableCanonicalHostname: !event.target.checked },
    });
  };
  const kerberosConflicts = options.jsonData?.kerberosEnabled ? conflictingWithKerberos(options) : [];

  return (
    <div className="gf-form-group">
      <DataSourceHttpSettings defaultUrl="http://localhost:8080" dataSourceConfig={options} onChange={onOptionsChange} />

      <h3 className="page-heading">Trino</h3>
      <div className="gf-form-group">
        <div className="gf-form-inline">
          <InlineField
            label="Impersonate logged in user"
            tooltip="If enabled, set the Trino session user to the current Grafana user. Anonymous users are not impersonated and run as the data source's user, or the OAuth impersonation user if set."
            labelWidth={26}
          >
            <InlineSwitch
              id="trino-settings-enable-impersonation"
              value={options.jsonData?.enableImpersonation ?? false}
              onChange={onEnableImpersonationChange}
            />
          </InlineField>
        </div>
        {options.jsonData?.enableImpersonation && (
          <div className="gf-form-inline">
            <InlineField
              label="Impersonate as"
              tooltip="Which attribute of the Grafana user to send as the Trino session user. Queries fail for users without an email if Email is selected."
              labelWidth={26}
            >
              <RadioButtonGroup
                options={SelectableImpersonationIdentities}
                value={options.jsonData?.impersonationIdentity ?? 'login'}
                onChange={onImpersonationIdentityChange}
              />
            </InlineField>
          </div>
        )}
        <div className="gf-form-inline">
          <InlineField label="Access token" tooltip="If set, use the access token for authentication to Trino" labelWidth={26}>
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
            <Input value={options.jsonData?.roles ?? ''} onChange={onRolesChange} width={40} />
          </InlineField>
        </div>
        <div className="gf-form-inline">
          <InlineField
            label="Client Tags"
            tooltip="A comma-separated list of strings, used to identify Trino resource groups."
            labelWidth={26}
          >
            <Input value={options.jsonData?.clientTags ?? ''} onChange={onClientTagsChange} width={60} placeholder="tag1,tag2,tag3" />
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
            <Input value={options.jsonData?.tokenUrl ?? ''} onChange={onTokenUrlChange} width={60} />
          </InlineField>
        </div>
        <div className="gf-form-inline">
          <InlineField label="Client id" tooltip="Required if Token URL is set" labelWidth={26}>
            <Input value={options.jsonData?.clientId ?? ''} onChange={onClientIdChange} width={60} />
          </InlineField>
        </div>
        <div className="gf-form-inline">
          <InlineField label="Client secret" tooltip="Required if Token URL is set" labelWidth={26}>
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
          <InlineField label="Impersonation user" tooltip="If set, this user will be used for impersonation in Trino" labelWidth={26}>
            <Input value={options.jsonData?.impersonationUser ?? ''} onChange={onImpersonationUserChange} width={60} />
          </InlineField>
        </div>
        {options.jsonData?.enableImpersonation && options.jsonData?.impersonationUser && (
          <Alert severity="warning" title="Impersonation user is only used for anonymous users">
            &quot;Impersonate logged in user&quot; is enabled, so signed-in Grafana users run as themselves. This
            impersonation user only applies to queries from anonymous users.
          </Alert>
        )}
      </div>

      <h3 className="page-heading">Kerberos Authentication</h3>
      <div className="gf-form-group">
        <div className="gf-form-inline">
          <InlineField
            label="Enable Kerberos"
            tooltip="Authenticate to Trino with Kerberos (SPNEGO). Requires an HTTPS URL. The configuration and keytab are stored with the data source; nothing is read from the Grafana server."
            labelWidth={26}
          >
            <InlineSwitch
              id="trino-settings-kerberos-enabled"
              value={options.jsonData?.kerberosEnabled ?? false}
              onChange={onKerberosEnabledChange}
            />
          </InlineField>
        </div>
        {options.jsonData?.kerberosEnabled && (
          <>
            {options.url && !/^https:\/\//i.test(options.url) && (
              <Alert severity="error" title="Kerberos requires HTTPS">
                Trino only accepts Kerberos authentication over HTTPS. Change the URL to use https://.
              </Alert>
            )}
            {kerberosConflicts.length > 0 && (
              <Alert severity="error" title="Kerberos cannot be combined with other authentication">
                These settings would replace the Kerberos credentials, remove them: {kerberosConflicts.join(', ')}
              </Alert>
            )}
            <div className="gf-form-inline">
              <InlineField
                label="Principal"
                tooltip="Kerberos principal to log in as with the keytab, without the realm."
                labelWidth={26}
              >
                <Input
                  value={options.jsonData?.kerberosPrincipal ?? ''}
                  onChange={onKerberosSettingChange('kerberosPrincipal')}
                  width={60}
                  placeholder="grafana"
                />
              </InlineField>
            </div>
            <div className="gf-form-inline">
              <InlineField
                label="Realm"
                tooltip="Realm of the principal."
                labelWidth={26}
              >
                <Input
                  value={options.jsonData?.kerberosRealm ?? ''}
                  onChange={onKerberosSettingChange('kerberosRealm')}
                  width={60}
                  placeholder="EXAMPLE.COM"
                />
              </InlineField>
            </div>
            <div className="gf-form-inline">
              <InlineField
                label="Config"
                tooltip="Content of the krb5.conf naming the KDCs of the realm and mapping hosts to realms."
                labelWidth={26}
              >
                <TextArea
                  value={options.jsonData?.kerberosConfig ?? ''}
                  onChange={onKerberosSettingChange('kerberosConfig')}
                  rows={8}
                  cols={58}
                  placeholder={KERBEROS_CONFIG_PLACEHOLDER}
                />
              </InlineField>
            </div>
            <div className="gf-form-inline">
              <InlineField
                label="Keytab"
                tooltip="The keytab file holding the principal's key, stored encrypted with the data source."
                labelWidth={26}
              >
                {options.secureJsonFields?.kerberosKeytab ? (
                  <div className="gf-form-inline">
                    <Input value="configured" disabled width={40} />
                    <Button variant="secondary" onClick={onResetKerberosKeytab}>
                      Reset
                    </Button>
                  </div>
                ) : (
                  <FileUpload onFileUpload={onKerberosKeytabUpload} showFileName>
                    Upload keytab
                  </FileUpload>
                )}
              </InlineField>
            </div>
            <div className="gf-form-inline">
              <InlineField
                label="Remote service name"
                tooltip="Service name of the Trino coordinator principal, substituted for ${SERVICE} in the service principal pattern."
                labelWidth={26}
              >
                <Input
                  value={options.jsonData?.kerberosRemoteServiceName ?? ''}
                  onChange={onKerberosSettingChange('kerberosRemoteServiceName')}
                  width={60}
                  placeholder="trino"
                />
              </InlineField>
            </div>
            <div className="gf-form-inline">
              <InlineField
                label="Service principal pattern"
                tooltip="Service principal of the Trino coordinator, with ${SERVICE} and ${HOST} replaced."
                labelWidth={26}
              >
                <Input
                  value={options.jsonData?.kerberosServicePrincipalPattern ?? ''}
                  onChange={onKerberosSettingChange('kerberosServicePrincipalPattern')}
                  width={60}
                  placeholder="${SERVICE}@${HOST}"
                />
              </InlineField>
            </div>
            <div className="gf-form-inline">
              <InlineField
                label="Use canonical hostname"
                tooltip="Resolve the Trino host to its canonical name through DNS before substituting it for ${HOST}. Disable to use the host from the URL as is."
                labelWidth={26}
              >
                <InlineSwitch
                  id="trino-settings-kerberos-use-canonical-hostname"
                  value={!(options.jsonData?.kerberosDisableCanonicalHostname ?? false)}
                  onChange={onKerberosUseCanonicalHostnameChange}
                />
              </InlineField>
            </div>
          </>
        )}
      </div>

      {config.secureSocksDSProxyEnabled && (
        <>
          <h3 className="page-heading">Other Settings</h3>
          <div className="gf-form-group">
            <div className="gf-form-inline">
              <InlineField
                label="Secure Socks Proxy"
                tooltip="Enable proxying the data source connection through the secure SOCKS proxy to a different network. Used by Grafana Cloud's Private Data Source Connect (PDC)."
                labelWidth={26}
              >
                <InlineSwitch
                  id="trino-settings-enable-secure-socks-proxy"
                  value={options.jsonData?.enableSecureSocksProxy ?? false}
                  onChange={onEnableSecureSocksProxyChange}
                />
              </InlineField>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

const KERBEROS_CONFIG_PLACEHOLDER = `[libdefaults]
  default_realm = EXAMPLE.COM

[realms]
  EXAMPLE.COM = {
    kdc = kdc.example.com
  }

[domain_realm]
  .example.com = EXAMPLE.COM`;

function base64(bytes: Uint8Array): string {
  let binary = '';
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary);
}

function conflictingWithKerberos(options: Props['options']): string[] {
  const conflicts: string[] = [];
  if (options.basicAuth && (options.secureJsonFields?.basicAuthPassword || options.secureJsonData?.basicAuthPassword)) {
    conflicts.push('basic auth password');
  }
  if (options.secureJsonFields?.accessToken || options.secureJsonData?.accessToken) {
    conflicts.push('access token');
  }
  if (
    options.jsonData?.tokenUrl ||
    options.jsonData?.clientId ||
    options.secureJsonFields?.clientSecret ||
    options.secureJsonData?.clientSecret
  ) {
    conflicts.push('OAuth Trino Authentication');
  }
  if (options.jsonData?.oauthPassThru) {
    conflicts.push('Forward OAuth Identity');
  }
  return conflicts;
}
