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
import { ArrowRight, BookOpen, GitBranch } from 'lucide-react'
import { useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'

import { NeonAurora } from '../neon-aurora'
import { HeroNetwork } from '../hero-network'

interface HeroProps {
  className?: string
  isAuthenticated?: boolean
}

export function Hero(props: HeroProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const sectionRef = useRef<HTMLElement>(null)
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.relay.example.com'

  const handlePointerMove = (event: React.PointerEvent<HTMLElement>) => {
    const section = sectionRef.current
    if (!section) return
    const rect = section.getBoundingClientRect()
    const x = event.clientX - rect.left
    const y = event.clientY - rect.top
    section.style.setProperty('--hero-spot-x', `${x}px`)
    section.style.setProperty('--hero-spot-y', `${y}px`)
  }

  const renderDocsButton = () => {
    const isExternal = docsUrl.startsWith('http')
    return (
      <Button
        variant='outline'
        className='group border-border/50 hover:border-border hover:bg-muted/50 inline-flex h-11 items-center gap-1.5 rounded-lg px-5 text-sm font-medium dark:border-white/10 dark:bg-white/[0.03] dark:hover:border-white/20 dark:hover:bg-white/[0.06]'
        render={
          isExternal ? (
            <a href={docsUrl} target='_blank' rel='noopener noreferrer' />
          ) : (
            <Link to={docsUrl} />
          )
        }
      >
        <BookOpen className='text-muted-foreground/80 group-hover:text-foreground size-4 transition-colors duration-200' />
        <span>{t('Docs')}</span>
      </Button>
    )
  }

  return (
    <section
      ref={sectionRef}
      onPointerMove={handlePointerMove}
      className='relative z-10 overflow-hidden px-6 pt-24 pb-16 md:pt-32 md:pb-24 lg:pt-36 lg:pb-28'
    >
      <NeonAurora subtle>
        <div className='neon-grid absolute inset-0 [mask-image:radial-gradient(ellipse_70%_60%_at_50%_20%,black_15%,transparent_75%)] opacity-[0.09] dark:opacity-16' />
        <div
          className='absolute inset-0 hidden transition-opacity duration-300 lg:block'
          style={{
            background:
              'radial-gradient(620px circle at var(--hero-spot-x, 50%) var(--hero-spot-y, 30%), color-mix(in oklch, var(--chart-1) 8%, transparent), transparent 70%)',
          }}
        />
      </NeonAurora>

      <div className='mx-auto grid max-w-6xl grid-cols-1 items-start gap-12 lg:grid-cols-12 lg:gap-8'>
        <div className='flex flex-col items-start text-left lg:col-span-7'>
          <div
            className='landing-animate-fade-up border-primary/25 bg-primary/5 text-primary dark:border-primary/30 dark:bg-primary/10 dark:text-primary mb-5 inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-[11px] font-medium opacity-0 shadow-xs backdrop-blur-sm'
            style={{ animationDelay: '0ms' }}
          >
            <span className='relative flex size-1.5'>
              <span className='bg-primary absolute inline-flex h-full w-full animate-ping rounded-full opacity-75' />
              <span className='bg-primary relative inline-flex size-1.5 rounded-full' />
            </span>
            <span>{t('Relay AI Gateway')}</span>
          </div>

          <h1
            className='landing-animate-fade-up text-[clamp(2.25rem,4.5vw,3.4rem)] leading-[1.1] font-bold tracking-tight'
            style={{ animationDelay: '60ms' }}
          >
            {t('One gateway for every model,')}
            <br />
            <span className='text-gradient-muted neon-text-drop inline-block bg-clip-text pb-1 text-transparent'>
              {t('policy-driven power for every AI app')}
            </span>
          </h1>

          <p
            className='text-muted-foreground/80 landing-animate-fade-up mt-6 max-w-xl text-base leading-relaxed opacity-0 md:text-[15px]'
            style={{ animationDelay: '120ms' }}
          >
            {t(
              'Relay connects domestic large models upward and every AI application downward — each app consumes AI under its own policies: on-demand, governed and metered.'
            )}
          </p>

          <div
            className='landing-animate-fade-up mt-8 flex flex-wrap items-center gap-3 opacity-0'
            style={{ animationDelay: '180ms' }}
          >
            {props.isAuthenticated ? (
              <>
                <Button
                  className='btn-neon group h-11 rounded-lg px-5 text-sm font-medium'
                  render={<Link to='/dashboard' />}
                >
                  {t('Go to Dashboard')}
                  <ArrowRight className='ml-1.5 size-4 transition-transform duration-200 group-hover:translate-x-0.5' />
                </Button>
                {renderDocsButton()}
              </>
            ) : (
              <>
                <Button
                  className='btn-neon group relative h-11 overflow-hidden rounded-lg px-5 text-sm font-medium'
                  render={<Link to='/sign-in' />}
                >
                  <span
                    aria-hidden
                    className='animate-neon-beam pointer-events-none absolute inset-y-0 left-0 w-1/3 bg-gradient-to-r from-transparent via-white/30 to-transparent'
                  />
                  {t('Get Started')}
                  <ArrowRight className='ml-1.5 size-4 transition-transform duration-200 group-hover:translate-x-0.5' />
                </Button>
                {renderDocsButton()}
              </>
            )}

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

          <div
            className='landing-animate-fade-up text-muted-foreground/60 mt-9 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs opacity-0'
            style={{ animationDelay: '240ms' }}
          >
            <span>{t('Continuously adapting domestic models')}</span>
            <span aria-hidden className='text-muted-foreground/30'>·</span>
            <span>{t('Private deployment ready')}</span>
            <span aria-hidden className='text-muted-foreground/30'>·</span>
            <span>{t('Running our own AI products today')}</span>
          </div>
        </div>

        <div
          className='landing-animate-fade-up flex w-full justify-center opacity-0 lg:col-span-5'
          style={{ animationDelay: '320ms' }}
        >
          <div className='relative w-full max-w-md'>
            <div
              aria-hidden
              className='absolute inset-0 -z-10 rounded-full bg-gradient-to-br from-violet-500/8 via-transparent to-purple-500/8 blur-2xl'
            />
            <div className='relative mx-auto aspect-square w-full'>
              <HeroNetwork />
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
