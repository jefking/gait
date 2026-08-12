# Gait

A small React + Go starter that builds into a single production container. The
Go server exposes the API and serves the compiled React application, including
files copied from `frontend/public`.

## Stack

- React 19 and TypeScript, built with Vite 8
- Tailwind CSS 4 through the official Vite plugin
- D3.js for data visualization
- Lucide React icons
- Go 1.26.5 with Chi v5
- `git-changes-by-day` CLI for backend Git history analysis
- A multi-stage Docker production image

## Project layout

```text
.
├── backend/
│   ├── cmd/server/       # Application entry point
│   └── internal/api/     # Router, handlers, and static file serving
├── frontend/
│   ├── public/images/    # Files served unchanged from /images/*
│   └── src/
│       ├── components/   # Reusable React components
│       └── lib/          # Browser-side API helpers
└── Dockerfile            # Production build and runtime image
```

## Local development

Requirements:

- Node.js 26.5.0 and npm 12.0.2
- Go 1.26.5
- Docker, for building the production image

Start the API from one terminal:

```sh
cd backend
go run ./cmd/server
```

Start the frontend from another terminal:

```sh
cd frontend
npm install
npm run dev
```

Open <http://localhost:5173>. Vite proxies `/api` requests to the Go server at
`http://localhost:8080`, so no development CORS configuration is required.

## Production container

Build and run the application:

```sh
docker build -t gait .
docker run --rm -p 8080:8080 gait
```

Then open <http://localhost:8080> or check the API directly:

```sh
curl http://localhost:8080/api/health
```

The container runs as an unprivileged user and includes a health check against
`GET /api/health`.

### Git changes CLI

The production image installs
[`git-changes-by-day`](https://github.com/moltenbot000/git-changes-by-day) at
`/usr/local/bin/git-changes-by-day`, along with its required `git` executable.
The tool is pinned to a specific upstream commit in the Dockerfile so container
builds remain reproducible. It is a runtime CLI, not a dependency of the
backend Go module.

Backend code can invoke it with `os/exec`, for example:

```go
command := exec.CommandContext(
    ctx,
    "git-changes-by-day",
    "-repo", repositoryPath,
    "-text-out", outputPath,
)
```

The target repository must be available inside the container, usually through
a read-only bind mount. The CLI and `git` both run as the container's
unprivileged `app` user.

The image provides a private, app-owned workspace at `/app/tmp` and sets
`TMPDIR=/app/tmp`. Go code should use `os.TempDir()` and `os.CreateTemp` for
temporary file I/O; both resolve to this directory in the container. CLI output
paths should also be created beneath `os.TempDir()`:

```go
outputPath := filepath.Join(os.TempDir(), "commit-text.csv")
```

The image build verifies that the `app` user can execute `git`, execute
`git-changes-by-day`, and write to `TMPDIR`.

To build with another upstream commit or version:

```sh
docker build \
  --build-arg GIT_CHANGES_BY_DAY_VERSION=<git-ref> \
  -t gait .
```

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | HTTP port used by the Go server |
| `STATIC_DIR` | `../frontend/dist` | Directory containing the compiled frontend |
| `TMPDIR` | Operating-system default | Temporary file workspace used by Go and child processes |

The container sets `STATIC_DIR=/app/public` and `TMPDIR=/app/tmp`. During
frontend development, Vite serves the UI, so the Go API does not require an
existing frontend build.

## Static images

Place images in `frontend/public/images`. Vite copies them into the production
build unchanged, and they are available from root-relative URLs:

```tsx
<img src="/images/example.png" alt="Example" />
```

`frontend/public/images/placeholder.svg` is included as a serving example.

## Checks

Run backend tests:

```sh
cd backend
go test ./...
```

Run frontend checks:

```sh
cd frontend
npm run lint
npm run typecheck
npm run build
```
