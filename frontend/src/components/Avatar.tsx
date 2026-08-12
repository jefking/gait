import { useState } from 'react'

interface AvatarProps {
  src?: string
  name: string
  size?: 'sm' | 'md' | 'lg'
}

const sizes = {
  sm: 'size-8 text-[10px]',
  md: 'size-11 text-xs',
  lg: 'size-14 text-sm',
}

export function Avatar({ src, name, size = 'md' }: AvatarProps) {
  const [failedSrc, setFailedSrc] = useState<string>()

  const initials =
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase())
      .join('') || '?'

  if (src && failedSrc !== src) {
    return (
      <img
        src={src}
        alt=""
        className={`${sizes[size]} shrink-0 rounded-xl bg-slate-800 object-cover ring-1 ring-white/10`}
        onError={() => setFailedSrc(src)}
      />
    )
  }

  return (
    <span
      aria-hidden="true"
      className={`${sizes[size]} grid shrink-0 place-items-center rounded-xl bg-gradient-to-br from-cyan-400/25 to-indigo-400/20 font-semibold text-cyan-100 ring-1 ring-white/10`}
    >
      {initials}
    </span>
  )
}
