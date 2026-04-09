// WikiHub — full-page wiki browser. Tabs: Entities | Timeline.
// Accessible at /projects/:id/wiki
import { useState, useEffect, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '@/services/api'
import type { WikiEntity, EntityType } from '@/services/api'
import TimelineView from '@/components/wiki/TimelineView'
import { useAuthStore } from '@/app/store/authStore'

type Tab = 'entities' | 'timeline'

export default function WikiHub() {
  const { id: projectId } = useParams<{ id: string }>()
  const token = useAuthStore((s) => s.accessToken) ?? ''
  const [tab, setTab] = useState<Tab>('entities')
  const [projectTitle, setProjectTitle] = useState('')

  useEffect(() => {
    if (!projectId || !token) return
    api.projects.get(token, projectId).then((p) => setProjectTitle(p.title)).catch(() => {})
  }, [token, projectId])

  if (!projectId) return null

  return (
    <div className="h-screen flex flex-col bg-brand-bg overflow-hidden">
      {/* Top nav */}
      <header className="h-11 shrink-0 flex items-center gap-3 px-4 bg-brand-bg-card border-b border-brand-border">
        <Link
          to={`/projects/${projectId}`}
          className="flex items-center gap-1.5 text-xs text-brand-muted hover:text-brand-text transition-colors"
        >
          <BackIcon />
          Editor
        </Link>
        <span className="text-brand-border">|</span>
        <span className="text-brand-cyan text-sm font-semibold">World Wiki</span>
        {projectTitle && (
          <>
            <span className="text-brand-border">—</span>
            <span className="text-xs text-brand-muted truncate max-w-xs">{projectTitle}</span>
          </>
        )}
      </header>

      {/* Tab bar */}
      <div className="shrink-0 flex items-center gap-1 px-4 border-b border-brand-border bg-brand-bg-card">
        <TabButton active={tab === 'entities'} onClick={() => setTab('entities')}>
          <BookIcon />
          Entities
        </TabButton>
        <TabButton active={tab === 'timeline'} onClick={() => setTab('timeline')}>
          <ClockIcon />
          Timeline
        </TabButton>
      </div>

      {/* Content */}
      <main className="flex-1 overflow-y-auto px-6 py-6 max-w-5xl w-full mx-auto">
        {tab === 'entities' && <EntitiesTab token={token} projectId={projectId} />}
        {tab === 'timeline' && <TimelineView token={token} projectId={projectId} />}
      </main>
    </div>
  )
}

// ── Entities tab ──────────────────────────────────────────────────────────────

const ENTITY_TYPES: EntityType[] = ['character', 'location', 'faction', 'item', 'concept', 'lore']

const TYPE_COLORS: Record<EntityType, string> = {
  character: 'text-brand-cyan bg-brand-cyan/10 border-brand-cyan/20',
  location: 'text-brand-gold bg-brand-gold/10 border-brand-gold/20',
  faction: 'text-brand-purple bg-brand-purple/10 border-brand-purple/20',
  item: 'text-emerald-400 bg-emerald-400/10 border-emerald-400/20',
  concept: 'text-sky-400 bg-sky-400/10 border-sky-400/20',
  lore: 'text-rose-400 bg-rose-400/10 border-rose-400/20',
}

function EntitiesTab({ token, projectId }: { token: string; projectId: string }) {
  const [entities, setEntities] = useState<WikiEntity[]>([])
  const [filter, setFilter] = useState<EntityType | 'all'>('all')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<WikiEntity | null>(null)
  const [showCreate, setShowCreate] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const list = await api.wiki.listEntities(token, projectId, filter === 'all' ? undefined : filter)
      setEntities(list)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load wiki')
    } finally {
      setLoading(false)
    }
  }, [token, projectId, filter])

  useEffect(() => { load() }, [load])

  useEffect(() => {
    if (selected) {
      const updated = entities.find((e) => e.id === selected.id)
      if (updated) setSelected(updated)
    }
  }, [entities])

  const handleCreated = (entity: WikiEntity) => {
    setEntities((prev) => [entity, ...prev])
    setSelected(entity)
    setShowCreate(false)
  }

  const handleUpdated = (entity: WikiEntity) => {
    setEntities((prev) => prev.map((e) => e.id === entity.id ? entity : e))
    setSelected(entity)
  }

  const handleDeleted = (id: string) => {
    setEntities((prev) => prev.filter((e) => e.id !== id))
    setSelected(null)
  }

  if (selected) {
    return (
      <EntityDetail
        token={token}
        projectId={projectId}
        entity={selected}
        onBack={() => setSelected(null)}
        onUpdated={handleUpdated}
        onDeleted={handleDeleted}
      />
    )
  }

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div className="flex flex-wrap gap-1.5">
          <FilterChip active={filter === 'all'} onClick={() => setFilter('all')}>All</FilterChip>
          {ENTITY_TYPES.map((t) => (
            <FilterChip key={t} active={filter === t} onClick={() => setFilter(t)}>
              {capitalize(t)}
            </FilterChip>
          ))}
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-brand-purple/10 text-brand-purple text-xs font-semibold hover:bg-brand-purple/20 transition-colors"
        >
          <PlusIcon />
          New Entity
        </button>
      </div>

      {/* State */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <SpinIcon className="w-5 h-5 text-brand-purple animate-spin" />
        </div>
      ) : error ? (
        <p className="text-sm text-red-400 py-6">{error}</p>
      ) : entities.length === 0 ? (
        <div className="text-center py-20 text-brand-muted">
          <p className="text-sm">{filter === 'all' ? 'No entities yet.' : `No ${filter}s yet.`}</p>
          <button onClick={() => setShowCreate(true)} className="mt-2 text-brand-purple text-sm hover:underline">
            Add the first one
          </button>
        </div>
      ) : (
        /* Card grid */
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {entities.map((entity) => (
            <button
              key={entity.id}
              onClick={() => setSelected(entity)}
              className="text-left p-4 rounded-xl bg-brand-bg-card border border-brand-border hover:border-brand-border/60 hover:bg-brand-bg-card/80 transition-colors"
            >
              <div className="flex items-center gap-2 mb-2">
                <span className={`px-2 py-0.5 rounded text-[10px] font-semibold border ${TYPE_COLORS[entity.type]}`}>
                  {entity.type}
                </span>
              </div>
              <h3 className="text-sm font-semibold text-brand-text mb-1">{entity.name}</h3>
              {entity.summary && (
                <p className="text-xs text-brand-muted line-clamp-2 leading-relaxed">{entity.summary}</p>
              )}
            </button>
          ))}
        </div>
      )}

      {/* Create modal */}
      {showCreate && (
        <CreateEntityModal
          token={token}
          projectId={projectId}
          onCreated={handleCreated}
          onClose={() => setShowCreate(false)}
        />
      )}
    </div>
  )
}

// ── Entity detail (full width) ────────────────────────────────────────────────

function EntityDetail({
  token,
  projectId,
  entity,
  onBack,
  onUpdated,
  onDeleted,
}: {
  token: string
  projectId: string
  entity: WikiEntity
  onBack: () => void
  onUpdated: (e: WikiEntity) => void
  onDeleted: (id: string) => void
}) {
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState(entity.name)
  const [summary, setSummary] = useState(entity.summary)
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [confirmDelete, setConfirmDelete] = useState(false)

  const handleSave = async () => {
    setSaving(true)
    setError(null)
    try {
      const updated = await api.wiki.updateEntity(token, projectId, entity.id, {
        name: name.trim(),
        summary: summary.trim(),
      })
      onUpdated(updated)
      setEditing(false)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    setDeleting(true)
    try {
      await api.wiki.deleteEntity(token, projectId, entity.id)
      onDeleted(entity.id)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Delete failed')
      setDeleting(false)
      setConfirmDelete(false)
    }
  }

  const attrEntries = Object.entries(entity.attributes ?? {})

  return (
    <div className="max-w-2xl space-y-6">
      <button
        onClick={onBack}
        className="flex items-center gap-1.5 text-sm text-brand-muted hover:text-brand-text transition-colors"
      >
        <BackIcon />
        Back to entities
      </button>

      <div className="p-6 rounded-2xl bg-brand-bg-card border border-brand-border space-y-4">
        {/* Name + type */}
        <div className="flex items-start justify-between gap-4">
          <div className="flex-1 min-w-0">
            {editing ? (
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="input-field text-lg w-full font-bold"
                autoFocus
              />
            ) : (
              <h2 className="text-lg font-bold text-brand-text">{entity.name}</h2>
            )}
            <span className={`inline-block mt-1.5 px-2 py-0.5 rounded text-xs font-semibold border ${TYPE_COLORS[entity.type]}`}>
              {entity.type}
            </span>
          </div>
          <button
            onClick={() => { setEditing((v) => !v); setError(null) }}
            className="text-brand-muted hover:text-brand-text transition-colors mt-1"
            title={editing ? 'Cancel' : 'Edit'}
          >
            {editing ? <XIcon /> : <EditIcon />}
          </button>
        </div>

        {/* Summary */}
        <div>
          <label className="block text-xs font-semibold text-brand-muted uppercase tracking-wider mb-2">Summary</label>
          {editing ? (
            <textarea
              value={summary}
              onChange={(e) => setSummary(e.target.value)}
              rows={5}
              className="input-field resize-none w-full"
            />
          ) : (
            <p className="text-sm text-brand-text leading-relaxed">
              {entity.summary || <span className="text-brand-muted italic">No summary.</span>}
            </p>
          )}
        </div>

        {/* Attributes */}
        {attrEntries.length > 0 && (
          <div>
            <label className="block text-xs font-semibold text-brand-muted uppercase tracking-wider mb-2">Attributes</label>
            <div className="grid grid-cols-2 gap-x-4 gap-y-1.5">
              {attrEntries.map(([k, v]) => (
                <div key={k} className="flex gap-2 text-sm">
                  <span className="text-brand-muted font-medium capitalize shrink-0">{k}:</span>
                  <span className="text-brand-text">{v}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        {error && <p className="text-xs text-red-400">{error}</p>}

        {/* Save */}
        {editing && (
          <button
            onClick={handleSave}
            disabled={saving || !name.trim()}
            className="px-4 py-2 rounded-lg bg-brand-gradient text-brand-bg text-sm font-semibold disabled:opacity-50"
          >
            {saving ? 'Saving…' : 'Save Changes'}
          </button>
        )}

        {/* Delete */}
        {!editing && (
          confirmDelete ? (
            <div className="flex items-center gap-3 pt-2 border-t border-brand-border">
              <span className="text-xs text-brand-muted flex-1">Delete this entity permanently?</span>
              <button onClick={() => setConfirmDelete(false)} className="px-3 py-1.5 rounded border border-brand-border text-brand-muted text-xs hover:text-brand-text transition-colors">
                Cancel
              </button>
              <button onClick={handleDelete} disabled={deleting} className="px-3 py-1.5 rounded border border-red-500/40 text-red-400 text-xs hover:bg-red-500/10 transition-colors disabled:opacity-50">
                {deleting ? 'Deleting…' : 'Delete'}
              </button>
            </div>
          ) : (
            <div className="pt-2 border-t border-brand-border">
              <button
                onClick={() => setConfirmDelete(true)}
                className="px-3 py-1.5 rounded border border-brand-border text-brand-muted text-xs hover:text-red-400 hover:border-red-500/40 transition-colors"
              >
                Delete Entity
              </button>
            </div>
          )
        )}
      </div>
    </div>
  )
}

// ── Create entity modal ───────────────────────────────────────────────────────

function CreateEntityModal({
  token,
  projectId,
  onCreated,
  onClose,
}: {
  token: string
  projectId: string
  onCreated: (e: WikiEntity) => void
  onClose: () => void
}) {
  const [type, setType] = useState<EntityType>('character')
  const [name, setName] = useState('')
  const [summary, setSummary] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    setSubmitting(true)
    setError(null)
    try {
      const entity = await api.wiki.createEntity(token, projectId, {
        type,
        name: name.trim(),
        summary: summary.trim(),
      })
      onCreated(entity)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create entity')
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm px-4">
      <div className="bg-brand-bg-card border border-brand-border rounded-2xl p-6 w-full max-w-sm shadow-card">
        <div className="flex items-center justify-between mb-5">
          <h2 className="text-base font-bold text-brand-text">New Wiki Entry</h2>
          <button onClick={onClose} className="text-brand-muted hover:text-brand-text transition-colors">
            <XIcon />
          </button>
        </div>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-brand-muted uppercase tracking-wider mb-2">Type</label>
            <div className="grid grid-cols-3 gap-1.5">
              {ENTITY_TYPES.map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setType(t)}
                  className={`py-1.5 rounded text-xs font-medium border transition-colors ${
                    type === t ? TYPE_COLORS[t] : 'border-brand-border text-brand-muted hover:text-brand-text'
                  }`}
                >
                  {capitalize(t)}
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
              placeholder={`e.g. ${type === 'character' ? 'Commander Voss' : type === 'location' ? 'Kepler Station' : 'United Earth Fleet'}`}
              className="input-field w-full"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-brand-muted uppercase tracking-wider mb-1.5">Summary</label>
            <textarea
              value={summary}
              onChange={(e) => setSummary(e.target.value)}
              placeholder="Brief description…"
              rows={3}
              className="input-field resize-none w-full"
            />
          </div>
          {error && <p className="text-xs text-red-400">{error}</p>}
          <div className="flex gap-2 pt-1">
            <button type="button" onClick={onClose} className="flex-1 py-2 rounded-lg border border-brand-border text-brand-muted text-sm hover:text-brand-text transition-colors">
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting || !name.trim()}
              className="flex-1 py-2 rounded-lg bg-brand-gradient text-brand-bg text-sm font-semibold disabled:opacity-50"
            >
              {submitting ? 'Creating…' : 'Create'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ── Shared ────────────────────────────────────────────────────────────────────

function FilterChip({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      className={`px-2.5 py-1 rounded text-xs font-semibold transition-colors ${
        active
          ? 'bg-brand-purple/20 text-brand-purple'
          : 'text-brand-muted hover:text-brand-text border border-brand-border'
      }`}
    >
      {children}
    </button>
  )
}

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      className={`flex items-center gap-1.5 px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
        active
          ? 'border-brand-cyan text-brand-cyan'
          : 'border-transparent text-brand-muted hover:text-brand-text'
      }`}
    >
      {children}
    </button>
  )
}

function capitalize(s: string) {
  return s.charAt(0).toUpperCase() + s.slice(1)
}

// ── Icons ─────────────────────────────────────────────────────────────────────

function PlusIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
      <path d="M8 3v10M3 8h10" />
    </svg>
  )
}

function BackIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M10 3L5 8l5 5" />
    </svg>
  )
}

function EditIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M11 2l3 3-8 8H3v-3L11 2z" />
    </svg>
  )
}

function XIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <path d="M3 3l10 10M13 3L3 13" />
    </svg>
  )
}

function BookIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 3h9a2 2 0 012 2v10a2 2 0 01-2 2H4a1 1 0 01-1-1V4a1 1 0 011-1z" />
      <path d="M13 3v14M7 7h3M7 10h3" />
    </svg>
  )
}

function ClockIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="10" cy="10" r="8" />
      <path d="M10 6v4l3 3" />
    </svg>
  )
}

function SpinIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
    </svg>
  )
}
