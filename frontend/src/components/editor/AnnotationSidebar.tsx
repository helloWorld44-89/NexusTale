// AnnotationSidebar — lists all annotations for the active scene.
// Clicking an annotation focuses and selects the annotated range in the textarea.
import { useState, useEffect, useCallback } from 'react'
import { api, type Annotation } from '@/services/api'

interface Props {
  token:        string
  projectId:    string
  sceneId:      string | null
  currentUserId: string
  ownerId:      string
  onJump:       (start: number, end: number) => void
  // Called when a new annotation is created via the selection popover in the editor
  newAnnotation?: Annotation | null
  onAnnotationConsumed?: () => void
}

const TYPE_COLORS: Record<string, string> = {
  note:       'text-yellow-400 bg-yellow-400/10 ring-yellow-400/30',
  suggestion: 'text-blue-400  bg-blue-400/10  ring-blue-400/30',
  question:   'text-purple-400 bg-purple-400/10 ring-purple-400/30',
}

const TYPE_LABEL: Record<string, string> = {
  note: 'Note', suggestion: 'Suggestion', question: 'Question',
}

export default function AnnotationSidebar({
  token, projectId, sceneId, currentUserId, ownerId, onJump,
  newAnnotation, onAnnotationConsumed,
}: Props) {
  const [annotations, setAnnotations] = useState<Annotation[]>([])
  const [loading, setLoading]         = useState(false)

  const isOwner = currentUserId === ownerId

  const load = useCallback(() => {
    if (!sceneId) { setAnnotations([]); return }
    setLoading(true)
    api.annotations.list(token, projectId, sceneId)
      .then(setAnnotations)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [token, projectId, sceneId])

  useEffect(() => { load() }, [load])

  // Merge in a newly created annotation from the editor popover.
  useEffect(() => {
    if (!newAnnotation) return
    setAnnotations(prev => [...prev, newAnnotation].sort((a, b) => a.start_char - b.start_char))
    onAnnotationConsumed?.()
  }, [newAnnotation, onAnnotationConsumed])

  async function handleResolve(ann: Annotation) {
    if (!sceneId) return
    try {
      const updated = await api.annotations.update(token, projectId, sceneId, ann.id, { resolved: true })
      setAnnotations(prev => prev.map(a => a.id === updated.id ? { ...a, resolved: true } : a))
    } catch { /* ignore */ }
  }

  async function handleDelete(ann: Annotation) {
    if (!sceneId) return
    try {
      await api.annotations.delete(token, projectId, sceneId, ann.id)
      setAnnotations(prev => prev.filter(a => a.id !== ann.id))
    } catch { /* ignore */ }
  }

  const open     = annotations.filter(a => !a.resolved)
  const resolved = annotations.filter(a => a.resolved)

  return (
    <div className="w-72 shrink-0 flex flex-col border-r border-brand-border bg-brand-bg overflow-hidden">
      <div className="px-4 py-3 border-b border-brand-border flex items-center justify-between">
        <span className="text-xs font-semibold text-brand-text flex items-center gap-2">
          <AnnotationIcon />
          Annotations
          {open.length > 0 && (
            <span className="px-1.5 py-0.5 rounded-full bg-yellow-400/10 text-yellow-400 text-[9px] font-bold">
              {open.length}
            </span>
          )}
        </span>
      </div>

      <div className="flex-1 overflow-y-auto">
        {!sceneId && (
          <p className="text-xs text-brand-muted px-4 py-6 text-center">Select a scene to see annotations.</p>
        )}
        {sceneId && loading && (
          <p className="text-xs text-brand-muted px-4 py-6 text-center">Loading…</p>
        )}
        {sceneId && !loading && annotations.length === 0 && (
          <div className="px-4 py-6 text-center space-y-1">
            <p className="text-xs text-brand-muted">No annotations yet.</p>
            <p className="text-[10px] text-brand-muted/60">Select text in the editor to add one.</p>
          </div>
        )}

        {open.length > 0 && (
          <div className="px-3 pt-3 space-y-2">
            {open.map(ann => (
              <AnnotationCard
                key={ann.id}
                ann={ann}
                isOwner={isOwner}
                currentUserId={currentUserId}
                onJump={onJump}
                onResolve={handleResolve}
                onDelete={handleDelete}
              />
            ))}
          </div>
        )}

        {resolved.length > 0 && (
          <details className="px-3 pt-3 pb-2">
            <summary className="text-[10px] text-brand-muted cursor-pointer select-none mb-2">
              {resolved.length} resolved
            </summary>
            <div className="space-y-2 opacity-60">
              {resolved.map(ann => (
                <AnnotationCard
                  key={ann.id}
                  ann={ann}
                  isOwner={isOwner}
                  currentUserId={currentUserId}
                  onJump={onJump}
                  onResolve={handleResolve}
                  onDelete={handleDelete}
                />
              ))}
            </div>
          </details>
        )}
      </div>
    </div>
  )
}

// ── Annotation card ───────────────────────────────────────────────────────────

function AnnotationCard({
  ann, isOwner, currentUserId, onJump, onResolve, onDelete,
}: {
  ann: Annotation
  isOwner: boolean
  currentUserId: string
  onJump: (start: number, end: number) => void
  onResolve: (ann: Annotation) => void
  onDelete: (ann: Annotation) => void
}) {
  const [editing, setEditing] = useState(false)

  const colorClass = TYPE_COLORS[ann.type] ?? TYPE_COLORS.note
  const canDelete  = isOwner || ann.author_id === currentUserId

  return (
    <div
      className={`rounded-lg border p-3 cursor-pointer transition-colors ${
        ann.resolved
          ? 'border-brand-border/40 bg-brand-bg/30'
          : 'border-brand-border bg-brand-bg-card hover:border-brand-border/80'
      }`}
      onClick={() => onJump(ann.start_char, ann.end_char)}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-1.5">
        <span className={`text-[9px] font-bold uppercase tracking-wide px-1.5 py-0.5 rounded-full ring-1 ${colorClass}`}>
          {TYPE_LABEL[ann.type]}
        </span>
        <span className="text-[9px] text-brand-muted">{ann.author_name}</span>
      </div>

      {/* Body */}
      {editing ? (
        <EditableBody
          initial={ann.body}
          onSave={async (_body) => {
            setEditing(false)
          }}
          onCancel={() => setEditing(false)}
        />
      ) : (
        <p className="text-[11px] text-brand-text/90 leading-relaxed line-clamp-4">{ann.body}</p>
      )}

      {/* Char range */}
      <p className="text-[9px] text-brand-muted/60 mt-1.5 font-mono">
        chars {ann.start_char}–{ann.end_char}
      </p>

      {/* Actions */}
      {!ann.resolved && (
        <div className="flex gap-1.5 mt-2" onClick={e => e.stopPropagation()}>
          {isOwner && (
            <button
              onClick={() => onResolve(ann)}
              className="px-2 py-0.5 rounded text-[9px] bg-green-500/10 text-green-400 hover:bg-green-500/20 transition-colors"
            >
              ✓ Resolve
            </button>
          )}
          {canDelete && (
            <button
              onClick={() => onDelete(ann)}
              className="px-2 py-0.5 rounded text-[9px] bg-red-500/10 text-red-400 hover:bg-red-500/20 transition-colors"
            >
              Delete
            </button>
          )}
        </div>
      )}
    </div>
  )
}

// ── Inline edit (stub — future enhancement) ───────────────────────────────────

function EditableBody({
  initial, onSave, onCancel,
}: { initial: string; onSave: (v: string) => void; onCancel: () => void }) {
  const [val, setVal] = useState(initial)
  return (
    <div onClick={e => e.stopPropagation()}>
      <textarea
        value={val}
        onChange={e => setVal(e.target.value)}
        rows={3}
        className="w-full bg-brand-bg border border-brand-border rounded px-2 py-1 text-xs text-brand-text resize-none focus:outline-none focus:border-brand-cyan/60"
      />
      <div className="flex gap-1.5 mt-1">
        <button onClick={() => onSave(val)} className="text-[9px] px-2 py-0.5 rounded bg-brand-cyan/10 text-brand-cyan">Save</button>
        <button onClick={onCancel} className="text-[9px] px-2 py-0.5 rounded text-brand-muted">Cancel</button>
      </div>
    </div>
  )
}

// ── Icon ──────────────────────────────────────────────────────────────────────

function AnnotationIcon() {
  return (
    <svg className="w-3.5 h-3.5 text-brand-muted" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2 4h12M2 8h8M2 12h5" />
      <circle cx="13" cy="11" r="2.5" fill="currentColor" stroke="none" className="text-yellow-400" />
    </svg>
  )
}
