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
import {
  Activity,
  CircleDollarSign,
  FileText,
  KeyRound,
  Route,
  SlidersHorizontal,
} from 'lucide-react'

import { AnimateInView } from '@/components/animate-in-view'

import { NeonAurora } from '../neon-aurora'

interface StrategyCard {
  title: string
  desc: string
  icon: React.ReactNode
}

export function Strategy() {
  const { t } = useTranslation()

  const cards: StrategyCard[] = [
    {
      title: t('Model Routing'),
      desc: t(
        'Scope which models each app can reach; distribute by weight and priority across providers, with automatic failover.'
      ),
      icon: <Route className='size-5' strokeWidth={1.5} />,
    },
    {
      title: t('Access Control'),
      desc: t(
        'App-level credentials, permission groups, key management and request-source control.'
      ),
      icon: <KeyRound className='size-5' strokeWidth={1.5} />,
    },
    {
      title: t('Rate Limits & Quotas'),
      desc: t(
        'Concurrency and rate caps per app or group, hard quota ceilings and overage protection.'
      ),
      icon: <SlidersHorizontal className='size-5' strokeWidth={1.5} />,
    },
    {
      title: t('Metering & Billing'),
      desc: t(
        'Costs are computed from model price and ratio automatically, attributed to each app precisely.'
      ),
      icon: <CircleDollarSign className='size-5' strokeWidth={1.5} />,
    },
    {
      title: t('Monitoring & Alerts'),
      desc: t(
        'Real-time visibility into calls, latency, success rate and failures, with instant alerting.'
      ),
      icon: <Activity className='size-5' strokeWidth={1.5} />,
    },
    {
      title: t('Logs & Audit'),
      desc: t(
        'Full call logs across the chain, so issues are traceable and consumption is auditable.'
      ),
      icon: <FileText className='size-5' strokeWidth={1.5} />,
    },
  ]

  return (
    <section className='relative z-10 overflow-hidden px-6 py-24 md:py-32'>
      <NeonAurora subtle>
        <div className='neon-grid absolute inset-0 [mask-image:radial-gradient(ellipse_60%_60%_at_50%_40%,black_10%,transparent_75%)] opacity-[0.1] dark:opacity-[0.16]' />
      </NeonAurora>

      <div className='relative mx-auto max-w-6xl'>
        <AnimateInView className='mx-auto mb-14 max-w-2xl text-center'>
          <p className='text-muted-foreground mb-3 flex items-center justify-center gap-2 text-xs font-medium tracking-widest uppercase'>
            {t('Apps & Policies')}
            <span className='bg-primary/40 h-px w-8' />
          </p>
          <h2 className='text-2xl font-bold tracking-tight md:text-3xl'>
            {t('Models are resources,')}
            <br />
            <span className='text-gradient-muted bg-clip-text text-transparent'>
              {t('policies are configuration')}
            </span>
          </h2>
          <p className='text-muted-foreground/80 mt-4 text-sm leading-relaxed md:text-[15px]'>
            {t(
              'On Relay every AI application is a first-class member: it declares which models and AI services it needs, and the platform supplies and governs them through app-level policies.'
            )}
          </p>
        </AnimateInView>

        <div className='border-border/40 bg-border/40 grid gap-px overflow-hidden rounded-2xl border md:grid-cols-3 dark:border-white/[0.07] dark:bg-white/[0.06]'>
          {cards.map((card, index) => (
            <AnimateInView
              key={card.title}
              delay={index * 80}
              animation='scale-in'
              className='spotlight-card group hover:bg-muted/20 bg-background/90 p-7 transition-colors duration-300 dark:bg-[#100b1d]/95 dark:hover:bg-white/[0.03]'
            >
              <div className='text-muted-foreground group-hover:text-primary border-border/50 bg-muted/30 group-hover:border-primary/40 mb-4 flex size-11 items-center justify-center rounded-xl border transition-all duration-300 dark:border-white/10 dark:bg-white/[0.04] dark:group-hover:border-violet-400/40 dark:group-hover:text-violet-300'>
                {card.icon}
              </div>
              <h3 className='text-sm font-semibold'>{card.title}</h3>
              <p className='text-muted-foreground mt-2 text-sm leading-relaxed'>
                {card.desc}
              </p>
            </AnimateInView>
          ))}
        </div>
      </div>
    </section>
  )
}
