import { CalendarRange } from 'lucide-react'
import { useState } from 'react'
import type { ActivityGranularity } from '../lib/api'
import { addUTCMonths } from '../lib/dates'

interface DateRangeSliderProps {
  availableFrom: string
  availableTo: string
  from: string
  to: string
  granularity?: ActivityGranularity
  onChange: (from: string, to: string) => void
}

interface DraftRange {
  scope: string
  start: number
}

interface SnapPoint {
  start: number
  labels: string[]
}

const dayMilliseconds = 24 * 60 * 60 * 1000
const rangePresets = [
  { id: 'month', label: '31 days', start: (maximum: Date) => addDays(maximum, -30) },
  { id: 'six-months', label: '6 months', start: (maximum: Date) => addUTCMonths(maximum, -6) },
  { id: 'year', label: '1 year', start: (maximum: Date) => addYears(maximum, -1) },
] as const

export function DateRangeSlider({
  availableFrom,
  availableTo,
  from,
  granularity,
  onChange,
}: DateRangeSliderProps) {
  const minimum = parseUTCDate(availableFrom)
  const maximum = parseUTCDate(availableTo)
  const totalDays = Math.max(0, Math.round((maximum.getTime() - minimum.getTime()) / dayMilliseconds))
  const rangeMaximum = Math.max(1, totalDays)
  const externalStart = clamp(dayOffset(minimum, parseUTCDate(from)), 0, totalDays)
  const scope = `${availableFrom}:${availableTo}:${from}`
  const [draft, setDraft] = useState<DraftRange>({ scope, start: externalStart })
  const selected = draft.scope === scope ? draft : { scope, start: externalStart }
  const selectedFrom = addDays(minimum, selected.start)
  const snapPoints = buildSnapPoints(minimum, maximum)
  const updateStart = (value: number) => {
    setDraft({ scope, start: clamp(value, 0, totalDays) })
  }
  const commit = () => {
    const snappedStart = closestSnapPoint(selected.start, snapPoints)
    setDraft({ scope, start: snappedStart })
    onChange(formatISODate(addDays(minimum, snappedStart)), availableTo)
  }
  const selectedSnap = closestSnapPoint(selected.start, snapPoints)
  const timelineStart = timelineX(selected.start, totalDays)

  return (
    <section className="mt-6 rounded-2xl border border-white/8 bg-slate-950/45 px-4 py-4 sm:px-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <CalendarRange aria-hidden="true" className="size-4 text-cyan-300" />
          <h3 className="text-sm font-medium text-slate-200">History range</h3>
          {granularity && (
            <span className="rounded-full border border-cyan-300/15 bg-cyan-300/8 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-cyan-200">
              {{ day: 'Daily', week: 'Weekly', month: 'Monthly' }[granularity]}
            </span>
          )}
        </div>
        <p className="text-xs tabular-nums text-slate-400">
          <time dateTime={formatISODate(selectedFrom)}>{formatDisplayDate(selectedFrom)}</time>
          <span className="px-2 text-slate-600">→</span>
          <time dateTime={availableTo}>{formatDisplayDate(maximum)}</time>
        </p>
      </div>

      <div className="relative mt-5 h-8" data-testid="date-range-slider">
        <svg viewBox="0 0 1000 32" preserveAspectRatio="none" className="absolute inset-0 h-8 w-full" aria-hidden="true">
          <line x1="9" x2="991" y1="16" y2="16" stroke="rgba(255,255,255,.1)" strokeWidth="6" strokeLinecap="round" />
          <line x1={timelineStart} x2="991" y1="16" y2="16" stroke="#22d3ee" strokeOpacity=".28" strokeWidth="12" strokeLinecap="round" />
          {snapPoints.map((point) => (
            <circle
              key={point.start}
              cx={timelineX(point.start, totalDays)}
              cy="16"
              r={point.start === selectedSnap ? 5 : 3.5}
              fill={point.start === selectedSnap ? '#67e8f9' : '#64748b'}
              stroke="#0f172a"
              strokeWidth="2"
              data-snap-label={point.labels.join(', ')}
            >
              <title>{point.labels.join(' · ')}</title>
            </circle>
          ))}
          <circle cx="991" cy="16" r="5" fill="#67e8f9" stroke="#0f172a" strokeWidth="2">
            <title>Latest date</title>
          </circle>
        </svg>
        <input
          type="range"
          aria-label="Oldest activity date"
          aria-valuetext={formatDisplayDate(selectedFrom)}
          min={0}
          max={rangeMaximum}
          value={selected.start}
          disabled={totalDays === 0}
          onChange={(event) => updateStart(Number(event.target.value))}
          onPointerUp={commit}
          onKeyUp={commit}
          onBlur={commit}
          className="date-range-input"
        />
      </div>

      <p className="sr-only" aria-live="polite">
        The end date is fixed at {formatDisplayDate(maximum)}. The start date snaps to {snapPoints.map((point) => point.labels.join(' or ')).join(', ')}.
      </p>

      <div className="flex justify-between text-[10px] tabular-nums text-slate-600">
        <time dateTime={availableFrom}>{formatDisplayDate(minimum)}</time>
        <time dateTime={availableTo}>{formatDisplayDate(maximum)}</time>
      </div>
    </section>
  )
}

function buildSnapPoints(minimum: Date, maximum: Date) {
  const points = new Map<number, SnapPoint>()
  const candidates = [
    ...rangePresets.map((preset) => ({ label: preset.label, date: preset.start(maximum) })),
    { label: 'All time', date: minimum },
  ]
  for (const candidate of candidates) {
    const date = candidate.date < minimum ? minimum : candidate.date
    const start = dayOffset(minimum, date)
    const point = points.get(start)
    if (point) point.labels.push(candidate.label)
    else points.set(start, { start, labels: [candidate.label] })
  }
  return [...points.values()].sort((left, right) => left.start - right.start)
}

function closestSnapPoint(value: number, points: SnapPoint[]) {
  return points.reduce((closest, point) =>
    Math.abs(point.start - value) < Math.abs(closest - value) ? point.start : closest,
  points[0]?.start ?? 0)
}

function timelineX(value: number, totalDays: number) {
  return 9 + (value / Math.max(1, totalDays)) * 982
}

function parseUTCDate(value: string) {
  return new Date(`${value}T00:00:00Z`)
}

function addDays(date: Date, days: number) {
  return new Date(date.getTime() + days * dayMilliseconds)
}

function addYears(date: Date, years: number) {
  const targetYear = date.getUTCFullYear() + years
  const targetMonth = date.getUTCMonth()
  const lastDay = new Date(Date.UTC(targetYear, targetMonth + 1, 0)).getUTCDate()
  return new Date(Date.UTC(targetYear, targetMonth, Math.min(date.getUTCDate(), lastDay)))
}

function dayOffset(from: Date, to: Date) {
  return Math.round((to.getTime() - from.getTime()) / dayMilliseconds)
}

function formatISODate(date: Date) {
  return date.toISOString().slice(0, 10)
}

function formatDisplayDate(date: Date) {
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    timeZone: 'UTC',
  }).format(date)
}

function clamp(value: number, minimum: number, maximum: number) {
  return Math.min(maximum, Math.max(minimum, value))
}
