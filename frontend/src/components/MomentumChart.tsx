import { axisBottom, axisLeft, line, max, scaleBand, scaleLinear, scaleUtc, select, stack, utcFormat, utcParse } from 'd3'
import { GitCommitHorizontal, Grid3X3, LineChart } from 'lucide-react'
import { memo, useEffect, useMemo, useRef, useState } from 'react'
import type { OverviewResponse, TimelinePoint } from '../lib/api'

interface MomentumChartProps { overview: OverviewResponse | null; loading: boolean }

const parseDate = utcParse('%Y-%m-%d')
const tickDate = utcFormat('%b %Y')
const workKeys = ['human_only', 'agent_only', 'mixed'] as const
const workColors = { human_only: '#22d3ee', agent_only: '#8b5cf6', mixed: '#f59e0b' }

export const MomentumChart = memo(function MomentumChart({ overview, loading }: MomentumChartProps) {
  const [view, setView] = useState<'timeline' | 'pulse'>('timeline')
  if (loading) return <div className="h-[520px] animate-pulse rounded-2xl bg-white/[0.03]" aria-label="Loading momentum and quality" />
  if (!overview || overview.timeline.length === 0) return <Empty />
  return (
    <div>
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap gap-3 text-xs text-slate-400">
          {workKeys.map((key) => <span key={key} className="inline-flex items-center gap-1.5"><span className="size-2.5 rounded-sm" style={{ backgroundColor: workColors[key] }} />{labels[key]}</span>)}
          <span className="inline-flex items-center gap-1.5"><span className="h-0.5 w-4 bg-white" />Pull requests</span>
        </div>
        <div className="flex rounded-xl border border-white/10 bg-slate-950 p-1">
          <ViewButton active={view === 'timeline'} onClick={() => setView('timeline')} icon={<LineChart className="size-3.5" />} label="Team mix" />
          <ViewButton active={view === 'pulse'} onClick={() => setView('pulse')} icon={<Grid3X3 className="size-3.5" />} label="Repository pulse" />
        </div>
      </div>
      {view === 'timeline' ? <Timeline overview={overview} /> : <RepositoryPulseChart overview={overview} />}
    </div>
  )
})

const labels = { human_only: 'Human-only', agent_only: 'Agent-only', mixed: 'Mixed' }

function Timeline({ overview }: { overview: OverviewResponse }) {
  const bars = useMemo(() => {
    const width = 1000, height = 270, margin = { top: 10, right: 20, bottom: 42, left: 54 }
    const dates = overview.timeline.map((point) => parseDate(point.date)).filter(Boolean) as Date[]
    const x = scaleBand<string>().domain(overview.timeline.map((point) => point.date)).range([margin.left, width - margin.right]).padding(.16)
    const layers = stack<TimelinePoint>().keys(workKeys)(overview.timeline)
    const y = scaleLinear().domain([0, Math.max(max(layers, (layer) => max(layer, (point) => point[1])) ?? 1, max(overview.timeline, (point) => point.pull_requests) ?? 0)]).nice().range([height - margin.bottom, margin.top])
    const pullLine = line<TimelinePoint>().x((point) => (x(point.date) ?? 0) + x.bandwidth() / 2).y((point) => y(point.pull_requests))
    const tickEvery = Math.max(1, Math.ceil(dates.length / 7))
    return { width, height, margin, x, y, layers, pullLine, ticks: overview.timeline.filter((_, index) => index % tickEvery === 0 || index === overview.timeline.length - 1) }
  }, [overview.timeline])
  return (
    <div>
      <h3 className="text-sm font-semibold text-slate-200">Commit frequency by work mode</h3>
      <p className="mt-1 text-xs text-slate-500">Buckets adapt from days to weeks and months as the selected range expands.</p>
      <div className="mt-3 overflow-x-auto">
        <svg viewBox={`0 0 ${bars.width} ${bars.height}`} className="min-w-[720px]" role="img" aria-label="Stacked commit frequency for classified human, agent, and mixed work">
          {bars.y.ticks(5).map((tick) => <g key={tick}><line x1={bars.margin.left} x2={bars.width - bars.margin.right} y1={bars.y(tick)} y2={bars.y(tick)} stroke="rgba(148,163,184,.12)" /><text x={bars.margin.left - 10} y={bars.y(tick) + 4} textAnchor="end" fill="#64748b" fontSize="11">{tick}</text></g>)}
          {bars.layers.map((layer) => <g key={layer.key} fill={workColors[layer.key as keyof typeof workColors]}>{layer.map((point, index) => <rect key={overview.timeline[index].date} x={bars.x(overview.timeline[index].date)} y={bars.y(point[1])} width={bars.x.bandwidth()} height={Math.max(0, bars.y(point[0]) - bars.y(point[1]))}><title>{overview.timeline[index].date}: {point[1] - point[0]} {labels[layer.key as keyof typeof labels].toLowerCase()} commits</title></rect>)}</g>)}
          <path d={bars.pullLine(overview.timeline) ?? undefined} fill="none" stroke="#f8fafc" strokeWidth="2" strokeDasharray="5 4" vectorEffect="non-scaling-stroke" />
          {overview.timeline.filter((point) => point.pull_requests > 0).map((point) => <circle key={`pull:${point.date}`} cx={(bars.x(point.date) ?? 0) + bars.x.bandwidth() / 2} cy={bars.y(point.pull_requests)} r="3" fill="#f8fafc"><title>{point.date}: {point.pull_requests} pull requests</title></circle>)}
          {bars.ticks.map((point) => <text key={point.date} x={(bars.x(point.date) ?? 0) + bars.x.bandwidth() / 2} y={bars.height - 15} textAnchor="middle" fill="#64748b" fontSize="11">{tickDate(parseDate(point.date)!)}</text>)}
        </svg>
      </div>
      <QualityLines overview={overview} />
      <div className="sr-only"><table><caption>Classified human and agent activity over time</caption><thead><tr><th>Date</th><th>Human-only</th><th>Agent-only</th><th>Mixed</th></tr></thead><tbody>{overview.timeline.map((point) => <tr key={point.date}><td>{point.date}</td><td>{point.human_only}</td><td>{point.agent_only}</td><td>{point.mixed}</td></tr>)}</tbody></table></div>
    </div>
  )
}

function QualityLines({ overview }: { overview: OverviewResponse }) {
  const svgRef = useRef<SVGSVGElement>(null)
  const dimensions = useMemo(() => {
    const width = 1000, height = 245, margin = { top: 18, right: 20, bottom: 42, left: 54 }
    const dates = overview.quality.map((point) => parseDate(point.date)).filter(Boolean) as Date[]
    const first = dates[0] ?? new Date(), last = dates.at(-1) ?? first
    return { width, height, margin, x: scaleUtc().domain(first.getTime() === last.getTime() ? [new Date(first.getTime() - 86400000), new Date(last.getTime() + 86400000)] : [first, last]).range([margin.left, width - margin.right]), y: scaleLinear().domain([0, 1]).range([height - margin.bottom, margin.top]) }
  }, [overview.quality])
  useEffect(() => {
    if (!svgRef.current) return
    const root = select(svgRef.current)
    root.selectAll('.axis').remove()
    root.append('g').attr('class', 'axis').attr('transform', `translate(0,${dimensions.height - dimensions.margin.bottom})`).call(axisBottom(dimensions.x).ticks(6).tickFormat((value) => tickDate(value as Date)))
    root.append('g').attr('class', 'axis').attr('transform', `translate(${dimensions.margin.left},0)`).call(axisLeft(dimensions.y).ticks(4).tickFormat((value) => `${Number(value) * 100}%`))
    root.selectAll('.axis text').attr('fill', '#64748b'); root.selectAll('.axis line,.axis path').attr('stroke', 'rgba(148,163,184,.16)')
  }, [dimensions])
  const series = [
    { key: 'merge_rate', label: 'PR merge rate', color: '#34d399' },
    { key: 'review_coverage', label: 'Review coverage', color: '#38bdf8' },
    { key: 'revert_rate', label: 'Revert rate', color: '#fb7185' },
    { key: 'retained_line_rate', label: '30-day retained lines', color: '#fbbf24' },
  ] as const
  const builder = line<{ date: Date; value: number }>().x((point) => dimensions.x(point.date)).y((point) => dimensions.y(point.value)).defined((point) => Number.isFinite(point.value))
  return (
    <div className="mt-6 border-t border-white/8 pt-5">
      <div className="flex flex-wrap items-start justify-between gap-3"><div><h3 className="text-sm font-semibold text-slate-200">Quality proxies</h3><p className="mt-1 text-xs text-slate-500">Separate measured outcomes; no composite quality score.</p></div><div className="flex flex-wrap gap-3 text-xs">{series.map((item) => <span key={item.key} className="inline-flex items-center gap-1.5 text-slate-400"><span className="h-0.5 w-4" style={{ backgroundColor: item.color }} />{item.label}</span>)}</div></div>
      <div className="mt-3 overflow-x-auto">
        <svg ref={svgRef} viewBox={`0 0 ${dimensions.width} ${dimensions.height}`} className="min-w-[720px]" role="img" aria-label="Quality proxy percentages over time">
          {series.map((item) => {
            const points = overview.quality.flatMap((point) => {
              const date = parseDate(point.date)
              const value = point[item.key]
              const sample = item.key === 'retained_line_rate' ? (point.retention_sample ?? 0) : item.key === 'revert_rate' ? point.commit_sample : point.pull_request_sample
              return date && value !== undefined ? [{ date, value, sample }] : []
            })
            return (
              <g key={item.key}>
                <path d={builder(points) ?? undefined} fill="none" stroke={item.color} strokeWidth="2.5" vectorEffect="non-scaling-stroke" />
                {points.map((point) => {
                  const day = point.date.toISOString().slice(0, 10)
                  return (
                    <circle key={point.date.toISOString()} cx={dimensions.x(point.date)} cy={dimensions.y(point.value)} r="3" fill={item.color}>
                      <title>{item.label} · {day}: {Math.round(point.value * 100)}% · sample {point.sample}</title>
                    </circle>
                  )
                })}
              </g>
            )
          })}
        </svg>
      </div>
      <MergeTimeLine overview={overview} />
      {overview.meta.unavailable?.includes('retained_line_rate_pending_enriched_git_analysis') && <p className="mt-2 text-xs text-amber-200/80">30-day retained-line analysis will appear after an enriched Git analysis refresh.</p>}
      <div className="sr-only"><table><caption>Quality proxies over time</caption><thead><tr><th>Date</th><th>Revert rate</th><th>Merge rate</th><th>Median merge hours</th><th>Review coverage</th><th>Retained-line rate</th><th>Commit sample</th><th>PR sample</th></tr></thead><tbody>{overview.quality.map((point) => <tr key={point.date}><td>{point.date}</td><td>{point.revert_rate}</td><td>{point.merge_rate}</td><td>{point.median_merge_hours}</td><td>{point.review_coverage}</td><td>{point.retained_line_rate}</td><td>{point.commit_sample}</td><td>{point.pull_request_sample}</td></tr>)}</tbody></table></div>
    </div>
  )
}

function MergeTimeLine({ overview }: { overview: OverviewResponse }) {
  const points = overview.quality.flatMap((point) => {
    const date = parseDate(point.date)
    return date && point.median_merge_hours !== undefined ? [{ date, day: point.date, value: point.median_merge_hours, sample: point.pull_request_sample }] : []
  })
  if (points.length === 0) return null
  const width = 1000, height = 150, margin = { top: 16, right: 20, bottom: 36, left: 54 }
  const first = points[0].date, last = points.at(-1)!.date
  const x = scaleUtc().domain(first.getTime() === last.getTime() ? [new Date(first.getTime() - 86400000), new Date(last.getTime() + 86400000)] : [first, last]).range([margin.left, width - margin.right])
  const y = scaleLinear().domain([0, max(points, (point) => point.value) ?? 1]).nice().range([height - margin.bottom, margin.top])
  const builder = line<(typeof points)[number]>().x((point) => x(point.date)).y((point) => y(point.value))
  return <div className="mt-5"><h4 className="text-xs font-medium text-slate-400">Median time to merge · hours</h4><div className="mt-2 overflow-x-auto"><svg viewBox={`0 0 ${width} ${height}`} className="min-w-[720px]" role="img" aria-label="Median pull request time to merge in hours">{y.ticks(3).map((tick) => <g key={tick}><line x1={margin.left} x2={width-margin.right} y1={y(tick)} y2={y(tick)} stroke="rgba(148,163,184,.1)" /><text x={margin.left-10} y={y(tick)+4} textAnchor="end" fill="#64748b" fontSize="11">{tick}h</text></g>)}<path d={builder(points) ?? undefined} fill="none" stroke="#c084fc" strokeWidth="2.5" vectorEffect="non-scaling-stroke" />{points.map((point) => <circle key={point.day} cx={x(point.date)} cy={y(point.value)} r="3" fill="#c084fc"><title>{point.day}: {Math.round(point.value)} hours · {point.sample} resolved PRs</title></circle>)}</svg></div></div>
}

function RepositoryPulseChart({ overview }: { overview: OverviewResponse }) {
  const rows = overview.repositories.slice(0, 30)
  const dates = overview.timeline.map((point) => point.date)
  const maximum = Math.max(1, ...rows.flatMap((row) => row.points.map(totalPulse)))
  const cell = 16, labelWidth = 190, width = labelWidth + dates.length * cell, height = 34 + rows.length * 23
  return (
    <div>
      <h3 className="text-sm font-semibold text-slate-200">Repository commit pulse</h3><p className="mt-1 text-xs text-slate-500">Brush the global range down to days to reveal compressed bursts of work.</p>
      <div className="mt-4 max-h-[560px] overflow-auto rounded-xl border border-white/8 bg-slate-950/55"><svg width={Math.max(760, width)} height={height} role="img" aria-label="Repository activity heatmap"><g transform="translate(0,28)">{rows.map((row, rowIndex) => <g key={row.repository_id} transform={`translate(0,${rowIndex * 23})`}><text x={labelWidth - 10} y={14} textAnchor="end" fill="#94a3b8" fontSize="11">{row.name.slice(0, 28)}</text>{row.points.map((point, columnIndex) => { const value = totalPulse(point); return <rect key={point.date} x={labelWidth + columnIndex * cell} y={1} width={cell - 2} height={17} rx="2" fill={value ? `rgba(139,92,246,${.12 + value / maximum * .88})` : 'rgba(255,255,255,.025)'}><title>{row.name} · {point.date}: {value} commits</title></rect> })}</g>)}</g></svg></div>
    </div>
  )
}

function totalPulse(point: { human_only: number; agent_only: number; mixed: number }) { return point.human_only + point.agent_only + point.mixed }
function ViewButton({ active, onClick, icon, label }: { active: boolean; onClick: () => void; icon: React.ReactNode; label: string }) { return <button type="button" onClick={onClick} className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium ${active ? 'bg-white/10 text-white' : 'text-slate-500 hover:text-slate-300'}`}>{icon}{label}</button> }
function Empty() { return <div className="grid min-h-80 place-items-center rounded-2xl border border-dashed border-white/10 text-center"><div><GitCommitHorizontal className="mx-auto size-7 text-slate-600" /><p className="mt-3 text-sm text-slate-400">No event-level activity matches these filters.</p></div></div> }
