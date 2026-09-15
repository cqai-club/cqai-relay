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

import {
  canRestoreAutomaticRecognition,
  deriveModelCategories,
} from '../model-catalog'

describe('derived model categories', () => {
  test('normalizes modality case and treats unknown inputs as multimodal', () => {
    expect(deriveModelCategories(['Image', 'Document'], ['TEXT'])).toEqual([
      'text-multimodal',
    ])
  })

  test('uses text for a text output even when the input list is absent', () => {
    expect(deriveModelCategories([], ['text'])).toEqual(['text'])
  })

  test('keeps specialized-only endpoints in other', () => {
    expect(
      deriveModelCategories(
        ['text'],
        ['text'],
        [],
        ['embeddings', 'jina-rerank']
      )
    ).toEqual(['other'])
  })

  test('lets specialized endpoints override incompatible legacy categories', () => {
    expect(deriveModelCategories([], [], ['text'], ['embeddings'])).toEqual([
      'other',
    ])
  })

  test('normalizes mutually exclusive legacy categories', () => {
    expect(
      deriveModelCategories([], [], ['other', 'text', 'text-multimodal'])
    ).toEqual(['text-multimodal'])
  })
})

describe('automatic recognition restore eligibility', () => {
  test('allows manual models and models with official sync disabled', () => {
    expect(
      canRestoreAutomaticRecognition({
        metadata_source: 'manual',
        sync_official: 1,
      })
    ).toBe(true)
    expect(
      canRestoreAutomaticRecognition({
        metadata_source: 'channel',
        sync_official: 0,
      })
    ).toBe(true)
  })

  test('keeps the action hidden for automatically managed models', () => {
    expect(
      canRestoreAutomaticRecognition({
        metadata_source: 'basellm_exact',
        sync_official: 1,
      })
    ).toBe(false)
    expect(canRestoreAutomaticRecognition()).toBe(false)
  })
})
