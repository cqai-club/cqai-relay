/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, test } from 'vitest'

import { buildOIDCOAuthUrl } from '../oauth'

describe('buildOIDCOAuthUrl', () => {
  test('uses the configured redirect URI and scope for Logto', () => {
    const result = new URL(
      buildOIDCOAuthUrl(
        'https://auth.example.com/oidc/auth',
        'client-id',
        'state-token',
        'https://relay.example.com/oauth/oidc',
        'openid profile email account:admin account:root',
        'https://account.example.com'
      )
    )

    expect(result.searchParams.get('client_id')).toBe('client-id')
    expect(result.searchParams.get('redirect_uri')).toBe(
      'https://relay.example.com/oauth/oidc'
    )
    expect(result.searchParams.get('scope')).toBe(
      'openid profile email account:admin account:root'
    )
    expect(result.searchParams.get('resource')).toBe(
      'https://account.example.com'
    )
    expect(result.searchParams.get('state')).toBe('state-token')
  })

  test('keeps backward-compatible defaults when optional values are absent', () => {
    const result = new URL(
      buildOIDCOAuthUrl('https://auth.example.com/oidc/auth', 'client-id', 'state-token')
    )

    expect(result.searchParams.get('redirect_uri')).toBe(
      `${window.location.origin}/oauth/oidc`
    )
    expect(result.searchParams.get('scope')).toBe('openid profile email')
    expect(result.searchParams.get('resource')).toBeNull()
  })
})
