# Gait

Gait is a personal GitHub statistics dashboard. It discovers every repository
visible to a supplied GitHub personal access token, keeps an app-owned clone of
each repository up to date, analyzes default-branch history with
[`git-changes-by-day`](https://github.com/moltenbot000/git-changes-by-day), and
combines the result with GitHub pull request history.

The React dashboard groups repositories by owner or organization and shows
adaptive daily, weekly, or monthly activity, repository totals, and contributor rankings. The Go
server performs all GitHub, Git, analysis, caching, and background-job work.

## Stack

- React 19, TypeScript, Vite, Tailwind CSS, D3, and Lucide
- Go 1.26.5 with Chi v5
- `git` and a pinned `git-changes-by-day` executable
- A multi-stage Docker image that serves the API and compiled frontend

## How syncing works

Every full page load opens the GitHub PAT dialog. Submitting a token starts an
asynchronous sync; if a cached snapshot exists, it can be viewed without
submitting another token.

The server:

1. Fetches the authenticated user and all repositories visible to the PAT.
2. Clones new repositories and fetches existing app-owned clones using four
   concurrent workers by default.
3. Checks out the latest default branch and runs `git-changes-by-day`.
4. Publishes that repository's commit statistics immediately, then fetches all
   pull requests on first use and only updated pull requests later.
5. Publishes the enriched repository again and atomically persists each
   incremental dashboard snapshot.

Repository workers expose their current `updating_git`, `analyzing`,
`pull_requests`, and `publishing` workflow step. The frontend receives
server-sent invalidation events and refreshes the canonical dashboard/activity
APIs as each step completes; slower polling remains as a reconnect fallback.

The PAT is held only in frontend and backend memory for the duration of the
request/job. It is not placed in clone URLs, Git remotes, files, snapshots,
logs, or browser storage. Git authentication is supplied only to the relevant
child process. Use HTTPS and external access control if the app is exposed
beyond a trusted local network.

### PAT access

A fine-grained PAT should grant read access to repository metadata, contents,
and pull requests for every repository to include. Organization repositories
may require SSO authorization. A token can only discover repositories that the
token itself is authorized to access. Repositories without pull-request read
permission remain visible, with PR statistics marked unavailable.

## Local development

Requirements:

- Node.js 26.5.0 and npm 12.0.2
- Go 1.26.5
- `git`
- `git-changes-by-day` on `PATH`

Install the analyzer pinned by the production image:

```sh
go install github.com/moltenbot000/git-changes-by-day@87ad8a8d0d770a120079a439cf3e9ab205c8456d
```

Start the API:

```sh
cd backend
go run ./cmd/server
```

Start the frontend in another terminal:

```sh
cd frontend
npm install
npm run dev
```

Open <http://localhost:5173>. Vite proxies `/api` to
`http://localhost:8080`. Local clones and reports are written to
`backend/data/` by default and are ignored by Git.

## Production container

Use a named volume so clones and statistics survive container replacement:

```sh
docker build -t gait .
docker run --rm \
  -p 8080:8080 \
  -v gait-data:/app/data \
  gait
```

Open <http://localhost:8080>. The image runs as an unprivileged user and the
volume contains only app-owned repository clones and reports—not the PAT.

## API

| Method | Route | Purpose |
| --- | --- | --- |
| `GET` | `/api/health` | Liveness check |
| `GET` | `/api/dashboard` | Latest snapshot and current sync status |
| `GET` | `/api/activity` | Adaptive owner or contributor activity |
| `GET` | `/api/events` | Live server-sent dashboard invalidations |
| `POST` | `/api/sync` | Start a background sync with `{ "pat": "…" }` |

`/api/activity` accepts `group_by=owner|contributor`,
`metric=commits|pull_requests`, optional numeric `owner_id` and
`repository_id` filters, and optional UTC `from`/`to` dates in `YYYY-MM-DD`
format. Responses include the oldest/latest available dates and automatically
use daily buckets for ranges up to 62 days, weekly buckets up to two years,
and monthly buckets for longer histories.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | Go HTTP port |
| `STATIC_DIR` | `../frontend/dist` | Compiled frontend directory |
| `DATA_DIR` | `./data` | Persistent clones, reports, and snapshot |
| `SYNC_CONCURRENCY` | `4` | Concurrent repository workers; capped at 16 |
| `TMPDIR` | Operating-system default | Temporary Go and CLI workspace |

The container sets `STATIC_DIR=/app/public`, `DATA_DIR=/app/data`, and
`TMPDIR=/app/tmp`.

## Checks

```sh
cd backend
go test ./...

cd ../frontend
npm run lint
npm run typecheck
npm test
npm run build
```

Commit statistics intentionally cover only commits reachable from the latest
default branch. Other branches, tags, submodules, and Git LFS contents are out
of scope. PR history is counted by author and creation month; open, closed, and
merged totals represent current status.
