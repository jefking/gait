import {
  Bot,
  CircleQuestionMark,
  FileDown,
  Fingerprint,
  Gauge,
  GitPullRequest,
  LayoutDashboard,
  RefreshCw,
  Settings,
  ShieldCheck,
  Trophy,
  UsersRound,
  X,
} from 'lucide-react'
import { useState } from 'react'
import type {
  ActorKind,
  DeliveryPerformanceMode,
  DeliveryQualityPoint,
  DeliveryRawMetrics,
  DeliveryResponse,
  IdentitySummary,
  Snapshot,
  SyncStatus,
} from '../lib/api'
import { isSyncActive } from '../lib/api'
import { Avatar } from './Avatar'
import { DateRangeSlider } from './DateRangeSlider'
import { IdentityRegistryView } from './IdentityRegistryView'
import { OwnerSelect } from './OwnerSelect'

interface DashboardViewProps {
  snapshot: Snapshot
  sync: SyncStatus
  delivery: DeliveryResponse | null
  deliveryLoading: boolean
  refreshing: boolean
  identities: IdentitySummary[]
  identitiesLoading: boolean
  ownerId?: number
  excludeDead: boolean
  dateFrom?: string
  dateTo?: string
  onOwnerChange: (id?: number) => void
  onDateRangeChange: (from: string, to: string) => void
  onIdentityChange: (key: string, update: { kind?: ActorKind; display_name?: string; canonical_key?: string; unmerge?: boolean }) => void
  onRefresh: () => void
  onSettings: () => void
}

const integer = new Intl.NumberFormat()
const percent = new Intl.NumberFormat(undefined, { style: 'percent', maximumFractionDigits: 0 })
const signedPercent = new Intl.NumberFormat(undefined, { style: 'percent', maximumFractionDigits: 0, signDisplay: 'exceptZero' })
const compact = new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 })

export function DashboardView({
  snapshot, sync, delivery, deliveryLoading, refreshing,
  identities, identitiesLoading, ownerId, excludeDead, dateFrom, dateTo,
  onOwnerChange, onDateRangeChange, onIdentityChange, onRefresh, onSettings,
}: DashboardViewProps) {
  const [view, setView] = useState<'dashboard' | 'identities'>('dashboard')

  const organizationOwners = snapshot.owners.filter((owner) => owner.owner.type === 'Organization')
  const unknownIdentities = identities.filter((identity) => identity.kind === 'unknown').length
  const meta = delivery?.meta
  const coverage = meta?.coverage
  const period = meta?.from && meta.to ? `${meta.from} → ${meta.to}` : 'Available merged-PR history'
  const scope = ownerId
    ? organizationOwners.find((owner) => owner.owner.id === ownerId)?.owner.login ?? 'Selected organization'
    : 'All organizations'
  const cards = [
    {
      label: 'Velocity vs opening pace',
      value: delivery?.summary.velocity_vs_baseline === undefined ? '—' : signedPercent.format(delivery.summary.velocity_vs_baseline),
      detail: 'Last 3 complete periods · baseline 100',
      icon: Gauge,
      tone: delivery?.summary.velocity_vs_baseline !== undefined && delivery.summary.velocity_vs_baseline < 0 ? 'text-rose-300' : 'text-cyan-300',
    },
    {
      label: 'Agent-associated share',
      value: delivery ? percent.format(delivery.summary.agent_associated_share) : '—',
      detail: 'Agent + collaborative index contribution',
      icon: Bot,
      tone: 'text-violet-300',
    },
    {
      label: 'Quality direction',
      value: delivery?.summary.quality_direction ?? '—',
      detail: 'Opening 4 vs latest 4 complete periods',
      icon: ShieldCheck,
      tone: delivery?.summary.quality_direction === 'declining' ? 'text-rose-300' : 'text-emerald-300',
    },
    {
      label: 'Leading work mode',
      value: delivery?.summary.leader ?? '—',
      detail: 'Largest cumulative index contribution · one leader',
      icon: UsersRound,
      tone: 'text-amber-300',
    },
  ]

  return (
    <div className="dashboard-shell mx-auto w-full max-w-[1560px] px-4 pb-20 pt-5 sm:px-6 lg:px-8">
      <header className="report-header flex flex-col justify-between gap-5 border-b border-white/8 pb-6 sm:flex-row sm:items-center">
        <div className="flex items-center gap-4">
          <a
            href="https://molten.bot"
            target="_blank"
            rel="noopener noreferrer"
            aria-label="Visit Molten.bot"
            className="shrink-0 rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-300"
          >
            <img src="/images/gait.svg" alt="" aria-hidden="true" className="h-12 w-auto object-contain" />
          </a>
          <div>
            <div className="flex items-center gap-2"><h1 className="text-xl font-semibold tracking-tight text-white">Gait</h1><span className="rounded-full border border-violet-300/15 bg-violet-300/[0.06] px-2 py-0.5 text-[10px] font-semibold uppercase tracking-widest text-violet-200">Team delivery evidence</span></div>
            <p className="mt-1 text-xs text-slate-500">Are human–agent teams shipping more without degrading quality? · updated {formatDate(snapshot.generated_at)}</p>
          </div>
        </div>
        <div className="flex items-center justify-between gap-3 sm:justify-end">
          <a href={snapshot.viewer.html_url} target="_blank" rel="noreferrer" className="flex items-center gap-3 rounded-xl px-2 py-1.5 transition hover:bg-white/5"><Avatar src={snapshot.viewer.avatar_url} name={snapshot.viewer.name || snapshot.viewer.login} size="sm" /><span className="hidden text-sm font-medium text-slate-300 sm:block">{snapshot.viewer.name || snapshot.viewer.login}</span></a>
          <div className="no-print flex gap-2"><HeaderButton label="Export PDF" onClick={() => window.print()} icon={<FileDown className="size-4" />} /><HeaderButton label="Refresh repositories" onClick={onRefresh} disabled={isSyncActive(sync)} icon={<RefreshCw className={`size-4 ${isSyncActive(sync) ? 'animate-spin' : ''}`} />} /><HeaderButton label="GitHub settings" onClick={onSettings} disabled={isSyncActive(sync)} icon={<Settings className="size-4" />} /></div>
        </div>
      </header>

      <SyncNotification sync={sync} />

      <nav className="no-print mt-4 flex flex-wrap items-center gap-2" aria-label="Primary views">
        <button type="button" onClick={() => setView('dashboard')} className={navClass(view === 'dashboard')}><LayoutDashboard className="size-4" /> Delivery</button>
        <button type="button" onClick={() => setView('identities')} className={navClass(view === 'identities')}><Fingerprint className="size-4" /> Identities <span className="rounded-full bg-black/20 px-2 py-0.5 text-xs tabular-nums">{identities.length}</span></button>
        {!identitiesLoading && unknownIdentities > 0 && <button type="button" onClick={() => setView('identities')} className="ml-auto inline-flex min-h-11 items-center gap-2 rounded-xl border border-amber-300/20 bg-amber-300/[0.07] px-3.5 text-sm font-medium text-amber-100"><CircleQuestionMark className="size-4" />{unknownIdentities} unattributed {unknownIdentities === 1 ? 'identity' : 'identities'}</button>}
      </nav>

      <GlobalScope
        owners={organizationOwners}
        ownerId={ownerId}
        excludeDead={excludeDead}
        meta={meta}
        dateFrom={dateFrom}
        dateTo={dateTo}
        onOwnerChange={onOwnerChange}
        onDateRangeChange={onDateRangeChange}
      />

      {view === 'identities' ? (
        <IdentityRegistryView identities={identities} loading={identitiesLoading} onChange={onIdentityChange} />
      ) : <>
        <section className="print-only report-cover" aria-label="Report context"><p className="report-kicker">Team delivery evidence</p><h2>Velocity with quality guardrails</h2><p className="report-period">{period}</p><dl><div><dt>Scope</dt><dd>{scope}</dd></div><div><dt>Attribution</dt><dd>{coverage ? `${coverage.attributed_pull_requests} of ${coverage.merged_pull_requests} merged PRs` : '—'}</dd></div><div><dt>Method</dt><dd>Repository-relative baseline 100 · equal-weight portfolio aggregation</dd></div></dl></section>

        <section aria-label="Delivery summary" aria-busy={refreshing} className="mt-6 rounded-3xl border border-cyan-300/15 bg-gradient-to-br from-cyan-300/[0.07] to-violet-300/[0.04] p-5 sm:p-7">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-cyan-300">Observed team evidence</p>
          <p className="mt-3 max-w-5xl text-xl font-medium leading-8 text-slate-100">{deliveryLoading ? 'Computing the scoped delivery evidence…' : delivery?.summary.narrative ?? 'No merged pull-request evidence is available for this scope.'}</p>
          <p className="mt-3 text-xs text-slate-500">{period} · {coverage?.unattributed_pull_requests ?? 0} unattributed merged PRs excluded from mode contributions · repository telemetry shows association, not causation.</p>
        </section>

        <section aria-label="Focused delivery signals" className="report-summary mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">{cards.map(({ label, value, detail, icon: Icon, tone }) => <article key={label} className="report-metric metric-card rounded-2xl border border-white/8 bg-slate-900/60 p-5"><div className="flex items-start justify-between gap-3"><div className="min-w-0"><p className="text-xs font-medium uppercase tracking-wider text-slate-500">{label}</p><p className="mt-2 truncate text-2xl font-semibold tracking-tight text-white">{deliveryLoading ? '…' : value}</p><p className="mt-1 text-xs text-slate-500">{detail}</p></div><span className={`rounded-xl bg-white/[0.04] p-2.5 ring-1 ring-white/8 ${tone}`}><Icon className="size-5" /></span></div></article>)}</section>

        <GraphSection eyebrow="Primary measure" title="Shipped velocity by work mode" description="Merged PRs and changed lines form a repository-relative baseline-100 index. The three contributions are additive; commits remain audit context only." icon={<Gauge className="size-4" />} updating={refreshing}>
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1.7fr)_minmax(24rem,1fr)]"><VelocityChart delivery={delivery} loading={deliveryLoading} /><RawTable delivery={delivery} loading={deliveryLoading} /></div>
        </GraphSection>

        <GraphSection eyebrow="Uncertainty" title="Agent-impact evidence" description="Fixed eight-week windows exclude adoption week. Controlled evidence is preferred; paired comparison is explicitly labeled as association." icon={<Bot className="size-4" />}>
          <ImpactPanel delivery={delivery} loading={deliveryLoading} />
        </GraphSection>

        <GraphSection eyebrow="Guardrails" title="Is quality going up or down?" description="Signals keep independent axes and samples. GitHub Actions incidence is not presented as coverage of external CI providers." icon={<ShieldCheck className="size-4" />}>
          <QualityMultiples delivery={delivery} loading={deliveryLoading} />
        </GraphSection>

        <GraphSection eyebrow="Flow + batch health" title="PR workload at the selected end date" description="Open-PR age and merged-PR batch distributions expose work waiting too long or arriving in oversized batches." icon={<GitPullRequest className="size-4" />}>
          <FlowHealth delivery={delivery} loading={deliveryLoading} />
        </GraphSection>

        <GraphSection eyebrow="Performance" title="Daily and overall leaders" description="Each active day is led by the participant composition with the largest shipped-work index contribution. Overall leadership sums those contributions across the selected period." icon={<Trophy className="size-4" />} updating={refreshing}>
          <PerformanceLeaders delivery={delivery} loading={deliveryLoading} />
        </GraphSection>

        <Methodology delivery={delivery} scope={scope} period={period} />
        <footer className="mt-8 flex flex-wrap items-center justify-between gap-3 border-t border-white/8 pt-5 text-xs text-slate-600"><p>{coverage?.attributed_pull_requests ?? 0} attributed · {coverage?.unattributed_pull_requests ?? 0} unattributed · {coverage?.truncated_commit_evidence_pull_requests ?? 0} commit lists truncated</p><p>Observed associations are not causal claims.</p></footer>
      </>}
    </div>
  )
}

function GlobalScope({ owners, ownerId, excludeDead, meta, dateFrom, dateTo, onOwnerChange, onDateRangeChange }: {
  owners: Snapshot['owners']; ownerId?: number; excludeDead: boolean; meta?: DeliveryResponse['meta']; dateFrom?: string; dateTo?: string;
  onOwnerChange:(id?:number)=>void; onDateRangeChange:(from:string,to:string)=>void
}) {
  return <section className="no-print mt-6 rounded-3xl border border-white/8 bg-slate-900/55 p-4 sm:p-5" aria-label="Global scope">
    <div className="mb-3"><p className="text-xs font-semibold uppercase tracking-widest text-cyan-300">Global scope</p><p className="mt-1 text-xs text-slate-500">Applies to every card, chart, table, identity, and export.</p></div>
    <OwnerSelect owners={owners} value={ownerId} onChange={onOwnerChange} />
    <p className="mt-3 text-xs text-slate-500">{excludeDead ? 'Dead repositories are excluded everywhere.' : 'Dead repositories are included everywhere.'}</p>
    {meta?.available_from && meta.available_to && dateFrom && dateTo && <DateRangeSlider availableFrom={meta.available_from} availableTo={meta.available_to} from={dateFrom} to={dateTo} granularity={meta.granularity} onChange={onDateRangeChange} />}
  </section>
}

function VelocityChart({ delivery, loading }: { delivery: DeliveryResponse | null; loading: boolean }) {
  if (loading) return <LoadingBlock label="Loading velocity chart" />
  const points = delivery?.velocity ?? []
  if (!points.length) return <EmptyBlock>Velocity requires attributed merged PRs and a non-zero opening baseline.</EmptyBlock>
  const width = 820, height = 340, left = 48, right = 18, top = 18, bottom = 48
  const maximum = Math.max(120, ...points.map((point) => point.total_index))
  const x = (index: number) => left + index * ((width - left - right) / Math.max(1, points.length - 1))
  const y = (value: number) => top + (maximum - value) / maximum * (height - top - bottom)
  const barWidth = Math.max(5, Math.min(34, (width - left - right) / Math.max(1, points.length) * .62))
  const totalPath = points.map((point, index) => `${index ? 'L' : 'M'} ${x(index)} ${y(point.total_index)}`).join(' ')
  return <div><div className="mb-3 flex flex-wrap gap-4 text-xs text-slate-400"><Legend color="#22d3ee" label="Human" /><Legend color="#8b5cf6" label="Agent" /><Legend color="#f59e0b" label="Collaborative" /><Legend color="#f8fafc" label="Total" line /></div><div className="overflow-x-auto"><svg role="img" aria-labelledby="velocity-title velocity-desc" viewBox={`0 0 ${width} ${height}`} className="min-w-[42rem] w-full"><title id="velocity-title">Stacked delivery velocity index by work mode</title><desc id="velocity-desc">Human, agent, and collaborative contributions sum to the total. Baseline is 100.</desc>{[0, 50, 100, 150, 200].filter((tick) => tick <= maximum).map((tick) => <g key={tick}><line x1={left} x2={width-right} y1={y(tick)} y2={y(tick)} stroke={tick===100 ? '#67e8f9' : '#334155'} strokeDasharray={tick===100 ? '6 5' : '2 7'} opacity={tick===100 ? .8 : .35} /><text x={left-8} y={y(tick)+4} fill="#64748b" fontSize="11" textAnchor="end">{tick}</text></g>)}{points.map((point,index)=>{const segments=[['human',point.human.index,'#22d3ee'],['agent',point.agent.index,'#8b5cf6'],['collaborative',point.collaborative.index,'#f59e0b']] as const;let cumulative=0;return <g key={point.date}>{segments.map(([key,value,color])=>{const upper=cumulative+value;const element=<rect key={key} x={x(index)-barWidth/2} y={y(upper)} width={barWidth} height={Math.max(0,y(cumulative)-y(upper))} fill={color} opacity={point.complete?.92:.35} rx="1" />;cumulative=upper;return element})}<text x={x(index)} y={height-22} fill="#64748b" fontSize="10" textAnchor="middle" transform={`rotate(-35 ${x(index)} ${height-22})`}>{point.date.slice(5)}</text></g>})}<path d={totalPath} fill="none" stroke="#f8fafc" strokeWidth="2" /><line x1={left} x2={width-right} y1={height-bottom} y2={height-bottom} stroke="#475569" /></svg></div><AccessibleVelocityTable delivery={delivery!} /></div>
}

function RawTable({ delivery, loading }: { delivery: DeliveryResponse | null; loading: boolean }) {
  if (loading) return <LoadingBlock label="Loading raw shipped-work table" />
  if (!delivery) return <EmptyBlock>No raw evidence is available.</EmptyBlock>
  const rows: Array<[string, DeliveryRawMetrics]> = [['Human',delivery.raw.human],['Agent',delivery.raw.agent],['Collaborative',delivery.raw.collaborative],['Total',delivery.raw.total]]
  return <div className="overflow-hidden rounded-2xl border border-white/8 bg-slate-950/45"><table className="w-full text-left text-xs"><caption className="px-4 py-3 text-left text-sm font-semibold text-slate-200">Auditable raw measures</caption><thead className="border-y border-white/8 text-slate-500"><tr><th className="px-3 py-2">Mode</th><th className="px-3 py-2 text-right">PRs</th><th className="px-3 py-2 text-right">Added</th><th className="px-3 py-2 text-right">Removed</th><th className="px-3 py-2 text-right">Changed</th><th className="px-3 py-2 text-right">Commits</th><th className="px-3 py-2 text-right">Direct</th></tr></thead><tbody>{rows.map(([label,row])=><tr key={label} className="border-b border-white/5 last:border-0"><th className="px-3 py-3 font-medium text-slate-300">{label}</th>{[row.merged_pull_requests,row.additions,row.deletions,row.changed_lines,row.commits,row.direct_commits].map((value,index)=><td key={index} className="px-3 py-3 text-right tabular-nums text-slate-400">{compact.format(value)}</td>)}</tr>)}</tbody></table><p className="border-t border-white/8 px-4 py-3 text-xs leading-5 text-slate-500">Changed lines = additions + deletions. Commits and direct commits never increase the velocity index.</p></div>
}

function ImpactPanel({ delivery, loading }: { delivery: DeliveryResponse | null; loading: boolean }) {
  if (loading) return <LoadingBlock label="Loading agent-impact evidence" />
  const impact = delivery?.impact
  if (!impact) return <EmptyBlock>Impact evidence is unavailable.</EmptyBlock>
  const qualityDeltas = impact.quality_deltas ?? []
  return <div className="grid gap-4 lg:grid-cols-[1.2fr_1fr_1fr]"><article className="rounded-2xl border border-white/8 bg-slate-950/45 p-5"><p className="text-xs uppercase tracking-wider text-slate-500">Evidence tier</p><p className="mt-2 text-lg font-semibold text-white">{impact.tier.replaceAll('_',' ')}</p><p className="mt-2 text-sm text-slate-400">{impact.verdict}</p></article><article className="rounded-2xl border border-white/8 bg-slate-950/45 p-5"><p className="text-xs uppercase tracking-wider text-slate-500">Velocity estimate</p><p className="mt-2 text-2xl font-semibold text-white">{impact.estimate === undefined ? '—' : `${impact.estimate >= 0 ? '+' : ''}${impact.estimate.toFixed(1)} points`}</p><p className="mt-2 text-xs text-slate-500">95% interval {impact.interval_low === undefined ? 'unavailable' : `${impact.interval_low.toFixed(1)} to ${impact.interval_high?.toFixed(1)}`}</p></article><article className="rounded-2xl border border-white/8 bg-slate-950/45 p-5"><p className="text-xs uppercase tracking-wider text-slate-500">Sample + coverage</p><dl className="mt-3 grid grid-cols-2 gap-3 text-sm"><Metric label="Treated" value={impact.treated_repositories} /><Metric label="Controls" value={impact.control_repositories} /><Metric label="Window" value={`${impact.pre_weeks}+${impact.post_weeks}w`} /><Metric label="Adoption" value={percent.format(impact.adoption_coverage)} /></dl></article>{qualityDeltas.length>0&&<div className="overflow-hidden rounded-2xl border border-white/8 bg-slate-950/45 lg:col-span-3"><table className="w-full text-left text-xs"><caption className="px-4 py-3 text-left font-medium text-slate-200">Separate quality deltas using the same fixed windows</caption><thead className="border-y border-white/8 text-slate-500"><tr><th className="px-4 py-2">Signal</th><th className="px-4 py-2 text-right">Delta</th><th className="px-4 py-2 text-right">95% interval</th><th className="px-4 py-2 text-right">n</th></tr></thead><tbody>{qualityDeltas.map((delta)=><tr key={delta.key} className="border-b border-white/5 last:border-0"><th className="px-4 py-3 font-medium text-slate-300">{delta.key.replaceAll('_',' ')}</th><td className="px-4 py-3 text-right tabular-nums text-slate-400">{delta.delta?.toFixed(3)??'—'}</td><td className="px-4 py-3 text-right tabular-nums text-slate-400">{delta.interval_low===undefined?'—':`${delta.interval_low.toFixed(3)} to ${delta.interval_high?.toFixed(3)}`}</td><td className="px-4 py-3 text-right tabular-nums text-slate-400">{delta.sample}</td></tr>)}</tbody></table></div>}</div>
}

const qualitySpecs: Array<{ key:keyof DeliveryQualityPoint; label:string; sample:keyof DeliveryQualityPoint; percent?:boolean }> = [
  {key:'actions_failure_incidence',label:'PRs with a failed Actions attempt',sample:'actions_pull_sample',percent:true},
  {key:'failed_actions_attempts',label:'Failed Actions attempts',sample:'total_actions_attempts'},
  {key:'total_actions_attempts',label:'Total Actions attempts',sample:'total_actions_attempts'},
  {key:'revert_rate',label:'Revert rate',sample:'commit_sample',percent:true},
  {key:'review_coverage',label:'Review coverage',sample:'review_sample',percent:true},
  {key:'retained_line_rate',label:'30-day retained-line rate',sample:'retention_sample',percent:true},
  {key:'median_merge_hours',label:'Median merge time (hours)',sample:'merge_time_sample'},
  {key:'p90_merge_hours',label:'p90 merge time (hours)',sample:'merge_time_sample'},
]

function QualityMultiples({ delivery, loading }: { delivery: DeliveryResponse | null; loading: boolean }) {
  if (loading) return <LoadingBlock label="Loading quality signals" />
  const points=delivery?.quality.points??[]
  if (!points.length) return <EmptyBlock>No quality samples are available.</EmptyBlock>
  const directionByKey=new Map((delivery?.quality.signals??[]).map((signal)=>[signal.key,signal]))
  return <div><div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">{qualitySpecs.map((spec)=><MiniQuality key={String(spec.key)} points={points} spec={spec} direction={directionByKey.get(String(spec.key))?.direction} />)}<article className="rounded-2xl border border-white/8 bg-slate-950/45 p-4"><p className="text-sm font-medium text-slate-200">GitHub Actions coverage</p><p className="mt-5 text-3xl font-semibold text-white">{integer.format(points.reduce((total,point)=>total+point.actions_pull_sample,0))}<span className="text-base font-normal text-slate-500"> PR-period samples</span></p><p className="mt-2 text-xs text-slate-500">GitHub Actions only · external CI providers are explicitly uncovered</p></article></div><AccessibleQualityTable points={points}/></div>
}

function MiniQuality({ points, spec, direction }: { points:DeliveryQualityPoint[];spec:typeof qualitySpecs[number];direction?:string }) {
  const values=points.map((point)=>typeof point[spec.key]==='number'?point[spec.key] as number:undefined)
  const defined=values.filter((value):value is number=>value!==undefined)
  const maximum=Math.max(...defined,1e-9), width=280,height=76
  const path=values.map((value,index)=>value===undefined?null:{x:index/(Math.max(1,values.length-1))*width,y:height-(value/maximum)*(height-8)}).reduce<string>((result,point,index)=>point?`${result}${result&&values[index-1]!==undefined?' L':' M'} ${point.x} ${point.y}`:result,'')
  let latestIndex=-1;for(let index=values.length-1;index>=0;index-=1){if(values[index]!==undefined){latestIndex=index;break}}const latest=latestIndex>=0?values[latestIndex]:undefined;const sample=latestIndex>=0?Number(points[latestIndex][spec.sample]):0
  return <article className="rounded-2xl border border-white/8 bg-slate-950/45 p-4"><div className="flex justify-between gap-3"><p className="text-sm font-medium text-slate-200">{spec.label}</p><span className={`text-[10px] uppercase tracking-wider ${direction==='improving'?'text-emerald-300':direction==='declining'?'text-rose-300':'text-slate-500'}`}>{direction??'sampled'}</span></div><svg aria-hidden="true" viewBox={`0 0 ${width} ${height}`} className="mt-3 h-20 w-full"><path d={path} fill="none" stroke="#67e8f9" strokeWidth="2" /><line x1="0" x2={width} y1={height-1} y2={height-1} stroke="#334155" /></svg><div className="flex justify-between text-xs text-slate-500"><span>latest {latest===undefined?'—':spec.percent?percent.format(latest):latest.toFixed(1)}</span><span>n={sample}</span></div></article>
}

function FlowHealth({ delivery, loading }: { delivery:DeliveryResponse|null;loading:boolean }) {
  if(loading)return <LoadingBlock label="Loading PR flow health"/>;const flow=delivery?.flow.summary;if(!flow)return <EmptyBlock>No PR-flow evidence is available.</EmptyBlock>
  const points=delivery?.flow.points??[]
  const metrics=[['Open PRs',flow.open_pull_requests],['Median open age',formatUnit(flow.median_open_age_days,'days')],['p90 open age',formatUnit(flow.p90_open_age_days,'days')],['Median changed lines',formatNumber(flow.median_changed_lines)],['p90 changed lines',formatNumber(flow.p90_changed_lines)],['Median commits / PR',formatNumber(flow.median_commits)],['p90 commits / PR',formatNumber(flow.p90_commits)],['Median additions / PR',formatNumber(flow.median_additions)],['p90 additions / PR',formatNumber(flow.p90_additions)],['Median deletions / PR',formatNumber(flow.median_deletions)],['p90 deletions / PR',formatNumber(flow.p90_deletions)]]
  return <div><div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">{metrics.map(([label,value])=><article key={label} className="rounded-2xl border border-white/8 bg-slate-950/45 p-4"><p className="text-xs text-slate-500">{label}</p><p className="mt-2 text-xl font-semibold text-white">{value}</p></article>)}</div><p className="mt-3 text-xs text-slate-500">As of {flow.as_of} · distribution sample n={flow.merged_pull_request_sample}. Additions and deletions per PR remain visible in the accessible flow table.</p><AccessibleFlowTable points={points}/></div>
}

const performanceModes: Array<{ key: DeliveryPerformanceMode; label: string; color: string }> = [
  { key: 'human', label: 'Human', color: '#22d3ee' },
  { key: 'human_human', label: 'Human + Human', color: '#34d399' },
  { key: 'human_agent', label: 'Human + Agent', color: '#f59e0b' },
  { key: 'agent', label: 'Agent', color: '#a78bfa' },
]

function PerformanceLeaders({ delivery, loading }: { delivery: DeliveryResponse | null; loading: boolean }) {
  if (loading) return <LoadingBlock label="Loading performance leaders" />
  const daily = delivery?.performance?.daily ?? []
  const overall = delivery?.performance?.overall
  if (!overall || !daily.length || overall.total_index <= 0) {
    return <EmptyBlock>Performance leaders require attributed merged PRs and a non-zero opening baseline.</EmptyBlock>
  }
  const width = 980, height = 236, left = 132, right = 24, top = 24, bottom = 40, rowGap = 43
  const start = Date.parse(`${delivery?.meta.from ?? daily[0].date}T00:00:00Z`)
  const end = Date.parse(`${delivery?.meta.to ?? daily.at(-1)?.date ?? daily[0].date}T00:00:00Z`)
  const span = Math.max(86_400_000, end - start)
  const x = (date: string) => left + ((Date.parse(`${date}T00:00:00Z`) - start) / span) * (width - left - right)
  const y = (mode: DeliveryPerformanceMode) => top + performanceModes.findIndex((candidate) => candidate.key === mode) * rowGap
  const maximum = Math.max(...daily.map((point) => point.total_index), 1)
  const dateLabels = [...new Set([daily[0].date, daily[Math.floor((daily.length - 1) / 2)].date, daily.at(-1)!.date])]
  const overallMaximum = Math.max(...performanceModes.map(({ key }) => overall[key].index))
  return <div>
    <div className="rounded-2xl border border-white/8 bg-slate-950/45 p-4 sm:p-5">
      <div className="flex flex-wrap items-end justify-between gap-2"><div><h3 className="text-sm font-semibold text-slate-200">Leader per active day</h3><p className="mt-1 text-xs text-slate-500">Dot size reflects that day’s total index contribution. Ties appear in every tied lane.</p></div><p className="text-xs tabular-nums text-slate-500">{integer.format(daily.length)} active {daily.length === 1 ? 'day' : 'days'}</p></div>
      <div className="mt-4 overflow-x-auto"><svg role="img" aria-labelledby="performance-title performance-desc" viewBox={`0 0 ${width} ${height}`} className="min-w-[46rem] w-full"><title id="performance-title">Daily performance leader by participant composition</title><desc id="performance-desc">Four lanes show which participant composition contributed the most baseline-relative shipped performance on each active day.</desc>{performanceModes.map(({ key, label, color })=><g key={key}><text x={left-12} y={y(key)+4} textAnchor="end" fill="#94a3b8" fontSize="12">{label}</text><line x1={left} x2={width-right} y1={y(key)} y2={y(key)} stroke="#334155" strokeDasharray="2 7" opacity=".65"/><circle cx={left-3} cy={y(key)} r="3" fill={color}/></g>)}{daily.flatMap((point)=>{const dayMaximum=Math.max(...performanceModes.map(({key})=>point[key].index));if(dayMaximum<=0)return[];const tied=performanceModes.filter(({key})=>Math.abs(point[key].index-dayMaximum)<1e-9);return tied.map(({key,label,color})=><circle key={`${point.date}-${key}`} cx={x(point.date)} cy={y(key)} r={3+Math.min(5,Math.sqrt(point.total_index/maximum)*5)} fill={color} fillOpacity={tied.length>1?.28:.82} stroke={color} strokeWidth={tied.length>1?2:1}><title>{point.date}: {tied.length>1?'tie including ':''}{label} · {point[key].index.toFixed(1)} index points · {point[key].merged_pull_requests} merged PRs</title></circle>)})}{dateLabels.map((date)=><g key={date}><line x1={x(date)} x2={x(date)} y1={height-bottom} y2={height-bottom+5} stroke="#64748b"/><text x={x(date)} y={height-16} fill="#64748b" fontSize="10" textAnchor={date===dateLabels[0]?'start':date===dateLabels.at(-1)?'end':'middle'}>{date}</text></g>)}</svg></div>
    </div>
    <div className="mt-5"><div className="flex flex-wrap items-end justify-between gap-2"><div><h3 className="text-sm font-semibold text-slate-200">Overall leaders</h3><p className="mt-1 text-xs text-slate-500">Cumulative contribution within the selected period.</p></div><span className="rounded-full border border-amber-300/20 bg-amber-300/[0.07] px-3 py-1 text-xs font-medium text-amber-100">{overall.leader === 'tie' ? 'Overall tie' : `${performanceModeLabel(overall.leader)} leads`}</span></div>
      <div className="mt-3 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">{performanceModes.map(({key,label,color})=>{const metric=overall[key];const leads=metric.index===overallMaximum;return <article key={key} className={`rounded-2xl border p-4 ${leads?'border-amber-300/30 bg-amber-300/[0.07]':'border-white/8 bg-slate-950/45'}`}><div className="flex items-center justify-between gap-2"><p className="text-sm font-semibold text-slate-200">{label}</p><span className="size-2.5 rounded-full" style={{background:color}}/>{leads&&<span className="sr-only">Overall leader</span>}</div><p className="mt-4 text-2xl font-semibold tabular-nums text-white">{metric.index.toFixed(1)}</p><p className="mt-1 text-xs text-slate-500">index points · {integer.format(metric.merged_pull_requests)} merged {metric.merged_pull_requests===1?'PR':'PRs'}</p></article>})}</div>
    </div>
    <AccessiblePerformanceTable delivery={delivery!}/>
  </div>
}

function performanceModeLabel(mode: DeliveryPerformanceMode | 'tie' | 'none') {
  return performanceModes.find((candidate) => candidate.key === mode)?.label ?? (mode === 'tie' ? 'Tie' : 'No mode')
}

function Methodology({ delivery, scope, period }: { delivery:DeliveryResponse|null;scope:string;period:string }) {
  const coverage=delivery?.meta.coverage
  return <section className="report-methodology mt-7 rounded-3xl border border-white/8 bg-slate-900/55 p-5 sm:p-6"><details><summary className="cursor-pointer text-sm font-semibold text-white">Methodology, coverage, and limitations</summary><div className="mt-5 grid gap-5 text-sm leading-6 text-slate-400 lg:grid-cols-2"><div><h3 className="font-medium text-slate-200">Velocity contract</h3><p className="mt-2">A shipped unit is a merged PR on its merge date. Per repository and mode: 100 × [0.5 × merged PRs ÷ baseline mean total PRs + 0.5 × changed lines ÷ baseline mean total changed lines]. An available dimension receives full weight when the other baseline is zero. Portfolio views equal-weight eligible repository indices.</p><p className="mt-2">Changed lines = additions + deletions. Commit count is batch context, never a positive velocity input. Human, agent, and collaborative contributions add exactly to total.</p></div><div><h3 className="font-medium text-slate-200">Attribution + performance</h3><p className="mt-2">Known PR authors, pre-merge reviewers, commit authors, and recognized co-authors determine work mode. Performance splits attributed PRs into one human, multiple humans, mixed human–agent, or agent-only participation. Daily leadership uses the largest index contribution; overall leadership sums daily contributions in the selected period.</p><p className="mt-2">Gait never infers agent use from prose or coding style. Unknown participants do not override known evidence; fully unknown PRs are excluded. Impact estimates use eight-week windows excluding adoption week. Repository telemetry supports controlled comparisons or observed associations, not unconditional causal claims.</p></div><div><h3 className="font-medium text-slate-200">Coverage</h3><p className="mt-2">{coverage?.detailed_pull_requests??0} enriched PRs; {coverage?.complete_commit_evidence_pull_requests??0} complete commit lists; {coverage?.truncated_commit_evidence_pull_requests??0} truncated; {coverage?.actions_covered_pull_requests??0} PRs with conclusive GitHub Actions evidence.{coverage?.actions_permission_denied?' Actions permission is missing.':''}</p></div><div><h3 className="font-medium text-slate-200">Research basis</h3><ul className="mt-2 list-disc pl-5"><li><a className="text-cyan-300" href="https://www.microsoft.com/en-us/research/publication/the-space-of-developer-productivity-theres-more-to-it-than-you-think/">SPACE framework</a></li><li><a className="text-cyan-300" href="https://dora.dev/guides/dora-metrics/">DORA delivery metrics</a> and <a className="text-cyan-300" href="https://cloud.google.com/blog/products/ai-machine-learning/announcing-the-2025-dora-report">2025 AI findings</a></li><li><a className="text-cyan-300" href="https://www.microsoft.com/en-us/research/publication/the-effects-of-generative-ai-on-high-skilled-work-evidence-from-three-field-experiments-with-software-developers/">Microsoft field experiments</a></li><li><a className="text-cyan-300" href="https://metr.org/blog/2026-02-24-uplift-update/">METR uplift update</a></li><li><a className="text-cyan-300" href="https://research.google/pubs/software-development-is-a-team-sport/">Software development is a team sport</a> and <a className="text-cyan-300" href="https://research.google/pubs/what-improves-developer-productivity-at-google-code-quality/">code-quality research</a></li></ul></div></div></details><div className="print-only mt-5"><h2>Methodology appendix</h2><p>Scope: {scope}. Period: {period}.</p><p>Exclusions and coverage: {delivery?.meta.unavailable?.join(', ')||'none reported'}.</p></div></section>
}

function AccessibleVelocityTable({delivery}:{delivery:DeliveryResponse}){return <table className="sr-only"><caption>Velocity index contributions over time</caption><thead><tr><th>Period</th><th>Human</th><th>Agent</th><th>Collaborative</th><th>Total</th><th>Complete</th></tr></thead><tbody>{delivery.velocity.map((point)=><tr key={point.date}><td>{point.date}</td><td>{point.human.index}</td><td>{point.agent.index}</td><td>{point.collaborative.index}</td><td>{point.total_index}</td><td>{point.complete?'yes':'no'}</td></tr>)}</tbody></table>}
function AccessiblePerformanceTable({delivery}:{delivery:DeliveryResponse}){return <table className="sr-only"><caption>Daily performance leaders by participant composition</caption><thead><tr><th>Date</th><th>Leader</th><th>Human</th><th>Human plus human</th><th>Human plus agent</th><th>Agent</th><th>Total</th></tr></thead><tbody>{(delivery.performance?.daily??[]).map((point)=><tr key={point.date}><td>{point.date}</td><td>{performanceModeLabel(point.leader)}</td><td>{point.human.index}</td><td>{point.human_human.index}</td><td>{point.human_agent.index}</td><td>{point.agent.index}</td><td>{point.total_index}</td></tr>)}</tbody></table>}
function AccessibleQualityTable({points}:{points:DeliveryQualityPoint[]}){return <table className="sr-only"><caption>Quality signals and samples by period</caption><thead><tr><th>Period</th><th>Actions failure incidence</th><th>Actions PR sample</th><th>Failed attempts</th><th>Total attempts</th><th>Revert rate</th><th>Commit sample</th><th>Review coverage</th><th>Review sample</th><th>Retained-line rate</th><th>Retention sample</th><th>Median merge hours</th><th>p90 merge hours</th><th>Merge sample</th></tr></thead><tbody>{points.map((point)=><tr key={point.date}><td>{point.date}</td><td>{point.actions_failure_incidence}</td><td>{point.actions_pull_sample}</td><td>{point.failed_actions_attempts}</td><td>{point.total_actions_attempts}</td><td>{point.revert_rate}</td><td>{point.commit_sample}</td><td>{point.review_coverage}</td><td>{point.review_sample}</td><td>{point.retained_line_rate}</td><td>{point.retention_sample}</td><td>{point.median_merge_hours}</td><td>{point.p90_merge_hours}</td><td>{point.merge_time_sample}</td></tr>)}</tbody></table>}
function AccessibleFlowTable({points}:{points:DeliveryResponse['flow']['points']}){return <table className="sr-only"><caption>PR flow and batch health by period</caption><thead><tr><th>Period</th><th>Merged PRs</th><th>Median changed lines</th><th>p90 changed lines</th><th>Median commits</th><th>p90 commits</th><th>Median additions</th><th>p90 additions</th><th>Median deletions</th><th>p90 deletions</th></tr></thead><tbody>{points.map((point)=><tr key={point.date}><td>{point.date}</td><td>{point.merged_pull_requests}</td><td>{point.median_changed_lines}</td><td>{point.p90_changed_lines}</td><td>{point.median_commits}</td><td>{point.p90_commits}</td><td>{point.median_additions}</td><td>{point.p90_additions}</td><td>{point.median_deletions}</td><td>{point.p90_deletions}</td></tr>)}</tbody></table>}
function Metric({label,value}:{label:string;value:string|number}){return <div><dt className="text-xs text-slate-500">{label}</dt><dd className="mt-1 font-medium text-slate-200">{value}</dd></div>}
function Legend({color,label,line=false}:{color:string;label:string;line?:boolean}){return <span className="inline-flex items-center gap-1.5"><span className={`${line?'h-0.5 w-4':'size-2.5 rounded-sm'}`} style={{background:color}}/>{label}</span>}
function LoadingBlock({label}:{label:string}){return <div aria-label={label} className="h-64 animate-pulse rounded-2xl bg-white/[0.03]"/>}
function EmptyBlock({children}:{children:React.ReactNode}){return <div className="grid min-h-40 place-items-center rounded-2xl border border-dashed border-white/10 p-6 text-center text-sm text-slate-500">{children}</div>}
function GraphSection({eyebrow,title,description,icon,updating=false,children}:{eyebrow:string;title:string;description:string;icon:React.ReactNode;updating?:boolean;children:React.ReactNode}){return <section aria-busy={updating} className="report-activity relative mt-7 rounded-3xl border border-white/8 bg-slate-900/55 p-4 shadow-2xl shadow-black/10 sm:p-6"><div className="mb-6 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between"><div><div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-widest text-cyan-300">{icon}{eyebrow}</div><h2 className="mt-2 text-xl font-semibold text-white">{title}</h2><p className="mt-1 max-w-3xl text-sm text-slate-500">{description}</p></div>{updating&&<span role="status" className="inline-flex items-center gap-1.5 text-xs text-cyan-200/70"><RefreshCw className="size-3.5 animate-spin"/>Updating</span>}</div>{children}</section>}
function HeaderButton({label,onClick,icon,disabled}:{label:string;onClick:()=>void;icon:React.ReactNode;disabled?:boolean}){return <button type="button" onClick={onClick} disabled={disabled} aria-label={label} title={label} className="grid size-10 place-items-center rounded-xl border border-white/10 bg-white/5 text-slate-200 transition hover:border-cyan-300/30 hover:bg-cyan-300/5 disabled:opacity-50">{icon}</button>}
function navClass(active:boolean){return `inline-flex min-h-11 items-center gap-2 rounded-xl border px-4 text-sm font-medium transition ${active?'border-cyan-300/30 bg-cyan-300/10 text-cyan-100':'border-white/8 bg-slate-900/55 text-slate-400 hover:border-white/15 hover:text-slate-200'}`}
function formatUnit(value:number|undefined,unit:string){return value===undefined?'—':`${value.toFixed(1)} ${unit}`}
function formatNumber(value:number|undefined){return value===undefined?'—':integer.format(Math.round(value))}
function formatDate(value:string){return new Intl.DateTimeFormat(undefined,{dateStyle:'medium',timeStyle:'short'}).format(new Date(value))}

export function SyncNotification({sync}:{sync:SyncStatus}){const[dismissedFailure,setDismissedFailure]=useState<string>();const active=isSyncActive(sync);const failureKey=`${sync.id??'sync'}:${sync.state}`;if(!active&&sync.state!=='failed')return null;if(sync.state==='failed'&&dismissedFailure===failureKey)return null;const progress=sync.total_repositories>0?Math.round(sync.completed_repositories/sync.total_repositories*100):0;const workflow=sync.current_workflows?.[0];return <aside aria-live="polite" aria-atomic="true" className={`no-print fixed right-4 top-4 z-40 w-[min(26rem,calc(100vw-2rem))] overflow-hidden rounded-2xl border bg-slate-900/95 shadow-2xl backdrop-blur-xl ${sync.state==='failed'?'border-rose-400/25 text-rose-100':'border-cyan-300/25 text-cyan-100'}`}><div className="flex items-start gap-3 px-4 py-3.5"><RefreshCw className={`mt-0.5 size-4 shrink-0 ${active?'animate-spin':''}`}/><div className="min-w-0 flex-1"><div className="flex justify-between gap-2"><p className="text-sm font-medium">{sync.message??sync.state}</p>{active&&sync.total_repositories>0&&<span className="text-xs tabular-nums opacity-75">{sync.completed_repositories} / {sync.total_repositories} repositories</span>}</div>{workflow&&<p className="mt-1 truncate text-xs opacity-65">{workflow.message}</p>}</div>{sync.state==='failed'&&<button type="button" onClick={()=>setDismissedFailure(failureKey)} aria-label="Dismiss sync notification" className="rounded-lg p-1.5 opacity-60 hover:bg-white/10"><X className="size-4"/></button>}</div>{active&&sync.total_repositories>0&&<div className="h-0.5 bg-white/5"><div className="h-full bg-cyan-300" style={{width:`${progress}%`}}/></div>}</aside>}
