export interface HealthResponse {
  status: 'ok'
}

export async function getHealth(signal?: AbortSignal): Promise<HealthResponse> {
  const response = await fetch('/api/health', {
    headers: { Accept: 'application/json' },
    signal,
  })

  if (!response.ok) {
    throw new Error(`Health check failed with status ${response.status}`)
  }

  const result: unknown = await response.json()

  if (
    typeof result !== 'object' ||
    result === null ||
    !('status' in result) ||
    result.status !== 'ok'
  ) {
    throw new Error('Health check returned an invalid response')
  }

  return { status: 'ok' }
}
