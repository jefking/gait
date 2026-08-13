import { Building2, Check } from 'lucide-react'
import type { OwnerSummary } from '../lib/api'
import { Avatar } from './Avatar'

interface OwnerSelectProps {
  owners: OwnerSummary[]
  value?: number
  onChange: (ownerId?: number) => void
}

export function OwnerSelect({ owners, value, onChange }: OwnerSelectProps) {
  return (
    <fieldset>
      <legend className="text-xs font-medium text-slate-500">Organizations</legend>
      <div className="mt-2 flex flex-wrap gap-2" role="radiogroup" aria-label="Organizations">
        <OwnerOption
          label="All organizations"
          selected={value === undefined}
          onClick={() => onChange()}
        />
        {owners.map((owner) => (
          <OwnerOption
            key={owner.owner.id}
            label={owner.owner.login}
            avatarURL={owner.owner.avatar_url}
            selected={value === owner.owner.id}
            onClick={() => onChange(owner.owner.id)}
          />
        ))}
      </div>
    </fieldset>
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
      role="radio"
      aria-checked={selected}
      onClick={onClick}
      className={`inline-flex min-h-10 items-center gap-2 rounded-xl border px-3 py-2 text-sm font-medium transition ${
        selected
          ? 'border-cyan-300/35 bg-cyan-300/10 text-cyan-100'
          : 'border-white/8 bg-slate-950/60 text-slate-400 hover:border-white/15 hover:text-slate-200'
      }`}
    >
      {avatarURL ? (
        <Avatar src={avatarURL} name={label} size="xs" />
      ) : (
        <span className="grid size-5 shrink-0 place-items-center rounded-md bg-white/5">
          <Building2 aria-hidden="true" className="size-3 text-slate-500" />
        </span>
      )}
      <span>{label}</span>
      {selected && <Check aria-hidden="true" className="size-4 shrink-0 text-cyan-300" />}
    </button>
  )
}
