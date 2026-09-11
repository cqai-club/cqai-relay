import { describe, expect, it } from 'vitest'

import type { Model } from '../../types'
import {
  transformFormDataToModelPayload,
  transformModelToFormDefaults,
} from '../model-form'

describe('model capability form mapping', () => {
  it('round-trips multiple capabilities into the model payload', () => {
    const model: Model = {
      id: 1,
      model_name: 'vision-model',
      description: 'Vision model',
      icon: 'CQAI',
      tags: 'vision,multimodal',
      endpoints: '',
      capabilities: ['image', 'text-multimodal'],
      status: 1,
      sync_official: 1,
      created_time: 0,
      updated_time: 0,
      name_rule: 0,
    }

    const defaults = transformModelToFormDefaults(model)
    const payload = transformFormDataToModelPayload(defaults)

    expect(defaults.capabilities).toEqual(['image', 'text-multimodal'])
    expect(payload.capabilities).toEqual(['image', 'text-multimodal'])
  })

  it('uses an empty capability selection for legacy model metadata', () => {
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

    expect(defaults.capabilities).toEqual([])
    expect(transformFormDataToModelPayload(defaults).capabilities).toEqual([])
  })
})
