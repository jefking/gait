import { afterEach, describe, expect, it, vi } from 'vitest'
import { getActivity, startSync } from './api'

afterEach(() => vi.unstubAllGlobals())

describe('API client', () => {
  it('serializes activity filters', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ group_by: 'contributor', metric: 'pull_requests', series: [] }), { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await getActivity({ groupBy: 'contributor', metric: 'pull_requests', ownerId: 7, repositoryId: 9 })
    expect(fetchMock.mock.calls[0][0]).toBe('/api/activity?group_by=contributor&metric=pull_requests&owner_id=7&repository_id=9')
  })

  it('sends a PAT in the one sync request body', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ sync: { state: 'discovering' } }), { status: 202 }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await startSync('ghp_secret')
    expect(fetchMock.mock.calls[0][1]).toEqual(expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ pat: 'ghp_secret' }),
    }))
  })
})
