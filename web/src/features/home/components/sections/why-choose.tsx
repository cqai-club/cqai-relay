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
  Building,
  CircleCheck,
  Cpu,
  Layers,
  ShieldCheck,
  Sparkles,
} from 'lucide-react'

import { AnimateInView } from '@/components/animate-in-view'

interface ChoosePoint {
  title: string
  desc: string
  icon: React.ReactNode
}

export function WhyChoose() {
  const { t } = useTranslation()

  const points: ChoosePoint[] = [
    {
      title: t('App-centric governance'),
      desc: t(
        'Not a bare API key — policy management organized around every application.'
      ),
      icon: <Layers className='size-5' strokeWidth={1.5} />,
    },
    {
      title: t('Domestic models first'),
      desc: t(
        'Adapted around the domestic model ecosystem, with a growing catalog.'
      ),
      icon: <Cpu className='size-5' strokeWidth={1.5} />,
    },
    {
      title: t('One protocol, low friction'),
      desc: t('OpenAI-compatible interfaces, so existing apps migrate smoothly.'),
      icon: <CircleCheck className='size-5' strokeWidth={1.5} />,
    },
    {
      title: t('Observable & metered'),
      desc: t('Transparent calls, costs and logs across the whole chain.'),
      icon: <Sparkles className='size-5' strokeWidth={1.5} />,
    },
    {
      title: t('Private deployment'),
      desc: t('Runs in enterprise environments for data security and compliance.'),
      icon: <ShieldCheck className='size-5' strokeWidth={1.5} />,
    },
    {
      title: t('Proven by dogfooding'),
      desc: t('Our own AI products have been running on it for a long time.'),
      icon: <Building className='size-5' strokeWidth={1.5} />,
    },
  ]

  return (
    <section className='border-border/50 relative z-10 overflow-hidden border-t px-6 py-24 md:py-32 dark:border-white/[0.06]'>
      <div className='relative mx-auto max-w-6xl'>
        <AnimateInView className='mx-auto mb-14 max-w-2xl text-center'>
          <p className='text-muted-foreground mb-3 flex items-center justify-center gap-2 text-xs font-medium tracking-widest uppercase'>
            {t('Why Choose Relay')}
            <span className='bg-primary/40 h-px w-8' />
          </p>
          <h2 className='text-2xl font-bold tracking-tight md:text-3xl'>
            {t('Built around your apps,')}
            <br />
            <span className='text-gradient-muted bg-clip-text text-transparent'>
              {t('ready for the models ahead')}
            </span>
          </h2>
        </AnimateInView>

        <div className='grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3'>
          {points.map((point, index) => (
            <AnimateInView
              key={point.title}
              delay={index * 60}
              animation='fade-up'
              className='h-full'
            >
              <div className='border-border/40 bg-muted/15 group flex h-full items-start gap-4 rounded-2xl border p-6 transition-colors duration-300 dark:border-white/[0.07] dark:bg-white/[0.02]'>
                <span className='text-muted-foreground group-hover:text-primary border-border/50 bg-muted/30 group-hover:border-primary/40 flex size-11 shrink-0 items-center justify-center rounded-xl border transition-all duration-300 dark:border-white/10 dark:bg-white/[0.04] dark:group-hover:border-violet-400/40 dark:group-hover:text-violet-300'>
                  {point.icon}
                </span>
                <div className='min-w-0'>
                  <h3 className='text-sm font-semibold'>{point.title}</h3>
                  <p className='text-muted-foreground mt-1.5 text-sm leading-relaxed'>
                    {point.desc}
                  </p>
                </div>
              </div>
            </AnimateInView>
          ))}
        </div>
      </div>
    </section>
  )
}
