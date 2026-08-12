import { Boxes, Container, Palette } from 'lucide-react'
import { ApiStatus } from './components/ApiStatus'

const stack = [
  {
    name: 'React + TypeScript',
    description: 'A typed frontend with fast Vite development builds.',
    icon: Boxes,
  },
  {
    name: 'Tailwind CSS',
    description: 'Utility-first styles compiled with the official Vite plugin.',
    icon: Palette,
  },
  {
    name: 'Go API',
    description: 'A Chi-powered API and static server in one container.',
    icon: Container,
  },
]

function App() {
  return (
    <main className="min-h-screen bg-slate-950 px-6 py-16 text-slate-100 sm:px-10">
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-12">
        <header className="max-w-3xl">
          <div className="mb-5 inline-flex items-center rounded-full border border-cyan-400/20 bg-cyan-400/10 px-3 py-1 text-sm font-medium text-cyan-300">
            Gait starter
          </div>
          <h1 className="text-4xl font-semibold tracking-tight text-white sm:text-6xl">
            React and Go, ready to move.
          </h1>
          <p className="mt-5 max-w-2xl text-lg leading-8 text-slate-300">
            A clean foundation for a typed React frontend and Go API, built and
            served together from one production container.
          </p>
        </header>

        <ApiStatus />

        <section aria-labelledby="stack-heading">
          <h2 id="stack-heading" className="sr-only">
            Included stack
          </h2>
          <div className="grid gap-4 md:grid-cols-3">
            {stack.map(({ name, description, icon: Icon }) => (
              <article
                key={name}
                className="rounded-2xl border border-white/10 bg-white/5 p-6 shadow-sm"
              >
                <Icon aria-hidden="true" className="size-6 text-cyan-300" />
                <h3 className="mt-4 font-semibold text-white">{name}</h3>
                <p className="mt-2 text-sm leading-6 text-slate-400">
                  {description}
                </p>
              </article>
            ))}
          </div>
        </section>
      </div>
    </main>
  )
}

export default App
