import {
  Bot,
  CircleQuestionMark,
  Database,
  GripVertical,
  Search,
  UserRound,
} from 'lucide-react'
import { useMemo, useState, type DragEvent } from 'react'
import type { ActorKind, IdentitySummary } from '../lib/api'
import { Avatar } from './Avatar'

interface IdentityRegistryViewProps {
  identities: IdentitySummary[]
  loading: boolean
  onChange: (key: string, update: { kind: ActorKind }) => void
}

const kindDetails = {
  human: {
    label: 'Human',
    plural: 'Humans',
    Icon: UserRound,
    active: 'border-cyan-300/35 bg-cyan-300/10 text-cyan-100',
    icon: 'bg-cyan-300/10 text-cyan-300 ring-cyan-300/15',
    drop: 'hover:border-cyan-300/40 hover:bg-cyan-300/[0.08]',
  },
  agent: {
    label: 'Agent',
    plural: 'Agents',
    Icon: Bot,
    active: 'border-violet-300/35 bg-violet-300/10 text-violet-100',
    icon: 'bg-violet-300/10 text-violet-300 ring-violet-300/15',
    drop: 'hover:border-violet-300/40 hover:bg-violet-300/[0.08]',
  },
  unknown: {
    label: 'Unknown',
    plural: 'Unknown',
    Icon: CircleQuestionMark,
    active: 'border-amber-300/35 bg-amber-300/10 text-amber-100',
    icon: 'bg-amber-300/10 text-amber-300 ring-amber-300/15',
    drop: 'hover:border-amber-300/40 hover:bg-amber-300/[0.08]',
  },
} satisfies Record<ActorKind, {
  label: string
  plural: string
  Icon: typeof Bot
  active: string
  icon: string
  drop: string
}>

const kinds: ActorKind[] = ['human', 'agent', 'unknown']

export function IdentityRegistryView({ identities, loading, onChange }: IdentityRegistryViewProps) {
  const unknownCount = identities.filter((identity) => identity.kind === 'unknown').length
  const [filter, setFilter] = useState<ActorKind>('unknown')
  const [query, setQuery] = useState('')
  const [draggedKey, setDraggedKey] = useState<string>()
  const [dragOverKind, setDragOverKind] = useState<ActorKind>()
  const counts = useMemo(() => Object.fromEntries(kinds.map((kind) => [kind, identities.filter((identity) => identity.kind === kind).length])) as Record<ActorKind, number>, [identities])
  const visibleIdentities = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase()
    return identities
      .filter((identity) => identity.kind === filter)
      .filter((identity) => !normalizedQuery || [identity.name, identity.login, ...identity.aliases ?? []].some((value) => value?.toLocaleLowerCase().includes(normalizedQuery)))
      .sort((left, right) => right.commits - left.commits || left.name.localeCompare(right.name))
  }, [filter, identities, query])

  const classify = (key: string, kind: ActorKind) => {
    const identity = identities.find((candidate) => candidate.key === key)
    if (!identity || identity.kind === kind) return
    onChange(key, { kind })
  }

  const drop = (event: DragEvent<HTMLDivElement>, kind: ActorKind) => {
    event.preventDefault()
    const key = event.dataTransfer.getData('text/plain') || draggedKey
    if (key) classify(key, kind)
    setDraggedKey(undefined)
    setDragOverKind(undefined)
  }

  return (
    <section className="mt-6" aria-labelledby="identity-registry-title">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <p className="flex items-center gap-2 text-xs font-semibold uppercase tracking-widest text-cyan-300">
            <UserRound aria-hidden="true" className="size-4" /> Actor classification
          </p>
          <h2 id="identity-registry-title" className="mt-2 text-2xl font-semibold tracking-tight text-white">Identity registry</h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-slate-400">Review each Git identity before Gait builds relationship insights. Drag an actor onto a classification target, or use the buttons on its card.</p>
          <p className="mt-2 flex items-center gap-2 text-xs text-slate-600"><Database aria-hidden="true" className="size-3.5" /> Classifications are saved automatically and restored after app restarts.</p>
        </div>
        {!loading && (
          <div className={`flex items-center gap-3 rounded-2xl border px-4 py-3 ${unknownCount > 0 ? 'border-amber-300/20 bg-amber-300/[0.07] text-amber-100' : 'border-emerald-300/20 bg-emerald-300/[0.07] text-emerald-100'}`} role="status">
            {unknownCount > 0
              ? <CircleQuestionMark aria-hidden="true" className="size-5 shrink-0 text-amber-300" />
              : <UserRound aria-hidden="true" className="size-5 shrink-0 text-emerald-300" />}
            <div>
              <p className="text-sm font-semibold">{unknownCount} unknown {unknownCount === 1 ? 'actor' : 'actors'}</p>
              <p className="text-xs opacity-70">{unknownCount > 0 ? 'Insights unlock when every actor is classified.' : 'All actors are classified.'}</p>
            </div>
          </div>
        )}
      </div>

      <div className="mt-6 grid gap-3 sm:grid-cols-3" aria-label="Classification drop targets">
        {kinds.map((kind) => {
          const { Icon, label, plural, active, icon, drop: dropClass } = kindDetails[kind]
          const isDragTarget = dragOverKind === kind
          return (
            <div
              key={kind}
              role="region"
              aria-label={`Drop actor to classify as ${label}`}
              onDragEnter={(event) => { event.preventDefault(); setDragOverKind(kind) }}
              onDragOver={(event) => { event.preventDefault(); event.dataTransfer.dropEffect = 'move' }}
              onDragLeave={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDragOverKind(undefined) }}
              onDrop={(event) => drop(event, kind)}
              className={`flex min-h-32 items-center justify-center gap-4 rounded-2xl border border-dashed bg-slate-900/55 p-5 text-left transition ${isDragTarget ? active : `border-white/10 ${dropClass}`}`}
            >
              <span className={`grid size-12 shrink-0 place-items-center rounded-2xl ring-1 ${icon}`}><Icon aria-hidden="true" className="size-6" /></span>
              <span>
                <span className="block text-base font-semibold text-slate-100">{label}</span>
                <span className="mt-1 block text-xs text-slate-500">Drop here · {counts[kind]} {counts[kind] === 1 ? label.toLocaleLowerCase() : plural.toLocaleLowerCase()}</span>
              </span>
            </div>
          )
        })}
      </div>

      <div className="mt-5 overflow-hidden rounded-3xl border border-white/8 bg-slate-900/55">
        <div className="flex flex-col gap-3 border-b border-white/8 p-4 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex gap-2 overflow-x-auto" aria-label="Filter identities">
            {kinds.map((kind) => {
              const { Icon, plural, active } = kindDetails[kind]
              const selected = filter === kind
              return (
                <button
                  key={kind}
                  type="button"
                  aria-pressed={selected}
                  onClick={() => setFilter(kind)}
                  className={`inline-flex min-h-11 shrink-0 items-center gap-2 rounded-xl border px-3.5 text-sm font-medium transition ${selected ? active : 'border-white/8 bg-slate-950/50 text-slate-400 hover:border-white/15 hover:text-slate-200'}`}
                >
                  <Icon aria-hidden="true" className="size-4" /> {plural}<span className="rounded-full bg-black/20 px-2 py-0.5 text-xs tabular-nums">{counts[kind]}</span>
                </button>
              )
            })}
          </div>
          <label className="relative block min-w-0 lg:w-80">
            <span className="sr-only">Search identities</span>
            <Search aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-slate-600" />
            <input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search identities" className="min-h-11 w-full rounded-xl border border-white/10 bg-slate-950 py-2 pl-10 pr-3 text-sm text-slate-200 outline-none transition placeholder:text-slate-600 focus:border-cyan-300/30 focus:ring-2 focus:ring-cyan-300/10" />
          </label>
        </div>

        {loading ? (
          <div className="grid gap-3 p-4 md:grid-cols-2" aria-label="Loading identity registry">
            {Array.from({ length: 6 }, (_, index) => <div key={index} className="h-24 animate-pulse rounded-2xl bg-white/[0.035]" />)}
          </div>
        ) : visibleIdentities.length > 0 ? (
          <div className="grid gap-3 p-4 md:grid-cols-2">
            {visibleIdentities.map((identity) => <IdentityCard key={identity.key} identity={identity} dragged={draggedKey === identity.key} onDragStart={(event) => { event.dataTransfer.effectAllowed = 'move'; event.dataTransfer.setData('text/plain', identity.key); setDraggedKey(identity.key) }} onDragEnd={() => { setDraggedKey(undefined); setDragOverKind(undefined) }} onClassify={(kind) => classify(identity.key, kind)} />)}
          </div>
        ) : (
          <div className="grid min-h-52 place-items-center px-6 py-12 text-center">
            <div><CircleQuestionMark aria-hidden="true" className="mx-auto size-7 text-slate-600" /><p className="mt-3 text-sm font-medium text-slate-300">No {kindDetails[filter].plural.toLocaleLowerCase()} found</p><p className="mt-1 text-xs text-slate-600">{query ? 'Try a different search.' : 'Choose another classification filter.'}</p></div>
          </div>
        )}
      </div>
    </section>
  )
}

function IdentityCard({ identity, dragged, onDragStart, onDragEnd, onClassify }: {
  identity: IdentitySummary
  dragged: boolean
  onDragStart: (event: DragEvent<HTMLElement>) => void
  onDragEnd: () => void
  onClassify: (kind: ActorKind) => void
}) {
  return (
    <article draggable onDragStart={onDragStart} onDragEnd={onDragEnd} className={`group flex min-w-0 cursor-grab items-center gap-3 rounded-2xl border border-white/8 bg-slate-950/55 p-3 transition hover:border-white/15 active:cursor-grabbing ${dragged ? 'opacity-40' : ''}`}>
      <GripVertical aria-hidden="true" className="size-4 shrink-0 text-slate-700 transition group-hover:text-slate-500" />
      <Avatar src={identity.avatar_url} name={identity.name} size="md" />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-slate-200">{identity.name}</p>
        <p className="mt-0.5 truncate text-xs text-slate-600">{identity.evidence.replaceAll('_', ' ')} · {formatActivity(identity)}</p>
      </div>
      <div className="flex shrink-0 gap-1" aria-label={`Classify ${identity.name}`}>
        {kinds.map((kind) => {
          const { Icon, label, active } = kindDetails[kind]
          const selected = identity.kind === kind
          return <button key={kind} type="button" aria-label={`Classify ${identity.name} as ${label}`} aria-pressed={selected} title={label} onClick={() => onClassify(kind)} className={`grid size-11 place-items-center rounded-xl border transition ${selected ? active : 'border-transparent text-slate-600 hover:border-white/10 hover:bg-white/5 hover:text-slate-300'}`}><Icon aria-hidden="true" className="size-5" /></button>
        })}
      </div>
    </article>
  )
}

function formatActivity(identity: IdentitySummary) {
  const activity = [`${identity.commits} ${identity.commits === 1 ? 'commit' : 'commits'}`]
  if (identity.pull_requests > 0) activity.push(`${identity.pull_requests} ${identity.pull_requests === 1 ? 'PR' : 'PRs'}`)
  if (identity.reviews > 0) activity.push(`${identity.reviews} ${identity.reviews === 1 ? 'review' : 'reviews'}`)
  return activity.join(' · ')
}
