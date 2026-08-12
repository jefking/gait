import {
  extent,
  line,
  max,
  scaleLinear,
  scaleOrdinal,
  scaleUtc,
  schemeTableau10,
  utcFormat,
  utcParse,
} from 'd3'
import { Eye, EyeOff, LineChart as LineChartIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import type { ActivityGranularity, ActivityResponse } from '../lib/api'
import { Avatar } from './Avatar'

interface ActivityChartProps {
  activity: ActivityResponse | null
  loading: boolean
}

const parseDate = utcParse('%Y-%m-%d')
const formatDay = utcFormat('%b %-d')
const formatMonth = utcFormat('%b %Y')
const compactNumber = new Intl.NumberFormat(undefined, { notation: 'compact' })

export function ActivityChart({ activity, loading }: ActivityChartProps) {
  const visibilityScope = `${activity?.group_by ?? ''}:${activity?.metric ?? ''}:${activity?.series.map((series) => series.key).join(',') ?? ''}`
  const [visibility, setVisibility] = useState<{ scope: string; selected: Set<string> }>({
    scope: visibilityScope,
    selected: new Set(activity?.series.map((series) => series.key) ?? []),
  })
  const selected = useMemo(
    () => visibility.scope === visibilityScope
      ? visibility.selected
      : new Set(activity?.series.map((series) => series.key) ?? []),
    [activity?.series, visibility, visibilityScope],
  )

  const chart = useMemo(() => {
    const width = 960
    const height = 340
    const margin = { top: 20, right: 22, bottom: 42, left: 58 }
    const prepared = (activity?.series ?? []).map((series) => ({
      ...series,
      parsedPoints: series.points.flatMap((point) => {
        const date = parseActivityDate(point.date, point.month)
        return date ? [{ date, value: point.value }] : []
      }),
    }))
    const visible = prepared.filter((series) => selected.has(series.key))
    const datedPoints = visible.flatMap((series) => series.parsedPoints)
    const domain = extent(datedPoints, (point) => point.date)
    const fallback = new Date(Date.UTC(new Date().getUTCFullYear(), new Date().getUTCMonth(), 1))
    const first = domain[0] ?? fallback
    const last = domain[1] ?? fallback
    const x = scaleUtc()
      .domain(first.getTime() === last.getTime() ? [new Date(first.getTime() - 86400000), new Date(last.getTime() + 86400000)] : [first, last])
      .range([margin.left, width - margin.right])
    const yMaximum = max(datedPoints, (point) => point.value) ?? 0
    const y = scaleLinear()
      .domain([0, Math.max(1, yMaximum)])
      .nice()
      .range([height - margin.bottom, margin.top])
    const colors = scaleOrdinal<string, string>(
      activity?.series.map((series) => series.key) ?? [],
      schemeTableau10,
    )
    const lineBuilder = line<{ date: Date; value: number }>()
      .x((point) => x(point.date))
      .y((point) => y(point.value))
    const uniqueMonths = Array.from(
      new Map(datedPoints.map((point) => [point.date.getTime(), point.date])).values(),
    ).sort((left, right) => left.getTime() - right.getTime())
    const tickStep = Math.max(1, Math.ceil(uniqueMonths.length / 6))
    const xTicks = uniqueMonths.filter(
      (_, index) => index % tickStep === 0 || index === uniqueMonths.length - 1,
    )

    return {
      width,
      height,
      margin,
      visible,
      x,
      y,
      colors,
      lineBuilder,
      xTicks,
      yMaximum,
      yTicks: y.ticks(Math.min(5, Math.max(1, Math.ceil(yMaximum)))),
    }
  }, [activity, selected])

  if (loading) {
    return <div className="h-[420px] animate-pulse rounded-2xl bg-white/[0.03]" aria-label="Loading activity chart" />
  }

  if (!activity || activity.series.length === 0) {
    return (
      <div className="grid min-h-80 place-items-center rounded-2xl border border-dashed border-white/10 bg-white/[0.02] text-center">
        <div>
          <LineChartIcon aria-hidden="true" className="mx-auto size-7 text-slate-600" />
          <p className="mt-3 text-sm text-slate-400">No activity matches these filters.</p>
        </div>
      </div>
    )
  }

  return (
    <div className="activity-chart">
      <div className="activity-legend flex flex-wrap gap-2" aria-label="Chart series">
        {activity.series.map((series) => {
          const isSelected = selected.has(series.key)
          return (
            <button
              key={series.key}
              type="button"
              onClick={() =>
                setVisibility((current) => {
                  const next = new Set(
                    current.scope === visibilityScope
                      ? current.selected
                      : activity.series.map((candidate) => candidate.key),
                  )
                  if (next.has(series.key)) next.delete(series.key)
                  else next.add(series.key)
                  return { scope: visibilityScope, selected: next }
                })
              }
              aria-pressed={isSelected}
              className={`activity-series inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs transition ${
                isSelected
                  ? 'border-white/10 bg-white/5 text-slate-200'
                  : 'border-white/5 bg-transparent text-slate-500'
              }`}
            >
              {series.avatar_url ? (
                <Avatar src={series.avatar_url} name={series.label} size="xs" />
              ) : (
                <span className="size-2 rounded-full" style={{ backgroundColor: chart.colors(series.key) }} />
              )}
              {series.label}
              <span className="text-slate-500">{compactNumber.format(series.total)}</span>
              {isSelected ? <Eye aria-hidden="true" className="no-print size-3" /> : <EyeOff aria-hidden="true" className="no-print size-3" />}
            </button>
          )
        })}
      </div>

      <div className="mt-5 overflow-x-auto">
        <svg
          viewBox={`0 0 ${chart.width} ${chart.height}`}
          data-y-maximum={chart.yMaximum}
          className="activity-chart-svg min-w-[680px] overflow-visible"
          role="img"
          aria-labelledby="activity-chart-title activity-chart-description"
        >
          <title id="activity-chart-title">Repository activity over time</title>
          <desc id="activity-chart-description">
            {activity.metric === 'commits' ? 'Commits' : 'Pull requests opened'} grouped by {activity.group_by} in {activity.granularity ?? 'monthly'} intervals.
          </desc>
          {chart.yTicks.map((tick) => (
            <g key={tick}>
              <line
                x1={chart.margin.left}
                x2={chart.width - chart.margin.right}
                y1={chart.y(tick)}
                y2={chart.y(tick)}
                stroke="rgba(148,163,184,0.12)"
              />
              <text x={chart.margin.left - 12} y={chart.y(tick) + 4} textAnchor="end" fill="#64748b" fontSize="11">
                {compactNumber.format(tick)}
              </text>
            </g>
          ))}
          {chart.xTicks.map((tick) => (
            <text
              key={tick.toISOString()}
              x={chart.x(tick)}
              y={chart.height - 14}
              textAnchor="middle"
              fill="#64748b"
              fontSize="11"
            >
              {formatActivityTick(tick, activity.granularity)}
            </text>
          ))}
          {chart.visible.map((series) => {
            return (
              <path
                key={series.key}
                data-series-key={series.key}
                d={chart.lineBuilder(series.parsedPoints) ?? undefined}
                fill="none"
                stroke={chart.colors(series.key)}
                strokeWidth="2.5"
                strokeLinejoin="round"
                strokeLinecap="round"
                vectorEffect="non-scaling-stroke"
              />
            )
          })}
        </svg>
      </div>

      <div className="sr-only">
        <table>
          <caption>Activity totals for the displayed series</caption>
          <thead><tr><th>Series</th><th>Total</th></tr></thead>
          <tbody>
            {activity.series.map((series) => (
              <tr key={series.key}><td>{series.label}</td><td>{series.total}</td></tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function formatActivityTick(date: Date, granularity?: ActivityGranularity) {
  return granularity === 'month' ? formatMonth(date) : formatDay(date)
}

function parseActivityDate(date?: string, month?: string) {
  const value = date || month
  if (!value) return null
  return parseDate(/^\d{4}-\d{2}$/.test(value) ? `${value}-01` : value)
}
