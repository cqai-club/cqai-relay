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
import { RefreshIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { TagInput } from '@/components/tag-input'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'

import { deriveModelCategories } from '../lib/model-catalog'
import type { ModelCategory } from '../types'

const CATEGORY_LABEL_KEYS: Record<ModelCategory, string> = {
  image: 'Image generation',
  video: 'Video generation',
  'text-multimodal': 'Multimodal text',
  text: 'Text',
  audio: 'Audio',
  other: 'Other',
}

type ModelCatalogFieldsProps = {
  inputModalities: string[]
  outputModalities: string[]
  supportedParameters: string[]
  contextLength: string
  maxOutputTokens: string
  fallbackCategories?: ModelCategory[]
  endpointTypes?: string[]
  contextLengthError?: string
  maxOutputTokensError?: string
  showRestoreAutomaticRecognition: boolean
  isRestoring: boolean
  onInputModalitiesChange: (value: string[]) => void
  onOutputModalitiesChange: (value: string[]) => void
  onSupportedParametersChange: (value: string[]) => void
  onContextLengthChange: (value: string) => void
  onMaxOutputTokensChange: (value: string) => void
  onRestoreAutomaticRecognition: () => Promise<boolean>
}

export function ModelCatalogFields(props: ModelCatalogFieldsProps) {
  const { t } = useTranslation()
  const [restoreConfirmOpen, setRestoreConfirmOpen] = useState(false)
  const categories = deriveModelCategories(
    props.inputModalities,
    props.outputModalities,
    props.fallbackCategories,
    props.endpointTypes
  )

  const handleRestore = async () => {
    const restored = await props.onRestoreAutomaticRecognition()
    if (restored) setRestoreConfirmOpen(false)
  }

  return (
    <FieldGroup>
      <div className='grid gap-5 sm:grid-cols-2'>
        <Field>
          <FieldLabel htmlFor='model-input-modalities'>
            {t('Input modalities')}
          </FieldLabel>
          <TagInput
            inputId='model-input-modalities'
            value={props.inputModalities}
            onChange={props.onInputModalitiesChange}
            placeholder={t('text, image, file')}
          />
          <FieldDescription>
            {t(
              'Use OpenRouter modality names and press Enter to add each one.'
            )}
          </FieldDescription>
        </Field>

        <Field>
          <FieldLabel htmlFor='model-output-modalities'>
            {t('Output modalities')}
          </FieldLabel>
          <TagInput
            inputId='model-output-modalities'
            value={props.outputModalities}
            onChange={props.onOutputModalitiesChange}
            placeholder={t('text, image, video')}
          />
          <FieldDescription>
            {t(
              'Use OpenRouter modality names and press Enter to add each one.'
            )}
          </FieldDescription>
        </Field>
      </div>

      <Field>
        <FieldLabel htmlFor='model-supported-parameters'>
          {t('Supported parameters')}
        </FieldLabel>
        <TagInput
          inputId='model-supported-parameters'
          value={props.supportedParameters}
          onChange={props.onSupportedParametersChange}
          placeholder={t('tools, reasoning, structured_outputs')}
        />
        <FieldDescription>
          {t('Only add parameters explicitly supported by the model catalog.')}
        </FieldDescription>
      </Field>

      <div className='grid gap-5 sm:grid-cols-2'>
        <Field data-invalid={Boolean(props.contextLengthError)}>
          <FieldLabel htmlFor='model-context-length'>
            {t('Context length')}
          </FieldLabel>
          <Input
            id='model-context-length'
            type='number'
            min='0'
            step='1'
            inputMode='numeric'
            value={props.contextLength}
            onChange={(event) =>
              props.onContextLengthChange(event.target.value)
            }
            aria-invalid={Boolean(props.contextLengthError)}
            placeholder='1050000'
          />
          <FieldDescription>
            {t('Leave empty when the catalog does not provide a limit.')}
          </FieldDescription>
          <FieldError>{props.contextLengthError}</FieldError>
        </Field>

        <Field data-invalid={Boolean(props.maxOutputTokensError)}>
          <FieldLabel htmlFor='model-max-output-tokens'>
            {t('Maximum output tokens')}
          </FieldLabel>
          <Input
            id='model-max-output-tokens'
            type='number'
            min='0'
            step='1'
            inputMode='numeric'
            value={props.maxOutputTokens}
            onChange={(event) =>
              props.onMaxOutputTokensChange(event.target.value)
            }
            aria-invalid={Boolean(props.maxOutputTokensError)}
            placeholder='128000'
          />
          <FieldDescription>
            {t('Leave empty when the catalog does not provide a limit.')}
          </FieldDescription>
          <FieldError>{props.maxOutputTokensError}</FieldError>
        </Field>
      </div>

      <FieldSet>
        <FieldLegend variant='label'>{t('Derived categories')}</FieldLegend>
        <FieldDescription>
          {t(
            'Categories are read-only and derived from the structured metadata.'
          )}
        </FieldDescription>
        <div
          className='flex flex-wrap gap-2'
          aria-label={t('Derived categories')}
        >
          {categories.map((category) => (
            <Badge key={category} variant='secondary'>
              {t(CATEGORY_LABEL_KEYS[category])}
            </Badge>
          ))}
        </div>
      </FieldSet>

      {props.showRestoreAutomaticRecognition && (
        <Alert>
          <HugeiconsIcon icon={RefreshIcon} aria-hidden='true' />
          <AlertTitle>{t('Automatic model recognition')}</AlertTitle>
          <AlertDescription>
            {t(
              'Restore catalog management and queue this model for recognition on the next sync.'
            )}
          </AlertDescription>
          <div className='col-start-2 mt-2'>
            <Button
              type='button'
              variant='outline'
              disabled={props.isRestoring}
              onClick={() => setRestoreConfirmOpen(true)}
            >
              {props.isRestoring ? (
                <Spinner data-icon='inline-start' />
              ) : (
                <HugeiconsIcon icon={RefreshIcon} data-icon='inline-start' />
              )}
              {t('Restore automatic recognition')}
            </Button>
          </div>
        </Alert>
      )}

      <ConfirmDialog
        open={restoreConfirmOpen}
        onOpenChange={setRestoreConfirmOpen}
        title={t('Restore automatic recognition')}
        desc={t(
          'The current values will remain until the next sync. Unsaved changes in this drawer will be discarded.'
        )}
        isLoading={props.isRestoring}
        handleConfirm={handleRestore}
      />
    </FieldGroup>
  )
}
