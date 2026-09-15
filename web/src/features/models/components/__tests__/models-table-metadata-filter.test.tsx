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
import { render, waitFor } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { getModels, getVendors, searchModels } from '../../api'
import { ModelsTable } from '../models-table'

vi.mock('@tanstack/react-router', () => ({
  getRouteApi: () => ({
    useSearch: () => ({}),
    useNavigate: () => vi.fn(),
  }),
}))

vi.mock('@/hooks', () => ({ useMediaQuery: () => false }))

vi.mock('@/hooks/use-table-url-state', () => ({
  useTableUrlState: () => ({
    globalFilter: '',
    onGlobalFilterChange: vi.fn(),
    columnFilters: [{ id: 'metadata_status', value: ['pending'] }],
    onColumnFiltersChange: vi.fn(),
    pagination: { pageIndex: 0, pageSize: 20 },
    onPaginationChange: vi.fn(),
    ensurePageInRange: vi.fn(),
  }),
}))

vi.mock('@/components/data-table', () => ({
  DataTablePage: () => <div>models table</div>,
  useDataTable: () => ({ table: {} }),
}))

vi.mock('../data-table-bulk-actions', () => ({
  DataTableBulkActions: () => null,
}))

vi.mock('../models-columns', () => ({ useModelsColumns: () => [] }))
vi.mock('../models-provider', () => ({
  useModels: () => ({ selectedVendor: null }),
}))

vi.mock('../../api', () => ({
  getModels: vi.fn(),
  getVendors: vi.fn(),
  searchModels: vi.fn(),
}))

describe('models metadata filters', () => {
  test('uses the search endpoint with the pending metadata status', async () => {
    vi.mocked(getVendors).mockResolvedValue({
      success: true,
      data: { items: [], total: 0, page: 1, page_size: 20 },
    })
    vi.mocked(searchModels).mockResolvedValue({
      success: true,
      data: { items: [], total: 0, page: 1, page_size: 20 },
    })
    vi.mocked(getModels).mockResolvedValue({
      success: true,
      data: { items: [], total: 0, page: 1, page_size: 20 },
    })
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    render(
      <QueryClientProvider client={queryClient}>
        <ModelsTable />
      </QueryClientProvider>
    )

    await waitFor(() => {
      expect(searchModels).toHaveBeenCalledWith(
        expect.objectContaining({ metadata_status: 'pending' })
      )
    })
    expect(getModels).not.toHaveBeenCalled()
    queryClient.clear()
  })
})
