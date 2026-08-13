# Gait

Gait is a team delivery evidence dashboard built around one question: **are
human–agent teams shipping more software without degrading quality?** It discovers
organization-owned repositories visible to a supplied GitHub personal access token,
keeps an app-owned clone of each repository up to date, analyzes default-branch history with
[`git-changes-by-day`](https://github.com/moltenbot000/git-changes-by-day), and
combines the result with enriched GitHub pull request and GitHub Actions history.

The dashboard reports merged-PR velocity for human, agent, and collaborative work,
keeps raw shipped-work measures auditable, and evaluates throughput beside build,
review, revert, retention, merge-time, and PR-flow guardrails. It has one leading
work mode, never an individual leaderboard. Attribution uses exact participant
evidence; unknown work remains visible as coverage but is excluded from mode indices.
Repository telemetry is presented as observed association rather than causal proof.

## Stack

- React 19, TypeScript, Vite, Tailwind CSS, D3, and Lucide
- Go 1.26.5 with Chi v5
- `git` and a pinned `git-changes-by-day` executable
- A multi-stage Docker image that serves the API and compiled frontend

## How syncing works

Set `GITHUB_TOKEN` at startup or submit a GitHub PAT through the first-use dialog.
When the environment variable is present and no cached snapshot exists, the server
starts the first asynchronous sync automatically. Later page loads immediately use
the cached snapshot. The server retains the token only in process memory, so refresh
can reuse it while the app is running. Saving another token in settings replaces the
retained one.

The server:

1. Fetches the authenticated user and organization-owned repositories visible to the PAT.
2. Clones new repositories and fetches existing app-owned clones using four
   concurrent workers by default.
3. Checks out the latest default branch and runs the pinned `git-changes-by-day`,
   consuming its ordered structured co-author arrays and keying every commit participant
   by a verified GitHub handle when available and normalized author email otherwise.
4. Enriches commit events with full messages, parents, and touched paths, then
   publishes commit statistics immediately.
5. Fetches pull requests, reviews, additions, deletions, changed files, commit counts,
   merge SHAs, and up to 250 commit/co-author records on first use, then enriches only
   new or updated pull requests.
6. Fetches PR-triggered GitHub Actions runs in calendar partitions, retaining rerun
   attempts, ETags, permission coverage, and freshness. Dense partitions split before
   GitHub's 1,000-result search cap.
7. Publishes the enriched repository again and atomically persists each
   incremental dashboard snapshot.

Repository workers expose their current `updating_git`, `analyzing`,
`pull_requests`, `delivery_evidence`, and `publishing` workflow step. The frontend receives
server-sent invalidation events and refreshes the canonical dashboard/insight
APIs as each step completes; slower polling remains as a reconnect fallback.

### Dead repository classification

Gait marks inactive repositories with a skull and can exclude them from the
repository portfolio. A repository's inactivity threshold is 25% of the
inclusive span between its first and last default-branch commits. The API
reports that threshold using a day, week, month, or year scale based on the
length of the working span, while retaining exact day counts. Empty
repositories use their GitHub creation time; repositories lacking both commit
and creation metadata remain `unknown`.

Each repository row in `/api/dashboard` includes `liveness` metadata with the
classification, activity bounds, working-span and inactivity days, threshold,
scale, reason, and evaluation time. Incremental `/api/events` snapshot events
also identify the updated repository and include its liveness metadata.

The PAT is held only in backend memory while the process is running. It is not
placed in clone URLs, Git remotes, app data, snapshots, logs, browser storage, or
child-process environments. Git authentication is supplied only to the relevant
Git command. A local `.env` is operator-managed plaintext and is ignored by Git;
restrict it to trusted users. Use HTTPS and external access control if the app is
exposed beyond a trusted local network.

### PAT access

A fine-grained PAT should grant read access to repository metadata, contents,
pull requests (including reviews), and Actions for every repository to include.
Organization repositories may require SSO authorization. A token can only discover repositories that the
token itself is authorized to access. Repositories without pull-request read
permission remain visible, with PR statistics marked unavailable. Missing Actions
permission preserves velocity and all non-build insights while explicitly marking
GitHub Actions coverage unavailable.

## Local development

Requirements:

- Node.js 26.5.0 and npm 12.0.2
- Go 1.26.5
- `git`
- `git-changes-by-day` on `PATH`

Install the analyzer pinned by the production image:

```sh
go install github.com/moltenbot000/git-changes-by-day@3a591122a07574b6930d79343e6a8b40bb37e7b9
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
cp .env.example .env
chmod 600 .env
# Set GITHUB_TOKEN in .env before starting the container.

docker build -t gait .
docker run --rm \
  --env-file .env \
  -p 8080:8080 \
  -v gait-data:/app/data \
  gait
```

Open <http://localhost:8080>. The image runs as an unprivileged user and the
volume contains only app-owned repository clones and reports—not the PAT. Users
with access to the Docker daemon can inspect container environment variables.

## API

| Method | Route | Purpose |
| --- | --- | --- |
| `GET` | `/api/health` | Liveness check |
| `GET` | `/api/dashboard` | Latest snapshot and current sync status |
| `GET` | `/api/insights/delivery` | Scoped velocity, raw measures, quality, flow, impact uncertainty, and coverage |
| `GET` | `/api/insights/network` | Evidence-separated collaboration identities and edges |
| `GET` | `/api/identities` | Scoped identity classifications and evidence |
| `PATCH` | `/api/identities/{key}` | Persist a classification, display, or alias override |
| `GET` | `/api/events` | Live server-sent dashboard invalidations |
| `POST` | `/api/sync` | Start a background sync; `{ "pat": "…" }` replaces the in-memory token, while `{}` reuses it |

Delivery, network, and identity reads share exactly the same optional
`organization_id`, `repository_id`, `from`, `to`, and
`exclude_dead=true|false` query. Dates use UTC `YYYY-MM-DD`. A repository always
implies its organization, and repository selectors are limited to that organization.
Responses automatically use daily buckets for ranges up to 62 days, weekly buckets
up to two years, and monthly buckets for longer histories. Personal repositories are
not synced or returned; existing personal-repository cache files are left untouched.

### Attribution and interpretation

The shipped unit is a merged PR, assigned to its merge date. Known PR authors,
reviews submitted before merge, commit authors, recognized co-authors, and
user-maintained overrides determine `human`, `agent`, or `collaborative` mode.
Structured co-author fields from `git-changes-by-day` are authoritative for refreshed
commit reports; full-message trailer parsing remains only for older reports. Agent
attribution uses exact GitHub Bot/App or known-signature evidence and never
guesses from prose, code style, or code volume. Fully unknown PRs are excluded and
reported as attribution coverage.

For each repository, the first four complete adaptive periods establish baseline
means. Each mode contributes `100 × [0.5 × mode PRs / baseline total PRs + 0.5 ×
mode changed lines / baseline total changed lines]`; an available dimension receives
full weight if the other denominator is zero. Changed lines are additions plus
deletions. Commit count is batch context and never increases the index. Organization
views equal-weight repository indices so a large repository cannot dominate unrelated
delivery systems.

Impact uses fixed eight-week windows on either side of the first confirmed
agent-involved merge and excludes adoption week. Three treated repositories with two
matched same-organization, no-agent controls each enable deterministic, bootstrapped
difference-in-differences; otherwise the API returns a paired pre/post association or
insufficient evidence. Quality deltas remain separate. Collaboration links retain
co-authorship, review, and handoff evidence without ranked pair lists.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | Go HTTP port |
| `STATIC_DIR` | `../frontend/dist` | Compiled frontend directory |
| `DATA_DIR` | `./data` | Persistent clones, reports, and snapshot |
| `SYNC_CONCURRENCY` | `4` | Concurrent repository workers; capped at 16 |
| `TMPDIR` | Operating-system default | Temporary Go and CLI workspace |
| `GITHUB_TOKEN` | unset | Fine-grained GitHub PAT retained in memory for syncs |

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

### Test coverage

Backend coverage is measured across all packages, including command and
integration-heavy workflow code:

```sh
cd backend
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

This test suite does not currently provide 100% backend statement coverage. At
the time of this change, the command above reports 75.6% overall (92.5% for
`internal/api`, 74.9% for `internal/dashboard`, and 0% for `cmd/server`). The
focused tests cover delivery-index invariants, attribution, quality sample gates,
controlled impact eligibility, API validation failures, GitHub error handling, and
identity persistence; server lifecycle, repository process
failures, and several background-sync branches remain coverage gaps.

Commit statistics intentionally cover only commits reachable from the latest
default branch. Other branches, tags, submodules, Git LFS contents, and CI providers
outside GitHub Actions are out of scope. Direct default-branch commits are shown
separately and excluded from PR-based shipped velocity.
