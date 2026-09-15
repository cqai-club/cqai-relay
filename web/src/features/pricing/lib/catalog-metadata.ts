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
import type { PricingModel } from '../types'

function normalizeCatalogItems(items?: readonly string[] | null): string[] {
  if (!items) return []
  return items.map((item) => item.trim()).filter(Boolean)
}

export function getCatalogInputModalities(model: PricingModel): string[] {
  return normalizeCatalogItems(
    model.architecture?.input_modalities ?? model.input_modalities
  )
}

export function getCatalogOutputModalities(model: PricingModel): string[] {
  return normalizeCatalogItems(
    model.architecture?.output_modalities ?? model.output_modalities
  )
}

export function getCatalogSupportedParameters(model: PricingModel): string[] {
  return normalizeCatalogItems(model.supported_parameters)
}

export function getCatalogCategories(model: PricingModel): string[] {
  return normalizeCatalogItems(model.categories)
}
