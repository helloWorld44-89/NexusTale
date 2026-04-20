import { useEffect, useState } from 'react'
import { api, type MergeRequest } from '@/services/api'

interface Props {
  projectId: string
  ownerId:   string
  currentUserId: string
  token:     string
}

export default function MergeRequestsPanel({ projectId, ownerId, currentUserId, token }: Props) {
  const [mrs, setMrs]       = useState<MergeRequest[]>([])
  const [loading, setLoading] = useState(true)
  const [openForm, setOpenForm] = useState(false)
  const [formBranch, setFormBranch] = useState('')
  const [formTitle, setFormTitle]   = useState('')
  const [formDesc, setFormDesc]     = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError]   = useState<string | null>(null)
  const [resolving, setResolving] = useState<string | null>(null)

  const isOwner = currentUserId === ownerId

  useEffect(() => {
    api.mergeRequests.list(token, projectId)
      .then(setMrs)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [token, projectId])

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      const mr = await api.mergeRequests.create(token, projectId, {
        from_branch: formBranch,
        title: formTitle,
        description: formDesc,
      })
      setMrs(prev => [mr, ...prev])
      setOpenForm(false)
      setFormBranch('')
      setFormTitle('')
      setFormDesc('')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to open merge request')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleResolve(mr: MergeRequest, action: 'approve' | 'reject' | 'merge') {
    setResolving(mr.id)
    try {
      const updated = await api.mergeRequests.resolve(token, projectId, mr.id, { action })
      setMrs(prev => prev.map(m => m.id === updated.id ? updated : m))
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : 'Action failed')
    } finally {
      setResolving(null)
    }
  }

  const openMrs    = mrs.filter(m => m.status === 'open' || m.status === 'approved')
  const closedMrs  = mrs.filter(m => m.status === 'rejected' || m.status === 'merged')

  return (
    <div className="bg-brand-bg-card border border-brand-border rounded-xl p-5">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-semibold text-brand-text flex items-center gap-2">
          <MRIcon />
          Merge Requests
          {openMrs.length > 0 && (
            <span className="px-1.5 py-0.5 rounded-full bg-brand-cyan/10 text-brand-cyan text-[10px] font-bold">
              {openMrs.length}
            </span>
          )}
        </h3>
        {!isOwner && !openForm && (
          <button
            onClick={() => setOpenForm(true)}
            className="text-xs px-3 py-1.5 rounded bg-brand-cyan/10 text-brand-cyan hover:bg-brand-cyan/20 transition-colors"
          >
            + Open MR
          </button>
        )}
      </div>

      {/* New MR form */}
      {openForm && (
        <form onSubmit={handleCreate} className="mb-4 p-3 bg-brand-bg border border-brand-border/60 rounded-lg space-y-2">
          <input
            value={formTitle}
            onChange={e => setFormTitle(e.target.value)}
            placeholder="Title"
            required
            className="w-full bg-transparent border border-brand-border rounded px-2 py-1.5 text-xs text-brand-text placeholder:text-brand-muted focus:outline-none focus:border-brand-cyan/60"
          />
          <input
            value={formBranch}
            onChange={e => setFormBranch(e.target.value)}
            placeholder="From branch (e.g. coauthor/alice)"
            required
            className="w-full bg-transparent border border-brand-border rounded px-2 py-1.5 text-xs text-brand-text placeholder:text-brand-muted focus:outline-none focus:border-brand-cyan/60"
          />
          <textarea
            value={formDesc}
            onChange={e => setFormDesc(e.target.value)}
            placeholder="Description (optional)"
            rows={2}
            className="w-full bg-transparent border border-brand-border rounded px-2 py-1.5 text-xs text-brand-text placeholder:text-brand-muted focus:outline-none focus:border-brand-cyan/60 resize-none"
          />
          {error && <p className="text-xs text-red-400">{error}</p>}
          <div className="flex gap-2">
            <button
              type="submit"
              disabled={submitting}
              className="px-3 py-1.5 rounded text-xs bg-brand-cyan text-brand-bg font-semibold hover:bg-brand-cyan/80 disabled:opacity-50 transition-colors"
            >
              {submitting ? 'Opening…' : 'Open Merge Request'}
            </button>
            <button
              type="button"
              onClick={() => { setOpenForm(false); setError(null) }}
              className="px-3 py-1.5 rounded text-xs text-brand-muted hover:text-brand-text transition-colors"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {loading ? (
        <p className="text-xs text-brand-muted py-2">Loading…</p>
      ) : mrs.length === 0 ? (
        <p className="text-xs text-brand-muted py-2">No merge requests yet.</p>
      ) : (
        <div className="space-y-2">
          {openMrs.map(mr => (
            <MRRow
              key={mr.id}
              mr={mr}
              isOwner={isOwner}
              resolving={resolving === mr.id}
              onResolve={handleResolve}
            />
          ))}
          {closedMrs.length > 0 && openMrs.length > 0 && (
            <div className="border-t border-brand-border/40 pt-2" />
          )}
          {closedMrs.map(mr => (
            <MRRow
              key={mr.id}
              mr={mr}
              isOwner={isOwner}
              resolving={false}
              onResolve={handleResolve}
            />
          ))}
        </div>
      )}
    </div>
  )
}

// ── MR Row ────────────────────────────────────────────────────────────────────

function MRRow({
  mr,
  isOwner,
  resolving,
  onResolve,
}: {
  mr: MergeRequest
  isOwner: boolean
  resolving: boolean
  onResolve: (mr: MergeRequest, action: 'approve' | 'reject' | 'merge') => void
}) {
  const statusColor =
    mr.status === 'open'     ? 'text-brand-cyan bg-brand-cyan/10' :
    mr.status === 'approved' ? 'text-green-400 bg-green-400/10' :
    mr.status === 'merged'   ? 'text-brand-purple bg-brand-purple/10' :
                               'text-brand-muted bg-brand-border/40'

  return (
    <div className="p-3 bg-brand-bg border border-brand-border/60 rounded-lg">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-xs font-medium text-brand-text truncate">{mr.title}</p>
          <p className="text-[10px] text-brand-muted mt-0.5">
            <span className="font-mono">{mr.from_branch}</span>
            {' → '}
            <span className="font-mono">{mr.to_branch}</span>
            {' · '}
            {mr.requester_name}
          </p>
          {mr.description && (
            <p className="text-[10px] text-brand-muted/70 mt-1 line-clamp-2">{mr.description}</p>
          )}
          {mr.reviewer_note && (
            <p className="text-[10px] text-brand-muted/70 mt-1 italic">Note: {mr.reviewer_note}</p>
          )}
        </div>
        <span className={`shrink-0 text-[9px] font-bold px-1.5 py-0.5 rounded-full uppercase tracking-wide ${statusColor}`}>
          {mr.status}
        </span>
      </div>

      {isOwner && (mr.status === 'open' || mr.status === 'approved') && (
        <div className="flex gap-1.5 mt-2.5">
          {mr.status === 'open' && (
            <button
              disabled={resolving}
              onClick={() => onResolve(mr, 'approve')}
              className="px-2 py-1 rounded text-[10px] bg-green-400/10 text-green-400 hover:bg-green-400/20 disabled:opacity-50 transition-colors"
            >
              Approve
            </button>
          )}
          <button
            disabled={resolving}
            onClick={() => onResolve(mr, 'merge')}
            className="px-2 py-1 rounded text-[10px] bg-brand-purple/10 text-brand-purple hover:bg-brand-purple/20 disabled:opacity-50 transition-colors"
          >
            {resolving ? 'Merging…' : 'Merge'}
          </button>
          {mr.status === 'open' && (
            <button
              disabled={resolving}
              onClick={() => onResolve(mr, 'reject')}
              className="px-2 py-1 rounded text-[10px] bg-red-400/10 text-red-400 hover:bg-red-400/20 disabled:opacity-50 transition-colors"
            >
              Reject
            </button>
          )}
        </div>
      )}
    </div>
  )
}

// ── Icon ──────────────────────────────────────────────────────────────────────

function MRIcon() {
  return (
    <svg className="w-3.5 h-3.5 text-brand-muted" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="4" cy="4" r="1.5" />
      <circle cx="12" cy="12" r="1.5" />
      <path d="M4 5.5v5a3 3 0 003 3h1.5" />
      <path d="M10 9l2 2-2 2" />
    </svg>
  )
}
