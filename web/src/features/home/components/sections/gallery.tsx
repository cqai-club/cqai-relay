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
import {
  ArrowUpRight,
  FilePen,
  Layers,
  MessageSquareText,
  Network,
  PackageOpen,
  Scissors,
  Sprout,
  WandSparkles,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'

import { NeonAurora } from '../neon-aurora'

interface AppCard {
  name: string
  tagline: string
  desc: string
  href: string
  icon: React.ReactNode
}

export function Gallery() {
  const { t } = useTranslation()

  const apps: AppCard[] = [
    {
      name: '易剪宝',
      tagline: t('AI video editing assistant'),
      desc: t('Video understanding, subtitles and smart finishing, powered by Relay.'),
      href: 'https://yijianbao.example.com',
      icon: <Scissors className='size-5' strokeWidth={1.5} />,
    },
    {
      name: '易文宝',
      tagline: t('AI writing & document assistant'),
      desc: t('Generation and rewriting with prompts and model policies managed in one place.'),
      href: 'https://yiwenbao.example.com',
      icon: <FilePen className='size-5' strokeWidth={1.5} />,
    },
    {
      name: '易销售宝',
      tagline: t('AI sales enablement'),
      desc: t('Script recommendations and content generation, throttled and billed per team.'),
      href: 'https://yixiaoshoubao.example.com',
      icon: <MessageSquareText className='size-5' strokeWidth={1.5} />,
    },
    {
      name: '无限画布',
      tagline: t('AI multimodal canvas'),
      desc: t('Image generation and understanding with unified routing across models.'),
      href: 'https://wuxianhuabu.example.com',
      icon: <WandSparkles className='size-5' strokeWidth={1.5} />,
    },
    {
      name: '生图工作台',
      tagline: t('Image generation workbench'),
      desc: t('A pooled image-model workspace with batch and parameter-level calls.'),
      href: 'https://shengtu.example.com',
      icon: <Layers className='size-5' strokeWidth={1.5} />,
    },
    {
      name: 'AI 轨迹加工平台',
      tagline: t('AI trajectory processing'),
      desc: t('Trajectory parsing, cleaning and analysis, scheduled over unified inference.'),
      href: 'https://guiji.example.com',
      icon: <Network className='size-5' strokeWidth={1.5} />,
    },
    {
      name: '智慧园林平台',
      tagline: t('Smart landscape platform'),
      desc: t('Image recognition and inspection analytics supplied under app policies.'),
      href: 'https://yuanlin.example.com',
      icon: <Sprout className='size-5' strokeWidth={1.5} />,
    },
    {
      name: '提示词中心',
      tagline: t('Prompt asset hub'),
      desc: t('Centralized prompt curation, reuse and versioning.'),
      href: 'https://tishici.example.com',
      icon: <PackageOpen className='size-5' strokeWidth={1.5} />,
    },
  ]

  return (
    <section className='relative z-10 overflow-hidden px-6 py-24 md:py-32'>
      <NeonAurora subtle>
        <div className='neon-dots absolute inset-0 [mask-image:radial-gradient(ellipse_60%_60%_at_50%_40%,black_10%,transparent_75%)] opacity-[0.12]' />
      </NeonAurora>

      <div className='relative mx-auto max-w-6xl'>
        <AnimateInView className='mx-auto mb-14 max-w-2xl text-center'>
          <p className='text-muted-foreground mb-3 flex items-center justify-center gap-2 text-xs font-medium tracking-widest uppercase'>
            {t('Product Gallery')}
            <span className='bg-primary/40 h-px w-8' />
          </p>
          <h2 className='text-2xl font-bold tracking-tight md:text-3xl'>
            {t('One gateway,')}
            <br />
            <span className='text-gradient-muted bg-clip-text text-transparent'>
              {t('a family of AI products')}
            </span>
          </h2>
          <p className='text-muted-foreground/80 mx-auto mt-4 max-w-xl text-sm leading-relaxed md:text-[15px]'>
            {t(
              'Every product below is onboarded on Relay — its own identity, its own model scope and quota, its own cost accounting. Click a card to visit the product.'
            )}
          </p>
        </AnimateInView>

        <div className='grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4'>
          {apps.map((app, index) => (
            <AnimateInView
              key={app.name}
              delay={index * 60}
              animation='fade-up'
              className='h-full'
            >
              <a
                href={app.href}
                target='_blank'
                rel='noopener noreferrer'
                aria-label={`${app.name} — ${app.tagline}`}
                className='spotlight-card group hover:bg-muted/20 border-border/40 bg-background/60 flex h-full flex-col rounded-2xl border p-5 transition-all duration-300 hover:-translate-y-1 dark:border-white/[0.07] dark:bg-white/[0.02] dark:hover:bg-white/[0.04]'
              >
                <div className='flex items-start justify-between'>
                  <span className='text-muted-foreground group-hover:text-primary border-border/50 bg-muted/30 group-hover:border-primary/40 flex size-11 items-center justify-center rounded-xl border transition-all duration-300 dark:border-white/10 dark:bg-white/[0.04] dark:group-hover:border-violet-400/40 dark:group-hover:text-violet-300'>
                    {app.icon}
                  </span>
                  <ArrowUpRight className='text-muted-foreground/40 group-hover:text-primary size-4 transition-all duration-300 group-hover:translate-x-0.5 group-hover:-translate-y-0.5' />
                </div>
                <h3 className='mt-4 text-base font-semibold'>{app.name}</h3>
                <p className='text-muted-foreground mt-0.5 text-xs'>{app.tagline}</p>
                <p className='text-muted-foreground mt-2 text-xs leading-relaxed'>
                  {app.desc}
                </p>
              </a>
            </AnimateInView>
          ))}
        </div>

        <p className='text-muted-foreground/40 mt-6 text-center text-[11px]'>
          {t('Product links are placeholders — replace with real URLs before launch.')}
        </p>
      </div>
    </section>
  )
}
