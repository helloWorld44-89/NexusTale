// TimelineView — chronological list of wiki timeline events with CRUD.
// Accepts optional `phases` + `structureName` from a selected novel structure;
// when provided, muted phase banners are overlaid between era groups.
import { useState, useEffect, useCallback, useMemo } from 'react'
import { api } from '@/services/api'
import type { WikiTimelineEvent } from '@/services/api'

interface TimelineViewProps {
  token: string
  projectId: string
  /** Phase list from the project's selected story structure. */
  phases?: Array<{ name: string }>
  /** Display name of the selected story structure (for the hint label). */
  structureName?: string
}

export default function TimelineView({ token, projectId, phases, structureName }: TimelineViewProps) {
  const [events, setEvents] = useState<WikiTimelineEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [editing, setEditing] = useState<WikiTimelineEvent | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const list = await api.wiki.listTimeline(token, projectId)
      setEvents(list)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load timeline')
    } finally {
      setLoading(false)
    }
  }, [token, projectId])

  useEffect(() => { load() }, [load])

  // Group events by era, sorted by earliest year in each group.
  const eraGroups = useMemo(() => {
    const map = new Map<string, WikiTimelineEvent[]>()
    for (const ev of events) {
      const key = ev.era ?? ''
      if (!map.has(key)) map.set(key, [])
      map.get(key)!.push(ev)
    }
    const entries = [...map.entries()]
    entries.sort(([, a], [, b]) => {
      const minYear = (evs: WikiTimelineEvent[]) =>
        evs.reduce((m, e) => (e.year != null ? Math.min(m, e.year) : m), Infinity)
      const ay = minYear(a)
      const by = minYear(b)
      if (ay === Infinity && by === Infinity) return 0
      if (ay === Infinity) return 1
      if (by === Infinity) return -1
      return ay - by
    })
    return entries
  }, [events])

  const handleCreated = (ev: WikiTimelineEvent) => {
    setEvents((prev) => [...prev, ev])
    setShowCreate(false)
  }

  const handleUpdated = (ev: WikiTimelineEvent) => {
    setEvents((prev) => prev.map((e) => e.id === ev.id ? ev : e))
    setEditing(null)
  }

  const handleDeleted = (id: string) => {
    setEvents((prev) => prev.filter((e) => e.id !== id))
  }

  return (
    <div className="space-y-4">
      {/* Header row */}
      <div className="flex items-center justify-between">
        <p className="text-sm text-brand-muted">
          {events.length} {events.length === 1 ? 'event' : 'events'}
        </p>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-brand-purple/10 text-brand-purple text-xs font-semibold hover:bg-brand-purple/20 transition-colors"
        >
          <PlusIcon />
          New Event
        </button>
      </div>

      {/* Create form */}
      {showCreate && (
        <EventForm
          token={token}
          projectId={projectId}
          events={events}
          onSaved={handleCreated}
          onCancel={() => setShowCreate(false)}
        />
      )}

      {/* State */}
      {loading ? (
        <div className="flex items-center justify-center py-16">
          <SpinIcon className="w-5 h-5 text-brand-purple animate-spin" />
        </div>
      ) : error ? (
        <p className="text-sm text-red-400 py-4">{error}</p>
      ) : events.length === 0 && !showCreate ? (
        <div className="text-center py-16 text-brand-muted">
          <p className="text-sm">No timeline events yet.</p>
          <button onClick={() => setShowCreate(true)} className="mt-2 text-brand-purple text-sm hover:underline">
            Add the first event
          </button>
        </div>
      ) : (
        /* Era-grouped event list with optional structure phase banners */
        <div className="space-y-6">
          {eraGroups.map(([era, groupEvents], groupIdx) => (
            <div key={era || '__no_era__'}>
              {/* Phase banner — shown when a structure is active */}
              {phases && phases[groupIdx] && (
                <div className="flex items-center gap-3 mb-3">
                  <div className="flex-1 h-px bg-brand-border/40" />
                  <span
                    className="text-xs text-brand-muted/70 italic font-medium tracking-wide shrink-0"
                    title={structureName ? `${structureName} · phase ${groupIdx + 1}` : undefined}
                  >
                    {phases[groupIdx].name}
                  </span>
                  <div className="flex-1 h-px bg-brand-border/40" />
                </div>
              )}

              <div className="space-y-2">
                {groupEvents.map((ev) =>
                  editing?.id === ev.id ? (
                    <EventForm
                      key={ev.id}
                      token={token}
                      projectId={projectId}
                      events={events}
                      existing={ev}
                      onSaved={handleUpdated}
                      onCancel={() => setEditing(null)}
                    />
                  ) : (
                    <EventCard
                      key={ev.id}
                      event={ev}
                      allEvents={events}
                      onEdit={() => setEditing(ev)}
                      onDeleted={handleDeleted}
                      token={token}
                      projectId={projectId}
                    />
                  ),
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ── Event card ────────────────────────────────────────────────────────────────

function EventCard({
  event,
  allEvents,
  onEdit,
  onDeleted,
  token,
  projectId,
}: {
  event: WikiTimelineEvent
  allEvents: WikiTimelineEvent[]
  onEdit: () => void
  onDeleted: (id: string) => void
  token: string
  projectId: string
}) {
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [deleting, setDeleting] = useState(false)

  const handleDelete = async () => {
    setDeleting(true)
    try {
      await api.wiki.deleteTimelineEvent(token, projectId, event.id)
      onDeleted(event.id)
    } catch {
      setDeleting(false)
      setConfirmDelete(false)
    }
  }

  const dateLine = formatDate(event)
  const anchorName = event.anchor_event_id
    ? allEvents.find((e) => e.id === event.anchor_event_id)?.name ?? 'Unknown anchor'
    : null

  return (
    <div className="group flex gap-4 p-4 rounded-xl bg-brand-bg-card border border-brand-border hover:border-brand-border/80 transition-colors">
      {/* Timeline spine */}
      <div className="flex flex-col items-center pt-1 shrink-0">
        <div className="w-2.5 h-2.5 rounded-full bg-brand-purple/60 ring-2 ring-brand-purple/20" />
        <div className="w-px flex-1 bg-brand-border mt-1" />
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0 pb-1">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-brand-text">{event.name}</h3>
            <div className="flex flex-wrap items-center gap-2 mt-0.5">
              {event.era && (
                <span className="text-xs text-brand-gold/80 font-medium">{event.era}</span>
              )}
              {dateLine && (
                <span className="text-xs text-brand-muted">{dateLine}</span>
              )}
              {anchorName && (
                <span className="text-xs text-brand-cyan/70 italic">anchored to {anchorName}</span>
              )}
            </div>
          </div>

          {/* Actions */}
          <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
            {!confirmDelete && (
              <button
                onClick={onEdit}
                className="p-1.5 rounded text-brand-muted hover:text-brand-text hover:bg-brand-border/40 transition-colors"
                title="Edit"
              >
                <EditIcon />
              </button>
            )}
            {confirmDelete ? (
              <div className="flex items-center gap-1">
                <button
                  onClick={() => setConfirmDelete(false)}
                  className="px-2 py-1 rounded text-xs text-brand-muted border border-brand-border hover:text-brand-text transition-colors"
                >
                  Cancel
                </button>
                <button
                  onClick={handleDelete}
                  disabled={deleting}
                  className="px-2 py-1 rounded text-xs text-red-400 border border-red-500/30 hover:bg-red-500/10 transition-colors disabled:opacity-50"
                >
                  {deleting ? '…' : 'Delete'}
                </button>
              </div>
            ) : (
              <button
                onClick={() => setConfirmDelete(true)}
                className="p-1.5 rounded text-brand-muted hover:text-red-400 hover:bg-brand-border/40 transition-colors"
                title="Delete"
              >
                <TrashIcon />
              </button>
            )}
          </div>
        </div>

        {event.description && (
          <p className="mt-1.5 text-xs text-brand-muted leading-relaxed line-clamp-2">{event.description}</p>
        )}
      </div>
    </div>
  )
}

// ── Event form (create + edit) ────────────────────────────────────────────────

function EventForm({
  token,
  projectId,
  events,
  existing,
  onSaved,
  onCancel,
}: {
  token: string
  projectId: string
  events: WikiTimelineEvent[]
  existing?: WikiTimelineEvent
  onSaved: (ev: WikiTimelineEvent) => void
  onCancel: () => void
}) {
  const [name, setName] = useState(existing?.name ?? '')
  const [description, setDescription] = useState(existing?.description ?? '')
  const [era, setEra] = useState(existing?.era ?? '')
  const [year, setYear] = useState(existing?.year?.toString() ?? '')
  const [month, setMonth] = useState(existing?.month?.toString() ?? '')
  const [day] = useState(existing?.day?.toString() ?? '')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    setSubmitting(true)
    setError(null)
    try {
      const base = {
        name: name.trim(),
        ...(description.trim() && { description: description.trim() }),
        ...(era.trim() && { era: era.trim() }),
        ...(year && { year: parseInt(year, 10) }),
        ...(month && { month: parseInt(month, 10) }),
        ...(day && { day: parseInt(day, 10) }),
      }
      let ev: WikiTimelineEvent
      if (existing) {
        ev = await api.wiki.updateTimelineEvent(token, projectId, existing.id, base)
      } else {
        ev = await api.wiki.createTimelineEvent(token, projectId, { ...base, name: base.name })
      }
      onSaved(ev)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save event')
      setSubmitting(false)
    }
  }

  // Anchor events are only available for other events (not self)
  const anchorOptions = events.filter((e) => e.id !== existing?.id)

  return (
    <form
      onSubmit={handleSubmit}
      className="p-4 rounded-xl bg-brand-bg-card border border-brand-purple/30 space-y-3"
    >
      <p className="text-xs font-semibold text-brand-purple uppercase tracking-wider">
        {existing ? 'Edit Event' : 'New Timeline Event'}
      </p>

      <div>
        <label className="block text-xs text-brand-muted mb-1">Name *</label>
        <input
          autoFocus
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. First Contact"
          className="input-field w-full"
        />
      </div>

      <div>
        <label className="block text-xs text-brand-muted mb-1">Description</label>
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="What happened?"
          rows={2}
          className="input-field resize-none w-full"
        />
      </div>

      <div className="grid grid-cols-4 gap-2">
        <div className="col-span-2">
          <label className="block text-xs text-brand-muted mb-1">Era</label>
          <input
            value={era}
            onChange={(e) => setEra(e.target.value)}
            placeholder="e.g. Pre-War"
            className="input-field w-full"
          />
        </div>
        <div>
          <label className="block text-xs text-brand-muted mb-1">Year</label>
          <input
            type="number"
            value={year}
            onChange={(e) => setYear(e.target.value)}
            placeholder="2340"
            className="input-field w-full"
          />
        </div>
        <div>
          <label className="block text-xs text-brand-muted mb-1">Month</label>
          <input
            type="number"
            value={month}
            onChange={(e) => setMonth(e.target.value)}
            placeholder="1"
            className="input-field w-full"
          />
        </div>
      </div>

      {anchorOptions.length > 0 && !existing && (
        <p className="text-xs text-brand-muted italic">
          Anchor offsets (relative events) can be set via the API for now.
        </p>
      )}

      {error && <p className="text-xs text-red-400">{error}</p>}

      <div className="flex gap-2 pt-1">
        <button
          type="button"
          onClick={onCancel}
          className="flex-1 py-1.5 rounded-lg border border-brand-border text-brand-muted text-sm hover:text-brand-text transition-colors"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={submitting || !name.trim()}
          className="flex-1 py-1.5 rounded-lg bg-brand-gradient text-brand-bg text-sm font-semibold disabled:opacity-50"
        >
          {submitting ? 'Saving…' : existing ? 'Save Changes' : 'Create'}
        </button>
      </div>
    </form>
  )
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function formatDate(ev: WikiTimelineEvent): string {
  if (ev.year == null) return ''
  let s = `Year ${ev.year}`
  if (ev.month != null) s += `, Month ${ev.month}`
  if (ev.day != null) s += `, Day ${ev.day}`
  return s
}

// ── Icons ─────────────────────────────────────────────────────────────────────

function PlusIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
      <path d="M8 3v10M3 8h10" />
    </svg>
  )
}

function EditIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M11 2l3 3-8 8H3v-3L11 2z" />
    </svg>
  )
}

function TrashIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 4h10M6 4V2h4v2M5 4v9a1 1 0 001 1h4a1 1 0 001-1V4" />
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
