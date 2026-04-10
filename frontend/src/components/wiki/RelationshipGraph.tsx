// RelationshipGraph — force-directed graph of wiki entities and relationships.
// Uses d3-force for layout; renders as React SVG. Supports pan/zoom via d3-zoom.
import { useEffect, useRef, useState, useCallback } from 'react'
import * as d3 from 'd3'
import { api } from '@/services/api'
import type { WikiGraph, EntityType } from '@/services/api'

interface Props {
  token: string
  projectId: string
  onSelectEntity: (id: string) => void
}

// ── d3 simulation types ────────────────────────────────────────────────────────

interface GraphNode extends d3.SimulationNodeDatum {
  id: string
  name: string
  type: EntityType
}

interface GraphLink extends d3.SimulationLinkDatum<GraphNode> {
  id: string
  relType: string
}

// ── Colors per entity type (mirrors WikiHub) ───────────────────────────────────

const NODE_FILL: Record<EntityType, string> = {
  character: '#00F0FF',
  location:  '#F4C95D',
  faction:   '#9F4BFF',
  item:      '#34D399',
  concept:   '#38BDF8',
  lore:      '#FB7185',
}

const NODE_STROKE: Record<EntityType, string> = {
  character: '#00c8d4',
  location:  '#c9a132',
  faction:   '#7a2fd1',
  item:      '#10b981',
  concept:   '#0ea5e9',
  lore:      '#e11d48',
}

// ── Component ─────────────────────────────────────────────────────────────────

export default function RelationshipGraph({ token, projectId, onSelectEntity }: Props) {
  const svgRef  = useRef<SVGSVGElement>(null)
  const zoomRef = useRef<d3.ZoomBehavior<SVGSVGElement, unknown> | null>(null)

  const [graph,   setGraph]   = useState<WikiGraph | null>(null)
  const [loading, setLoading] = useState(true)
  const [error,   setError]   = useState<string | null>(null)
  const [selected, setSelected] = useState<string | null>(null)
  const [nodes,   setNodes]   = useState<GraphNode[]>([])
  const [links,   setLinks]   = useState<GraphLink[]>([])

  // ── Load graph data ──────────────────────────────────────────────────────────

  useEffect(() => {
    if (!token || !projectId) return
    setLoading(true)
    api.wiki.graph(token, projectId)
      .then(setGraph)
      .catch((e: unknown) => setError(e instanceof Error ? e.message : 'Failed to load graph'))
      .finally(() => setLoading(false))
  }, [token, projectId])

  // ── Run d3 force simulation ──────────────────────────────────────────────────

  useEffect(() => {
    if (!graph) return

    const simNodes: GraphNode[] = graph.entities.map((e) => ({
      id:   e.id,
      name: e.name,
      type: e.type,
    }))

    const idSet = new Set(simNodes.map((n) => n.id))
    const simLinks: GraphLink[] = graph.relationships
      .filter((r) => idSet.has(r.from_entity_id) && idSet.has(r.to_entity_id))
      .map((r) => ({
        id:      r.id,
        source:  r.from_entity_id,
        target:  r.to_entity_id,
        relType: r.type,
      }))

    const sim = d3.forceSimulation<GraphNode>(simNodes)
      .force('link',    d3.forceLink<GraphNode, GraphLink>(simLinks).id((d) => d.id).distance(130))
      .force('charge',  d3.forceManyBody().strength(-350))
      .force('center',  d3.forceCenter(0, 0))
      .force('collide', d3.forceCollide(44))

    // Run to convergence synchronously, then snapshot into React state
    sim.tick(200)
    setNodes(simNodes.map((n) => ({ ...n })))
    setLinks(simLinks.map((l) => ({ ...l })))
    sim.stop()
  }, [graph])

  // ── d3 zoom on SVG ──────────────────────────────────────────────────────────

  useEffect(() => {
    if (!svgRef.current || nodes.length === 0) return

    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.15, 5])
      .on('zoom', (event) => {
        const g = svgRef.current?.querySelector('g.zoom-layer')
        if (g) d3.select(g).attr('transform', event.transform.toString())
      })

    const svg = d3.select(svgRef.current)
    svg.call(zoom)
    zoomRef.current = zoom

    // Initial transform: center the graph
    const { clientWidth: w, clientHeight: h } = svgRef.current
    svg.call(zoom.transform, d3.zoomIdentity.translate(w / 2, h / 2))
  }, [nodes.length])

  const handleReset = useCallback(() => {
    if (!svgRef.current || !zoomRef.current) return
    const { clientWidth: w, clientHeight: h } = svgRef.current
    d3.select(svgRef.current)
      .transition().duration(400)
      .call(zoomRef.current.transform, d3.zoomIdentity.translate(w / 2, h / 2))
  }, [])

  // ── Render ───────────────────────────────────────────────────────────────────

  if (loading) {
    return (
      <div className="flex items-center justify-center h-96">
        <svg className="animate-spin h-6 w-6 text-brand-purple" viewBox="0 0 24 24" fill="none">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
        </svg>
      </div>
    )
  }

  if (error) {
    return <p className="text-sm text-red-400 py-10 text-center">{error}</p>
  }

  if (nodes.length === 0) {
    return (
      <div className="flex items-center justify-center h-96 text-brand-muted">
        <p className="text-sm">No entities yet — add some in the Entities tab to see the graph.</p>
      </div>
    )
  }

  return (
    <div className="relative w-full" style={{ height: 'calc(100vh - 180px)' }}>
      {/* Legend */}
      <div className="absolute top-3 left-3 z-10 flex flex-wrap gap-2 pointer-events-none">
        {(Object.entries(NODE_FILL) as [EntityType, string][]).map(([type, color]) => (
          <span
            key={type}
            className="flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-medium bg-brand-bg-card border border-brand-border"
            style={{ color }}
          >
            <span className="w-2 h-2 rounded-full inline-block" style={{ backgroundColor: color }} />
            {type}
          </span>
        ))}
      </div>

      {/* Reset zoom */}
      <button
        onClick={handleReset}
        className="absolute top-3 right-3 z-10 px-2.5 py-1 rounded text-xs text-brand-text-muted border border-brand-border bg-brand-bg-card hover:text-brand-text hover:border-brand-cyan/40 transition-colors"
      >
        Reset view
      </button>

      {/* Graph */}
      <svg
        ref={svgRef}
        className="w-full h-full cursor-grab active:cursor-grabbing"
      >
        <defs>
          <marker id="graph-arrow" markerWidth="8" markerHeight="6" refX="24" refY="3" orient="auto">
            <path d="M0,0 L0,6 L8,3 z" fill="rgb(var(--brand-border))" />
          </marker>
        </defs>

        <g className="zoom-layer">
          {/* Edges */}
          {links.map((link) => {
            const s = link.source as GraphNode
            const t = link.target as GraphNode
            if (s.x == null || t.x == null) return null
            const mx = (s.x + t.x) / 2
            const my = (s.y! + t.y!) / 2
            return (
              <g key={link.id}>
                <line
                  x1={s.x} y1={s.y}
                  x2={t.x} y2={t.y}
                  stroke="rgb(var(--brand-border))"
                  strokeWidth={1.5}
                  strokeOpacity={0.7}
                  markerEnd="url(#graph-arrow)"
                />
                {link.relType && (
                  <text
                    x={mx} y={my}
                    textAnchor="middle"
                    dominantBaseline="middle"
                    fontSize={9}
                    fill="rgb(var(--brand-muted))"
                    className="pointer-events-none select-none"
                    dy={-7}
                  >
                    {link.relType}
                  </text>
                )}
              </g>
            )
          })}

          {/* Nodes */}
          {nodes.map((node) => {
            const isSel  = node.id === selected
            const fill   = NODE_FILL[node.type]
            const stroke = NODE_STROKE[node.type]
            const r      = isSel ? 22 : 18
            return (
              <g
                key={node.id}
                transform={`translate(${node.x ?? 0},${node.y ?? 0})`}
                onClick={() => { setSelected(node.id); onSelectEntity(node.id) }}
                className="cursor-pointer"
              >
                {isSel && (
                  <circle r={r + 6} fill={fill} fillOpacity={0.12} />
                )}
                <circle
                  r={r}
                  fill={fill}
                  fillOpacity={isSel ? 0.85 : 0.2}
                  stroke={stroke}
                  strokeWidth={isSel ? 2.5 : 1.5}
                />
                <text
                  textAnchor="middle"
                  dominantBaseline="middle"
                  fontSize={10}
                  fontWeight={isSel ? 700 : 500}
                  fill={isSel ? fill : 'rgb(var(--brand-text))'}
                  className="pointer-events-none select-none"
                  dy={r + 12}
                >
                  {node.name.length > 14 ? node.name.slice(0, 13) + '…' : node.name}
                </text>
              </g>
            )
          })}
        </g>
      </svg>
    </div>
  )
}
