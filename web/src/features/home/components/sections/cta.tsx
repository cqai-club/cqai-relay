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
import { Link } from '@tanstack/react-router'
import { ArrowRight, GitBranch } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { Button } from '@/components/ui/button'

import { NeonAurora } from '../neon-aurora'

interface CTAProps {
  className?: string
  isAuthenticated?: boolean
}

export function CTA(props: CTAProps) {
  const { t } = useTranslation()

  return (
    <section className='relative z-10 overflow-hidden px-6 py-24 md:py-32'>
      <div className='relative mx-auto max-w-4xl'>
        <NeonAurora className='-inset-6 -z-10' />
        <div className='relative rounded-[2rem]'>
          <div
            aria-hidden
            className='neon-flow-border pointer-events-none absolute -inset-px rounded-[2rem]'
          />
          <div className='bg-background/80 relative overflow-hidden rounded-[2rem] border border-transparent px-6 py-16 backdrop-blur-xl md:px-16 md:py-20 dark:bg-[#0d0a1e]/80'>
            <div
              aria-hidden
              className='absolute inset-0 bg-gradient-to-br from-violet-500/[0.07] via-transparent to-fuchsia-500/[0.07]'
            />
            <div
              aria-hidden
              className='animate-neon-beam pointer-events-none absolute inset-y-0 left-0 hidden w-1/4 bg-gradient-to-r from-transparent via-white/[0.06] to-transparent md:block'
            />

            <AnimateInView
              className='relative mx-auto max-w-2xl text-center'
              animation='scale-in'
            >
              <h2 className='text-3xl leading-tight font-bold tracking-tight md:text-4xl'>
                {t('Give every AI app')}
                <br />
                <span className='text-gradient-muted neon-text-drop inline-block bg-clip-text pb-1 text-transparent'>
                  {t('one governed brain')}
                </span>
              </h2>
              <p className='text-muted-foreground/80 mx-auto mt-5 max-w-md text-sm leading-relaxed md:text-base'>
                {t(
                  'Connect, configure and ship — one platform for model management and application supply.'
                )}
              </p>
              <div className='mt-8 flex flex-wrap items-center justify-center gap-3'>
                {!props.isAuthenticated && (
                  <Button
                    className='btn-neon group relative h-11 overflow-hidden rounded-lg px-5'
                    render={<Link to='/sign-in' />}
                  >
                    <span
                      aria-hidden
                      className='animate-neon-beam pointer-events-none absolute inset-y-0 left-0 w-1/3 bg-gradient-to-r from-transparent via-white/30 to-transparent'
                    />
                    {t('Get Started')}
                    <ArrowRight className='ml-1 size-3.5 transition-transform duration-200 group-hover:translate-x-0.5' />
                  </Button>
                )}
                <Button
                  variant='outline'
                  className='neon-ring border-border/60 h-11 rounded-lg px-5 dark:border-white/10 dark:bg-white/[0.03] dark:hover:bg-white/[0.06]'
                  render={<Link to='/dashboard' />}
                >
                  {props.isAuthenticated
                    ? t('Go to Dashboard')
                    : t('Explore Console')}
                </Button>
                <Button
                  variant='ghost'
                  className='text-muted-foreground hover:text-foreground inline-flex h-11 items-center gap-1.5 rounded-lg px-3 text-sm font-medium'
                  render={
                    <a
                      href='https://github.com/cqai-club/cqai-relay'
                      target='_blank'
                      rel='noopener noreferrer'
                    />
                  }
                >
                  <GitBranch className='size-4' />
                  <span>GitHub</span>
                </Button>
              </div>
              <p className='text-muted-foreground/50 mt-6 text-xs'>
                {t(
                  'Supports private deployment and custom domestic model onboarding.'
                )}
              </p>
            </AnimateInView>
          </div>
        </div>
      </div>
    </section>
  )
}
