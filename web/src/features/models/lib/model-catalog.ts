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
import type { Model, ModelCategory } from '../types'

const AUDIO_MODALITIES = new Set(['audio', 'speech', 'transcription'])
const SPECIALIZED_ENDPOINTS = new Set([
  'embeddings',
  'jina-rerank',
  'openai-alpha-search',
])
const GENERATIVE_ENDPOINTS = new Set([
  'openai',
  'openai-response',
  'openai-response-compact',
  'anthropic',
  'gemini',
  'image-generation',
  'openai-video',
])

function normalizeDerivedCategories(
  categories: readonly ModelCategory[]
): ModelCategory[] {
  const categorySet = new Set(categories)
  if (categorySet.has('text-multimodal')) categorySet.delete('text')
  if (categorySet.size > 1) categorySet.delete('other')
  return categorySet.size > 0 ? [...categorySet] : ['other']
}

export function canRestoreAutomaticRecognition(
  model?: Pick<Model, 'metadata_source' | 'sync_official'> | null
): boolean {
  return model?.metadata_source === 'manual' || model?.sync_official === 0
}

export function deriveModelCategories(
  inputModalities: readonly string[],
  outputModalities: readonly string[],
  fallbackCategories: readonly ModelCategory[] = [],
  endpointTypes: readonly string[] = []
): ModelCategory[] {
  const inputSet = new Set(
    inputModalities.map((item) => item.trim().toLowerCase()).filter(Boolean)
  )
  const outputSet = new Set(
    outputModalities.map((item) => item.trim().toLowerCase()).filter(Boolean)
  )

  const endpointSet = new Set(
    endpointTypes.map((item) => item.trim().toLowerCase()).filter(Boolean)
  )
  const hasSpecializedEndpoint = [...endpointSet].some((endpoint) =>
    SPECIALIZED_ENDPOINTS.has(endpoint)
  )
  const hasGenerativeEndpoint = [...endpointSet].some((endpoint) =>
    GENERATIVE_ENDPOINTS.has(endpoint)
  )
  if (hasSpecializedEndpoint && !hasGenerativeEndpoint) return ['other']

  if (inputSet.size === 0 && outputSet.size === 0) {
    return normalizeDerivedCategories(fallbackCategories)
  }

  const categories: ModelCategory[] = []
  if (outputSet.has('image')) categories.push('image')
  if (outputSet.has('video')) categories.push('video')

  const hasAudio = [...inputSet, ...outputSet].some((modality) =>
    AUDIO_MODALITIES.has(modality)
  )
  if (hasAudio) categories.push('audio')

  if (outputSet.has('text')) {
    const hasNonTextInput = [...inputSet].some(
      (modality) => modality !== 'text'
    )
    if (hasNonTextInput) {
      categories.push('text-multimodal')
    } else {
      categories.push('text')
    }
  }

  return normalizeDerivedCategories(categories)
}
