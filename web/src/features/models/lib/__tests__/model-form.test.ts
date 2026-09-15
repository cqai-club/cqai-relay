import { describe, expect, it } from 'vitest'

import type { Model } from '../../types'
import {
  modelFormSchema,
  transformFormDataToModelPayload,
  transformModelToFormDefaults,
} from '../model-form'

describe('model catalog metadata form mapping', () => {
  it('round-trips structured metadata without submitting derived categories', () => {
    const model: Model = {
      id: 1,
      model_name: 'vision-model',
      description: 'Vision model',
      icon: 'CQAI',
      tags: 'vision,multimodal',
      endpoints: '',
      capabilities: ['image', 'text-multimodal'],
      input_modalities: ['text', 'image'],
      output_modalities: ['text'],
      supported_parameters: ['tools', 'reasoning'],
      context_length: 1_050_000,
      max_output_tokens: 128_000,
      status: 1,
      sync_official: 1,
      created_time: 0,
      updated_time: 0,
      name_rule: 0,
    }

    const defaults = transformModelToFormDefaults(model)
    const payload = transformFormDataToModelPayload(defaults)

    expect(defaults.input_modalities).toEqual(['text', 'image'])
    expect(defaults.output_modalities).toEqual(['text'])
    expect(defaults.supported_parameters).toEqual(['tools', 'reasoning'])
    expect(defaults.context_length).toBe('1050000')
    expect(defaults.max_output_tokens).toBe('128000')
    expect(payload).toMatchObject({
      input_modalities: ['text', 'image'],
      output_modalities: ['text'],
      supported_parameters: ['tools', 'reasoning'],
      context_length: 1_050_000,
      max_output_tokens: 128_000,
    })
    expect(payload).not.toHaveProperty('capabilities')
  })

  it('uses empty structured values and zero limits for legacy metadata', () => {
    const model: Model = {
      id: 2,
      model_name: 'legacy-model',
      status: 1,
      sync_official: 1,
      created_time: 0,
      updated_time: 0,
      name_rule: 0,
    }

    const defaults = transformModelToFormDefaults(model)

    expect(defaults.input_modalities).toEqual([])
    expect(defaults.output_modalities).toEqual([])
    expect(defaults.supported_parameters).toEqual([])
    expect(defaults.context_length).toBe('')
    expect(defaults.max_output_tokens).toBe('')
    expect(transformFormDataToModelPayload(defaults)).toMatchObject({
      input_modalities: [],
      output_modalities: [],
      supported_parameters: [],
      context_length: 0,
      max_output_tokens: 0,
    })
  })

  it('trims and de-duplicates catalog lists before submission', () => {
    const defaults = transformModelToFormDefaults({
      id: 3,
      model_name: 'catalog-model',
      status: 1,
      sync_official: 1,
      created_time: 0,
      updated_time: 0,
      name_rule: 0,
    })

    const payload = transformFormDataToModelPayload({
      ...defaults,
      input_modalities: [' text ', 'Image', 'text', ''],
      supported_parameters: [' Tools ', 'tools'],
    })

    expect(payload.input_modalities).toEqual(['text', 'image'])
    expect(payload.supported_parameters).toEqual(['tools'])
  })

  it('rejects negative, fractional, and unsafe catalog limits', () => {
    const invalidValues = ['-1', '1.5', '9007199254740992']

    for (const contextLength of invalidValues) {
      const result = modelFormSchema.safeParse({
        model_name: 'invalid-limit-model',
        context_length: contextLength,
      })

      expect(result.success).toBe(false)
    }
  })
})
