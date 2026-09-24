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
