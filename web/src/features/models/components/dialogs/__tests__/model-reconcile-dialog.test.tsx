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
import { useState } from 'react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  applyModelReconcile,
  getModelAliases,
  previewModelReconcile,
  retireModelAliases,
} from '../../../api'
import { ModelReconcileDialog } from '../model-reconcile-dialog'

vi.mock('../../../api', () => ({
  applyModelReconcile: vi.fn(),
  getModelAliases: vi.fn(),
  previewModelReconcile: vi.fn(),
  retireModelAliases: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}))

function renderDialog() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <ModelReconcileDialog open onOpenChange={() => undefined} />
    </QueryClientProvider>
  )
  return queryClient
}

function renderControlledDialog() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

  function ControlledDialog() {
    const [open, setOpen] = useState(true)
    return (
      <>
        <button type='button' onClick={() => setOpen(true)}>
          Reopen model identification
        </button>
        <ModelReconcileDialog open={open} onOpenChange={setOpen} />
      </>
    )
  }

  render(
    <QueryClientProvider client={queryClient}>
      <ControlledDialog />
    </QueryClientProvider>
  )
  return queryClient
}

describe('model reconciliation dialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(previewModelReconcile).mockResolvedValue({
      success: true,
      data: {
        safe_matches: [
          {
            channel_id: 7,
            channel_name: 'CommandCode',
            upstream_model: 'OpenAI/GPT-5',
            canonical_model: 'gpt-5',
            match_type: 'normalized',
            metadata_source: 'basellm_normalized',
            create_alias: true,
            capabilities: ['text-multimodal'],
            retire_after: 2_000_000_000,
          },
        ],
        pending: [
          {
            channel_id: 7,
            channel_name: 'CommandCode',
            upstream_model: 'codex-auto-review',
            reason: 'not_found',
          },
        ],
        conflicts: [
          {
            channel_id: 8,
            channel_name: 'Other channel',
            upstream_model: 'vendor/duplicate',
            canonical_model: 'duplicate',
            reason: 'alias_occupied',
          },
        ],
        aliases: [],
      },
    })
    vi.mocked(getModelAliases).mockResolvedValue({
      success: true,
      data: [
        {
          id: 3,
          alias_name: 'OpenAI/GPT-5',
          canonical_model_name: 'gpt-5',
          status: 'active',
          retire_after: 2_000_000_000,
          source: 'basellm_normalized',
          request_count: 12,
          created_time: 1,
          updated_time: 1,
        },
      ],
    })
    vi.mocked(applyModelReconcile).mockResolvedValue({
      success: true,
      data: { applied: 1 },
    })
    vi.mocked(retireModelAliases).mockResolvedValue({
      success: true,
      data: { migrated_tokens: 1, retired_aliases: 1 },
    })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  test('shows pending and conflict views from the preview', async () => {
    const user = userEvent.setup()
    const queryClient = renderDialog()

    await screen.findByText('OpenAI/GPT-5')
    await user.click(
      screen.getByRole('tab', { name: /Pending identification/ })
    )
    expect(screen.getByText('codex-auto-review')).toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: /Match conflicts/ }))
    expect(screen.getByText('vendor/duplicate')).toBeInTheDocument()
    expect(screen.getByText(/alias_occupied/)).toBeInTheDocument()

    queryClient.clear()
  })

  test('applies only the selected safe preview item', async () => {
    const user = userEvent.setup()
    const queryClient = renderDialog()

    const applyButton = await screen.findByRole('button', {
      name: /Apply selected safe matches/,
    })
    await user.click(applyButton)

    await waitFor(() => {
      expect(applyModelReconcile).toHaveBeenCalledWith({
        items: [
          {
            channel_id: 7,
            upstream_model: 'OpenAI/GPT-5',
            canonical_model: 'gpt-5',
          },
        ],
      })
    })

    queryClient.clear()
  })

  test('selects all safe matches again after the dialog is reopened', async () => {
    const user = userEvent.setup()
    const queryClient = renderControlledDialog()

    await screen.findByText('OpenAI/GPT-5')
    await user.click(
      screen.getByRole('checkbox', { name: /Select model match/ })
    )
    expect(
      screen.getByRole('button', { name: /Apply selected safe matches \(0\)/ })
    ).toBeDisabled()

    await user.click(screen.getByRole('button', { name: 'Close' }))
    await user.click(
      screen.getByRole('button', { name: 'Reopen model identification' })
    )

    expect(
      await screen.findByRole('button', {
        name: /Apply selected safe matches \(1\)/,
      })
    ).toBeEnabled()

    queryClient.clear()
  })

  test('retires an alias only after administrator confirmation', async () => {
    const user = userEvent.setup()
    const queryClient = renderDialog()

    await screen.findByText('OpenAI/GPT-5')
    await user.click(screen.getByRole('tab', { name: /Legacy aliases/ }))
    await user.click(screen.getByRole('button', { name: 'Retire alias' }))

    await waitFor(() => {
      expect(window.confirm).toHaveBeenCalledTimes(1)
      expect(retireModelAliases).toHaveBeenCalledWith(['OpenAI/GPT-5'])
    })

    queryClient.clear()
  })

  test('keeps alias management available when the catalog preview fails', async () => {
    const user = userEvent.setup()
    vi.mocked(previewModelReconcile).mockRejectedValue(
      new Error('catalog unavailable')
    )
    const queryClient = renderDialog()

    await screen.findByText('Failed to load model reconciliation data')
    await user.click(screen.getByRole('tab', { name: /Legacy aliases/ }))
    expect(screen.getByText('OpenAI/GPT-5')).toBeInTheDocument()

    queryClient.clear()
  })
})
