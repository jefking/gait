import {
  forceCenter,
  forceCollide,
  forceLink,
  forceManyBody,
  forceSimulation,
  type SimulationLinkDatum,
  type SimulationNodeDatum,
} from 'd3'
import { Bot, CircleQuestionMark, Maximize2, Minimize2, Move, Network, Pause, Play, UserRound, UsersRound, ZoomIn, ZoomOut } from 'lucide-react'
import { memo, useEffect, useMemo, useRef, useState } from 'react'
import type { ActorKind, NetworkEdge, NetworkNode, NetworkResponse } from '../lib/api'
import { Avatar } from './Avatar'

type EdgeMetric = 'interaction_days' | 'coauthorships' | 'review_interactions' | 'handoffs'

interface DrawNode extends SimulationNodeDatum, NetworkNode {
  radius: number
}

interface DrawEdge extends SimulationLinkDatum<DrawNode> {
  source: string | DrawNode
  target: string | DrawNode
  data: NetworkEdge
}

interface ViewTransform {
  x: number
  y: number
  scale: number
}

interface PointerGesture {
  id: number
  startX: number
  startY: number
  lastX: number
  lastY: number
  moved: boolean
}

interface TeamNetworkProps {
  network: NetworkResponse | null
  loading: boolean
  selectedKey?: string
  onSelect: (key?: string) => void
  onClassify: (key: string, kind: ActorKind) => void
  onRename: (key: string, displayName: string) => void
  onMerge: (key: string, canonicalKey: string) => void
  onUnmerge: (key: string) => void
}

const width = 960
const height = 470
const colors: Record<ActorKind, string> = {
  human: '#22d3ee',
  agent: '#a78bfa',
  unknown: '#94a3b8',
}

export const TeamNetwork = memo(function TeamNetwork({ network, loading, selectedKey, onSelect, onClassify, onRename, onMerge, onUnmerge }: TeamNetworkProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const viewportRef = useRef<HTMLDivElement>(null)
  const drawNodes = useRef<DrawNode[]>([])
  const drawEdges = useRef<DrawEdge[]>([])
  const positions = useRef(new Map<string, { x: number; y: number }>())
  const viewport = useRef<ViewTransform>({ x: 0, y: 0, scale: 1 })
  const viewportChanged = useRef(false)
  const redraw = useRef<() => void>(() => undefined)
  const selectedKeyRef = useRef(selectedKey)
  const selectedPairRef = useRef<string | undefined>(undefined)
  const pointerGesture = useRef<PointerGesture | undefined>(undefined)
  const [metric, setMetric] = useState<EdgeMetric>('interaction_days')
  const [periodIndex, setPeriodIndex] = useState<number>()
  const [playing, setPlaying] = useState(false)
  const [panning, setPanning] = useState(false)
  const [expanded, setExpanded] = useState(false)
  const [selectedPair, setSelectedPair] = useState<string>()
  const selected = network?.nodes.find((node) => node.key === selectedKey)
  const selectedEdge = network?.edges.find((edge) => edgeKey(edge) === selectedPair)
  const periods = useMemo(() => Array.from(new Set(network?.edges.flatMap((edge) => edge.periods ?? []) ?? [])).sort(), [network])
  const activePeriodIndex = Math.min(periodIndex ?? Math.max(0, periods.length - 1), Math.max(0, periods.length - 1))
  const activePeriod = periods[activePeriodIndex]
  const visibleNetwork = useMemo(() => network ? {
    ...network,
    edges: activePeriod ? network.edges.filter((edge) => (edge.periods ?? []).some((period) => period <= activePeriod)) : network.edges,
  } : null, [activePeriod, network])

  useEffect(() => {
    if (!playing || periods.length < 2 || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
    const timer = window.setInterval(() => setPeriodIndex((current) => {
      const next = (current ?? 0) + 1
      if (next >= periods.length - 1) {
        setPlaying(false)
        return periods.length - 1
      }
      return next
    }), 900)
    return () => window.clearInterval(timer)
  }, [periods.length, playing])

  useEffect(() => {
    selectedKeyRef.current = selectedKey
    selectedPairRef.current = selectedPair
    redraw.current()
  }, [selectedKey, selectedPair])

  useEffect(() => {
    const syncFullscreenState = () => setExpanded(document.fullscreenElement === viewportRef.current)
    document.addEventListener('fullscreenchange', syncFullscreenState)
    return () => document.removeEventListener('fullscreenchange', syncFullscreenState)
  }, [])

  useEffect(() => {
    if (!expanded) return
    const previousOverflow = document.body.style.overflow
    const exitOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && document.fullscreenElement !== viewportRef.current) setExpanded(false)
    }
    document.body.style.overflow = 'hidden'
    document.addEventListener('keydown', exitOnEscape)
    return () => {
      document.body.style.overflow = previousOverflow
      document.removeEventListener('keydown', exitOnEscape)
    }
  }, [expanded])

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas || !visibleNetwork || visibleNetwork.nodes.length === 0) return
    const context = canvas.getContext('2d')
    if (!context) return
    const maximumActivity = Math.max(1, ...visibleNetwork.nodes.map((node) => node.activity))
    const nodes: DrawNode[] = visibleNetwork.nodes.map((node, index) => {
      const previous = positions.current.get(node.key)
      return {
        ...node,
        radius: 10 + Math.sqrt(node.activity / maximumActivity) * 15,
        x: previous?.x ?? width / 2 + Math.cos((index / Math.max(1, visibleNetwork.nodes.length)) * Math.PI * 2) * 150,
        y: previous?.y ?? height / 2 + Math.sin((index / Math.max(1, visibleNetwork.nodes.length)) * Math.PI * 2) * 120,
      }
    })
    const edges: DrawEdge[] = visibleNetwork.edges.map((edge) => ({ source: edge.source, target: edge.target, data: edge }))
    const maxEdge = Math.max(1, ...visibleNetwork.edges.map((edge) => edge[metric]))
    const reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
    const render = () => {
      context.clearRect(0, 0, width, height)
      context.fillStyle = '#020617'
      context.fillRect(0, 0, width, height)
      context.save()
      context.translate(viewport.current.x, viewport.current.y)
      context.scale(viewport.current.scale, viewport.current.scale)
      for (const edge of edges) {
        const source = edge.source as DrawNode
        const target = edge.target as DrawNode
        if (!source.x || !source.y || !target.x || !target.y) continue
        const value = edge.data[metric]
        context.beginPath()
        context.moveTo(source.x, source.y)
        context.lineTo(target.x, target.y)
        context.strokeStyle = edgeKey(edge.data) === selectedPairRef.current ? 'rgba(255,255,255,.9)' : edge.data.pair_type === 'human_agent'
          ? 'rgba(196,181,253,.48)'
          : edge.data.pair_type === 'human_human'
            ? 'rgba(103,232,249,.35)'
            : 'rgba(148,163,184,.22)'
        context.lineWidth = 0.75 + (value / maxEdge) * 5
        context.stroke()
      }
      for (const node of nodes) {
        if (!node.x || !node.y) continue
        positions.current.set(node.key, { x: node.x, y: node.y })
        drawActor(context, node, node.key === selectedKeyRef.current, viewport.current.scale)
      }
      context.restore()
    }
    redraw.current = render

    const simulation = forceSimulation(nodes)
      .force('link', forceLink<DrawNode, DrawEdge>(edges).id((node) => node.key).distance(85).strength(0.35))
      .force('charge', forceManyBody().strength(-210))
      .force('collide', forceCollide<DrawNode>().radius((node) => node.radius + 10))
      .force('center', forceCenter(width / 2, height / 2))
      .on('tick', render)
      .on('end', () => {
        if (!viewportChanged.current) viewport.current = fitTransform(nodes)
        render()
      })
    drawNodes.current = nodes
    drawEdges.current = edges
    if (reducedMotion) {
      simulation.stop()
      for (let index = 0; index < 180; index += 1) simulation.tick()
      if (!viewportChanged.current) viewport.current = fitTransform(nodes)
      render()
    }
    return () => { simulation.stop(); redraw.current = () => undefined }
  }, [visibleNetwork, metric])

  if (loading) return <div className="h-[470px] animate-pulse rounded-2xl bg-white/[0.03]" aria-label="Loading team constellation" />
  if (!network || network.nodes.length === 0) return <EmptyNetwork />

  const canvasPoint = (element: HTMLCanvasElement, clientX: number, clientY: number) => {
    const bounds = element.getBoundingClientRect()
    return {
      x: ((clientX - bounds.left) / bounds.width) * width,
      y: ((clientY - bounds.top) / bounds.height) * height,
    }
  }

  const worldPoint = (element: HTMLCanvasElement, clientX: number, clientY: number) => {
    const point = canvasPoint(element, clientX, clientY)
    return {
      x: (point.x - viewport.current.x) / viewport.current.scale,
      y: (point.y - viewport.current.y) / viewport.current.scale,
    }
  }

  const applyViewport = (next: ViewTransform) => {
    viewport.current = next
    viewportChanged.current = true
    redraw.current()
  }

  const zoomAt = (x: number, y: number, factor: number) => {
    const current = viewport.current
    const scale = Math.max(0.3, Math.min(4, current.scale * factor))
    const worldX = (x - current.x) / current.scale
    const worldY = (y - current.y) / current.scale
    applyViewport({ x: x - worldX * scale, y: y - worldY * scale, scale })
  }

  const fitNetwork = () => applyViewport(fitTransform(drawNodes.current))

  const toggleFullscreen = async () => {
    const element = viewportRef.current
    if (!element) return
    if (expanded) {
      if (document.fullscreenElement === element && document.exitFullscreen) {
        try {
          await document.exitFullscreen()
        } finally {
          setExpanded(false)
        }
      } else {
        setExpanded(false)
      }
      return
    }
    if (element.requestFullscreen) {
      try {
        await element.requestFullscreen()
        setExpanded(true)
        return
      } catch {
        // Fall back to a viewport-filling overlay when native fullscreen is unavailable.
      }
    }
    setExpanded(true)
  }

  const selectFromCanvas = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const { x, y } = worldPoint(event.currentTarget, event.clientX, event.clientY)
    const hitPadding = 8 / viewport.current.scale
    const candidate = drawNodes.current
      .map((node) => ({ node, distance: Math.hypot((node.x ?? 0) - x, (node.y ?? 0) - y) }))
      .filter(({ node, distance }) => distance <= visualNodeRadius(node, viewport.current.scale) + hitPadding)
      .sort((left, right) => left.distance - right.distance)[0]?.node
    if (candidate) {
      setSelectedPair(undefined)
      onSelect(candidate.key)
      return
    }
    const pair = drawEdges.current
      .map((edge) => ({ edge, distance: pointToSegmentDistance(x, y, edge.source as DrawNode, edge.target as DrawNode) }))
      .filter(({ distance }) => distance <= 9 / viewport.current.scale)
      .sort((left, right) => left.distance - right.distance)[0]?.edge.data
    setSelectedPair(pair ? edgeKey(pair) : undefined)
    onSelect(undefined)
  }

  const startPan = (event: React.PointerEvent<HTMLCanvasElement>) => {
    if (event.button !== 0) return
    event.currentTarget.setPointerCapture?.(event.pointerId)
    pointerGesture.current = { id: event.pointerId, startX: event.clientX, startY: event.clientY, lastX: event.clientX, lastY: event.clientY, moved: false }
    setPanning(true)
  }

  const movePan = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const gesture = pointerGesture.current
    if (!gesture || gesture.id !== event.pointerId) return
    const bounds = event.currentTarget.getBoundingClientRect()
    const deltaX = ((event.clientX - gesture.lastX) / bounds.width) * width
    const deltaY = ((event.clientY - gesture.lastY) / bounds.height) * height
    gesture.lastX = event.clientX
    gesture.lastY = event.clientY
    if (Math.hypot(event.clientX - gesture.startX, event.clientY - gesture.startY) > 4) gesture.moved = true
    applyViewport({ ...viewport.current, x: viewport.current.x + deltaX, y: viewport.current.y + deltaY })
  }

  const endPan = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const gesture = pointerGesture.current
    if (!gesture || gesture.id !== event.pointerId) return
    pointerGesture.current = undefined
    event.currentTarget.releasePointerCapture?.(event.pointerId)
    setPanning(false)
    if (!gesture.moved) selectFromCanvas(event)
  }

  const cancelPan = (event: React.PointerEvent<HTMLCanvasElement>) => {
    if (pointerGesture.current?.id !== event.pointerId) return
    pointerGesture.current = undefined
    setPanning(false)
  }

  const handleWheel = (event: React.WheelEvent<HTMLCanvasElement>) => {
    event.preventDefault()
    const point = canvasPoint(event.currentTarget, event.clientX, event.clientY)
    zoomAt(point.x, point.y, Math.exp(-event.deltaY * 0.0015))
  }

  const handleCanvasKey = (event: React.KeyboardEvent<HTMLCanvasElement>) => {
    const panDistance = 48
    if (event.key === 'ArrowLeft') applyViewport({ ...viewport.current, x: viewport.current.x - panDistance })
    else if (event.key === 'ArrowRight') applyViewport({ ...viewport.current, x: viewport.current.x + panDistance })
    else if (event.key === 'ArrowUp') applyViewport({ ...viewport.current, y: viewport.current.y - panDistance })
    else if (event.key === 'ArrowDown') applyViewport({ ...viewport.current, y: viewport.current.y + panDistance })
    else if (event.key === '+' || event.key === '=') zoomAt(width / 2, height / 2, 1.25)
    else if (event.key === '-') zoomAt(width / 2, height / 2, 0.8)
    else if (event.key === '0') fitNetwork()
    else return
    event.preventDefault()
  }

  return (
    <div>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-3 text-xs text-slate-400">
          <Legend kind="human" label="Human" />
          <Legend kind="agent" label="Agent" />
          {network.meta.truncated && <span className="text-amber-200">Showing 75 of {network.total_identities} identities</span>}
        </div>
        <div className="flex flex-wrap gap-3">
          <label className="text-xs text-slate-500">Inspect identity<select value={selectedKey ?? ''} onChange={(event) => onSelect(event.target.value || undefined)} className="ml-2 max-w-48 rounded-lg border border-white/10 bg-slate-950 px-2 py-1.5 text-slate-200"><option value="">Choose actor</option>{network.nodes.map((node) => <option key={node.key} value={node.key}>{node.name}</option>)}</select></label>
          <label className="text-xs text-slate-500">
            Edge weight
            <select value={metric} onChange={(event) => setMetric(event.target.value as EdgeMetric)} className="ml-2 rounded-lg border border-white/10 bg-slate-950 px-2 py-1.5 text-slate-200">
              <option value="interaction_days">Interaction days</option>
              <option value="handoffs">Commit handoffs</option>
              <option value="coauthorships">Co-authorships</option>
              <option value="review_interactions">Reviews</option>
            </select>
          </label>
        </div>
      </div>
      {periods.length > 1 && <div className="no-print mb-4 flex flex-wrap items-center gap-3 rounded-xl border border-white/8 bg-slate-950/50 px-3 py-2"><button type="button" onClick={() => { if (!playing && activePeriodIndex >= periods.length - 1) setPeriodIndex(0); setPlaying((current) => !current) }} disabled={window.matchMedia?.('(prefers-reduced-motion: reduce)').matches} className="inline-flex items-center gap-1.5 rounded-lg border border-white/10 px-2.5 py-1.5 text-xs text-slate-300 disabled:opacity-40">{playing ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}{playing ? 'Pause' : 'Play'}</button><label className="flex min-w-52 flex-1 items-center gap-3 text-xs text-slate-500">Network period<input aria-label="Network playback period" type="range" min={0} max={periods.length-1} value={activePeriodIndex} onChange={(event) => { setPlaying(false); setPeriodIndex(Number(event.target.value)) }} className="w-full accent-cyan-300" /></label><time className="w-24 text-right text-xs tabular-nums text-slate-400">{activePeriod}</time></div>}
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_280px]">
        <div ref={viewportRef} data-testid="team-constellation-viewport" className={`min-w-0 overflow-hidden bg-slate-950 ${expanded ? 'fixed inset-0 z-50 rounded-none border-0' : 'relative rounded-2xl border border-white/8'}`}>
          <canvas
            ref={canvasRef}
            width={width}
            height={height}
            tabIndex={0}
            onPointerDown={startPan}
            onPointerMove={movePan}
            onPointerUp={endPan}
            onPointerCancel={cancelPan}
            onWheel={handleWheel}
            onKeyDown={handleCanvasKey}
            className={`${expanded ? 'h-full' : 'h-auto'} w-full touch-none outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-cyan-300/60 ${panning ? 'cursor-grabbing' : 'cursor-grab'}`}
            aria-label="Interactive team constellation. Human nodes contain a user icon and agent nodes contain a bot icon."
            aria-describedby="team-constellation-help"
          />
          <div className="no-print absolute right-3 top-3 flex gap-1 rounded-2xl border border-white/10 bg-slate-950/85 p-1.5 shadow-xl backdrop-blur" aria-label="Constellation view controls">
            <button type="button" onClick={() => zoomAt(width / 2, height / 2, 1.25)} aria-label="Zoom in team constellation" title="Zoom in" className="grid size-11 place-items-center rounded-xl text-slate-300 transition hover:bg-white/10 hover:text-white"><ZoomIn aria-hidden="true" className="size-5" /></button>
            <button type="button" onClick={() => zoomAt(width / 2, height / 2, 0.8)} aria-label="Zoom out team constellation" title="Zoom out" className="grid size-11 place-items-center rounded-xl text-slate-300 transition hover:bg-white/10 hover:text-white"><ZoomOut aria-hidden="true" className="size-5" /></button>
            <button type="button" onClick={() => void toggleFullscreen()} aria-label={expanded ? 'Exit full screen team constellation' : 'Expand team constellation to full screen'} aria-pressed={expanded} title={expanded ? 'Exit full screen' : 'Full screen'} className="grid size-11 place-items-center rounded-xl text-slate-300 transition hover:bg-white/10 hover:text-white">{expanded ? <Minimize2 aria-hidden="true" className="size-5" /> : <Maximize2 aria-hidden="true" className="size-5" />}</button>
          </div>
          <p id="team-constellation-help" className="pointer-events-none absolute bottom-3 left-3 inline-flex items-center gap-2 rounded-xl border border-white/8 bg-slate-950/80 px-3 py-2 text-xs text-slate-400 backdrop-blur"><Move aria-hidden="true" className="size-4 text-cyan-300" /> Drag to pan · Scroll to zoom</p>
        </div>
        <aside className="rounded-2xl border border-white/8 bg-slate-950/60 p-4">
          {selected ? (
            <IdentityDetail node={selected} allNodes={network.nodes} onClassify={onClassify} onRename={onRename} onMerge={onMerge} onUnmerge={onUnmerge} />
          ) : selectedEdge ? (
            <PairDetail edge={selectedEdge} nodes={network.nodes} />
          ) : (
            <div className="grid h-full min-h-48 place-items-center text-center">
              <div><UsersRound className="mx-auto size-7 text-slate-600" /><p className="mt-3 text-sm text-slate-400">Select a node to inspect its evidence and correct its identity.</p></div>
            </div>
          )}
        </aside>
      </div>
      <div className="sr-only">
        <table>
          <caption>Team identities and activity</caption>
          <thead><tr><th>Identity</th><th>Kind</th><th>Activity</th><th>Action</th></tr></thead>
          <tbody>{network.nodes.map((node) => <tr key={node.key}><td>{node.name}</td><td>{node.kind}</td><td>{node.activity}</td><td><button type="button" onClick={() => onSelect(node.key)}>Inspect</button></td></tr>)}</tbody>
        </table>
      </div>
      <div className="sr-only"><table><caption>Collaboration pairs and evidence</caption><thead><tr><th>Pair</th><th>Interaction days</th><th>Co-authorships</th><th>Reviews</th><th>Handoffs</th><th>Action</th></tr></thead><tbody>{network.edges.map((edge) => <tr key={edgeKey(edge)}><td>{pairLabel(edge, network.nodes)}</td><td>{edge.interaction_days}</td><td>{edge.coauthorships}</td><td>{edge.review_interactions}</td><td>{edge.handoffs}</td><td><button type="button" onClick={() => { onSelect(undefined); setSelectedPair(edgeKey(edge)) }}>Inspect pair</button></td></tr>)}</tbody></table></div>
      <div className="print-only report-table-wrap">
        <table className="report-table"><caption>Strongest collaboration pairs</caption><thead><tr><th>Pair</th><th>Interaction days</th><th>Co-authored</th><th>Reviews</th><th>Handoffs</th></tr></thead><tbody>{[...network.edges].sort((a, b) => b.interaction_days - a.interaction_days).slice(0, 12).map((edge) => <tr key={edgeKey(edge)}><td>{pairLabel(edge, network.nodes)}</td><td>{edge.interaction_days}</td><td>{edge.coauthorships}</td><td>{edge.review_interactions}</td><td>{edge.handoffs}</td></tr>)}</tbody></table>
      </div>
    </div>
  )
})

function fitTransform(nodes: DrawNode[]): ViewTransform {
  const positioned = nodes.filter((node) => Number.isFinite(node.x) && Number.isFinite(node.y))
  if (positioned.length === 0) return { x: 0, y: 0, scale: 1 }
  const minX = Math.min(...positioned.map((node) => (node.x ?? 0) - node.radius))
  const maxX = Math.max(...positioned.map((node) => (node.x ?? 0) + node.radius))
  const minY = Math.min(...positioned.map((node) => (node.y ?? 0) - node.radius))
  const maxY = Math.max(...positioned.map((node) => (node.y ?? 0) + node.radius + 20))
  const padding = 56
  const graphWidth = Math.max(1, maxX - minX)
  const graphHeight = Math.max(1, maxY - minY)
  const scale = Math.max(0.3, Math.min(1.25, (width - padding * 2) / graphWidth, (height - padding * 2) / graphHeight))
  return {
    x: width / 2 - ((minX + maxX) / 2) * scale,
    y: height / 2 - ((minY + maxY) / 2) * scale,
    scale,
  }
}

function drawActor(context: CanvasRenderingContext2D, node: DrawNode, selected: boolean, viewScale: number) {
  const x = node.x ?? 0
  const y = node.y ?? 0
  const radius = visualNodeRadius(node, viewScale)
  context.beginPath()
  if (node.kind === 'agent') {
    context.moveTo(x, y - radius)
    context.lineTo(x + radius, y)
    context.lineTo(x, y + radius)
    context.lineTo(x - radius, y)
    context.closePath()
  } else if (node.kind === 'unknown') {
    context.rect(x - radius * .72, y - radius * .72, radius * 1.44, radius * 1.44)
  } else {
    context.arc(x, y, radius, 0, Math.PI * 2)
  }
  context.fillStyle = colors[node.kind]
  context.fill()
  context.strokeStyle = selected ? '#ffffff' : '#0f172a'
  context.lineWidth = (selected ? 4 : 2) / viewScale
  context.stroke()
  drawActorIcon(context, node, radius)
  if (selected || radius * viewScale > 15) {
    context.font = `${12 / viewScale}px Inter, sans-serif`
    context.textAlign = 'center'
    context.fillStyle = '#e2e8f0'
    context.fillText(node.name.slice(0, 22), x, y + radius + 17 / viewScale)
  }
}

function visualNodeRadius(node: DrawNode, viewScale: number) {
  return Math.max(node.radius, 13 / Math.max(0.01, viewScale))
}

function drawActorIcon(context: CanvasRenderingContext2D, node: DrawNode, radius: number) {
  const x = node.x ?? 0
  const y = node.y ?? 0
  const scale = radius * 1.25 / 24
  context.save()
  context.translate(x - 12 * scale, y - 12 * scale)
  context.scale(scale, scale)
  context.strokeStyle = 'rgba(2,6,23,.9)'
  context.lineWidth = 2
  context.lineCap = 'round'
  context.lineJoin = 'round'

  if (node.kind === 'human') {
    // Lucide UserRound geometry.
    context.beginPath()
    context.arc(12, 8, 5, 0, Math.PI * 2)
    context.stroke()
    context.beginPath()
    context.moveTo(20, 21)
    context.arc(12, 21, 8, 0, Math.PI, true)
    context.stroke()
  } else if (node.kind === 'agent') {
    // Lucide Bot geometry.
    context.beginPath()
    context.moveTo(12, 8)
    context.lineTo(12, 4)
    context.lineTo(8, 4)
    context.stroke()
    roundedRectangle(context, 4, 8, 16, 12, 2)
    context.stroke()
    context.beginPath()
    context.moveTo(2, 14)
    context.lineTo(4, 14)
    context.moveTo(20, 14)
    context.lineTo(22, 14)
    context.moveTo(9, 13)
    context.lineTo(9, 15)
    context.moveTo(15, 13)
    context.lineTo(15, 15)
    context.stroke()
  } else {
    // Lucide CircleQuestionMark geometry, used if unknowns are included.
    context.beginPath()
    context.arc(12, 12, 10, 0, Math.PI * 2)
    context.stroke()
    context.beginPath()
    context.moveTo(9.09, 9)
    context.bezierCurveTo(9.48, 7.52, 10.7, 6.7, 12.17, 6.7)
    context.bezierCurveTo(13.83, 6.7, 15, 7.75, 15, 9.3)
    context.bezierCurveTo(15, 11.2, 12, 12, 12, 14)
    context.moveTo(12, 17)
    context.lineTo(12.01, 17)
    context.stroke()
  }
  context.restore()
}

function roundedRectangle(context: CanvasRenderingContext2D, x: number, y: number, boxWidth: number, boxHeight: number, radius: number) {
  context.beginPath()
  context.moveTo(x + radius, y)
  context.lineTo(x + boxWidth - radius, y)
  context.quadraticCurveTo(x + boxWidth, y, x + boxWidth, y + radius)
  context.lineTo(x + boxWidth, y + boxHeight - radius)
  context.quadraticCurveTo(x + boxWidth, y + boxHeight, x + boxWidth - radius, y + boxHeight)
  context.lineTo(x + radius, y + boxHeight)
  context.quadraticCurveTo(x, y + boxHeight, x, y + boxHeight - radius)
  context.lineTo(x, y + radius)
  context.quadraticCurveTo(x, y, x + radius, y)
  context.closePath()
}

function Legend({ kind, label }: { kind: ActorKind; label: string }) {
  return <span className="inline-flex items-center gap-1.5"><span aria-hidden="true" className="grid size-6 place-items-center rounded-lg text-slate-950" style={{ backgroundColor: colors[kind] }}><ActorIcon kind={kind} /></span>{label}</span>
}

function IdentityDetail({ node, allNodes, onClassify, onRename, onMerge, onUnmerge }: { node: NetworkNode; allNodes: NetworkNode[]; onClassify: TeamNetworkProps['onClassify']; onRename: TeamNetworkProps['onRename']; onMerge: TeamNetworkProps['onMerge']; onUnmerge: TeamNetworkProps['onUnmerge'] }) {
  return (
    <div>
      <div className="flex items-center gap-3"><Avatar src={node.avatar_url} name={node.name} size="md" /><div className="min-w-0"><p className="truncate font-semibold text-white">{node.name}</p><p className="text-xs text-slate-500">{node.evidence.replaceAll('_', ' ')} · {node.confidence}</p></div></div>
      <dl className="mt-5 grid grid-cols-3 gap-2 text-center"><MiniStat label="Commits" value={node.commits} /><MiniStat label="PRs" value={node.pull_requests} /><MiniStat label="Reviews" value={node.reviews} /></dl>
      <label className="mt-5 block text-xs font-medium text-slate-500">Display name<input key={`${node.key}:${node.name}`} defaultValue={node.name} onBlur={(event) => { const value = event.target.value.trim(); if (value && value !== node.name) onRename(node.key, value) }} className="mt-1.5 w-full rounded-lg border border-white/10 bg-slate-900 px-3 py-2 text-sm text-slate-200" /></label>
      <label className="mt-3 block text-xs font-medium text-slate-500">Classification
        <select value={node.kind} onChange={(event) => onClassify(node.key, event.target.value as ActorKind)} className="mt-1.5 w-full rounded-lg border border-white/10 bg-slate-900 px-3 py-2 text-sm text-slate-200">
          <option value="human">Human</option><option value="agent">Agent</option><option value="unknown">Unknown</option>
        </select>
      </label>
      <label className="mt-3 block text-xs font-medium text-slate-500">Merge alias into
        <select value="" onChange={(event) => event.target.value && onMerge(node.key, event.target.value)} className="mt-1.5 w-full rounded-lg border border-white/10 bg-slate-900 px-3 py-2 text-sm text-slate-200">
          <option value="">Keep separate</option>{allNodes.filter((candidate) => candidate.key !== node.key).map((candidate) => <option key={candidate.key} value={candidate.key}>{candidate.name}</option>)}
        </select>
      </label>
      {node.aliases && node.aliases.length > 1 && <div className="mt-3 text-xs text-slate-500"><p>Aliases</p><div className="mt-1 space-y-1">{node.aliases.filter((alias) => alias !== node.canonical_key).map((alias) => <div key={alias} className="flex items-center justify-between gap-2"><span className="truncate">{alias}</span><button type="button" onClick={() => onUnmerge(alias)} className="text-cyan-300 hover:text-cyan-100">Unmerge</button></div>)}</div></div>}
    </div>
  )
}

function PairDetail({ edge, nodes }: { edge: NetworkEdge; nodes: NetworkNode[] }) {
  return <div><div className="flex items-center gap-2"><UsersRound className="size-5 text-violet-300" /><p className="font-semibold text-white">{pairLabel(edge, nodes)}</p></div><p className="mt-2 text-xs text-slate-500">{edge.pair_type.replaceAll('_', ' ')} evidence across {edge.repositories.length} repositories</p><dl className="mt-5 grid grid-cols-2 gap-2 text-center"><MiniStat label="Interaction days" value={edge.interaction_days} /><MiniStat label="Co-authorships" value={edge.coauthorships} /><MiniStat label="Reviews" value={edge.review_interactions} /><MiniStat label="Handoffs" value={edge.handoffs} /></dl><div className="mt-5"><p className="text-xs font-medium text-slate-500">Repositories</p><ul className="mt-2 space-y-1 text-xs text-slate-300">{edge.repositories.slice(0, 12).map((repository) => <li key={repository}>{repository}</li>)}</ul></div></div>
}

function edgeKey(edge: NetworkEdge) { return `${edge.source}\0${edge.target}` }
function pairLabel(edge: NetworkEdge, nodes: NetworkNode[]) { const names = new Map(nodes.map((node) => [node.key, node.name])); return `${names.get(edge.source) ?? edge.source} × ${names.get(edge.target) ?? edge.target}` }
function pointToSegmentDistance(x: number, y: number, source: DrawNode, target: DrawNode) { const x1=source.x??0,y1=source.y??0,x2=target.x??0,y2=target.y??0,dx=x2-x1,dy=y2-y1,length=dx*dx+dy*dy; if(!length)return Math.hypot(x-x1,y-y1);const t=Math.max(0,Math.min(1,((x-x1)*dx+(y-y1)*dy)/length));return Math.hypot(x-(x1+t*dx),y-(y1+t*dy)) }

function MiniStat({ label, value }: { label: string; value: number }) { return <div className="rounded-xl bg-white/[0.035] p-2"><dt className="text-[10px] uppercase tracking-wide text-slate-600">{label}</dt><dd className="mt-1 text-sm font-semibold text-slate-200">{value}</dd></div> }

function EmptyNetwork() { return <div className="grid min-h-80 place-items-center rounded-2xl border border-dashed border-white/10 text-center"><div><Network className="mx-auto size-7 text-slate-600" /><p className="mt-3 text-sm text-slate-400">No collaboration evidence matches this scope.</p><p className="mt-1 text-xs text-slate-600">Unknown identities remain visible once event-level analysis is available.</p></div></div> }

export function ActorIcon({ kind }: { kind: ActorKind }) { return kind === 'agent' ? <Bot className="size-4" /> : kind === 'human' ? <UserRound className="size-4" /> : <CircleQuestionMark className="size-4" /> }
