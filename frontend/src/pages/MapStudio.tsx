// MapStudio — the D-Map canvas editor. Draw region shapes, place/scale/
// rotate icon symbols on top, optionally link a symbol to a wiki entity or
// another map, then Save writes the whole layout back in one PUT call.
// No autosave — mirrors the wiki entity edit→save pattern, not scene
// autosave, so a mid-draw layout never persists accidentally.
// Accessible at /projects/:id/maps/:mid
import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams, Link } from 'react-router-dom'
import { Stage, Layer, Line, Group, Circle, Ellipse, Rect, Transformer } from 'react-konva'
import type Konva from 'konva'
import { api } from '@/services/api'
import type { MapEntry, MapRegion, MapSymbol, WikiEntity } from '@/services/api'
import { paletteForMapType, REGION_COLORS } from '@/components/maps/palette'
import { MapIcon } from '@/components/maps/MapIcons'
import { useAuthStore } from '@/app/store/authStore'

const STAGE_WIDTH = 900
const STAGE_HEIGHT = 600

type Point = { x: number; y: number }
type Mode = 'idle' | 'drawing-region' | 'placing-symbol'
type LinkTarget = { kind: 'entity' | 'map'; id: string; name: string }

function pointInPolygon(pt: Point, poly: Point[]): boolean {
  let inside = false
  for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
    const xi = poly[i].x, yi = poly[i].y
    const xj = poly[j].x, yj = poly[j].y
    const intersect = yi > pt.y !== yj > pt.y && pt.x < ((xj - xi) * (pt.y - yi)) / (yj - yi) + xi
    if (intersect) inside = !inside
  }
  return inside
}

function flattenPoints(points: Point[]): number[] {
  return points.flatMap((p) => [p.x, p.y])
}

export default function MapStudio() {
  const { id: projectId, mid: mapId } = useParams<{ id: string; mid: string }>()
  const token = useAuthStore((s) => s.accessToken) ?? ''
  const [map, setMap] = useState<MapEntry | null>(null)
  const [name, setName] = useState('')
  const [editingName, setEditingName] = useState(false)
  const [regions, setRegions] = useState<MapRegion[]>([])
  const [symbols, setSymbols] = useState<MapSymbol[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [saveOk, setSaveOk] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [mode, setMode] = useState<Mode>('idle')
  const [drawPoints, setDrawPoints] = useState<Point[]>([])
  const [pendingRegion, setPendingRegion] = useState<Point[] | null>(null)
  const [pendingIcon, setPendingIcon] = useState<string | null>(null)
  const [selectedSymbolId, setSelectedSymbolId] = useState<string | null>(null)

  const stageRef = useRef<Konva.Stage>(null)
  const trRef = useRef<Konva.Transformer>(null)
  const nodeRefs = useRef<Map<string, Konva.Group>>(new Map())

  useEffect(() => {
    if (!projectId || !mapId || !token) return
    setLoading(true)
    api.maps.get(token, projectId, mapId)
      .then((m) => {
        setMap(m)
        setName(m.name)
        setRegions(m.layout?.regions ?? [])
        setSymbols(m.layout?.symbols ?? [])
      })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : 'Failed to load map'))
      .finally(() => setLoading(false))
  }, [token, projectId, mapId])

  // Bind the Transformer to the selected symbol's Konva node.
  useEffect(() => {
    const tr = trRef.current
    if (!tr) return
    const node = selectedSymbolId ? nodeRefs.current.get(selectedSymbolId) : null
    tr.nodes(node ? [node] : [])
    tr.getLayer()?.batchDraw()
  }, [selectedSymbolId, symbols.length])

  const palette = paletteForMapType(map?.map_type ?? 'world')

  const handleSave = async () => {
    if (!projectId || !mapId) return
    setSaving(true)
    setError(null)
    try {
      const updated = await api.maps.update(token, projectId, mapId, {
        name: name.trim() || map?.name,
        layout: { regions, symbols },
      })
      setMap(updated)
      setEditingName(false)
      setSaveOk(true)
      setTimeout(() => setSaveOk(false), 2000)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save map')
    } finally {
      setSaving(false)
    }
  }

  const stagePoint = (): Point | null => {
    const stage = stageRef.current
    if (!stage) return null
    const p = stage.getPointerPosition()
    return p ? { x: p.x, y: p.y } : null
  }

  const handleStageClick = (e: Konva.KonvaEventObject<MouseEvent>) => {
    const p = stagePoint()
    if (!p) return

    if (mode === 'drawing-region') {
      setDrawPoints((prev) => [...prev, p])
      return
    }

    if (mode === 'placing-symbol' && pendingIcon) {
      const region = regions.find((r) => pointInPolygon(p, r.points))
      const symbol: MapSymbol = {
        id: crypto.randomUUID(),
        icon_type: pendingIcon,
        x: p.x,
        y: p.y,
        scale: 1,
        rotation: 0,
        region_id: region?.id,
      }
      setSymbols((prev) => [...prev, symbol])
      setSelectedSymbolId(symbol.id)
      setPendingIcon(null)
      setMode('idle')
      return
    }

    // Idle: clicking empty stage background deselects. The background Rect
    // (named "bg") intercepts pointer events, so the Stage itself is never
    // e.target for a "background" click — check the name instead.
    if (e.target === stageRef.current || e.target.name() === 'bg') {
      setSelectedSymbolId(null)
    }
  }

  const finishRegion = () => {
    // The browser fires two click events immediately before a dblclick, at
    // the same coordinates — drop that duplicate trailing point so closing
    // a shape by double-click doesn't leave a zero-length final edge.
    let points = drawPoints
    if (points.length >= 2) {
      const last = points[points.length - 1]
      const prev = points[points.length - 2]
      if (last.x === prev.x && last.y === prev.y) points = points.slice(0, -1)
    }

    if (points.length < 3) {
      setDrawPoints([])
      setMode('idle')
      return
    }
    setPendingRegion(points)
    setDrawPoints([])
  }

  const confirmRegion = (regionType: string, color: string) => {
    if (!pendingRegion) return
    setRegions((prev) => [...prev, { id: crypto.randomUUID(), type: regionType, color, points: pendingRegion }])
    setPendingRegion(null)
    setMode('idle')
  }

  const handleRegionDragEnd = (regionId: string, node: Konva.Line) => {
    const dx = node.x()
    const dy = node.y()
    node.position({ x: 0, y: 0 })
    setRegions((prev) => prev.map((r) =>
      r.id === regionId ? { ...r, points: r.points.map((pt) => ({ x: pt.x + dx, y: pt.y + dy })) } : r,
    ))
  }

  const handleSymbolTransformEnd = (symbolId: string, node: Konva.Group) => {
    setSymbols((prev) => prev.map((s) =>
      s.id === symbolId
        ? { ...s, x: node.x(), y: node.y(), rotation: node.rotation(), scale: node.scaleX() }
        : s,
    ))
  }

  const deleteSelectedSymbol = () => {
    if (!selectedSymbolId) return
    setSymbols((prev) => prev.filter((s) => s.id !== selectedSymbolId))
    setSelectedSymbolId(null)
  }

  const selectedSymbol = symbols.find((s) => s.id === selectedSymbolId) ?? null

  if (!projectId || !mapId) return null

  return (
    <div className="h-screen flex flex-col bg-brand-bg overflow-hidden">
      <header className="h-11 shrink-0 flex items-center gap-3 px-4 bg-brand-bg-card border-b border-brand-border">
        <Link
          to={`/projects/${projectId}/maps`}
          className="flex items-center gap-1.5 text-xs text-brand-muted hover:text-brand-text transition-colors"
        >
          <BackIcon />
          Maps
        </Link>
        <span className="text-brand-border">|</span>
        {editingName ? (
          <input
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            onBlur={() => setEditingName(false)}
            onKeyDown={(e) => { if (e.key === 'Enter') setEditingName(false) }}
            className="bg-transparent border-b border-brand-cyan text-sm text-brand-text focus:outline-none"
          />
        ) : (
          <button onClick={() => setEditingName(true)} className="text-brand-cyan text-sm font-semibold hover:underline">
            {name || 'Untitled Map'}
          </button>
        )}
        {map && (
          <span className="px-2 py-0.5 rounded text-[10px] font-semibold border border-brand-purple/40 text-brand-purple">
            {map.map_type}
          </span>
        )}
        <div className="flex-1" />
        {saveOk && <span className="text-xs text-emerald-400">Saved.</span>}
        {error && <span className="text-xs text-red-400">{error}</span>}
        <button
          onClick={handleSave}
          disabled={saving || loading}
          className="px-3 py-1.5 rounded-lg bg-brand-gradient text-brand-bg text-xs font-semibold disabled:opacity-50"
        >
          {saving ? 'Saving…' : 'Save'}
        </button>
      </header>

      {loading ? (
        <div className="flex-1 flex items-center justify-center text-brand-muted text-sm">Loading…</div>
      ) : (
        <div className="flex-1 flex overflow-hidden">
          {/* Toolbar */}
          <div className="w-52 shrink-0 border-r border-brand-border bg-brand-bg-card overflow-y-auto p-3 space-y-4">
            <div>
              <p className="text-[10px] font-semibold text-brand-muted uppercase tracking-wider mb-1.5">Regions</p>
              <button
                onClick={() => {
                  setMode(mode === 'drawing-region' ? 'idle' : 'drawing-region')
                  setPendingIcon(null)
                  setDrawPoints([])
                }}
                className={`w-full text-left px-2.5 py-1.5 rounded text-xs font-medium border transition-colors ${
                  mode === 'drawing-region'
                    ? 'border-brand-cyan/40 text-brand-cyan bg-brand-cyan/10'
                    : 'border-brand-border text-brand-muted hover:text-brand-text'
                }`}
              >
                {mode === 'drawing-region' ? 'Click points, double-click to close' : '+ Draw Region'}
              </button>
            </div>

            <div>
              <p className="text-[10px] font-semibold text-brand-muted uppercase tracking-wider mb-1.5">Symbols</p>
              <div className="grid grid-cols-4 gap-1.5">
                {palette.map((item) => (
                  <button
                    key={item.icon_type}
                    title={item.label}
                    onClick={() => {
                      setPendingIcon(pendingIcon === item.icon_type ? null : item.icon_type)
                      setMode(pendingIcon === item.icon_type ? 'idle' : 'placing-symbol')
                      setDrawPoints([])
                    }}
                    className={`aspect-square flex items-center justify-center rounded border transition-colors ${
                      pendingIcon === item.icon_type
                        ? 'border-brand-cyan/40 text-brand-cyan bg-brand-cyan/10'
                        : 'border-brand-border text-brand-muted hover:text-brand-text'
                    }`}
                  >
                    <MapIcon type={item.icon_type} className="w-4 h-4" />
                  </button>
                ))}
              </div>
              {pendingIcon && (
                <p className="text-[10px] text-brand-cyan mt-1.5">Click the canvas to place it.</p>
              )}
            </div>

            {selectedSymbol && (
              <SymbolPanel
                key={selectedSymbol.id}
                symbol={selectedSymbol}
                token={token}
                projectId={projectId}
                onLink={(target) => setSymbols((prev) => prev.map((s) =>
                  s.id === selectedSymbol.id ? { ...s, entity_id: target ? target.id : undefined } : s,
                ))}
                onDelete={deleteSelectedSymbol}
              />
            )}
          </div>

          {/* Canvas */}
          <div className="flex-1 overflow-auto p-4">
            <div className="inline-block border border-brand-border rounded-lg overflow-hidden bg-white">
              <Stage
                ref={stageRef}
                width={STAGE_WIDTH}
                height={STAGE_HEIGHT}
                onClick={handleStageClick}
                onDblClick={() => { if (mode === 'drawing-region') finishRegion() }}
              >
                <Layer>
                  <Rect name="bg" x={0} y={0} width={STAGE_WIDTH} height={STAGE_HEIGHT} fill="#eef1f6" />

                  {regions.map((r) => (
                    <Line
                      key={r.id}
                      points={flattenPoints(r.points)}
                      closed
                      fill={r.color}
                      stroke="rgba(0,0,0,0.25)"
                      strokeWidth={1}
                      draggable={mode === 'idle'}
                      onDragEnd={(e) => handleRegionDragEnd(r.id, e.target as Konva.Line)}
                    />
                  ))}

                  {drawPoints.length > 0 && (
                    <Line
                      points={flattenPoints(drawPoints)}
                      stroke="#00F0FF"
                      strokeWidth={2}
                      dash={[4, 4]}
                    />
                  )}
                  {drawPoints.map((p, i) => (
                    <Circle key={i} x={p.x} y={p.y} radius={3} fill="#00F0FF" />
                  ))}

                  {symbols.map((s) => (
                    <Group
                      key={s.id}
                      x={s.x}
                      y={s.y}
                      rotation={s.rotation}
                      scaleX={s.scale}
                      scaleY={s.scale}
                      draggable={mode === 'idle'}
                      ref={(node) => { if (node) nodeRefs.current.set(s.id, node); else nodeRefs.current.delete(s.id) }}
                      onClick={(e) => { e.cancelBubble = true; if (mode === 'idle') setSelectedSymbolId(s.id) }}
                      onDragEnd={(e) => handleSymbolTransformEnd(s.id, e.target as Konva.Group)}
                      onTransformEnd={(e) => handleSymbolTransformEnd(s.id, e.target as Konva.Group)}
                    >
                      {renderSymbolShape(s.icon_type)}
                    </Group>
                  ))}

                  <Transformer ref={trRef} rotateEnabled resizeEnabled={false} />
                </Layer>
              </Stage>
            </div>
          </div>
        </div>
      )}

      {pendingRegion && (
        <RegionTypePopup onCancel={() => setPendingRegion(null)} onConfirm={confirmRegion} />
      )}
    </div>
  )
}

// ── Canvas symbol shape (Konva primitives, not DOM SVG) ────────────────────

function renderSymbolShape(iconType: string) {
  const stroke = '#1a1a2e'
  switch (iconType) {
    case 'mountain':
      return <Line points={[-10, 8, 0, -10, 10, 8]} closed fill="#8a8a9a" stroke={stroke} strokeWidth={1} />
    case 'lake':
    case 'ocean':
      return <Ellipse radiusX={11} radiusY={7} fill="#3a8fc2" stroke={stroke} strokeWidth={1} />
    case 'river':
      return <Line points={[-10, -6, -2, 2, 4, -2, 10, 8]} stroke="#3a8fc2" strokeWidth={3} />
    case 'forest':
      return <Line points={[0, -10, -8, 8, 8, 8]} closed fill="#2d6a2d" stroke={stroke} strokeWidth={1} />
    case 'desert':
      return <Circle radius={9} fill="#d9b563" stroke={stroke} strokeWidth={1} />
    case 'swamp':
      return <Ellipse radiusX={10} radiusY={6} fill="#5a6b3a" stroke={stroke} strokeWidth={1} />
    case 'city':
      return <Rect x={-8} y={-8} width={16} height={16} fill="#a45c3a" stroke={stroke} strokeWidth={1} />
    case 'star':
      return <Circle radius={7} fill="#f4c95d" stroke={stroke} strokeWidth={1} />
    case 'planet':
      return <Ellipse radiusX={10} radiusY={4} fill="none" stroke="#9f4bff" strokeWidth={1.5} />
    case 'moon':
      return <Circle radius={8} fill="#c9c9d9" stroke={stroke} strokeWidth={1} />
    case 'nebula':
      return <Circle radius={11} fill="#9f4bff" opacity={0.5} />
    case 'asteroid_field':
      return <Circle radius={9} fill="#6b6b7a" stroke={stroke} strokeWidth={1} dash={[2, 2]} />
    case 'space_station':
      return <Rect x={-6} y={-6} width={12} height={12} rotation={45} fill="#4a4a5a" stroke={stroke} strokeWidth={1} />
    case 'wormhole':
      return <Circle radius={10} fill="none" stroke="#00F0FF" strokeWidth={2} />
    case 'black_hole':
      return <Circle radius={9} fill="#0a0a12" stroke="#4a4a5a" strokeWidth={2} />
    default:
      return <Circle radius={8} fill="#8a8a9a" stroke={stroke} strokeWidth={1} />
  }
}

// ── Region type/color confirmation popup ────────────────────────────────────

function RegionTypePopup({ onCancel, onConfirm }: { onCancel: () => void; onConfirm: (type: string, color: string) => void }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm px-4">
      <div className="bg-brand-bg-card border border-brand-border rounded-xl p-5 w-full max-w-sm">
        <h3 className="text-sm font-semibold text-brand-text mb-3">Region type</h3>
        <div className="grid grid-cols-2 gap-2">
          {REGION_COLORS.map((rc) => (
            <button
              key={rc.type}
              onClick={() => onConfirm(rc.type, rc.color)}
              className="flex items-center gap-2 px-2.5 py-2 rounded-lg border border-brand-border hover:border-brand-border/60 transition-colors text-left"
            >
              <span className="w-3.5 h-3.5 rounded-full shrink-0" style={{ backgroundColor: rc.color }} />
              <span className="text-xs text-brand-text">{rc.label}</span>
            </button>
          ))}
        </div>
        <button onClick={onCancel} className="mt-3 text-xs text-brand-muted hover:text-brand-text transition-colors">
          Cancel
        </button>
      </div>
    </div>
  )
}

// ── Selected symbol side panel ──────────────────────────────────────────────

function SymbolPanel({
  symbol, token, projectId, onLink, onDelete,
}: {
  symbol: MapSymbol
  token: string
  projectId: string
  onLink: (target: LinkTarget | null) => void
  onDelete: () => void
}) {
  const [query, setQuery] = useState('')
  const [entities, setEntities] = useState<WikiEntity[]>([])
  const [maps, setMaps] = useState<MapEntry[]>([])

  const loadCandidates = useCallback(() => {
    Promise.all([api.wiki.listEntities(token, projectId), api.maps.list(token, projectId)])
      .then(([e, m]) => { setEntities(e); setMaps(m) })
      .catch(() => {})
  }, [token, projectId])

  useEffect(() => { loadCandidates() }, [loadCandidates])

  const linkedName = symbol.entity_id
    ? entities.find((x) => x.id === symbol.entity_id)?.name
      ?? maps.find((x) => x.id === symbol.entity_id)?.name
      ?? null
    : null

  const q = query.toLowerCase()
  const entityResults = q ? entities.filter((e) => e.name.toLowerCase().includes(q)).slice(0, 6) : []
  const mapResults = q ? maps.filter((m) => m.name.toLowerCase().includes(q)).slice(0, 6) : []

  return (
    <div className="border-t border-brand-border pt-3 space-y-2">
      <p className="text-[10px] font-semibold text-brand-muted uppercase tracking-wider">Selected symbol</p>
      <p className="text-xs text-brand-text capitalize">{symbol.icon_type.replace('_', ' ')}</p>

      {linkedName ? (
        <div className="flex items-center justify-between px-2 py-1.5 rounded bg-brand-cyan/10 text-brand-cyan text-xs">
          <span className="truncate">{linkedName}</span>
          <button onClick={() => { onLink(null); setQuery('') }} className="shrink-0 ml-1.5 hover:text-brand-text">×</button>
        </div>
      ) : (
        <div>
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Link to entity or map…"
            className="input-field w-full text-xs"
          />
          {(entityResults.length > 0 || mapResults.length > 0) && (
            <div className="mt-1 max-h-40 overflow-y-auto border border-brand-border rounded-lg divide-y divide-brand-border">
              {entityResults.map((e) => (
                <button
                  key={e.id}
                  onClick={() => { onLink({ kind: 'entity', id: e.id, name: e.name }); setQuery('') }}
                  className="w-full text-left px-2 py-1.5 text-xs text-brand-text hover:bg-brand-border/20"
                >
                  {e.name} <span className="text-brand-muted">· {e.type}</span>
                </button>
              ))}
              {mapResults.map((m) => (
                <button
                  key={m.id}
                  onClick={() => { onLink({ kind: 'map', id: m.id, name: m.name }); setQuery('') }}
                  className="w-full text-left px-2 py-1.5 text-xs text-brand-text hover:bg-brand-border/20"
                >
                  {m.name} <span className="text-brand-muted">· map</span>
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      <button
        onClick={onDelete}
        className="w-full px-2.5 py-1.5 rounded border border-brand-border text-brand-muted text-xs hover:text-red-400 hover:border-red-500/40 transition-colors"
      >
        Delete Symbol
      </button>
    </div>
  )
}

function BackIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 15l-5-5 5-5" />
    </svg>
  )
}
