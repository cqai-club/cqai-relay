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
import { useId, type ComponentType, type CSSProperties, type SVGProps } from 'react'
import { useTranslation } from 'react-i18next'

import { DeepSeek, Doubao, Kling, Moonshot, Qwen, Zhipu } from '@lobehub/icons'

interface UpstreamModel {
  id: string
  label: string
  tone: string
  /** Brand icon; omitted for the "more models" node. */
  Icon?: ComponentType<SVGProps<SVGSVGElement>>
}

const UPSTREAM_MODELS: UpstreamModel[] = [
  { id: 'deepseek', label: 'DeepSeek', tone: '#4B8BEA', Icon: DeepSeek },
  { id: 'qwen', label: 'Qwen', tone: '#7667E8', Icon: Qwen },
  { id: 'glm', label: 'GLM', tone: '#0EB6C0', Icon: Zhipu },
  { id: 'kimi', label: 'Kimi', tone: '#2B7CFF', Icon: Moonshot },
  { id: 'doubao', label: 'Doubao', tone: '#325AB4', Icon: Doubao },
  { id: 'kling', label: 'Kling', tone: '#C9A6E8', Icon: Kling },
  { id: 'more', label: 'More', tone: 'var(--muted-foreground)' },
]

const DOWNSTREAM_APPS = [
  '易剪宝',
  '易文宝',
  '易销售宝',
  '无限画布',
  '生图工作台',
  'AI 轨迹加工',
  '智慧园林',
  '提示词中心',
] as const

/** Model node x-centers: 7 across a 960-wide canvas, 128px apart. */
const MODEL_X = [96, 224, 352, 480, 608, 736, 864]
/** App node x-centers: 8 across a 960-wide canvas, 110px apart. */
const APP_X = [96, 206, 316, 426, 536, 646, 756, 866]
const MODEL_Y = 62
const APP_Y = 470
const GATEWAY_TOP = 205
const GATEWAY_BOTTOM = 295
const GATEWAY_CX = 480

/**
 * Vertical architecture flow diagram for the home "Architecture" section.
 *
 * Layout (top → bottom):
 *   model pool (multiple upstream model nodes)
 *        │  requests flow DOWN and converge
 *        ▼
 *   Relay AI Gateway (unified access / routing / policies / metering)
 *        │  responses flow DOWN and fan out
 *        ▼
 *   our AI apps (multiple downstream application nodes)
 *
 * Pure SVG + CSS keyframes, theme-adaptive via CSS variables. Signal dots
 * travel along each path to visualise traffic; reduced-motion users get a
 * static diagram.
 */
export function ArchitectureDiagram() {
  const { t } = useTranslation()
  const uid = useId().replaceAll(/[^a-zA-Z0-9_-]/g, '')
  const gid = (suffix: string) => `relay-arch-${uid}-${suffix}`
  const glowId = gid('glow')
  const arrowId = gid('arrow')

  const upstreamPaths = MODEL_X.map(
    (x) => `M${x} ${MODEL_Y + 30} C${x} ${GATEWAY_TOP - 70} ${GATEWAY_CX} ${GATEWAY_TOP - 60} ${GATEWAY_CX} ${GATEWAY_TOP + 8}`
  )
  const downstreamPaths = APP_X.map(
    (x) => `M${GATEWAY_CX} ${GATEWAY_BOTTOM - 8} C${GATEWAY_CX} ${GATEWAY_BOTTOM + 70} ${x} ${GATEWAY_BOTTOM + 80} ${x} ${APP_Y - 28}`
  )
  // Reverse (return) paths so signal dots can travel back up the same curves.
  const upstreamReturnPaths = MODEL_X.map(
    (x) => `M${GATEWAY_CX} ${GATEWAY_TOP + 8} C${GATEWAY_CX} ${GATEWAY_TOP - 60} ${x} ${GATEWAY_TOP - 70} ${x} ${MODEL_Y + 30}`
  )
  const downstreamReturnPaths = APP_X.map(
    (x) => `M${x} ${APP_Y - 28} C${x} ${GATEWAY_BOTTOM + 80} ${GATEWAY_CX} ${GATEWAY_BOTTOM + 70} ${GATEWAY_CX} ${GATEWAY_BOTTOM - 8}`
  )

  return (
    <svg
      className='relay-arch h-full w-full'
      viewBox='0 0 960 560'
      fill='none'
      xmlns='http://www.w3.org/2000/svg'
      role='img'
      aria-labelledby={`${gid('title')} ${gid('desc')}`}
    >
      <title id={gid('title')}>{t('Relay architecture diagram')}</title>
      <desc id={gid('desc')}>
        {t(
          'Traffic flows in both directions between the upstream model pool, the Relay gateway and the downstream AI applications.'
        )}
      </desc>

      <defs>
        <filter id={glowId} x='-80%' y='-80%' width='260%' height='260%'>
          <feGaussianBlur stdDeviation='3.5' result='blur' />
          <feMerge>
            <feMergeNode in='blur' />
            <feMergeNode in='SourceGraphic' />
          </feMerge>
        </filter>
        <marker
          id={arrowId}
          viewBox='0 0 8 8'
          refX='7'
          refY='4'
          markerWidth='4.5'
          markerHeight='4.5'
          orient='auto'
        >
          <path d='M0 0L8 4L0 8Z' fill='var(--primary)' fillOpacity='0.55' />
        </marker>
      </defs>

      {/* ── Two-way traffic lines (models ↔ gateway) ── */}
      <g className='arch-in'>
        {UPSTREAM_MODELS.map((model, i) => {
          const path = upstreamPaths[i]
          const returnPath = upstreamReturnPaths[i]
          return (
            <g key={model.id}>
              <path d={path} className='arch-line-base' markerEnd={`url(#${arrowId})`} />
              <path d={path} className='arch-line-flow' />
              <path d={path} className='arch-line-flow-up' />
              <circle className='arch-dot' r='3' fill={model.tone} filter={`url(#${glowId})`}>
                <animateMotion
                  path={path}
                  begin={`${-i * 0.4}s`}
                  dur='2.6s'
                  repeatCount='indefinite'
                />
              </circle>
              <circle className='arch-dot arch-dot-up' r='2.5'>
                <animateMotion
                  path={returnPath}
                  begin={`${-i * 0.4 - 1.3}s`}
                  dur='2.6s'
                  repeatCount='indefinite'
                />
              </circle>
            </g>
          )
        })}
      </g>

      {/* ── Two-way traffic lines (gateway ↔ apps) ── */}
      <g className='arch-out'>
        {DOWNSTREAM_APPS.map((app, i) => {
          const path = downstreamPaths[i]
          const returnPath = downstreamReturnPaths[i]
          return (
            <g key={app}>
              <path d={path} className='arch-line-base' markerEnd={`url(#${arrowId})`} />
              <path d={path} className='arch-line-flow' />
              <path d={path} className='arch-line-flow-up' />
              <circle className='arch-dot' r='3' fill={app.includes('易') ? '#F0BE97' : '#B6D59C'} filter={`url(#${glowId})`}>
                <animateMotion
                  path={path}
                  begin={`${-i * 0.3}s`}
                  dur='2.4s'
                  repeatCount='indefinite'
                />
              </circle>
              <circle className='arch-dot arch-dot-up' r='2.5'>
                <animateMotion
                  path={returnPath}
                  begin={`${-i * 0.3 - 1.2}s`}
                  dur='2.4s'
                  repeatCount='indefinite'
                />
              </circle>
            </g>
          )
        })}
      </g>

      {/* ── Upstream model nodes ── */}
      {UPSTREAM_MODELS.map((model, i) => {
        const cx = MODEL_X[i]
        const x = cx - 55
        const labelWide = model.label.length > 6
        const Icon = model.Icon
        const isMore = !Icon
        let textOffset = 8
        if (isMore) {
          textOffset = 0
        } else if (labelWide) {
          textOffset = 10
        }
        return (
          <g
            key={model.id}
            className='arch-node'
            role='img'
            aria-label={isMore ? t('More models') : model.label}
            style={{ '--arch-delay': `${120 + i * 70}ms` } as CSSProperties}
          >
            <rect
              x={x}
              y={MODEL_Y - 30}
              width='110'
              height='44'
              rx='10'
              className={isMore ? 'arch-node-box arch-node-box-dash' : 'arch-node-box'}
            />
            {Icon ? (
              <svg
                x={cx - 50}
                y={MODEL_Y - 19}
                width='22'
                height='22'
                viewBox='0 0 24 24'
                style={{ color: model.tone }}
              >
                <Icon width='100%' height='100%' />
              </svg>
            ) : null}
            <text
              x={cx + textOffset}
              y={MODEL_Y - 2}
              textAnchor='middle'
              className={isMore ? 'arch-more-label' : 'arch-node-label'}
            >
              {isMore ? t('More models') : model.label}
            </text>
            <circle cx={cx + 47} cy={MODEL_Y - 20} r='2' className='arch-status' />
          </g>
        )
      })}

      {/* ── Gateway hub ── */}
      <g className='arch-node' style={{ '--arch-delay': '620ms' } as CSSProperties}>
        <rect
          x={GATEWAY_CX - 170}
          y={GATEWAY_TOP}
          width='340'
          height={GATEWAY_BOTTOM - GATEWAY_TOP}
          rx='18'
          className='arch-gateway'
        />
        <text
          x={GATEWAY_CX}
          y={GATEWAY_TOP + 42}
          textAnchor='middle'
          className='arch-gateway-title'
        >
          Relay AI Gateway
        </text>
        <text
          x={GATEWAY_CX}
          y={GATEWAY_TOP + 66}
          textAnchor='middle'
          className='arch-gateway-sub'
        >
          {t('Unified access · routing · policies · metering')}
        </text>
      </g>

      {/* ── Downstream app nodes ── */}
      {DOWNSTREAM_APPS.map((app, i) => {
        const cx = APP_X[i]
        const x = cx - 40
        const wide = app.length > 4
        return (
          <g
            key={app}
            className='arch-node'
            role='img'
            aria-label={app}
            style={{ '--arch-delay': `${900 + i * 70}ms` } as CSSProperties}
          >
            <rect x={x} y={APP_Y - 26} width='80' height='42' rx='10' className='arch-node-box' />
            <circle cx={cx - 26} cy={APP_Y - 5} r='3.5' fill={app.includes('易') ? '#F0BE97' : '#B6D59C'} />
            <text
              x={cx + (wide ? 8 : 6)}
              y={APP_Y + 1}
              textAnchor='middle'
              className='arch-app-label'
            >
              {app}
            </text>
            <circle cx={cx + 32} cy={APP_Y - 17} r='2' className='arch-status' />
          </g>
        )
      })}
    </svg>
  )
}
