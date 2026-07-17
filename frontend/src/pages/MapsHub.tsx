// MapsHub — list/create/delete maps for a project. Clicking a card opens
// the full Map Studio canvas editor (a separate route), not an inline
// detail panel — unlike wiki entities, a map's "detail view" is the editor.
// Accessible at /projects/:id/maps
import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api } from '@/services/api'
import type { MapEntry } from '@/services/api'
import { MAP_TYPES } from '@/components/maps/palette'
import { useAuthStore } from '@/app/store/authStore'

const TYPE_LABELS: Record<string, string> = {
  world: 'World', region: 'Region', city: 'City',
  galaxy: 'Galaxy', planet: 'Planet', custom: 'Custom',
}

export default function MapsHub() {
  const { id: projectId } = useParams<{ id: string }>()
  const token = useAuthStore((s) => s.accessToken) ?? ''
  const navigate = useNavigate()

  const [maps, setMaps] = useState<MapEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)

  const load = useCallback(async () => {
    if (!projectId || !token) return
    setLoading(true)
    setError(null)
    try {
      setMaps(await api.maps.list(token, projectId))
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load maps')
    } finally {
      setLoading(false)
    }
  }, [token, projectId])

  useEffect(() => { load() }, [load])

  const handleCreated = (m: MapEntry) => {
    navigate(`/projects/${projectId}/maps/${m.id}`)
  }

  const handleDeleted = (id: string) => {
    setMaps((prev) => prev.filter((m) => m.id !== id))
  }

  if (!projectId) return null

  return (
    <div className="h-screen flex flex-col bg-brand-bg overflow-hidden">
      <header className="h-11 shrink-0 flex items-center gap-3 px-4 bg-brand-bg-card border-b border-brand-border">
        <Link
          to={`/projects/${projectId}`}
          className="flex items-center gap-1.5 text-xs text-brand-muted hover:text-brand-text transition-colors"
        >
          <BackIcon />
          Editor
        </Link>
        <span className="text-brand-border">|</span>
        <span className="text-brand-cyan text-sm font-semibold">Maps</span>
      </header>

      <main className="flex-1 overflow-y-auto px-6 py-6 max-w-5xl w-full mx-auto">
        <div className="flex items-center justify-between mb-4">
          <h1 className="text-lg font-bold text-brand-text">Maps</h1>
          <button
            onClick={() => setShowCreate(true)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-brand-purple/10 text-brand-purple text-xs font-semibold hover:bg-brand-purple/20 transition-colors"
          >
            <PlusIcon />
            New Map
          </button>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-20">
            <SpinIcon className="w-5 h-5 text-brand-purple animate-spin" />
          </div>
        ) : error ? (
          <p className="text-sm text-red-400 py-6">{error}</p>
        ) : maps.length === 0 ? (
          <div className="text-center py-20 text-brand-muted">
            <p className="text-sm">No maps yet.</p>
            <button onClick={() => setShowCreate(true)} className="mt-2 text-brand-purple text-sm hover:underline">
              Draw the first one
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {maps.map((m) => (
              <MapCard key={m.id} map={m} allMaps={maps} onDelete={handleDeleted} token={token} projectId={projectId} />
            ))}
          </div>
        )}

        {showCreate && (
          <CreateMapModal
            token={token}
            projectId={projectId}
            existingMaps={maps}
            onCreated={handleCreated}
            onClose={() => setShowCreate(false)}
          />
        )}
      </main>
    </div>
  )
}

function MapCard({
  map, allMaps, token, projectId, onDelete,
}: {
  map: MapEntry
  allMaps: MapEntry[]
  token: string
  projectId: string
  onDelete: (id: string) => void
}) {
  const navigate = useNavigate()
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [deleting, setDeleting] = useState(false)

  const parent = map.parent_entity_id ? allMaps.find((m) => m.id === map.parent_entity_id) : undefined

  const handleDelete = async (e: React.MouseEvent) => {
    e.stopPropagation()
    setDeleting(true)
    try {
      await api.maps.delete(token, projectId, map.id)
      onDelete(map.id)
    } catch {
      setDeleting(false)
    }
  }

  return (
    <div
      onClick={() => navigate(`/projects/${projectId}/maps/${map.id}`)}
      className="cursor-pointer text-left p-4 rounded-xl bg-brand-bg-card border border-brand-border hover:border-brand-border/60 hover:bg-brand-bg-card/80 transition-colors"
    >
      <div className="flex items-center justify-between mb-2">
        <span className="px-2 py-0.5 rounded text-[10px] font-semibold border border-brand-purple/40 text-brand-purple">
          {TYPE_LABELS[map.map_type] ?? map.map_type}
        </span>
        {confirmDelete ? (
          <div className="flex items-center gap-1.5" onClick={(e) => e.stopPropagation()}>
            <button onClick={() => setConfirmDelete(false)} className="text-[10px] text-brand-muted hover:text-brand-text">Cancel</button>
            <button onClick={handleDelete} disabled={deleting} className="text-[10px] text-red-400 hover:text-red-300 disabled:opacity-50">
              {deleting ? 'Deleting…' : 'Delete'}
            </button>
          </div>
        ) : (
          <button
            onClick={(e) => { e.stopPropagation(); setConfirmDelete(true) }}
            className="text-brand-muted hover:text-red-400 transition-colors"
            title="Delete map"
          >
            <XIcon />
          </button>
        )}
      </div>
      <h3 className="text-sm font-semibold text-brand-text mb-1">{map.name}</h3>
      {parent && <p className="text-xs text-brand-muted">Zooms into {parent.name}</p>}
    </div>
  )
}

function CreateMapModal({
  token, projectId, existingMaps, onCreated, onClose,
}: {
  token: string
  projectId: string
  existingMaps: MapEntry[]
  onCreated: (m: MapEntry) => void
  onClose: () => void
}) {
  const [name, setName] = useState('')
  const [mapType, setMapType] = useState<string>('world')
  const [parentId, setParentId] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    setSubmitting(true)
    setError(null)
    try {
      const map = await api.maps.create(token, projectId, {
        name: name.trim(),
        map_type: mapType,
        parent_entity_id: parentId || undefined,
      })
      onCreated(map)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create map')
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/60 backdrop-blur-sm px-4 pb-4 sm:pb-0">
      <div className="bg-brand-bg-card border border-brand-border rounded-2xl w-full max-w-xl shadow-card flex flex-col max-h-[85dvh]">
        <div className="flex items-center justify-between px-6 pt-6 pb-4 shrink-0">
          <h2 className="text-base font-bold text-brand-text">New Map</h2>
          <button onClick={onClose} className="text-brand-muted hover:text-brand-text transition-colors">
            <XIcon />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-col flex-1 min-h-0 overflow-y-auto px-6 pb-6 space-y-4">
          <div>
            <label className="block text-xs font-medium text-brand-muted uppercase tracking-wider mb-2">Scale</label>
            <div className="grid grid-cols-3 sm:grid-cols-6 gap-1.5">
              {MAP_TYPES.map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setMapType(t)}
                  className={`py-1.5 rounded text-xs font-medium border transition-colors ${
                    mapType === t
                      ? 'border-brand-purple/40 text-brand-purple bg-brand-purple/10'
                      : 'border-brand-border text-brand-muted hover:text-brand-text'
                  }`}
                >
                  {TYPE_LABELS[t]}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-brand-muted uppercase tracking-wider mb-1.5">Name *</label>
            <input
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. The Shattered Continent"
              className="input-field w-full"
            />
          </div>

          {existingMaps.length > 0 && (
            <div>
              <label className="block text-xs font-medium text-brand-muted uppercase tracking-wider mb-1.5">
                Parent map (optional)
              </label>
              <select
                value={parentId}
                onChange={(e) => setParentId(e.target.value)}
                className="input-field w-full"
              >
                <option value="">None</option>
                {existingMaps.map((m) => (
                  <option key={m.id} value={m.id}>{m.name}</option>
                ))}
              </select>
              <p className="text-[10px] text-brand-muted mt-1">
                E.g. this city map zooms into a region map — pick the region as the parent.
              </p>
            </div>
          )}

          {error && <p className="text-xs text-red-400">{error}</p>}

          <button
            type="submit"
            disabled={submitting || !name.trim()}
            className="px-4 py-2 rounded-lg bg-brand-gradient text-brand-bg text-sm font-semibold disabled:opacity-50"
          >
            {submitting ? 'Creating…' : 'Create Map'}
          </button>
        </form>
      </div>
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

function PlusIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
      <path d="M10 4v12M4 10h12" />
    </svg>
  )
}

function XIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
      <path d="M5 5l10 10M15 5L5 15" />
    </svg>
  )
}

function SpinIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none">
      <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" strokeOpacity="0.25" />
      <path d="M22 12a10 10 0 00-10-10" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
    </svg>
  )
}
