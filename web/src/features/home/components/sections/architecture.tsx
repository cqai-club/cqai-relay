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
import { Route } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'

import { ArchitectureDiagram } from '../architecture-diagram'

export function Architecture() {
  const { t } = useTranslation()

  return (
    <section className='border-border/50 relative z-10 overflow-hidden border-t px-6 py-24 md:py-32 dark:border-white/[0.06]'>
      <div className='relative mx-auto max-w-6xl'>
        <AnimateInView className='mx-auto mb-14 max-w-2xl text-center'>
          <p className='text-muted-foreground mb-3 flex items-center justify-center gap-2 text-xs font-medium tracking-widest uppercase'>
            {t('Architecture')}
            <span className='bg-primary/40 h-px w-8' />
          </p>
          <h2 className='text-2xl font-bold tracking-tight md:text-3xl'>
            {t('Models above, applications below,')}
            <br />
            <span className='text-gradient-muted bg-clip-text text-transparent'>
              {t('Relay in the middle')}
            </span>
          </h2>
        </AnimateInView>

        <AnimateInView animation='fade-up' className='mx-auto max-w-5xl'>
          <div className='relative mx-auto aspect-[960/560] w-full'>
            <ArchitectureDiagram />
          </div>
        </AnimateInView>

        <div className='mt-8 flex justify-center'>
          <div className='text-muted-foreground/50 inline-flex items-center gap-2 text-xs'>
            <Route className='size-3.5' aria-hidden />
            <span>{t('Upstream to downstream in a single governed path')}</span>
          </div>
        </div>
      </div>
    </section>
  )
}
