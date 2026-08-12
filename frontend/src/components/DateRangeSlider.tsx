import { CalendarRange } from 'lucide-react'
import { useState } from 'react'
import type { ActivityGranularity } from '../lib/api'

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
  end: number
}

const dayMilliseconds = 24 * 60 * 60 * 1000
const rangePresets = [
  { id: 'week', label: '7 days', start: (maximum: Date) => addDays(maximum, -6) },
  { id: 'month', label: '31 days', start: (maximum: Date) => addDays(maximum, -30) },
  { id: 'year', label: '1 year', start: (maximum: Date) => addYears(maximum, -1) },
] as const

export function DateRangeSlider({
  availableFrom,
  availableTo,
  from,
  to,
  granularity,
  onChange,
}: DateRangeSliderProps) {
  const minimum = parseUTCDate(availableFrom)
  const maximum = parseUTCDate(availableTo)
  const totalDays = Math.max(0, Math.round((maximum.getTime() - minimum.getTime()) / dayMilliseconds))
  const rangeMaximum = Math.max(1, totalDays)
  const externalStart = clamp(dayOffset(minimum, parseUTCDate(from)), 0, totalDays)
  const externalEnd = clamp(dayOffset(minimum, parseUTCDate(to)), externalStart, totalDays)
  const scope = `${availableFrom}:${availableTo}:${from}:${to}`
  const [draft, setDraft] = useState<DraftRange>({ scope, start: externalStart, end: externalEnd })
  const selected = draft.scope === scope ? draft : { scope, start: externalStart, end: externalEnd }
  const selectedFrom = addDays(minimum, selected.start)
  const selectedTo = addDays(minimum, selected.end)
  const startPercent = totalDays === 0 ? 0 : (selected.start / totalDays) * 100
  const endPercent = totalDays === 0 ? 100 : (selected.end / totalDays) * 100

  const updateStart = (value: number) => {
    setDraft({ scope, start: Math.min(value, selected.end), end: selected.end })
  }
  const updateEnd = (value: number) => {
    setDraft({ scope, start: selected.start, end: Math.max(value, selected.start) })
  }
  const commit = () => {
    onChange(formatISODate(selectedFrom), formatISODate(selectedTo))
  }
  const chooseRange = (candidate: Date) => {
    const nextFrom = candidate < minimum ? minimum : candidate
    onChange(formatISODate(nextFrom), availableTo)
  }
  const allTimeSelected = formatISODate(selectedFrom) === availableFrom
    && formatISODate(selectedTo) === availableTo

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
          <time dateTime={formatISODate(selectedTo)}>{formatDisplayDate(selectedTo)}</time>
        </p>
      </div>

      <div className="mt-4 flex flex-wrap gap-2" role="group" aria-label="Quick history ranges">
        {rangePresets.map((preset) => {
          const presetFrom = preset.start(maximum) < minimum ? minimum : preset.start(maximum)
          const selectedPreset = !allTimeSelected
            && formatISODate(selectedFrom) === formatISODate(presetFrom)
            && formatISODate(selectedTo) === availableTo
          return (
            <button
              key={preset.id}
              type="button"
              aria-pressed={selectedPreset}
              onClick={() => chooseRange(presetFrom)}
              className={`rounded-lg border px-3 py-1.5 text-xs font-medium transition ${selectedPreset ? 'border-cyan-300/35 bg-cyan-300/10 text-cyan-100' : 'border-white/8 bg-white/[0.025] text-slate-400 hover:border-cyan-300/20 hover:text-slate-200'}`}
            >
              {preset.label}
            </button>
          )
        })}
        <button
          type="button"
          aria-pressed={allTimeSelected}
          onClick={() => chooseRange(minimum)}
          className={`rounded-lg border px-3 py-1.5 text-xs font-medium transition ${allTimeSelected ? 'border-cyan-300/35 bg-cyan-300/10 text-cyan-100' : 'border-white/8 bg-white/[0.025] text-slate-400 hover:border-cyan-300/20 hover:text-slate-200'}`}
        >
          All time
        </button>
      </div>

      <div className="relative mt-4 h-8" data-testid="date-range-slider">
        <div className="absolute left-0 right-0 top-1/2 h-1.5 -translate-y-1/2 rounded-full bg-white/8" />
        <div
          className="absolute top-1/2 h-1.5 -translate-y-1/2 rounded-full bg-cyan-300"
          style={{ left: `${startPercent}%`, right: `${100 - endPercent}%` }}
        />
        <input
          type="range"
          aria-label="Oldest activity date"
          min={0}
          max={rangeMaximum}
          value={selected.start}
          disabled={totalDays === 0}
          onChange={(event) => updateStart(Number(event.target.value))}
          onPointerUp={commit}
          onKeyUp={commit}
          onBlur={commit}
          className="date-range-input"
          style={{ zIndex: selected.start >= selected.end - 1 ? 3 : 2 }}
        />
        <input
          type="range"
          aria-label="Latest activity date"
          min={0}
          max={rangeMaximum}
          value={selected.end}
          disabled={totalDays === 0}
          onChange={(event) => updateEnd(Number(event.target.value))}
          onPointerUp={commit}
          onKeyUp={commit}
          onBlur={commit}
          className="date-range-input"
          style={{ zIndex: 2 }}
        />
      </div>

      <div className="flex justify-between text-[10px] tabular-nums text-slate-600">
        <time dateTime={availableFrom}>{formatDisplayDate(minimum)}</time>
        <time dateTime={availableTo}>{formatDisplayDate(maximum)}</time>
      </div>
    </section>
  )
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
