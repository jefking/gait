import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import type { DashboardResponse } from './lib/api'

const cachedDashboard: DashboardResponse = {
  sync: {
    state: 'idle',
    total_repositories: 0,
    completed_repositories: 0,
    failed_repositories: 0,
  },
  snapshot: {
    generated_at: '2025-01-02T03:04:05Z',
    viewer: {
      login: 'octocat',
      name: 'The Octocat',
      avatar_url: '',
      html_url: 'https://github.com/octocat',
    },
    totals: {
      owners: 1,
      repositories: 1,
      contributors: 1,
      commits: 2,
      files_changed: 2,
      lines_added: 4,
      lines_deleted: 1,
      pull_requests_opened: 1,
      pull_requests_open: 1,
      pull_requests_closed: 0,
      pull_requests_merged: 0,
      repositories_without_pr_access: 0,
      dead_repositories: 1,
    },
    owners: [
      {
        owner: { id: 1, login: 'octo-org', type: 'Organization', avatar_url: '', html_url: '' },
        repositories: 1,
        contributors: 1,
        commits: 2,
        lines_added: 4,
        lines_deleted: 1,
        pull_requests_opened: 1,
        dead_repositories: 1,
      },
    ],
    contributors: [
      { key: 'github:octocat', login: 'octocat', name: 'The Octocat', commits: 2, pull_requests: 1, repositories: 1, owners: 1, lines_added: 4, lines_deleted: 1 },
    ],
    repositories: [
      {
        id: 10,
        name: 'hello-world',
        full_name: 'octo-org/hello-world',
        html_url: 'https://github.com/octo-org/hello-world',
        default_branch: 'main',
        private: false,
        archived: false,
        fork: false,
        owner: { id: 1, login: 'octo-org', type: 'Organization', avatar_url: '', html_url: '' },
        commits: 2,
        contributors: 1,
        files_changed: 2,
        lines_added: 4,
        lines_deleted: 1,
        pull_requests: { opened: 1, open: 1, closed: 0, merged: 0 },
        sync_status: 'synced',
        liveness: {
          state: 'dead',
          is_dead: true,
          basis: 'default_branch_commits',
          scale: 'month',
          threshold_value: 3,
          threshold_days: 91,
          active_span_days: 365,
          inactive_days: 400,
          evaluated_at: '2025-01-02T00:00:00Z',
        },
      },
    ],
  },
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('App', () => {
  it('loads cached data by default and opens credentials from settings', async () => {
    const fetchMock = mockAPI()
    const print = vi.spyOn(window, 'print').mockImplementation(() => undefined)
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(await screen.findByText('Owners and repositories')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'hello-world' })).toBeInTheDocument()
    expect(screen.getByLabelText(/Dead repository/)).toBeInTheDocument()
    expect(screen.queryByText('Dead')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'GitHub settings' }))
    fireEvent.click(screen.getByRole('tab', { name: 'Projects' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Exclude dead projects' }))
    expect(screen.queryByRole('link', { name: 'hello-world' })).not.toBeInTheDocument()
    expect(screen.getByText('All repositories are excluded by project settings.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    fireEvent.click(screen.getByRole('button', { name: /Export PDF/ }))
    expect(print).toHaveBeenCalledOnce()
    fireEvent.click(screen.getByRole('button', { name: 'GitHub settings' }))
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: 'GitHub' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('blocks first use, submits a PAT, clears the input, and starts asynchronous progress', async () => {
    const fetchMock = mockAPI({
      snapshot: null,
      sync: {
        state: 'idle',
        total_repositories: 0,
        completed_repositories: 0,
        failed_repositories: 0,
      },
    })
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)
    const input = await screen.findByLabelText('GitHub personal access token')
    fireEvent.change(input, { target: { value: 'ghp_browser_secret' } })
    fireEvent.click(screen.getByRole('button', { name: 'Connect and sync' }))

    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url, init]) =>
        url === '/api/sync' && typeof init?.body === 'string' && init.body.includes('ghp_browser_secret'),
      )).toBe(true)
    })
    expect(input).toHaveValue('')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })
})

function mockAPI(dashboard: DashboardResponse = cachedDashboard) {
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    if (url === '/api/dashboard') return jsonResponse(dashboard)
    if (url.startsWith('/api/activity')) {
      return jsonResponse({ group_by: 'owner', metric: 'commits', series: [] })
    }
    if (url === '/api/sync' && init?.method === 'POST') {
      return jsonResponse({
        sync: {
          id: 'sync-1',
          state: 'discovering',
          total_repositories: 0,
          completed_repositories: 0,
          failed_repositories: 0,
          message: 'Connecting to GitHub',
        },
      }, 202)
    }
    return jsonResponse({ error: 'not found' }, 404)
  })
}

function jsonResponse(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}
