import { CircleCheck, CircleX, LoaderCircle } from 'lucide-react'
import { useEffect, useState } from 'react'
import { getHealth } from '../lib/api'

type ConnectionState = 'loading' | 'connected' | 'unavailable'

const statusContent = {
  loading: {
    title: 'Checking API connection',
    description: 'Waiting for the Go health endpoint to respond.',
    icon: LoaderCircle,
    iconClassName: 'animate-spin text-slate-300',
  },
  connected: {
    title: 'API connected',
    description: 'The React frontend can reach the Go service.',
    icon: CircleCheck,
    iconClassName: 'text-emerald-300',
  },
  unavailable: {
    title: 'API unavailable',
    description: 'Start the Go server, then refresh this page.',
    icon: CircleX,
    iconClassName: 'text-rose-300',
  },
} satisfies Record<
  ConnectionState,
  {
    title: string
    description: string
    icon: typeof CircleCheck
    iconClassName: string
  }
>

export function ApiStatus() {
  const [connectionState, setConnectionState] =
    useState<ConnectionState>('loading')

  useEffect(() => {
    const controller = new AbortController()

    getHealth(controller.signal)
      .then(() => setConnectionState('connected'))
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === 'AbortError') {
          return
        }

        setConnectionState('unavailable')
      })

    return () => controller.abort()
  }, [])

  const status = statusContent[connectionState]
  const StatusIcon = status.icon

  return (
    <section
      aria-live="polite"
      className="flex items-start gap-4 rounded-2xl border border-white/10 bg-slate-900 p-5 shadow-xl shadow-black/20"
    >
      <div className="rounded-xl bg-white/5 p-3">
        <StatusIcon
          aria-hidden="true"
          className={`size-6 ${status.iconClassName}`}
        />
      </div>
      <div>
        <h2 className="font-semibold text-white">{status.title}</h2>
        <p className="mt-1 text-sm text-slate-400">{status.description}</p>
      </div>
    </section>
  )
}
