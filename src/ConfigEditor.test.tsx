import React from 'react';
import { render, screen } from '@testing-library/react';
import { DataSourceSettings } from '@grafana/data';
import { ConfigEditor } from './ConfigEditor';
import { TrinoDataSourceOptions, TrinoSecureJsonData } from './types';

const WARNING = 'Impersonation user is only used for anonymous users';

function renderEditor(jsonData: TrinoDataSourceOptions) {
  const options = {
    jsonData,
    secureJsonData: {},
    secureJsonFields: {},
  } as unknown as DataSourceSettings<TrinoDataSourceOptions, TrinoSecureJsonData>;
  render(<ConfigEditor options={options} onOptionsChange={jest.fn()} />);
}

describe('ConfigEditor impersonation warning', () => {
  it('warns when both impersonation settings are set', () => {
    renderEditor({ enableImpersonation: true, impersonationUser: 'service-account' });
    expect(screen.getByText(WARNING)).toBeInTheDocument();
  });

  it.each([
    { enableImpersonation: true },
    { enableImpersonation: false, impersonationUser: 'service-account' },
  ])('does not warn for %o', (jsonData) => {
    renderEditor(jsonData);
    expect(screen.queryByText(WARNING)).not.toBeInTheDocument();
  });
});

describe('ConfigEditor Kerberos section', () => {
  const HTTPS_REQUIRED = 'Kerberos requires HTTPS';
  const CONFLICT = 'Kerberos cannot be combined with other authentication';

  function renderKerberosEditor(overrides: Partial<DataSourceSettings<TrinoDataSourceOptions, TrinoSecureJsonData>>) {
    const options = {
      url: 'https://trino.example.com:8443',
      jsonData: { kerberosEnabled: true },
      secureJsonData: {},
      secureJsonFields: {},
      ...overrides,
    } as unknown as DataSourceSettings<TrinoDataSourceOptions, TrinoSecureJsonData>;
    render(<ConfigEditor options={options} onOptionsChange={jest.fn()} />);
  }

  it('hides the Kerberos settings while Kerberos is disabled', () => {
    renderKerberosEditor({ jsonData: {} });
    expect(screen.queryByText('Keytab path')).not.toBeInTheDocument();
  });

  it('shows the Kerberos settings without warnings for an HTTPS URL', () => {
    renderKerberosEditor({});
    expect(screen.getByText('Keytab path')).toBeInTheDocument();
    expect(screen.queryByText(HTTPS_REQUIRED)).not.toBeInTheDocument();
    expect(screen.queryByText(CONFLICT)).not.toBeInTheDocument();
  });

  it('warns about an HTTP URL', () => {
    renderKerberosEditor({ url: 'http://trino.example.com:8080' });
    expect(screen.getByText(HTTPS_REQUIRED)).toBeInTheDocument();
  });

  it('lists the settings that would replace the Kerberos credentials', () => {
    renderKerberosEditor({
      basicAuth: true,
      jsonData: { kerberosEnabled: true, tokenUrl: 'https://idp.example.com/token', oauthPassThru: true },
      secureJsonFields: { basicAuthPassword: true, accessToken: true },
    });
    expect(screen.getByText(CONFLICT)).toBeInTheDocument();
    expect(
      screen.getByText(/remove them: basic auth password, access token, OAuth Trino Authentication, Forward OAuth Identity$/)
    ).toBeInTheDocument();
  });
});
