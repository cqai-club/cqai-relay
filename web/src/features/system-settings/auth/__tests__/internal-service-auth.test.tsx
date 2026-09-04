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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  generateInternalProvisionToken,
  getInternalProvisionTokenStatus,
  revealInternalProvisionToken,
  updateInternalProvisionToken,
} from '../../api'
import { SettingsPageProvider } from '../../components/settings-page-context'
import { InternalServiceAuthSection } from '../internal-service-auth-section'

vi.mock('../../api', () => ({
  generateInternalProvisionToken: vi.fn(),
  getInternalProvisionTokenStatus: vi.fn(),
  revealInternalProvisionToken: vi.fn(),
  updateInternalProvisionToken: vi.fn(),
}))

vi.mock('@/features/auth/secure-verification', () => ({
  SecureVerificationDialog: () => null,
  useSecureVerification: () => ({
    open: false,
    methods: { has2FA: true, hasPasskey: false, passkeySupported: false },
    state: { method: '2fa', loading: false, code: '' },
    withVerification: (apiCall: (proofToken?: string) => Promise<unknown>) =>
      apiCall('verified-proof'),
    executeVerification: vi.fn(),
    cancel: vi.fn(),
    setCode: vi.fn(),
    switchMethod: vi.fn(),
  }),
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}))

const generatedToken = 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG'
const revealedToken = 'revealed-token-abcdefghijklmnopqrstuvwxyz012345'

function renderSection() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  const actionsContainer = document.createElement('div')
  document.body.appendChild(actionsContainer)
  render(
    <QueryClientProvider client={queryClient}>
      <SettingsPageProvider actionsContainer={actionsContainer}>
        <InternalServiceAuthSection />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
  return { queryClient, actionsContainer }
}

describe('internal service authentication settings', () => {
  beforeEach(() => {
    vi.mocked(getInternalProvisionTokenStatus).mockResolvedValue({
      success: true,
      message: '',
      data: {
        configured: false,
        source: 'environment',
        encryption_ready: true,
      },
    })
    vi.mocked(generateInternalProvisionToken).mockResolvedValue({
      success: true,
      message: '',
      data: { token: generatedToken },
    })
    vi.mocked(revealInternalProvisionToken).mockResolvedValue({
      success: true,
      message: '',
      data: { token: revealedToken },
    })
    vi.mocked(updateInternalProvisionToken).mockResolvedValue({
      success: true,
      message: '',
    })
  })

  test('generates a visible token and saves only after explicit confirmation', async () => {
    const user = userEvent.setup()
    const { queryClient, actionsContainer } = renderSection()

    const generateButton = await screen.findByRole('button', {
      name: 'Generate secure token',
    })
    await user.click(generateButton)

    const input = screen.getByLabelText('NEW_API_INTERNAL_TOKEN')
    expect(input).toHaveValue(generatedToken)
    expect(input).toHaveAttribute('type', 'text')
    expect(updateInternalProvisionToken).not.toHaveBeenCalled()

    const saveButton = screen.getByRole('button', {
      name: 'Save internal token',
    })
    await waitFor(() => expect(saveButton).toBeEnabled())
    await user.click(saveButton)

    await waitFor(() =>
      expect(updateInternalProvisionToken).toHaveBeenCalledWith(generatedToken)
    )
    queryClient.clear()
    actionsContainer.remove()
  })

  test('reveals the saved token only through the verification callback', async () => {
    vi.mocked(getInternalProvisionTokenStatus).mockResolvedValue({
      success: true,
      message: '',
      data: {
        configured: true,
        source: 'database',
        encryption_ready: true,
      },
    })
    const user = userEvent.setup()
    const { queryClient, actionsContainer } = renderSection()

    await user.click(
      await screen.findByRole('button', { name: 'Reveal saved token' })
    )

    await waitFor(() =>
      expect(revealInternalProvisionToken).toHaveBeenCalledWith(
        'verified-proof'
      )
    )
    expect(screen.getByLabelText('NEW_API_INTERNAL_TOKEN')).toHaveValue(
      revealedToken
    )
    queryClient.clear()
    actionsContainer.remove()
  })
})
