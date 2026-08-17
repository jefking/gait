# Gait

Gait is a team delivery evidence dashboard built around one question: **are
human–agent teams shipping more software without degrading quality?** It processes
repositories owned by one configured GitHub target—either the authenticated personal
account or one active organization—using a supplied GitHub personal access token,
keeps an app-owned clone of each repository up to date, analyzes default-branch history with
[`git-changes-by-day`](https://github.com/moltenbot000/git-changes-by-day), and
combines the result with enriched GitHub pull request and GitHub Actions history.

The dashboard reports merged-PR velocity for human, agent, and collaborative work,
keeps raw shipped-work measures auditable, and evaluates throughput beside build,
review, revert, retention, merge-time, and PR-flow guardrails. Daily and overall
performance leaders compare solo-human, multi-human, human–agent, and agent-only
code authorship—never individuals. Attribution requires complete commit-author and
recognized co-author evidence; unknown work remains visible but is excluded from mode indices.
Repository telemetry is presented as observed association rather than causal proof.

## Stack

- React 19, TypeScript, Vite, Tailwind CSS, D3, and Lucide
- Go 1.26.5 with Chi v5
- `git` and a pinned `git-changes-by-day` executable
- A multi-stage Docker image that serves the API and compiled frontend

## How syncing works

Set `GITHUB_TOKEN` at startup or submit a GitHub PAT through the first-use dialog.
Gait first validates the token and discovers the authenticated personal account plus
active organization memberships, then asks the user to select exactly one target.
An environment token with no configured target opens this selection step without
starting a sync. Later page loads immediately use the selected target's cached
snapshot. The server retains the token only in process memory, so refresh can reuse it
while the app is running. Submitting another token in settings replaces the retained
one; the selected target identity, but never the token, is persisted.

The server:

1. Fetches `/user` and active `/user/memberships/orgs`, then lists only repositories
   directly owned by the selected personal account or organization. Private and public
   repositories, forks, and archived repositories are included when token-visible.
2. Conditionally revalidates the selected target's paginated repository catalog.
   Unchanged repositories with a valid analyzed default-branch HEAD skip Git fetch and
   local analysis. A changed catalog timestamp triggers a fetch; changed HEADs,
   force-pushes, default-branch changes, and missing or upgraded caches trigger a full
   correctness-preserving local reanalysis.
3. Clones new repositories and fetches changed app-owned clones using four
   concurrent workers by default.
4. Checks out the latest default branch and runs the pinned `git-changes-by-day`,
   consuming its ordered structured co-author arrays and keying every commit participant
   by a verified GitHub handle when available and normalized author email otherwise.
5. Enriches commit events with full messages, parents, and touched paths, then
   publishes commit statistics immediately.
6. Fetches pull requests, reviews, additions, deletions, changed files, commit counts,
   merge SHAs, and up to 250 commit/co-author records on first use or cache-schema
   upgrade, then enriches only new or updated pull requests.
7. Fetches PR-triggered GitHub Actions runs in calendar partitions, retaining rerun
   attempts, ETags, permission coverage, and freshness. Dense partitions split before
   GitHub's 1,000-result search cap.
8. Publishes the enriched repository again and atomically persists a snapshot for the
   selected target. Repository clones and reports remain globally keyed by stable
   repository ID, so target switches and repository renames reuse verified caches.

Switching targets activates that target's prior snapshot immediately and refreshes its
delta in the background. Repositories removed from a target or hidden by a permission
change disappear from its snapshot, while their low-level caches remain untouched.

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

A fine-grained PAT should grant read access to organization membership, repository
metadata, contents, pull requests (including reviews), and Actions for the selected
owner's repositories.
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
| `GET` | `/api/dashboard` | Active target snapshot, sync status, selected/discovered targets, and non-secret token availability |
| `GET` | `/api/insights/delivery` | Scoped velocity, daily/overall performance, raw measures, quality, flow, impact uncertainty, and coverage |
| `GET` | `/api/insights/network` | Evidence-separated collaboration identities and edges |
| `GET` | `/api/identities` | Scoped identity classifications and evidence |
| `PATCH` | `/api/identities/{key}` | Persist a classification, display, or alias override |
| `GET` | `/api/events` | Live server-sent dashboard invalidations |
| `POST` | `/api/github/targets` | Validate or replace the retained PAT and discover `{ viewer, targets }`; `pat` is optional when one is retained |
| `PUT` | `/api/configuration/github-target` | Select one discovered `target_id`, activate its cache, and start a background refresh |
| `POST` | `/api/sync` | Refresh the configured target using the retained PAT; the request body must be `{}` |

Delivery, network, and identity reads are inherently scoped to the configured target
and share the optional `from`, `to`, and `exclude_dead=true|false` query. The removed
`organization_id` parameter receives a clear validation error.
Dates use UTC `YYYY-MM-DD`.
Responses automatically use daily buckets for ranges up to 62 days, weekly buckets
up to two years, and monthly buckets for longer histories.

### Attribution and interpretation

The shipped unit is a merged PR, assigned to its merge date. A complete PR commit
list, its commit authors, recognized co-authors, and user-maintained identity overrides
determine `human`, `agent`, or `collaborative` code authorship. PR openers, reviewers,
and the person who performs the merge are workflow participants, not authorship
signals. The performance breakdown further separates one human, multiple humans,
mixed human–agent authorship, and agent-only authorship. Each active day's leader
has the largest delivery-index contribution; the overall leader sums those daily
contributions inside the selected range.
Structured co-author fields from `git-changes-by-day` are authoritative for refreshed
commit reports; full-message trailer parsing remains only for older reports. Agent
attribution uses exact GitHub Bot/App or known-signature evidence and never
guesses from prose, code style, or code volume. Incomplete commit lists and PRs with
unresolved commit authors remain visible as authorship unknown and are excluded from
mode indices and leadership.

For each repository, the first four complete adaptive periods establish baseline
means. Each mode contributes `100 × [0.5 × mode PRs / baseline total PRs + 0.5 ×
mode changed lines / baseline total changed lines]`; an available dimension receives
full weight if the other denominator is zero. Changed lines are additions plus
deletions. Commit count is batch context and never increases the index. Owner
views equal-weight repository indices so a large repository cannot dominate unrelated
delivery systems.

Impact uses fixed eight-week windows on either side of the first confirmed
agent-involved merge and excludes adoption week. Three treated repositories with two
matched same-owner, no-agent controls each enable deterministic, bootstrapped
difference-in-differences; otherwise the API returns a paired pre/post association or
insufficient evidence. Quality deltas remain separate. Collaboration links retain
co-authorship, review, and handoff evidence without ranked pair lists.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | Go HTTP port |
| `STATIC_DIR` | `../frontend/dist` | Compiled frontend directory |
| `DATA_DIR` | `./data` | Persistent configuration, target snapshots, repository catalogs, clones, and reports |
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
