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
import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

type AuroraBlob = {
  background: string
  className?: string
}

interface NeonAuroraProps {
  /** Extra classes for the root layer. */
  className?: string
  /** Radial glow blobs. Defaults to the token-driven cyan/blue/violet trio. */
  blobs?: AuroraBlob[]
  /** Extra decor (grids, particles, beams) rendered above the blobs. */
  children?: ReactNode
  /** Reduce ambient glow for subtle sections (stats strip etc.). */
  subtle?: boolean
}

const DEFAULT_BLOBS: AuroraBlob[] = [
  {
    background:
      'radial-gradient(closest-side, color-mix(in oklch, var(--chart-1) 60%, transparent), transparent 72%)',
    className: 'left-[-10%] top-[-12%] size-[55vw] max-md:size-[80vw]',
  },
  {
    background:
      'radial-gradient(closest-side, color-mix(in oklch, var(--chart-2) 55%, transparent), transparent 72%)',
    className:
      'right-[-8%] top-[6%] size-[46vw] max-md:size-[70vw] animation-delay-slow',
  },
  {
    background:
      'radial-gradient(closest-side, color-mix(in oklch, var(--chart-3) 50%, transparent), transparent 72%)',
    className:
      'bottom-[-18%] left-[24%] size-[42vw] max-md:size-[64vw] animation-delay-slow',
  },
]

/**
 * Shared "neon circuit" section backdrop: drifting aurora blobs that follow
 * the active theme palette plus any extra decor the section needs.
 */
export function NeonAurora(props: NeonAuroraProps) {
  const { className, blobs = DEFAULT_BLOBS, children, subtle } = props

  return (
    <div
      aria-hidden='true'
      className={cn(
        'pointer-events-none absolute inset-0 -z-10 overflow-hidden',
        className
      )}
    >
      {blobs.map((blob) => (
        <div
          key={blob.background}
          className={cn(
            'animate-neon-aura absolute rounded-full blur-3xl will-change-transform',
            subtle && 'opacity-70',
            blob.className
          )}
          style={{ background: blob.background }}
        />
      ))}
      {children}
    </div>
  )
}
