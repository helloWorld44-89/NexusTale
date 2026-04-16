// ResearchNotesTab — freeform per-project scratchpad for web quotes,
// worldbuilding facts, and craft references. Lives in WikiHub under the
// "Research" tab. Notes can be pinned into the AI context window via
// the Context Pins panel in the Editor.
import { useState, useCallback, useEffect, useRef } from 'react'
import { api } from '@/services/api'
import type { ResearchNote } from '@/services/api'

interface ResearchNotesTabProps {
  token:     string
  projectId: string
}

type View = 'list' | 'detail'

export default function ResearchNotesTab({ token, projectId }: ResearchNotesTabProps) {
  const [notes,   setNotes]   = useState<ResearchNote[]>([])
  const [loading, setLoading] = useState(true)
  const [view,    setView]    = useState<View>('list')
  const [active,  setActive]  = useState<ResearchNote | null>(null)
  const [creating, setCreating] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const list = await api.research.list(token, projectId)
      setNotes(list)
    } finally {
      setLoading(false)
    }
  }, [token, projectId])

  useEffect(() => { load() }, [load])

  const handleSelect = (note: ResearchNote) => {
    setActive(note)
    setView('detail')
    setCreating(false)
  }

  const handleNew = () => {
    setActive(null)
    setCreating(true)
    setView('detail')
  }

  const handleSaved = (note: ResearchNote) => {
    setNotes((prev) => {
      const exists = prev.find((n) => n.id === note.id)
      return exists
        ? prev.map((n) => n.id === note.id ? note : n).sort((a, b) =>
            new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
          )
        : [note, ...prev]
    })
    setActive(note)
    setCreating(false)
  }

  const handleDeleted = (id: string) => {
    setNotes((prev) => prev.filter((n) => n.id !== id))
    setActive(null)
    setView('list')
  }

  // ── list view ────────────────────────────────────────────────────────────────

  if (view === 'list' || (view === 'detail' && !creating && !active)) {
    return (
      <div className="space-y-4">
        {/* toolbar */}
        <div className="flex items-center justify-between">
          <p className="text-sm text-brand-muted">
            Paste quotes, worldbuilding facts, and craft references. Pin notes to the AI context in the Editor.
          </p>
          <button
            onClick={handleNew}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-brand-purple/10 text-brand-purple text-xs font-semibold hover:bg-brand-purple/20 transition-colors shrink-0 ml-4"
          >
            <PlusIcon />
            New Note
          </button>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-20">
            <SpinIcon className="w-5 h-5 text-brand-purple animate-spin" />
          </div>
        ) : notes.length === 0 ? (
          <div className="text-center py-20 text-brand-muted">
            <p className="text-sm">No research notes yet.</p>
            <button onClick={handleNew} className="mt-2 text-brand-purple text-sm hover:underline">
              Add the first one
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {notes.map((note) => (
              <NoteCard key={note.id} note={note} onClick={() => handleSelect(note)} />
            ))}
          </div>
        )}
      </div>
    )
  }

  // ── detail / create view ──────────────────────────────────────────────────

  return (
    <NoteDetail
      token={token}
      projectId={projectId}
      note={active}
      onBack={() => { setView('list'); setCreating(false) }}
      onSaved={handleSaved}
      onDeleted={handleDeleted}
    />
  )
}

// ── note card ─────────────────────────────────────────────────────────────────

function NoteCard({ note, onClick }: { note: ResearchNote; onClick: () => void }) {
  const relTime = (() => {
    const diff = Date.now() - new Date(note.updated_at).getTime()
    const days = Math.floor(diff / 86_400_000)
    if (days === 0) return 'today'
    if (days === 1) return 'yesterday'
    return `${days}d ago`
  })()

  const bodyPreview = note.body.slice(0, 140)

  return (
    <button
      onClick={onClick}
      className="text-left p-4 rounded-xl bg-brand-bg-card border border-brand-border hover:border-brand-border/60 hover:bg-brand-bg-card/80 transition-colors space-y-2"
    >
      <div className="flex items-start justify-between gap-2">
        <h3 className="text-sm font-semibold text-brand-text leading-snug">{note.title}</h3>
        <span className="text-[10px] text-brand-muted shrink-0">{relTime}</span>
      </div>
      {bodyPreview && (
        <p className="text-xs text-brand-muted leading-relaxed line-clamp-3">{bodyPreview}</p>
      )}
      <div className="flex items-center gap-1.5 flex-wrap">
        {note.source_url && (
          <span className="inline-flex items-center gap-1 text-[10px] text-brand-cyan/70">
            <LinkIcon />
            source
          </span>
        )}
        {note.tags.slice(0, 4).map((tag) => (
          <span key={tag} className="px-1.5 py-0.5 rounded text-[10px] bg-brand-purple/10 text-brand-purple/80">
            {tag}
          </span>
        ))}
      </div>
    </button>
  )
}

// ── note detail / edit ────────────────────────────────────────────────────────

function NoteDetail({
  token,
  projectId,
  note,
  onBack,
  onSaved,
  onDeleted,
}: {
  token:      string
  projectId:  string
  note:       ResearchNote | null   // null = creating new
  onBack:     () => void
  onSaved:    (n: ResearchNote) => void
  onDeleted:  (id: string) => void
}) {
  const [title,     setTitle]     = useState(note?.title     ?? '')
  const [body,      setBody]      = useState(note?.body      ?? '')
  const [sourceURL, setSourceURL] = useState(note?.source_url ?? '')
  const [tagInput,  setTagInput]  = useState(note?.tags.join(', ') ?? '')
  const [saving,    setSaving]    = useState(false)
  const [deleting,  setDeleting]  = useState(false)
  const [confirmDel,setConfirmDel]= useState(false)
  const [error,     setError]     = useState<string | null>(null)
  const [dirty,     setDirty]     = useState(false)

  const autoSaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Auto-save existing notes after 1.5s of inactivity.
  const scheduleAutoSave = useCallback((newTitle: string, newBody: string, newUrl: string, newTagStr: string) => {
    if (!note) return // creation is explicit
    if (autoSaveTimer.current) clearTimeout(autoSaveTimer.current)
    autoSaveTimer.current = setTimeout(async () => {
      const tags = parseTags(newTagStr)
      try {
        const updated = await api.research.update(token, projectId, note.id, {
          title: newTitle || 'Untitled Note',
          body: newBody,
          source_url: newUrl,
          tags,
        })
        onSaved(updated)
        setDirty(false)
      } catch {
        // silent auto-save failure
      }
    }, 1500)
  }, [token, projectId, note, onSaved])

  const handleChange = (field: 'title' | 'body' | 'source_url' | 'tags', value: string) => {
    setDirty(true)
    const newTitle     = field === 'title'      ? value : title
    const newBody      = field === 'body'       ? value : body
    const newUrl       = field === 'source_url' ? value : sourceURL
    const newTagStr    = field === 'tags'       ? value : tagInput
    if (field === 'title')      setTitle(value)
    if (field === 'body')       setBody(value)
    if (field === 'source_url') setSourceURL(value)
    if (field === 'tags')       setTagInput(value)
    scheduleAutoSave(newTitle, newBody, newUrl, newTagStr)
  }

  const handleSave = async () => {
    setSaving(true)
    setError(null)
    const tags = parseTags(tagInput)
    try {
      let saved: ResearchNote
      if (note) {
        saved = await api.research.update(token, projectId, note.id, {
          title: title.trim() || 'Untitled Note',
          body,
          source_url: sourceURL.trim(),
          tags,
        })
      } else {
        saved = await api.research.create(token, projectId, {
          title: title.trim() || 'Untitled Note',
          body,
          source_url: sourceURL.trim(),
          tags,
        })
      }
      onSaved(saved)
      setDirty(false)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!note) return
    setDeleting(true)
    try {
      await api.research.delete(token, projectId, note.id)
      onDeleted(note.id)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Delete failed')
      setDeleting(false)
      setConfirmDel(false)
    }
  }

  const isNew = !note

  return (
    <div className="max-w-2xl space-y-4">
      <div className="flex items-center gap-3">
        <button
          onClick={onBack}
          className="flex items-center gap-1.5 text-sm text-brand-muted hover:text-brand-text transition-colors"
        >
          <BackIcon />
          All notes
        </button>
        {!isNew && (
          <span className="text-xs text-brand-muted opacity-50">
            {dirty ? 'Unsaved changes…' : 'Auto-saved'}
          </span>
        )}
      </div>

      <div className="p-6 rounded-2xl bg-brand-bg-card border border-brand-border space-y-4">
        {/* Title */}
        <input
          value={title}
          onChange={(e) => handleChange('title', e.target.value)}
          placeholder="Note title…"
          className="input-field w-full text-base font-semibold"
          autoFocus={isNew}
        />

        {/* Body */}
        <div>
          <label className="block text-xs font-semibold text-brand-muted uppercase tracking-wider mb-1.5">Notes</label>
          <textarea
            value={body}
            onChange={(e) => handleChange('body', e.target.value)}
            placeholder="Paste quotes, research, worldbuilding facts…"
            rows={10}
            className="input-field resize-y w-full text-sm leading-relaxed"
          />
        </div>

        {/* Source URL */}
        <div>
          <label className="block text-xs font-semibold text-brand-muted uppercase tracking-wider mb-1.5">Source URL</label>
          <input
            value={sourceURL}
            onChange={(e) => handleChange('source_url', e.target.value)}
            placeholder="https://…"
            type="url"
            className="input-field w-full text-sm"
          />
        </div>

        {/* Tags */}
        <div>
          <label className="block text-xs font-semibold text-brand-muted uppercase tracking-wider mb-1.5">Tags</label>
          <input
            value={tagInput}
            onChange={(e) => handleChange('tags', e.target.value)}
            placeholder="magic, character, worldbuilding  (comma-separated)"
            className="input-field w-full text-sm"
          />
          {parseTags(tagInput).length > 0 && (
            <div className="flex flex-wrap gap-1.5 mt-2">
              {parseTags(tagInput).map((tag) => (
                <span key={tag} className="px-2 py-0.5 rounded text-xs bg-brand-purple/15 text-brand-purple">
                  {tag}
                </span>
              ))}
            </div>
          )}
        </div>

        {error && <p className="text-xs text-red-400">{error}</p>}

        {/* Actions */}
        <div className="flex items-center gap-3 pt-2 border-t border-brand-border">
          {isNew && (
            <button
              onClick={handleSave}
              disabled={saving}
              className="px-4 py-2 rounded-lg bg-brand-gradient text-brand-bg text-sm font-semibold disabled:opacity-50"
            >
              {saving ? 'Creating…' : 'Create Note'}
            </button>
          )}
          {!isNew && (
            <>
              {dirty && (
                <button
                  onClick={handleSave}
                  disabled={saving}
                  className="px-4 py-2 rounded-lg bg-brand-gradient text-brand-bg text-sm font-semibold disabled:opacity-50"
                >
                  {saving ? 'Saving…' : 'Save'}
                </button>
              )}
              {confirmDel ? (
                <>
                  <span className="text-xs text-brand-muted flex-1">Delete this note permanently?</span>
                  <button onClick={() => setConfirmDel(false)} className="px-3 py-1.5 rounded border border-brand-border text-brand-muted text-xs hover:text-brand-text transition-colors">
                    Cancel
                  </button>
                  <button onClick={handleDelete} disabled={deleting} className="px-3 py-1.5 rounded border border-red-500/40 text-red-400 text-xs hover:bg-red-500/10 transition-colors disabled:opacity-50">
                    {deleting ? 'Deleting…' : 'Delete'}
                  </button>
                </>
              ) : (
                <button
                  onClick={() => setConfirmDel(true)}
                  className="px-3 py-1.5 rounded border border-brand-border text-brand-muted text-xs hover:text-red-400 hover:border-red-500/40 transition-colors ml-auto"
                >
                  Delete Note
                </button>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}

// ── helpers ───────────────────────────────────────────────────────────────────

function parseTags(raw: string): string[] {
  return raw
    .split(/[,\s]+/)
    .map((t) => t.trim().toLowerCase())
    .filter(Boolean)
}

// ── icons ─────────────────────────────────────────────────────────────────────

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

function LinkIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M6 10l4-4M4.5 11.5a2.5 2.5 0 01-3.5-3.5l3-3a2.5 2.5 0 013.5 0M11.5 4.5a2.5 2.5 0 013.5 3.5l-3 3a2.5 2.5 0 01-3.5 0" />
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
