import { line, scaleLinear, scaleOrdinal, scalePoint, schemeTableau10 } from 'd3'
import { Trophy } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import type { RankingResponse } from '../lib/api'

interface RankChartProps { rankings: RankingResponse | null; loading: boolean }
const compact = new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 })

export function RankChart({ rankings, loading }: RankChartProps) {
  if (loading) return <div className="h-[430px] animate-pulse rounded-2xl bg-white/[0.03]" aria-label="Loading rank trajectories" />
  if (!rankings || rankings.leaderboard.length === 0) return <Empty />
  return <div className="grid items-stretch gap-6 xl:grid-cols-[minmax(0,1fr)_320px]"><BumpChart rankings={rankings} /><Leaderboard rankings={rankings} /></div>
}

function BumpChart({ rankings }: { rankings: RankingResponse }) {
  const plotRef = useRef<HTMLDivElement>(null)
  const [dimensions, setDimensions] = useState({ width: 900, height: 350 })

  useEffect(() => {
    const element = plotRef.current
    if (!element) return
    const measure = () => {
      const bounds = element.getBoundingClientRect()
      const next = {
        width: Math.max(680, Math.round(bounds.width || element.clientWidth || 900)),
        height: Math.max(350, Math.round(bounds.height || element.clientHeight || 350)),
      }
      setDimensions((current) => current.width === next.width && current.height === next.height ? current : next)
    }
    measure()
    if (typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver(measure)
    observer.observe(element)
    return () => observer.disconnect()
  }, [])

  const chart = useMemo(() => {
    const { width, height } = dimensions
    const margin = { top: 25, right: 135, bottom: 45, left: 42 }
    const dates = Array.from(new Set(rankings.trajectories.flatMap((series) => series.points.map((point) => point.date)))).sort()
    const maxRank = Math.max(1, ...rankings.trajectories.flatMap((series) => series.points.map((point) => point.rank)))
    const x = scalePoint<string>().domain(dates).range([margin.left, width - margin.right]).padding(.25)
    const y = scaleLinear().domain([1, Math.max(2, maxRank)]).range([margin.top, height - margin.bottom])
    const colors = scaleOrdinal<string, string>(rankings.trajectories.map((series) => series.key), schemeTableau10)
    const builder = line<{ date: string; rank: number }>().x((point) => x(point.date) ?? margin.left).y((point) => y(point.rank))
    const tickStep = Math.max(1, Math.ceil(dates.length / 6))
    return { width, height, margin, dates, maxRank, x, y, colors, builder, ticks: dates.filter((_, index) => index % tickStep === 0 || index === dates.length - 1) }
  }, [dimensions, rankings])
  return <div className="flex min-h-[430px] flex-col"><h3 className="text-sm font-semibold text-slate-200">Rank over time</h3><p className="mt-1 text-xs text-slate-500">Each period is ranked by {rankings.metric.replaceAll('_', ' ')}; no combined impact score.</p><div ref={plotRef} className="mt-3 min-h-[350px] flex-1 overflow-x-auto" data-testid="rank-trajectory-plot"><svg width={chart.width} height={chart.height} viewBox={`0 0 ${chart.width} ${chart.height}`} className="block max-w-none" role="img" aria-label={`${rankings.cohort} rank trajectories by ${rankings.metric}`}>
    {Array.from({ length: Math.min(10, chart.maxRank) }, (_, index) => index + 1).map((rank) => <g key={rank}><line x1={chart.margin.left} x2={chart.width-chart.margin.right} y1={chart.y(rank)} y2={chart.y(rank)} stroke="rgba(148,163,184,.1)" /><text x={chart.margin.left-10} y={chart.y(rank)+4} textAnchor="end" fill="#64748b" fontSize="11">#{rank}</text></g>)}
    {rankings.trajectories.map((series) => <g key={series.key}><path d={chart.builder(series.points) ?? undefined} fill="none" stroke={chart.colors(series.key)} strokeWidth="2.5" opacity=".86" />{series.points.map((point) => <circle key={point.date} cx={chart.x(point.date)} cy={chart.y(point.rank)} r="3.5" fill={chart.colors(series.key)}><title>{series.label} · {point.date}: rank {point.rank}, value {compact.format(point.value)}</title></circle>)}{series.points.length > 0 && <text x={(chart.x(series.points.at(-1)!.date) ?? 0)+8} y={chart.y(series.points.at(-1)!.rank)+4} fill={chart.colors(series.key)} fontSize="11">{series.label.slice(0,18)}</text>}</g>)}
    {chart.ticks.map((date) => <text key={date} x={chart.x(date)} y={chart.height-14} textAnchor="middle" fill="#64748b" fontSize="11">{date.slice(0,7)}</text>)}
  </svg></div></div>
}

function Leaderboard({ rankings }: { rankings: RankingResponse }) {
  return <aside><div className="flex items-center gap-2"><Trophy className="size-4 text-amber-300" /><h3 className="text-sm font-semibold text-slate-200">Current leaderboard</h3></div><p className="mt-1 text-xs text-slate-500">{rankings.favorable_direction === 'lower' ? 'Lower is favorable' : 'Higher is favorable'} for this metric.</p><ol className="mt-4 space-y-2">{rankings.leaderboard.slice(0,12).map((entry) => <li key={entry.key} className="rounded-xl border border-white/7 bg-slate-950/45 px-3 py-2.5"><div className="flex items-center gap-3"><span className="w-5 text-right text-xs tabular-nums text-slate-600">{entry.rank}</span><span className="min-w-0 flex-1 truncate text-sm text-slate-300">{entry.label}</span><span className="text-sm font-semibold tabular-nums text-cyan-300">{formatMetric(entry.value, rankings.metric)}</span></div><div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 pl-8 text-[10px] text-slate-600">{Object.entries(entry.metrics).map(([metric, value]) => <span key={metric}>{metric.replaceAll('_', ' ')} {formatMetric(value, metric)}</span>)}</div></li>)}</ol></aside>
}

function formatMetric(value: number, metric: string) { return metric.endsWith('_rate') ? new Intl.NumberFormat(undefined,{style:'percent',maximumFractionDigits:1}).format(value) : compact.format(value) }
function Empty() { return <div className="grid min-h-72 place-items-center rounded-2xl border border-dashed border-white/10 text-center"><div><Trophy className="mx-auto size-7 text-slate-600" /><p className="mt-3 text-sm text-slate-400">No eligible identities or pairs for this ranking.</p></div></div> }
