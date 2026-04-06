// GitPanel — left-panel view for Chronicle (commit), Timelines (branch), and Lore (history).
// Mounted in Editor when leftPanel === 'git'.
import { useState, useEffect, useCallback } from 'react'
import { api } from '@/services/api'
import type { Timeline, ChronicleEntry, GitStatus } from '@/services/api'

interface GitPanelProps {
  token: string
  projectId: string
}

type View = 'timelines' | 'lore'

export default function GitPanel({ token, projectId }: GitPanelProps) {
  const [view, setView] = useState<View>('timelines')
  const [status, setStatus] = useState<GitStatus | null>(null)
  const [timelines, setTimelines] = useState<Timeline[]>([])
  const [lore, setLore] = useState<ChronicleEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showChronicle, setShowChronicle] = useState(false)
  const [showDiverge, setShowDiverge] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const loadAll = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [s, tl, lo] = await Promise.all([
        api.git.status(token, projectId),
        api.git.timelines(token, projectId),
        api.git.lore(token, projectId),
      ])
      setStatus(s)
      setTimelines(tl)
      setLore(lo)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load git state')
    } finally {
      setLoading(false)
    }
  }, [token, projectId])

  useEffect(() => { loadAll() }, [loadAll])

  const handleTravel = async (timelineName: string) => {
    setBusy(true)
    setActionError(null)
    try {
      await api.git.travel(token, projectId, timelineName)
      await loadAll()
    } catch (e: unknown) {
      setActionError(e instanceof Error ? e.message : 'Travel failed')
    } finally {
      setBusy(false)
    }
  }

  const handleCanonize = async (timelineName: string) => {
    setBusy(true)
    setActionError(null)
    try {
      await api.git.canonize(token, projectId, timelineName)
      await loadAll()
    } catch (e: unknown) {
      setActionError(e instanceof Error ? e.message : 'Canonize failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="w-72 flex flex-col border-r border-brand-border bg-brand-bg shrink-0 overflow-hidden">
      {/* Header */}
      <div className="px-4 pt-4 pb-2 shrink-0">
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-semibold text-brand-muted uppercase tracking-wider">Chronicle</span>
          <button
            onClick={() => setShowChronicle(true)}
            className="flex items-center gap-1 px-2 py-1 rounded text-xs bg-brand-cyan/10 text-brand-cyan hover:bg-brand-cyan/20 transition-colors"
          >
            <CommitIcon />
            Save
          </button>
        </div>

        {/* Active timeline badge */}
        {status && (
          <div className="flex items-center gap-1.5 text-xs text-brand-muted">
            <BranchIcon />
            <span className="text-brand-text font-medium">{status.current_timeline}</span>
            {status.dirty && <span className="text-brand-gold ml-auto">unsaved changes</span>}
          </div>
        )}
      </div>

      {/* Tab bar */}
      <div className="flex border-b border-brand-border shrink-0">
        {(['timelines', 'lore'] as View[]).map((v) => (
          <button
            key={v}
            onClick={() => setView(v)}
            className={`flex-1 py-1.5 text-xs font-medium transition-colors ${
              view === v
                ? 'text-brand-cyan border-b border-brand-cyan'
                : 'text-brand-muted hover:text-brand-text'
            }`}
          >
            {v === 'timelines' ? 'Timelines' : 'Lore'}
          </button>
        ))}
      </div>

      {/* Action error */}
      {actionError && (
        <div className="mx-3 mt-2 px-3 py-2 rounded bg-red-500/10 border border-red-500/30 text-red-400 text-xs shrink-0">
          {actionError}
        </div>
      )}

      {/* Content */}
      <div className="flex-1 overflow-y-auto min-h-0">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <SpinIcon className="w-4 h-4 text-brand-cyan animate-spin" />
          </div>
        ) : error ? (
          <div className="px-4 py-4 text-xs text-red-400">{error}</div>
        ) : view === 'timelines' ? (
          <TimelinesView
            timelines={timelines}
            busy={busy}
            onTravel={handleTravel}
            onCanonize={handleCanonize}
            onDiverge={() => setShowDiverge(true)}
          />
        ) : (
          <LoreView entries={lore} />
        )}
      </div>

      {/* Chronicle modal */}
      {showChronicle && (
        <ChronicleModal
          token={token}
          projectId={projectId}
          onDone={() => { setShowChronicle(false); loadAll() }}
          onClose={() => setShowChronicle(false)}
        />
      )}

      {/* Diverge modal */}
      {showDiverge && (
        <DivergeModal
          token={token}
          projectId={projectId}
          onDone={() => { setShowDiverge(false); loadAll() }}
          onClose={() => setShowDiverge(false)}
        />
      )}
    </div>
  )
}

// ── Timelines view ────────────────────────────────────────────────────────────

function TimelinesView({
  timelines,
  busy,
  onTravel,
  onCanonize,
  onDiverge,
}: {
  timelines: Timeline[]
  busy: boolean
  onTravel: (name: string) => void
  onCanonize: (name: string) => void
  onDiverge: () => void
}) {
  return (
    <div className="px-3 py-2 space-y-1.5">
      {timelines.map((tl) => (
        <div
          key={tl.name}
          className={`rounded-lg border p-3 ${
            tl.is_active
              ? 'border-brand-cyan/30 bg-brand-cyan/5'
              : 'border-brand-border bg-brand-bg-card'
          }`}
        >
          <div className="flex items-center gap-2 mb-1">
            <BranchIcon className={tl.is_active ? 'text-brand-cyan' : 'text-brand-muted'} />
            <span className={`text-sm font-medium flex-1 truncate ${tl.is_active ? 'text-brand-cyan' : 'text-brand-text'}`}>
              {tl.name}
            </span>
            {tl.is_canon && (
              <span className="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-brand-gold/20 text-brand-gold">canon</span>
            )}
            {tl.is_active && (
              <span className="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-brand-cyan/20 text-brand-cyan">active</span>
            )}
          </div>

          {tl.last_chronicle && (
            <p className="text-xs text-brand-muted truncate pl-5 mb-2">{tl.last_chronicle.note}</p>
          )}

          <div className="flex gap-1.5 pl-5">
            {!tl.is_active && (
              <button
                onClick={() => onTravel(tl.name)}
                disabled={busy}
                className="px-2 py-0.5 rounded text-xs border border-brand-border text-brand-muted hover:text-brand-text hover:border-brand-cyan/40 transition-colors disabled:opacity-40"
              >
                Travel
              </button>
            )}
            {!tl.is_canon && (
              <button
                onClick={() => onCanonize(tl.name)}
                disabled={busy}
                className="px-2 py-0.5 rounded text-xs border border-brand-border text-brand-muted hover:text-brand-gold hover:border-brand-gold/40 transition-colors disabled:opacity-40"
              >
                Canonize
              </button>
            )}
          </div>
        </div>
      ))}

      <button
        onClick={onDiverge}
        className="w-full mt-1 py-2 rounded-lg border border-dashed border-brand-border text-brand-muted hover:text-brand-text hover:border-brand-cyan/40 text-xs transition-colors flex items-center justify-center gap-1.5"
      >
        <PlusIcon />
        New Timeline
      </button>
    </div>
  )
}

// ── Lore view ─────────────────────────────────────────────────────────────────

function LoreView({ entries }: { entries: ChronicleEntry[] }) {
  if (entries.length === 0) {
    return <p className="px-4 py-6 text-xs text-brand-muted text-center">No chronicles yet. Save your first checkpoint.</p>
  }

  return (
    <div className="px-3 py-2 space-y-1">
      {entries.map((e) => (
        <div key={e.sha} className="py-2 border-b border-brand-border/50 last:border-0">
          <div className="flex items-center gap-2 mb-0.5">
            <span className="font-mono text-[10px] text-brand-purple bg-brand-purple/10 px-1.5 py-0.5 rounded">{e.short_sha}</span>
            <span className="text-[10px] text-brand-muted ml-auto">{formatDate(e.timestamp)}</span>
          </div>
          <p className="text-xs text-brand-text leading-relaxed">{e.note}</p>
        </div>
      ))}
    </div>
  )
}

// ── Chronicle modal ───────────────────────────────────────────────────────────

function ChronicleModal({
  token,
  projectId,
  onDone,
  onClose,
}: {
  token: string
  projectId: string
  onDone: () => void
  onClose: () => void
}) {
  const [note, setNote] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!note.trim()) return
    setSubmitting(true)
    setError(null)
    try {
      const res = await api.git.chronicle(token, projectId, note.trim())
      // Response is either ChronicleResponse (201) or NothingToChronicle (200)
      if ('sha' in res) {
        setResult(`Saved at ${res.short_sha}`)
      } else {
        setResult(res.message ?? 'Nothing changed since last chronicle.')
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Chronicle failed')
      setSubmitting(false)
    }
  }

  if (result) {
    return (
      <Backdrop>
        <ModalCard>
          <div className="text-center py-4">
            <p className="text-brand-cyan font-semibold mb-1">Chronicle saved</p>
            <p className="text-sm text-brand-muted">{result}</p>
          </div>
          <button onClick={onDone} className="w-full py-2 rounded-lg bg-brand-gradient text-brand-bg text-sm font-semibold mt-4">
            Done
          </button>
        </ModalCard>
      </Backdrop>
    )
  }

  return (
    <Backdrop>
      <ModalCard>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-base font-bold text-brand-text">Save Chronicle</h2>
          <CloseButton onClick={onClose} />
        </div>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-brand-muted uppercase tracking-wider mb-1.5">Note</label>
            <textarea
              autoFocus
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="Describe this checkpoint…"
              rows={3}
              className="input-field resize-none w-full"
            />
          </div>
          {error && <p className="text-red-400 text-xs">{error}</p>}
          <div className="flex gap-2">
            <button type="button" onClick={onClose} className="flex-1 py-2 rounded-lg border border-brand-border text-brand-muted text-sm hover:text-brand-text transition-colors">
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting || !note.trim()}
              className="flex-1 py-2 rounded-lg bg-brand-gradient text-brand-bg text-sm font-semibold disabled:opacity-50"
            >
              {submitting ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </ModalCard>
    </Backdrop>
  )
}

// ── Diverge modal ─────────────────────────────────────────────────────────────

function DivergeModal({
  token,
  projectId,
  onDone,
  onClose,
}: {
  token: string
  projectId: string
  onDone: () => void
  onClose: () => void
}) {
  const [name, setName] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const slug = name.trim().toLowerCase().replace(/\s+/g, '-')
    if (!slug) return
    setSubmitting(true)
    setError(null)
    try {
      await api.git.diverge(token, projectId, slug)
      onDone()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Diverge failed')
      setSubmitting(false)
    }
  }

  return (
    <Backdrop>
      <ModalCard>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-base font-bold text-brand-text">New Timeline</h2>
          <CloseButton onClick={onClose} />
        </div>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-brand-muted uppercase tracking-wider mb-1.5">Timeline name</label>
            <input
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. alternate-ending"
              className="input-field w-full"
            />
            <p className="text-[10px] text-brand-muted mt-1">Spaces become hyphens. Cannot be "canon".</p>
          </div>
          {error && <p className="text-red-400 text-xs">{error}</p>}
          <div className="flex gap-2">
            <button type="button" onClick={onClose} className="flex-1 py-2 rounded-lg border border-brand-border text-brand-muted text-sm hover:text-brand-text transition-colors">
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting || !name.trim()}
              className="flex-1 py-2 rounded-lg bg-brand-gradient text-brand-bg text-sm font-semibold disabled:opacity-50"
            >
              {submitting ? 'Creating…' : 'Diverge'}
            </button>
          </div>
        </form>
      </ModalCard>
    </Backdrop>
  )
}

// ── Shared modal primitives ───────────────────────────────────────────────────

function Backdrop({ children }: { children: React.ReactNode }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm px-4">
      {children}
    </div>
  )
}

function ModalCard({ children }: { children: React.ReactNode }) {
  return (
    <div className="bg-brand-bg-card border border-brand-border rounded-2xl p-6 w-full max-w-sm shadow-card">
      {children}
    </div>
  )
}

function CloseButton({ onClick }: { onClick: () => void }) {
  return (
    <button onClick={onClick} className="text-brand-muted hover:text-brand-text transition-colors">
      <svg className="w-4 h-4" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
        <path d="M3 3l10 10M13 3L3 13" />
      </svg>
    </button>
  )
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function formatDate(iso: string) {
  const d = new Date(iso)
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) + ' ' +
    d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
}

// ── Icons ─────────────────────────────────────────────────────────────────────

function BranchIcon({ className }: { className?: string }) {
  return (
    <svg className={`w-3.5 h-3.5 shrink-0 ${className ?? 'text-brand-muted'}`} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="4" cy="3" r="1.5" />
      <circle cx="4" cy="13" r="1.5" />
      <circle cx="12" cy="6" r="1.5" />
      <path d="M4 4.5v7M4 4.5C4 7 12 7.5 12 7.5" />
    </svg>
  )
}

function CommitIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <circle cx="8" cy="8" r="2.5" />
      <path d="M8 1v4.5M8 10.5V15M1 8h4.5M10.5 8H15" />
    </svg>
  )
}

function PlusIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
      <path d="M8 3v10M3 8h10" />
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
