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

interface ModelGroup {
  id: string
  label: string
  models: string[]
}

export function ModelEcosystem() {
  const { t } = useTranslation()

  const groups: ModelGroup[] = [
    {
      id: 'text',
      label: t('Text / Reasoning'),
      models: [
        'DeepSeek',
        'Qwen',
        'GLM',
        'Kimi',
        'Doubao',
        'ERNIE Bot',
        'MiniMax',
        'Spark',
      ],
    },
    {
      id: 'image',
      label: t('Image / Multimodal'),
      models: ['Wanx', 'Seedream', 'Kolors'],
    },
    {
      id: 'video',
      label: t('Video Generation'),
      models: ['Kling', 'Wan', 'Vidu'],
    },
  ]

  const points = [
    {
      title: t('One protocol'),
      desc: t('OpenAI-compatible API — a single code path reaches every model.'),
    },
    {
      title: t('Plug and play'),
      desc: t('Add or replace a model without touching downstream apps.'),
    },
    {
      title: t('Private & open models'),
      desc: t('Self-hosted and open-source models fit right in.'),
    },
    {
      title: t('Always expanding'),
      desc: t('The list keeps growing as new models ship.'),
    },
  ]

  return (
    <section className='border-border/50 relative z-10 overflow-hidden border-t px-6 py-24 md:py-32 dark:border-white/[0.06]'>
      <div className='relative mx-auto max-w-6xl'>
        <AnimateInView className='mx-auto mb-14 max-w-2xl text-center'>
          <p className='text-muted-foreground mb-3 flex items-center justify-center gap-2 text-xs font-medium tracking-widest uppercase'>
            {t('Supported Models')}
            <span className='bg-primary/40 h-px w-8' />
          </p>
          <h2 className='text-2xl font-bold tracking-tight md:text-3xl'>
            {t('Leading large models,')}
            <br />
            <span className='text-gradient-muted bg-clip-text text-transparent'>
              {t('all behind one gateway')}
            </span>
          </h2>
          <p className='text-muted-foreground/80 mt-4 text-sm leading-relaxed md:text-[15px]'>
            {t(
              'Continuously adapted to the latest model ecosystem — text, image, video and voice — unified interfaces that are ready to use on day one.'
            )}
          </p>
        </AnimateInView>

        <div className='space-y-8'>
          {groups.map((group, groupIndex) => (
            <AnimateInView key={group.id} delay={groupIndex * 100}>
              <div className='border-border/40 bg-muted/15 flex flex-col items-center gap-4 rounded-2xl border px-6 py-5 md:flex-row md:gap-8 dark:border-white/[0.07] dark:bg-white/[0.02]'>
                <span className='text-muted-foreground shrink-0 text-xs font-semibold tracking-widest uppercase'>
                  {group.label}
                </span>
                <div className='flex flex-wrap items-center justify-center gap-2.5 md:justify-start'>
                  {group.models.map((model) => (
                    <span
                      key={model}
                      className='border-border/40 text-foreground/80 hover:border-primary/40 hover:text-primary bg-background/60 rounded-lg border px-3.5 py-1.5 text-xs font-semibold transition-colors duration-200 dark:bg-white/[0.03]'
                    >
                      {model}
                    </span>
                  ))}
                </div>
              </div>
            </AnimateInView>
          ))}
        </div>

        <div className='mt-12 grid grid-cols-2 gap-8 md:grid-cols-4 md:gap-12'>
          {points.map((point, index) => (
            <AnimateInView
              key={point.title}
              delay={index * 100}
              animation='fade-up'
              className='flex flex-col items-center text-center'
            >
              <h3 className='text-sm font-semibold'>{point.title}</h3>
              <p className='text-muted-foreground mt-2 max-w-[220px] text-xs leading-relaxed'>
                {point.desc}
              </p>
            </AnimateInView>
          ))}
        </div>
      </div>
    </section>
  )
}
