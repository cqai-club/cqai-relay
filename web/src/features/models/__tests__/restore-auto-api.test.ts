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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { restoreModelAutoRecognition } from '../api'

vi.mock('@/lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('restore automatic model recognition API', () => {
  beforeEach(() => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: true, data: { id: 42 } },
    })
  })

  test('posts without a body to the model restore endpoint', async () => {
    const response = await restoreModelAutoRecognition(42)

    expect(api.post).toHaveBeenCalledWith('/api/models/42/restore_auto')
    expect(response).toMatchObject({ success: true, data: { id: 42 } })
  })
})
