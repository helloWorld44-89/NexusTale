// WikiPanel — left-panel view for browsing and editing wiki entities.
// Mounted in Editor when leftPanel === 'wiki'.
import { useState, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api } from '@/services/api'
import type { WikiEntity, EntityType } from '@/services/api'

interface WikiPanelProps {
  token: string
  projectId: string
}

const ENTITY_TYPES: EntityType[] = ['character', 'location', 'faction', 'item', 'concept', 'lore']

const TYPE_COLORS: Record<EntityType, string> = {
  character: 'text-brand-cyan bg-brand-cyan/10',
  location: 'text-brand-gold bg-brand-gold/10',
  faction: 'text-brand-purple bg-brand-purple/10',
  item: 'text-emerald-400 bg-emerald-400/10',
  concept: 'text-sky-400 bg-sky-400/10',
  lore: 'text-rose-400 bg-rose-400/10',
}

export default function WikiPanel({ token, projectId }: WikiPanelProps) {
  const [entities, setEntities] = useState<WikiEntity[]>([])
  const [filter, setFilter] = useState<EntityType | 'all'>('all')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<WikiEntity | null>(null)
  const [showCreate, setShowCreate] = useState(false)

  const loadEntities = useCallback(async () => {
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

  useEffect(() => { loadEntities() }, [loadEntities])

  // Refresh selected entity when list changes
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

  const handleDeleted = (entityId: string) => {
    setEntities((prev) => prev.filter((e) => e.id !== entityId))
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
    <div className="w-72 flex flex-col border-r border-brand-border bg-brand-bg shrink-0 overflow-hidden">
      {/* Header */}
      <div className="px-4 pt-4 pb-2 shrink-0">
        <div className="flex items-center justify-between mb-3">
          <span className="text-xs font-semibold text-brand-muted uppercase tracking-wider">World Wiki</span>
          <div className="flex items-center gap-1">
            <Link
              to={`/projects/${projectId}/wiki`}
              className="p-1 rounded text-brand-muted hover:text-brand-cyan transition-colors"
              title="Open full wiki"
            >
              <ExternalIcon />
            </Link>
            <button
              onClick={() => setShowCreate(true)}
              className="flex items-center gap-1 px-2 py-1 rounded text-xs bg-brand-purple/10 text-brand-purple hover:bg-brand-purple/20 transition-colors"
            >
              <PlusIcon />
              New
            </button>
          </div>
        </div>

        {/* Type filter */}
        <div className="flex flex-wrap gap-1">
          <FilterChip active={filter === 'all'} onClick={() => setFilter('all')}>All</FilterChip>
          {ENTITY_TYPES.map((t) => (
            <FilterChip key={t} active={filter === t} onClick={() => setFilter(t)}>
              {capitalize(t)}
            </FilterChip>
          ))}
        </div>
      </div>

      {/* Entity list */}
      <div className="flex-1 overflow-y-auto min-h-0 px-3 py-1">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <SpinIcon className="w-4 h-4 text-brand-purple animate-spin" />
          </div>
        ) : error ? (
          <p className="text-xs text-red-400 px-1 py-4">{error}</p>
        ) : entities.length === 0 ? (
          <p className="text-xs text-brand-muted text-center py-8">
            {filter === 'all' ? 'No entities yet.' : `No ${filter}s yet.`}
          </p>
        ) : (
          <div className="space-y-1 py-1">
            {entities.map((entity) => (
              <button
                key={entity.id}
                onClick={() => setSelected(entity)}
                className="w-full text-left px-3 py-2.5 rounded-lg border border-transparent hover:border-brand-border hover:bg-brand-bg-card transition-colors"
              >
                <div className="flex items-center gap-2 mb-0.5">
                  <span className={`px-1.5 py-0.5 rounded text-[10px] font-semibold ${TYPE_COLORS[entity.type]}`}>
                    {entity.type}
                  </span>
                  <span className="text-sm font-medium text-brand-text truncate flex-1">{entity.name}</span>
                </div>
                {entity.summary && (
                  <p className="text-xs text-brand-muted line-clamp-1 pl-0.5">{entity.summary}</p>
                )}
              </button>
            ))}
          </div>
        )}
      </div>

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

// ── Entity detail ─────────────────────────────────────────────────────────────

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
      const updated = await api.wiki.updateEntity(token, projectId, entity.id, { name: name.trim(), summary: summary.trim() })
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
    <div className="w-72 flex flex-col border-r border-brand-border bg-brand-bg shrink-0 overflow-hidden">
      {/* Header */}
      <div className="px-4 pt-4 pb-3 border-b border-brand-border shrink-0">
        <button
          onClick={onBack}
          className="flex items-center gap-1 text-xs text-brand-muted hover:text-brand-text transition-colors mb-3"
        >
          <BackIcon />
          Back
        </button>
        <div className="flex items-start gap-2">
          <div className="flex-1 min-w-0">
            {editing ? (
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="input-field text-sm w-full"
                autoFocus
              />
            ) : (
              <h2 className="text-sm font-bold text-brand-text truncate">{entity.name}</h2>
            )}
            <span className={`inline-block mt-1 px-1.5 py-0.5 rounded text-[10px] font-semibold ${TYPE_COLORS[entity.type]}`}>
              {entity.type}
            </span>
          </div>
          <button
            onClick={() => { setEditing((v) => !v); setError(null) }}
            className="text-brand-muted hover:text-brand-text transition-colors shrink-0 mt-0.5"
            title={editing ? 'Cancel' : 'Edit'}
          >
            {editing ? <XIcon /> : <EditIcon />}
          </button>
        </div>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto min-h-0 px-4 py-3 space-y-4">
        {/* Summary */}
        <div>
          <label className="block text-[10px] font-semibold text-brand-muted uppercase tracking-wider mb-1">Summary</label>
          {editing ? (
            <textarea
              value={summary}
              onChange={(e) => setSummary(e.target.value)}
              rows={4}
              className="input-field resize-none w-full text-sm"
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
            <label className="block text-[10px] font-semibold text-brand-muted uppercase tracking-wider mb-2">Attributes</label>
            <div className="space-y-1.5">
              {attrEntries.map(([k, v]) => (
                <div key={k} className="flex gap-2 text-xs">
                  <span className="text-brand-muted font-medium capitalize shrink-0">{k}</span>
                  <span className="text-brand-text">{v}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        {error && <p className="text-xs text-red-400">{error}</p>}

        {/* Actions */}
        {editing && (
          <button
            onClick={handleSave}
            disabled={saving || !name.trim()}
            className="w-full py-2 rounded-lg bg-brand-gradient text-brand-bg text-sm font-semibold disabled:opacity-50"
          >
            {saving ? 'Saving…' : 'Save Changes'}
          </button>
        )}

        {/* Delete */}
        {!editing && (
          confirmDelete ? (
            <div className="space-y-2">
              <p className="text-xs text-brand-muted">Delete this entity permanently?</p>
              <div className="flex gap-2">
                <button onClick={() => setConfirmDelete(false)} className="flex-1 py-1.5 rounded border border-brand-border text-brand-muted text-xs hover:text-brand-text transition-colors">
                  Cancel
                </button>
                <button onClick={handleDelete} disabled={deleting} className="flex-1 py-1.5 rounded border border-red-500/40 text-red-400 text-xs hover:bg-red-500/10 transition-colors disabled:opacity-50">
                  {deleting ? 'Deleting…' : 'Delete'}
                </button>
              </div>
            </div>
          ) : (
            <button
              onClick={() => setConfirmDelete(true)}
              className="w-full py-1.5 rounded border border-brand-border text-brand-muted text-xs hover:text-red-400 hover:border-red-500/40 transition-colors"
            >
              Delete Entity
            </button>
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
      const entity = await api.wiki.createEntity(token, projectId, { type, name: name.trim(), summary: summary.trim() })
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
            <svg className="w-4 h-4" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
              <path d="M3 3l10 10M13 3L3 13" />
            </svg>
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Type selector */}
          <div>
            <label className="block text-xs font-medium text-brand-muted uppercase tracking-wider mb-2">Type</label>
            <div className="grid grid-cols-3 gap-1.5">
              {ENTITY_TYPES.map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setType(t)}
                  className={`py-1.5 rounded text-xs font-medium transition-colors ${
                    type === t
                      ? `${TYPE_COLORS[t]} border border-current`
                      : 'border border-brand-border text-brand-muted hover:text-brand-text'
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

// ── Shared sub-components ─────────────────────────────────────────────────────

function FilterChip({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      className={`px-2 py-0.5 rounded text-[10px] font-semibold transition-colors ${
        active
          ? 'bg-brand-purple/20 text-brand-purple'
          : 'text-brand-muted hover:text-brand-text border border-brand-border'
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

function ExternalIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M7 3H3a1 1 0 00-1 1v9a1 1 0 001 1h9a1 1 0 001-1V9" />
      <path d="M10 2h4v4M14 2L8 8" />
    </svg>
  )
}

function BackIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
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

function SpinIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
    </svg>
  )
}
