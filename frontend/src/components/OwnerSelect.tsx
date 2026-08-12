import { Building2, Check, ChevronDown } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import type { OwnerSummary } from '../lib/api'
import { Avatar } from './Avatar'

interface OwnerSelectProps {
  owners: OwnerSummary[]
  value?: number
  onChange: (ownerId?: number) => void
}

export function OwnerSelect({ owners, value, onChange }: OwnerSelectProps) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const selected = owners.find((owner) => owner.owner.id === value)

  useEffect(() => {
    if (!open) return
    const closeOnOutsideClick = (event: PointerEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) setOpen(false)
    }
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    document.addEventListener('pointerdown', closeOnOutsideClick)
    document.addEventListener('keydown', closeOnEscape)
    return () => {
      document.removeEventListener('pointerdown', closeOnOutsideClick)
      document.removeEventListener('keydown', closeOnEscape)
    }
  }, [open])

  const choose = (ownerId?: number) => {
    onChange(ownerId)
    setOpen(false)
  }

  return (
    <div ref={containerRef} className="relative block text-xs font-medium text-slate-500">
      <span id="owner-filter-label">Owner</span>
      <button
        type="button"
        aria-labelledby="owner-filter-label owner-filter-value"
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
        className="mt-1.5 flex w-full items-center gap-2 rounded-lg border border-white/10 bg-slate-950 px-3 py-2 text-left text-sm text-slate-200 outline-none transition focus:border-cyan-400/50 focus:ring-2 focus:ring-cyan-400/10"
      >
        {selected ? (
          <Avatar src={selected.owner.avatar_url} name={selected.owner.login} size="xs" />
        ) : (
          <span className="grid size-5 shrink-0 place-items-center rounded-md bg-white/5">
            <Building2 aria-hidden="true" className="size-3 text-slate-500" />
          </span>
        )}
        <span id="owner-filter-value" className="min-w-0 flex-1 truncate">
          {selected?.owner.login ?? 'All owners'}
        </span>
        <ChevronDown aria-hidden="true" className={`size-4 shrink-0 text-slate-500 transition ${open ? 'rotate-180' : ''}`} />
      </button>

      {open && (
        <div
          role="listbox"
          aria-labelledby="owner-filter-label"
          className="absolute left-0 right-0 top-full z-30 mt-2 max-h-72 overflow-y-auto rounded-xl border border-white/10 bg-slate-900 p-1.5 shadow-2xl shadow-black/50"
        >
          <OwnerOption label="All owners" selected={!value} onClick={() => choose()} />
          {owners.map((owner) => (
            <OwnerOption
              key={owner.owner.id}
              label={owner.owner.login}
              avatarURL={owner.owner.avatar_url}
              selected={owner.owner.id === value}
              onClick={() => choose(owner.owner.id)}
            />
          ))}
        </div>
      )}
    </div>
  )
}

function OwnerOption({
  label,
  avatarURL,
  selected,
  onClick,
}: {
  label: string
  avatarURL?: string
  selected: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      role="option"
      aria-selected={selected}
      onClick={onClick}
      className={`flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left text-sm transition ${
        selected ? 'bg-cyan-300/10 text-cyan-100' : 'text-slate-300 hover:bg-white/5 hover:text-white'
      }`}
    >
      {avatarURL ? (
        <Avatar src={avatarURL} name={label} size="sm" />
      ) : (
        <span className="grid size-8 shrink-0 place-items-center rounded-xl bg-white/5">
          <Building2 aria-hidden="true" className="size-4 text-slate-500" />
        </span>
      )}
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {selected && <Check aria-hidden="true" className="size-4 shrink-0 text-cyan-300" />}
    </button>
  )
}
