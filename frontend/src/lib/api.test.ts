import { afterEach, describe, expect, it, vi } from 'vitest'
import { getIdentities, getInsightDelivery, getInsightNetwork, startSync, subscribeToDashboardEvents } from './api'

afterEach(() => vi.unstubAllGlobals())

describe('API client', () => {
  it('serializes one global scope for delivery, network, and identities', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({}), { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const scope = {
      organizationId: 7,
      excludeDead: true,
      from: '2024-01-02',
      to: '2024-03-04',
    }
    await Promise.all([getInsightDelivery(scope), getInsightNetwork(scope), getIdentities(scope)])
    const query = 'organization_id=7&exclude_dead=true&from=2024-01-02&to=2024-03-04'
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      `/api/insights/delivery?${query}`,
      `/api/insights/network?${query}`,
      `/api/identities?${query}`,
    ])
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
