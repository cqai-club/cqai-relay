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
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'

interface MockStat {
  value: string
  unit: string
  label: string
}

export function Adoption() {
  const { t } = useTranslation()

  const stats: MockStat[] = [
    {
      value: '8',
      unit: '+',
      label: t('In-house AI apps onboarded'),
    },
    {
      value: '20',
      unit: '+',
      label: t('Domestic models under one roof'),
    },
    {
      value: '1.2M',
      unit: '+',
      label: t('Calls handled every day'),
    },
  ]

  return (
    <section className='border-border/50 relative z-10 overflow-hidden border-t px-6 py-24 md:py-32 dark:border-white/[0.06]'>
      <div className='relative mx-auto max-w-6xl'>
        <AnimateInView className='mx-auto mb-14 max-w-2xl text-center'>
          <p className='text-muted-foreground mb-3 flex items-center justify-center gap-2 text-xs font-medium tracking-widest uppercase'>
            {t('Dogfooding')}
            <span className='bg-primary/40 h-px w-8' />
          </p>
          <h2 className='text-2xl font-bold tracking-tight md:text-3xl'>
            {t('Our own AI apps')}
            <br />
            <span className='text-gradient-muted bg-clip-text text-transparent'>
              {t('already run on Relay')}
            </span>
          </h2>
          <p className='text-muted-foreground/80 mx-auto mt-4 max-w-xl text-sm leading-relaxed md:text-[15px]'>
            {t(
              'Relay is not built only for the outside world. From creative tools to industry platforms, our products run real workloads, real policies and real cost accounting on it. Dogfooding is the best proof.'
            )}
          </p>
        </AnimateInView>

        <div className='grid grid-cols-1 gap-6 sm:grid-cols-3 md:gap-8'>
          {stats.map((stat, index) => (
            <AnimateInView
              key={stat.label}
              delay={index * 120}
              animation='fade-up'
              className='border-border/40 bg-muted/15 flex flex-col items-center rounded-2xl border px-6 py-8 text-center dark:border-white/[0.07] dark:bg-white/[0.02]'
            >
              <span className='neon-text-drop from-foreground to-foreground/60 bg-gradient-to-b bg-clip-text text-4xl font-bold tracking-tight text-transparent tabular-nums md:text-5xl dark:from-white dark:to-violet-200/80'>
                {stat.value}
                {stat.unit}
              </span>
              <span className='text-muted-foreground/80 mt-3 text-xs leading-relaxed'>
                {stat.label}
              </span>
            </AnimateInView>
          ))}
        </div>

        <p className='text-muted-foreground/40 mt-6 text-center text-[11px]'>
          {t('Mock data for illustration — replace with real numbers before launch.')}
        </p>
      </div>
    </section>
  )
}
