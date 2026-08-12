import { extent, scaleLinear } from 'd3'
import { ArrowRight, Bot, GitBranch } from 'lucide-react'
import { useMemo } from 'react'
import type { RampResponse } from '../lib/api'

interface RampChartProps { ramps: RampResponse | null; loading: boolean }

const percent = new Intl.NumberFormat(undefined, { style: 'percent', maximumFractionDigits: 0, signDisplay: 'exceptZero' })

export function RampChart({ ramps, loading }: RampChartProps) {
  if (loading) return <div className="h-[440px] animate-pulse rounded-2xl bg-white/[0.03]" aria-label="Loading observed lift" />
  if (!ramps || ramps.handoffs.length === 0 && ramps.adoptions.length === 0) return <Empty />
  return <div>{ramps.handoffs.length > 0 ? <HandoffScatter ramps={ramps} /> : <EmptyHandoffs />}<AdoptionList ramps={ramps} /></div>
}

function HandoffScatter({ ramps }: { ramps: RampResponse }) {
  const chart = useMemo(() => {
    const width = 900, height = 370, margin = { top: 25, right: 30, bottom: 55, left: 70 }
    const points = ramps.handoffs.filter((point) => point.observed_lift !== undefined)
    const xMax = Math.max(1, ...points.map((point) => point.interaction_days))
    const liftExtent = extent(points, (point) => point.observed_lift!)
    const lower = Math.min(-.25, liftExtent[0] ?? -.25), upper = Math.max(.25, liftExtent[1] ?? .25)
    const episodeMax = Math.max(1, ...points.map((point) => point.completed_episodes))
    return { width, height, margin, points, x: scaleLinear().domain([0, xMax]).nice().range([margin.left, width - margin.right]), y: scaleLinear().domain([lower, upper]).nice().range([height - margin.bottom, margin.top]), radius: scaleLinear().domain([1, episodeMax]).range([6, 18]) }
  }, [ramps])
  return (
    <div>
      <div className="overflow-x-auto"><svg viewBox={`0 0 ${chart.width} ${chart.height}`} className="min-w-[680px]" role="img" aria-label="Observed output lift after human to agent handoffs">
        {chart.y.ticks(5).map((tick) => <g key={tick}><line x1={chart.margin.left} x2={chart.width-chart.margin.right} y1={chart.y(tick)} y2={chart.y(tick)} stroke={tick === 0 ? 'rgba(255,255,255,.4)' : 'rgba(148,163,184,.12)'} /><text x={chart.margin.left-12} y={chart.y(tick)+4} textAnchor="end" fill="#64748b" fontSize="11">{percent.format(tick)}</text></g>)}
        {chart.x.ticks(6).map((tick) => <text key={tick} x={chart.x(tick)} y={chart.height-25} textAnchor="middle" fill="#64748b" fontSize="11">{tick}</text>)}
        <text x={chart.width/2} y={chart.height-5} textAnchor="middle" fill="#94a3b8" fontSize="12">Collaboration frequency · interaction days</text>
        <text transform={`translate(18 ${chart.height/2}) rotate(-90)`} textAnchor="middle" fill="#94a3b8" fontSize="12">Observed commit lift</text>
        {chart.points.map((point) => { const quality = point.quality_delta; const color = quality === undefined ? '#94a3b8' : quality >= 0 ? '#34d399' : '#fb7185'; return <circle key={point.key} cx={chart.x(point.interaction_days)} cy={chart.y(point.observed_lift!)} r={chart.radius(point.completed_episodes)} fill={color} fillOpacity=".75" stroke={point.rank_eligible ? '#fff' : '#0f172a'} strokeWidth="2"><title>{point.human.name} → {point.agent.name}: {percent.format(point.observed_lift!)} observed lift across {point.completed_episodes} completed handoffs and {point.interaction_days} interaction days{quality === undefined ? '' : `; ${percent.format(quality)} favorable revert-rate change`}</title></circle> })}
      </svg></div>
      <div className="mt-2 flex flex-wrap gap-4 text-xs text-slate-500"><span><span className="mr-1 inline-block size-2 rounded-full bg-emerald-400" />Favorable quality association</span><span><span className="mr-1 inline-block size-2 rounded-full bg-rose-400" />Unfavorable quality association</span><span>White ring: at least 3 completed episodes</span></div>
      {ramps.handoffs.some((point) => point.mature && point.observed_lift === undefined) && <div className="mt-3 flex flex-wrap gap-2">{ramps.handoffs.filter((point) => point.mature && point.observed_lift === undefined).map((point) => <span key={point.key} className="rounded-full border border-violet-300/15 bg-violet-300/[.06] px-3 py-1 text-xs text-violet-200">{point.human.name} → {point.agent.name}: new activity ({point.baseline} → {point.after})</span>)}</div>}
      <p className="mt-3 rounded-xl border border-amber-300/10 bg-amber-300/[0.04] px-3 py-2 text-xs leading-5 text-amber-100/70">Observed lift is a before/after association in the same repository session. It does not establish that the agent caused the change.</p>
      <div className="sr-only"><table><caption>Human to agent handoff comparisons</caption><thead><tr><th>Human</th><th>Agent</th><th>Interaction days</th><th>Completed episodes</th><th>Observed lift</th></tr></thead><tbody>{ramps.handoffs.map((point) => <tr key={point.key}><td>{point.human.name}</td><td>{point.agent.name}</td><td>{point.interaction_days}</td><td>{point.completed_episodes}</td><td>{point.observed_lift === undefined ? point.mature ? 'New activity' : 'Maturing' : percent.format(point.observed_lift)}</td></tr>)}</tbody></table></div>
    </div>
  )
}

function AdoptionList({ ramps }: { ramps: RampResponse }) {
  if (ramps.adoptions.length === 0) return null
  return <div className="mt-6 border-t border-white/8 pt-5"><div className="flex items-center gap-2"><GitBranch className="size-4 text-violet-300" /><h3 className="text-sm font-semibold text-slate-200">Repository adoption windows</h3></div><div className="mt-3 grid gap-2 lg:grid-cols-2">{ramps.adoptions.slice(-8).map((item) => <article key={item.repository_id} className="flex items-center gap-3 rounded-xl border border-white/7 bg-slate-950/45 p-3"><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium text-slate-200">{item.repository}</p><p className="mt-1 text-xs text-slate-500">First confirmed agent evidence {item.adopted_at} · {item.mature ? `complete ${ramps.meta.adoption_days}-day window` : 'still maturing'}</p></div><div className="text-right"><p className={`text-sm font-semibold ${item.absolute_change >= 0 ? 'text-emerald-300' : 'text-rose-300'}`}>{item.observed_lift === undefined ? item.mature ? 'New activity' : 'Maturing' : percent.format(item.observed_lift)}</p><p className="text-[10px] text-slate-600">{item.baseline} → {item.after} commits</p></div></article>)}</div></div>
}

function Empty() { return <div className="grid min-h-72 place-items-center rounded-2xl border border-dashed border-white/10 text-center"><div><Bot className="mx-auto size-7 text-slate-600" /><p className="mt-3 text-sm text-slate-400">No human <ArrowRight className="inline size-3" /> agent handoffs are visible yet.</p><p className="mt-1 text-xs text-slate-600">Classify agent identities or widen the selected date range.</p></div></div> }
function EmptyHandoffs() { return <div className="rounded-2xl border border-dashed border-white/10 px-4 py-8 text-center text-sm text-slate-500">No human → agent handoff episodes match this range; repository adoption evidence is shown below.</div> }
