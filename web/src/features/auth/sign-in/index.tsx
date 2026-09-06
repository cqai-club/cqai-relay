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
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { createOAuthFlow } from '@/features/auth/api'
import { buildOIDCOAuthUrl } from '@/features/auth/lib/oauth'
import { useStatus } from '@/hooks/use-status'

import { AuthLayout } from '../auth-layout'

export function SignIn() {
  const { t } = useTranslation()
  const { status, loading, error } = useStatus()
  const [loginError, setLoginError] = useState<string | null>(null)
  const startedRef = useRef(false)

  useEffect(() => {
    if (loading || startedRef.current) return
    startedRef.current = true

    if (
      error ||
      !status?.oidc_enabled ||
      !status.oidc_client_id ||
      !status.oidc_authorization_endpoint
    ) {
      setLoginError(t('Logto login is not configured correctly.'))
      return
    }

    void (async () => {
      try {
        const state = await createOAuthFlow('oidc', 'login')
        const url = buildOIDCOAuthUrl(
          status.oidc_authorization_endpoint!,
          status.oidc_client_id!,
          state,
          status.oidc_redirect_uri,
          status.oidc_scope,
          status.oidc_resource
        )
        window.location.replace(url)
      } catch {
        setLoginError(t('Unable to start Logto login.'))
      }
    })()
  }, [error, loading, status, t])

  if (loginError) {
    return (
      <AuthLayout>
        <div className='space-y-6 text-center'>
          <h2 className='text-2xl font-semibold'>{t('Login unavailable')}</h2>
          <p className='text-muted-foreground'>{loginError}</p>
          <Button onClick={() => window.location.replace('/')}>
            {t('Back to Home')}
          </Button>
        </div>
      </AuthLayout>
    )
  }

  return (
    <AuthLayout>
      <div className='space-y-4 text-center'>
        <h2 className='text-2xl font-semibold'>{t('Redirecting to Logto')}</h2>
        <p className='text-muted-foreground'>
          {t('Please wait while we redirect you to the unified login.')}
        </p>
      </div>
    </AuthLayout>
  )
}
