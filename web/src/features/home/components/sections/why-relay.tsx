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

import { NeonAurora } from '../neon-aurora'

export function WhyRelay() {
  const { t } = useTranslation()

  const problems = [
    t('Adding a new model means touching application code over and over.'),
    t('Text, image, video and voice capabilities each come with different call formats.'),
    t('Which models an app can use, and what it costs, is decided ad hoc.'),
    t('No failover when an upstream provider breaks or rate-limits us.'),
  ]

  const answers = [
    t('Models are integrated once; apps always talk to the same unified API.'),
    t('Policies are decoupled from models — switch or add models without touching code.'),
    t('Each app gets its own model scope, limits, quota and billing at a glance.'),
    t('Multiple providers back each other up with automatic failover.'),
  ]

  return (
    <section className='relative z-10 overflow-hidden px-6 py-24 md:py-32'>
      <NeonAurora subtle>
        <div className='neon-dots absolute inset-0 [mask-image:radial-gradient(ellipse_60%_60%_at_50%_40%,black_10%,transparent_75%)] opacity-[0.12]' />
      </NeonAurora>

      <div className='relative mx-auto max-w-6xl'>
        <AnimateInView className='mx-auto mb-16 max-w-2xl text-center'>
          <p className='text-muted-foreground mb-3 flex items-center justify-center gap-2 text-xs font-medium tracking-widest uppercase'>
            {t('Why Relay')}
            <span className='bg-primary/40 h-px w-8' />
          </p>
          <h2 className='text-2xl font-bold tracking-tight md:text-3xl'>
            {t('More models and more apps,')}
            <br />
            <span className='text-gradient-muted bg-clip-text text-transparent'>
              {t('the real bottleneck is in between')}
            </span>
          </h2>
        </AnimateInView>

        <div className='grid gap-6 md:grid-cols-2 md:gap-8'>
          <AnimateInView animation='fade-up' className='h-full'>
            <div className='border-border/40 bg-muted/20 h-full rounded-2xl border p-7 md:p-8 dark:border-white/[0.07] dark:bg-white/[0.02]'>
              <p className='text-foreground/70 mb-5 text-sm font-semibold tracking-wide'>
                {t('Without a gateway')}
              </p>
              <ul className='space-y-4'>
                {problems.map((problem, index) => (
                  <li key={problem} className='flex items-start gap-3'>
                    <span className='border-border/50 bg-border/30 text-muted-foreground mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold tabular-nums'>
                      {index + 1}
                    </span>
                    <span className='text-muted-foreground text-sm leading-relaxed'>
                      {problem}
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          </AnimateInView>

          <AnimateInView animation='fade-up' delay={120} className='h-full'>
            <div className='spotlight-card border-primary/30 relative h-full overflow-hidden rounded-2xl border bg-background/80 p-7 md:p-8 dark:border-violet-400/20 dark:bg-[#100b1d]/90'>
              <div
                aria-hidden
                className='absolute inset-0 bg-gradient-to-br from-violet-500/[0.06] via-transparent to-fuchsia-500/[0.06]'
              />
              <div className='relative'>
                <p className='text-primary mb-5 text-sm font-semibold tracking-wide'>
                  {t('What Relay solves')}
                </p>
                <ul className='space-y-4'>
                  {answers.map((answer, index) => (
                    <li key={answer} className='flex items-start gap-3'>
                      <span className='bg-primary/10 text-primary mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold tabular-nums dark:bg-violet-400/10 dark:text-violet-300'>
                        {index + 1}
                      </span>
                      <span className='text-muted-foreground text-sm leading-relaxed'>
                        {answer}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          </AnimateInView>
        </div>

        <AnimateInView className='mt-12 text-center' animation='fade-up'>
          <p className='text-foreground/70 mx-auto max-w-2xl text-base leading-relaxed md:text-lg'>
            {t(
              'Integration, routing, governance and metering are done once by Relay — application teams just build their product.'
            )}
          </p>
        </AnimateInView>
      </div>
    </section>
  )
}
