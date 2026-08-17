import { Building2, Check, KeyRound, LoaderCircle, ShieldCheck, SlidersHorizontal, UserRound } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import type { OwnerIdentity } from '../lib/api'
import { Avatar } from './Avatar'

interface TokenModalProps {
  open: boolean
  mode?: 'connect' | 'settings'
  hasCachedData: boolean
  submitting: boolean
  error?: string
  targets: OwnerIdentity[]
  selectedTarget?: OwnerIdentity
  tokenAvailable: boolean
  excludeDead?: boolean
  onExcludeDeadChange?: (exclude: boolean) => void
  onDiscover: (pat?: string) => Promise<void>
  onSelectTarget: (targetId: number) => Promise<void>
  onClose: () => void
}

function GitHubMark({ className }: { className?: string }) {
  return <img src="/images/github.svg" alt="" aria-hidden="true" className={className} />
}

export function TokenModal({
  open, ...props
}: TokenModalProps) {
  if (!open) return null
  return <TokenModalContent {...props} />
}

function TokenModalContent({
  mode = 'connect', hasCachedData, submitting, error, targets, selectedTarget,
  tokenAvailable, excludeDead = false, onExcludeDeadChange, onDiscover, onSelectTarget, onClose,
}: Omit<TokenModalProps, 'open'>) {
  const [pat, setPAT] = useState('')
  const [activeTab, setActiveTab] = useState<'github' | 'projects'>('github')
  const [step, setStep] = useState<'credentials' | 'target'>(targets.length > 0 ? 'target' : 'credentials')
  const [targetId, setTargetId] = useState<number | undefined>(selectedTarget?.id)
  const [draftExcludeDead, setDraftExcludeDead] = useState(excludeDead)
  const inputRef = useRef<HTMLInputElement>(null)
  const validTargetId = targets.some((target) => target.id === targetId) ? targetId : undefined

  useEffect(() => {
    if (step !== 'credentials') return
    const timeout = window.setTimeout(() => inputRef.current?.focus(), 0)
    return () => window.clearTimeout(timeout)
  }, [step])

  const submitPAT = (event: FormEvent) => {
    event.preventDefault()
    const token = pat.trim()
    if (!token || submitting) return
    setPAT('')
    void onDiscover(token).then(() => setStep('target')).catch(() => undefined)
  }

  const discoverWithRetainedToken = () => {
    if (!submitting) void onDiscover().then(() => setStep('target')).catch(() => undefined)
  }

  const selectTarget = () => {
    if (validTargetId && !submitting) void onSelectTarget(validTargetId).catch(() => undefined)
  }

  return (
    <div className="fixed inset-0 z-50 grid min-h-dvh place-items-center overflow-y-auto bg-slate-950/85 p-4 backdrop-blur-md">
      <section role="dialog" aria-modal="true" aria-labelledby="pat-title" aria-describedby="pat-description" className="w-full max-w-lg overflow-hidden rounded-3xl border border-white/10 bg-slate-900 shadow-2xl shadow-black/60">
        <div className="border-b border-white/10 bg-gradient-to-br from-cyan-400/10 via-transparent to-indigo-500/10 px-6 py-7 sm:px-8">
          <div className="mb-5 flex size-12 items-center justify-center rounded-2xl bg-cyan-400/10 text-cyan-300 ring-1 ring-cyan-300/20"><GitHubMark className="size-7 object-contain invert" /></div>
          <h1 id="pat-title" className="text-2xl font-semibold tracking-tight text-white">{mode === 'settings' ? 'Settings' : 'Connect your GitHub history'}</h1>
          <p id="pat-description" className="mt-3 text-sm leading-6 text-slate-300">{mode === 'settings' ? 'Manage GitHub credentials and choose the single account used by this dashboard.' : 'Connect GitHub, then choose your personal account or one organization.'}</p>
        </div>

        {mode === 'settings' && (
          <div role="tablist" aria-label="Settings sections" className="flex gap-1 border-b border-white/10 px-6 pt-3 sm:px-8">
            {([['github', GitHubMark, 'GitHub'], ['projects', SlidersHorizontal, 'Projects']] as const).map(([tab, Icon, label]) => (
              <button key={tab} type="button" role="tab" aria-selected={activeTab === tab} onClick={() => setActiveTab(tab)} className={`inline-flex items-center gap-2 border-b-2 px-3 py-3 text-sm font-medium transition ${activeTab === tab ? 'border-cyan-300 text-cyan-200' : 'border-transparent text-slate-500 hover:text-slate-300'}`}><Icon aria-hidden="true" className={tab === 'github' ? 'size-4 object-contain invert' : 'size-4'} />{label}</button>
            ))}
          </div>
        )}

        {mode === 'settings' && activeTab === 'projects' ? (
          <div role="tabpanel" className="space-y-5 px-6 py-6 sm:px-8">
            <div><h2 className="text-base font-semibold text-white">Project filters</h2><p className="mt-1 text-sm leading-6 text-slate-400">Choose which projects appear in portfolio and graphed activity.</p></div>
            <label className="flex cursor-pointer items-start justify-between gap-5 rounded-xl border border-white/8 bg-white/[0.03] p-4"><span><span className="block text-sm font-medium text-slate-200">Exclude dead projects</span><span className="mt-1 block text-xs leading-5 text-slate-500">Hide projects marked dead and remove their activity from graph data.</span></span><input type="checkbox" aria-label="Exclude dead projects" checked={draftExcludeDead} onChange={(event) => setDraftExcludeDead(event.target.checked)} className="mt-0.5 size-5 shrink-0 accent-cyan-300" /></label>
            <div className="flex justify-end"><button type="button" onClick={() => { onExcludeDeadChange?.(draftExcludeDead); onClose() }} className="rounded-xl bg-cyan-300 px-5 py-2.5 text-sm font-semibold text-slate-950 transition hover:bg-cyan-200">Done</button></div>
          </div>
        ) : step === 'credentials' ? (
          <form onSubmit={submitPAT} className="space-y-5 px-6 py-6 sm:px-8">
            <label className="block"><span className="mb-2 flex items-center gap-2 text-sm font-medium text-slate-200"><KeyRound aria-hidden="true" className="size-4 text-cyan-300" />GitHub personal access token</span><input ref={inputRef} type="password" name="github-pat" value={pat} onChange={(event) => setPAT(event.target.value)} autoComplete="off" spellCheck={false} placeholder="github_pat_… or ghp_…" disabled={submitting} className="w-full rounded-xl border border-white/10 bg-slate-950 px-4 py-3 font-mono text-sm text-white outline-none transition placeholder:text-slate-600 focus:border-cyan-400/60 focus:ring-4 focus:ring-cyan-400/10 disabled:opacity-60" /></label>
            {error && <p role="alert" className="rounded-xl border border-rose-400/20 bg-rose-400/10 px-4 py-3 text-sm text-rose-200">{error}</p>}
            <div className="rounded-xl border border-white/8 bg-white/[0.03] p-4 text-xs leading-5 text-slate-400"><p className="flex items-start gap-2"><ShieldCheck aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-emerald-300" />The token is kept only in server memory and is never written to Git, disk, or browser storage.</p><p className="mt-2 pl-6">Grant metadata, contents, pull request, and Actions read access for repositories to include.</p></div>
            <div className="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
              {hasCachedData && <button type="button" onClick={onClose} disabled={submitting} className="rounded-xl px-4 py-2.5 text-sm font-medium text-slate-300 transition hover:bg-white/5 hover:text-white disabled:opacity-50">{mode === 'settings' ? 'Cancel' : 'View cached data'}</button>}
              {tokenAvailable && <button type="button" onClick={discoverWithRetainedToken} disabled={submitting} className="rounded-xl border border-white/10 px-4 py-2.5 text-sm font-medium text-slate-200 disabled:opacity-50">Use retained token</button>}
              <button type="submit" disabled={!pat.trim() || submitting} className="inline-flex items-center justify-center gap-2 rounded-xl bg-cyan-300 px-5 py-2.5 text-sm font-semibold text-slate-950 transition hover:bg-cyan-200 disabled:cursor-not-allowed disabled:opacity-50">{submitting && <LoaderCircle aria-hidden="true" className="size-4 animate-spin" />}{submitting ? 'Connecting…' : 'Continue'}</button>
            </div>
          </form>
        ) : (
          <div className="space-y-5 px-6 py-6 sm:px-8">
            <div><h2 className="text-base font-semibold text-white">Choose one GitHub owner</h2><p className="mt-1 text-sm leading-6 text-slate-400">Only repositories directly owned by this account or organization will be processed.</p></div>
            <div role="radiogroup" aria-label="GitHub owner" className="space-y-2">
              {targets.map((target) => {
                const selected = target.id === validTargetId
                const personal = target.type.toLowerCase() === 'user'
                return <button key={`${target.type}:${target.id}`} type="button" role="radio" aria-checked={selected} onClick={() => setTargetId(target.id)} className={`flex w-full items-center gap-3 rounded-xl border p-3 text-left transition ${selected ? 'border-cyan-300/40 bg-cyan-300/10' : 'border-white/8 bg-slate-950/50 hover:border-white/15'}`}>
                  {target.avatar_url ? <Avatar src={target.avatar_url} name={target.login} size="sm" /> : <span className="grid size-9 place-items-center rounded-xl bg-white/5">{personal ? <UserRound className="size-4" /> : <Building2 className="size-4" />}</span>}
                  <span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium text-slate-100">{target.login}</span><span className="block text-xs text-slate-500">{personal ? 'Personal account' : 'Organization'}</span></span>{selected && <Check aria-hidden="true" className="size-5 text-cyan-300" />}
                </button>
              })}
            </div>
            {error && <p role="alert" className="rounded-xl border border-rose-400/20 bg-rose-400/10 px-4 py-3 text-sm text-rose-200">{error}</p>}
            <div className="flex flex-col-reverse gap-3 sm:flex-row sm:justify-between"><button type="button" onClick={() => setStep('credentials')} disabled={submitting} className="rounded-xl px-4 py-2.5 text-sm font-medium text-slate-400 hover:bg-white/5 hover:text-white">Change token</button><div className="flex gap-2">{hasCachedData && <button type="button" onClick={onClose} disabled={submitting} className="rounded-xl px-4 py-2.5 text-sm font-medium text-slate-300">Cancel</button>}<button type="button" onClick={selectTarget} disabled={!validTargetId || submitting} className="inline-flex items-center justify-center gap-2 rounded-xl bg-cyan-300 px-5 py-2.5 text-sm font-semibold text-slate-950 disabled:opacity-50">{submitting && <LoaderCircle aria-hidden="true" className="size-4 animate-spin" />}{submitting ? 'Starting sync…' : 'Select and sync'}</button></div></div>
          </div>
        )}
      </section>
    </div>
  )
}
