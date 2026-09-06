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
import { useId, type ComponentType } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Activity,
  BarChart3,
  CreditCard,
  Eye,
  Gauge,
  Route,
  ScrollText,
  ShieldCheck,
} from 'lucide-react'

import { DeepSeek, Doubao, Kling, Moonshot, Qwen, Zhipu } from '@lobehub/icons'

const CX = 300
const CY = 300

const HUB_R = 56
const CAP_R = 104

type Capability = {
  id: string
  labelKey: string
  Icon: ComponentType<{ className?: string; strokeWidth?: number | string }>
}

const CAPABILITIES: Capability[] = [
  { id: 'routing', labelKey: 'Routing', Icon: Route },
  { id: 'policies', labelKey: 'Policies', Icon: ShieldCheck },
  { id: 'billing', labelKey: 'Billing', Icon: CreditCard },
  { id: 'ratelimit', labelKey: 'Rate limits', Icon: Gauge },
  { id: 'analytics', labelKey: 'Analytics', Icon: BarChart3 },
  { id: 'metering', labelKey: 'Metering', Icon: Activity },
  { id: 'observability', labelKey: 'Observability', Icon: Eye },
  { id: 'audit', labelKey: 'Audit', Icon: ScrollText },
]

type OuterNode = {
  id: string
  label: string
  icon?: ComponentType<{ width?: string | number; height?: string | number }>
  tone?: string
}

const ALL_NODES: OuterNode[] = [
  { id: 'deepseek', label: 'DeepSeek', icon: DeepSeek, tone: '#4B8BEA' },
  { id: 'qwen', label: 'Qwen', icon: Qwen, tone: '#7667E8' },
  { id: 'glm', label: 'GLM', icon: Zhipu, tone: '#0EB6C0' },
  { id: 'kimi', label: 'Kimi', icon: Moonshot, tone: '#2B7CFF' },
  { id: 'doubao', label: 'Doubao', icon: Doubao, tone: '#325AB4' },
  { id: 'kling', label: 'Kling', icon: Kling, tone: '#C9A6E8' },
  { id: 'clip', label: '易剪宝', tone: '#F0BE97' },
  { id: 'doc', label: '易文宝', tone: '#B6D59C' },
  { id: 'canvas', label: '无限画布', tone: '#A5C8F0' },
  { id: 'workbench', label: '生图工作台', tone: '#E8C4A0' },
  { id: 'garden', label: '智慧园林', tone: '#9CD6B8' },
  { id: 'prompt', label: '提示词中心', tone: '#C9B6E8' },
]

// Three tilted elliptical orbits (classic atom look). Models and apps are
// mixed across orbits so the motion never reads as two uniform rings.
const ORBITS = [
  { tilt: 0, rx: 256, ry: 168, dur: 30, nodeIds: ['deepseek', 'clip', 'qwen', 'doc'] },
  { tilt: 60, rx: 256, ry: 168, dur: 38, nodeIds: ['glm', 'canvas', 'kimi', 'workbench'] },
  { tilt: 120, rx: 256, ry: 168, dur: 46, nodeIds: ['doubao', 'garden', 'kling', 'prompt'] },
] as const

/**
 * Hero-side "App + Policy" network visualisation.
 *
 * A hub in the middle (AI data flow) is ringed by eight gateway capabilities
 * (routing, policies, billing, rate limits, analytics, metering,
 * observability, audit) that slowly revolve on an inner track, while model
 * and app chips orbit tilted elliptical tracks outside. Pure SVG + SMIL /
 * CSS keyframes; prefers-reduced-motion users see a static network.
 */
export function HeroNetwork() {
  const { t } = useTranslation()
  const uid = useId().replaceAll(/[^a-zA-Z0-9_-]/g, '')
  const gid = (suffix: string) => `hero-net-${uid}-${suffix}`
  const glowId = gid('glow')

  return (
    <svg
      className='hero-net h-full w-full'
      viewBox='0 0 600 600'
      fill='none'
      xmlns='http://www.w3.org/2000/svg'
      role='img'
      aria-labelledby={`${gid('title')} ${gid('desc')}`}
    >
      <title id={gid('title')}>{t('Gateway capability network')}</title>
      <desc id={gid('desc')}>
        {t(
          'AI data flows through the Relay gateway, governed by routing, policies, billing, rate limits, analytics, metering, observability and audit.'
        )}
      </desc>

      <defs>
        <filter id={glowId} x='-80%' y='-80%' width='260%' height='260%'>
          <feGaussianBlur stdDeviation='2.5' result='blur' />
          <feMerge>
            <feMergeNode in='blur' />
            <feMergeNode in='SourceGraphic' />
          </feMerge>
        </filter>
      </defs>

      {/* inner track the capability nodes revolve on */}
      <circle cx={CX} cy={CY} r={CAP_R} className='hero-net-orbit' />

      {/* capability nodes slowly revolving around the hub */}
      {CAPABILITIES.map((cap, i) => {
        const CapIcon = cap.Icon
        return (
          <g key={cap.id} className='hero-net-cap'>
            <circle r='21' className='hero-net-cap-circle' />
            <foreignObject
              x={-9}
              y={-9}
              width='18'
              height='18'
              className='hero-net-cap-ico'
            >
              <div className='flex h-full w-full items-center justify-center text-primary'>
                <CapIcon className='size-4' strokeWidth={1.75} />
              </div>
            </foreignObject>
            <text y='38' textAnchor='middle' className='hero-net-cap-label'>
              {t(cap.labelKey)}
            </text>
            <animateMotion
              dur='90s'
              begin={`${-i * (90 / 8)}s`}
              repeatCount='indefinite'
              rotate='0'
              path={`M ${CX + CAP_R} ${CY} a ${CAP_R} ${CAP_R} 0 1 1 -${CAP_R * 2} 0 a ${CAP_R} ${CAP_R} 0 1 1 ${CAP_R * 2} 0`}
            />
          </g>
        )
      })}

      {/* ── tilted elliptical orbits (atom style) ── */}
      {ORBITS.map((orbit) => {
        const ellipsePath = `M ${CX - orbit.rx} ${CY} a ${orbit.rx} ${orbit.ry} 0 1 1 ${orbit.rx * 2} 0 a ${orbit.rx} ${orbit.ry} 0 1 1 ${-orbit.rx * 2} 0`
        return (
          <g key={orbit.tilt} transform={`rotate(${orbit.tilt} ${CX} ${CY})`}>
            <ellipse
              cx={CX}
              cy={CY}
              rx={orbit.rx}
              ry={orbit.ry}
              className='hero-net-track'
            />
            {orbit.nodeIds.map((id, i) => {
              const node = ALL_NODES.find((n) => n.id === id)
              if (!node) return null
              return (
                <g key={id} className='hero-net-electron'>
                  <g transform={`rotate(${-orbit.tilt} 0 0)`}>
                    <circle r='17' className='hero-net-chip-ring' />
                    {node.icon ? (
                      <svg
                        x={-9}
                        y={-9}
                        width='18'
                        height='18'
                        viewBox='0 0 24 24'
                        style={{ color: node.tone }}
                      >
                        <node.icon width='100%' height='100%' />
                      </svg>
                    ) : (
                      <text
                        y='4.5'
                        textAnchor='middle'
                        className='hero-net-chip-app'
                        style={{ fill: node.tone }}
                      >
                        {node.label.slice(0, 2)}
                      </text>
                    )}
                    <text y='33' textAnchor='middle' className='hero-net-chip-label'>
                      {node.label}
                    </text>
                  </g>
                  <animateMotion
                    dur={`${orbit.dur}s`}
                    begin={`${-i * (orbit.dur / 4)}s`}
                    repeatCount='indefinite'
                    rotate='0'
                    path={ellipsePath}
                  />
                </g>
              )
            })}
          </g>
        )
      })}

      {/* center hub: AI data flow */}
      <g className='hero-net-hub'>
        <circle cx={CX} cy={CY} r={HUB_R} className='hero-net-hub-ring' />
        <circle cx={CX} cy={CY} r={HUB_R - 9} className='hero-net-hub-core' />
        {/* rotating data flow rings */}
        <g className='hero-net-flow-ring hero-net-flow-ring-a'>
          <circle cx={CX} cy={CY} r={HUB_R - 17} className='hero-net-flow-track' />
          <circle cx={CX - (HUB_R - 17)} cy={CY} r='2.6' className='hero-net-flow-dot' filter={`url(#${glowId})`} />
        </g>
        <g className='hero-net-flow-ring hero-net-flow-ring-b'>
          <circle cx={CX} cy={CY} r={HUB_R - 27} className='hero-net-flow-track' />
          <circle cx={CX + (HUB_R - 27)} cy={CY} r='2' className='hero-net-flow-dot' filter={`url(#${glowId})`} />
        </g>
        <text x={CX} y={CY + 4} textAnchor='middle' className='hero-net-hub-title'>
          {t('AI')}
        </text>
        <text x={CX} y={CY + 20} textAnchor='middle' className='hero-net-hub-sub'>
          {t('data flow')}
        </text>
      </g>
    </svg>
  )
}
