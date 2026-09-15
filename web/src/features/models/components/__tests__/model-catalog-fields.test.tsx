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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, test, vi } from 'vitest'

import { ModelCatalogFields } from '../model-catalog-fields'

function CatalogFieldsHarness(props: {
  onRestore?: () => Promise<boolean>
  showRestore?: boolean
}) {
  const [inputModalities, setInputModalities] = useState<string[]>(['text'])
  const [outputModalities, setOutputModalities] = useState<string[]>(['text'])
  const [supportedParameters, setSupportedParameters] = useState<string[]>([])
  const [contextLength, setContextLength] = useState('')
  const [maxOutputTokens, setMaxOutputTokens] = useState('')

  return (
    <ModelCatalogFields
      inputModalities={inputModalities}
      outputModalities={outputModalities}
      supportedParameters={supportedParameters}
      contextLength={contextLength}
      maxOutputTokens={maxOutputTokens}
      showRestoreAutomaticRecognition={props.showRestore ?? true}
      isRestoring={false}
      onInputModalitiesChange={setInputModalities}
      onOutputModalitiesChange={setOutputModalities}
      onSupportedParametersChange={setSupportedParameters}
      onContextLengthChange={setContextLength}
      onMaxOutputTokensChange={setMaxOutputTokens}
      onRestoreAutomaticRecognition={props.onRestore ?? (async () => true)}
    />
  )
}

describe('model catalog metadata fields', () => {
  test('updates the read-only category when a non-text input modality is added', async () => {
    const user = userEvent.setup()
    render(<CatalogFieldsHarness />)

    expect(screen.getByText('Text')).toBeInTheDocument()

    const input = screen.getByLabelText('Input modalities')
    await user.type(input, 'image{Enter}')

    expect(screen.getByText('Multimodal text')).toBeInTheDocument()
    expect(screen.queryByText('Text')).not.toBeInTheDocument()
  })

  test('normalizes modality case before deriving the category preview', async () => {
    const user = userEvent.setup()
    render(<CatalogFieldsHarness />)

    const input = screen.getByLabelText('Input modalities')
    await user.clear(input)
    await user.type(input, 'Image{Enter}')

    expect(screen.getByText('Multimodal text')).toBeInTheDocument()
  })

  test('edits structured parameter and numeric limit fields', async () => {
    const user = userEvent.setup()
    render(<CatalogFieldsHarness showRestore={false} />)

    await user.type(
      screen.getByLabelText('Supported parameters'),
      'tools{Enter}'
    )
    await user.type(screen.getByLabelText('Context length'), '1050000')
    await user.type(screen.getByLabelText('Maximum output tokens'), '128000')

    expect(screen.getByText('tools')).toBeInTheDocument()
    expect(screen.getByLabelText('Context length')).toHaveValue(1_050_000)
    expect(screen.getByLabelText('Maximum output tokens')).toHaveValue(128_000)
  })

  test('restores automatic recognition only after confirmation', async () => {
    const user = userEvent.setup()
    const onRestore = vi.fn().mockResolvedValue(true)
    render(<CatalogFieldsHarness onRestore={onRestore} />)

    await user.click(
      screen.getByRole('button', { name: 'Restore automatic recognition' })
    )
    expect(onRestore).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'Continue' }))

    await waitFor(() => expect(onRestore).toHaveBeenCalledTimes(1))
  })

  test('hides the restore action for an automatically managed model', () => {
    render(<CatalogFieldsHarness showRestore={false} />)

    expect(
      screen.queryByRole('button', { name: 'Restore automatic recognition' })
    ).not.toBeInTheDocument()
  })
})
