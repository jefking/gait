import { afterEach, describe, expect, it, vi } from 'vitest'
import { getActivity, startSync, subscribeToDashboardEvents } from './api'

afterEach(() => vi.unstubAllGlobals())

describe('API client', () => {
  it('serializes activity filters', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ group_by: 'contributor', metric: 'pull_requests', series: [] }), { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await getActivity({
      groupBy: 'contributor',
      metric: 'pull_requests',
      ownerId: 7,
      repositoryId: 9,
      excludeDead: true,
      from: '2024-01-02',
      to: '2024-03-04',
    })
    expect(fetchMock.mock.calls[0][0]).toBe('/api/activity?group_by=contributor&metric=pull_requests&owner_id=7&repository_id=9&exclude_dead=true&from=2024-01-02&to=2024-03-04')
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

  it('requests a refresh without resending a PAT', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ sync: { state: 'discovering' } }), { status: 202 }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await startSync()
    expect(fetchMock.mock.calls[0][1]).toEqual(expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({}),
    }))
  })

  it('subscribes to live dashboard invalidations and closes cleanly', () => {
    class FakeEventSource {
      static instance: FakeEventSource
      listeners = new Map<string, EventListenerOrEventListenerObject>()
      closed = false

      constructor(readonly url: string) {
        FakeEventSource.instance = this
      }

      addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
        this.listeners.set(type, listener)
      }

      close() {
        this.closed = true
      }

      emit(type: string, data: string) {
        const listener = this.listeners.get(type)
        const event = new MessageEvent(type, { data })
        if (typeof listener === 'function') listener(event)
        else listener?.handleEvent(event)
      }
    }
    vi.stubGlobal('EventSource', FakeEventSource)
    const received = vi.fn()

    const unsubscribe = subscribeToDashboardEvents(received)
    expect(FakeEventSource.instance.url).toBe('/api/events')
    FakeEventSource.instance.emit('dashboard', '{"type":"snapshot","revision":12}')
    expect(received).toHaveBeenCalledWith({ type: 'snapshot', revision: 12 })

    unsubscribe()
    expect(FakeEventSource.instance.closed).toBe(true)
  })
})
