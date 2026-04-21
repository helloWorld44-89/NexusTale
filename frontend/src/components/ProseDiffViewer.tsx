import { useState, useMemo, useEffect } from 'react'
import DiffMatchPatch from 'diff-match-patch'
import { api, type MergeRequest, type MRDiffResponse, type SceneDiff } from '@/services/api'

const dmp = new DiffMatchPatch()

// ── Types ─────────────────────────────────────────────────────────────────────

type Resolution = 'canon' | 'coauthor' | 'manual'
interface SceneResolution {
  type: Resolution
  manualContent?: string
}

interface Props {
  mr: MergeRequest
  token: string
  projectId: string
  onClose: () => void
  onMerged: (updated: MergeRequest) => void
}

// ── Diff parsing ──────────────────────────────────────────────────────────────

// Reconstructs before (canon) and after (coauthor) text from a unified diff.
function extractTexts(unifiedDiff: string): { canon: string; coauthor: string } {
  const canonLines: string[] = []
  const coauthorLines: string[] = []

  for (const line of unifiedDiff.split('\n')) {
    if (
      line.startsWith('--- ') || line.startsWith('+++ ') ||
      line.startsWith('@@ ') || line.startsWith('diff ') ||
      line.startsWith('index ') || line.startsWith('new file') ||
      line.startsWith('deleted file') || line.startsWith('\\ No newline')
    ) continue

    if (line.startsWith('-')) {
      canonLines.push(line.slice(1))
    } else if (line.startsWith('+')) {
      coauthorLines.push(line.slice(1))
    } else {
      const content = line.startsWith(' ') ? line.slice(1) : line
      canonLines.push(content)
      coauthorLines.push(content)
    }
  }

  return {
    canon: canonLines.join('\n').trim(),
    coauthor: coauthorLines.join('\n').trim(),
  }
}

// ── Main component ────────────────────────────────────────────────────────────

export default function ProseDiffViewer({ mr, token, projectId, onClose, onMerged }: Props) {
  const [diffData, setDiffData] = useState<MRDiffResponse | null>(null)
  const [loading, setLoading]   = useState(true)
  const [fetchError, setFetchError] = useState<string | null>(null)
  const [resolutions, setResolutions] = useState<Map<string, SceneResolution>>(new Map())
  const [merging, setMerging]   = useState(false)
  const [mergeError, setMergeError] = useState<string | null>(null)

  useEffect(() => {
    api.mergeRequests.getDiff(token, projectId, mr.id)
      .then(data => setDiffData(data))
      .catch(e => setFetchError(e instanceof Error ? e.message : 'Failed to load diff'))
      .finally(() => setLoading(false))
  }, [token, projectId, mr.id])

  const sceneDiffs  = diffData?.scene_diffs ?? []
  const totalScenes = sceneDiffs.length
  const resolvedCount = resolutions.size
  const allResolved = totalScenes === 0 || resolvedCount === totalScenes

  function setResolution(key: string, res: SceneResolution) {
    setResolutions(prev => new Map(prev).set(key, res))
  }

  function acceptAll() {
    const next = new Map<string, SceneResolution>()
    for (const sd of sceneDiffs) next.set(sdKey(sd), { type: 'coauthor' })
    setResolutions(next)
  }

  function keepAllCanon() {
    const next = new Map<string, SceneResolution>()
    for (const sd of sceneDiffs) next.set(sdKey(sd), { type: 'canon' })
    setResolutions(next)
  }

  async function handleMerge() {
    setMerging(true)
    setMergeError(null)
    try {
      const updated = await api.mergeRequests.resolve(token, projectId, mr.id, { action: 'merge' })
      onMerged(updated)
    } catch (e: unknown) {
      setMergeError(e instanceof Error ? e.message : 'Merge failed')
    } finally {
      setMerging(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-black/85 flex flex-col">
      {/* Header */}
      <div className="shrink-0 flex items-center justify-between px-6 py-4 bg-brand-bg-card border-b border-brand-border">
        <div>
          <h2 className="text-sm font-semibold text-brand-text">{mr.title}</h2>
          <p className="text-[10px] text-brand-muted mt-0.5">
            <span className="font-mono">{mr.from_branch}</span>
            {' → '}
            <span className="font-mono">{mr.to_branch}</span>
            {mr.requester_name && ` · ${mr.requester_name}`}
          </p>
        </div>
        <button
          onClick={onClose}
          className="p-1.5 rounded hover:bg-brand-border/40 text-brand-muted hover:text-brand-text transition-colors"
          aria-label="Close"
        >
          <XIcon />
        </button>
      </div>

      {/* Scrollable body */}
      <div className="flex-1 overflow-y-auto px-6 py-5 space-y-4 max-w-5xl mx-auto w-full">
        {loading && <p className="text-xs text-brand-muted py-8 text-center">Loading diff…</p>}
        {fetchError && <p className="text-xs text-red-400 py-8 text-center">{fetchError}</p>}

        {diffData && totalScenes === 0 && (
          <div className="text-center py-16">
            <p className="text-sm text-brand-text font-medium mb-1">No content changes</p>
            <p className="text-xs text-brand-muted">This is a clean fast-forward merge — no scene diffs to review.</p>
          </div>
        )}

        {diffData && totalScenes > 0 && (
          <>
            {/* Bulk actions + progress */}
            <div className="flex items-center justify-between">
              <div className="flex gap-2">
                <button
                  onClick={acceptAll}
                  className="px-3 py-1.5 rounded text-xs bg-green-500/10 text-green-400 hover:bg-green-500/20 transition-colors"
                >
                  Accept All Co-author
                </button>
                <button
                  onClick={keepAllCanon}
                  className="px-3 py-1.5 rounded text-xs bg-brand-border/40 text-brand-muted hover:bg-brand-border/60 transition-colors"
                >
                  Keep All Canon
                </button>
              </div>
              <span className="text-xs text-brand-muted">
                {resolvedCount} of {totalScenes} reviewed
              </span>
            </div>

            {/* Per-scene diff cards */}
            {sceneDiffs.map(sd => (
              <SceneDiffCard
                key={sdKey(sd)}
                sd={sd}
                resolution={resolutions.get(sdKey(sd))}
                onResolve={res => setResolution(sdKey(sd), res)}
              />
            ))}
          </>
        )}
      </div>

      {/* Footer */}
      <div className="shrink-0 flex items-center justify-between px-6 py-4 bg-brand-bg-card border-t border-brand-border">
        <div className="text-xs text-red-400">
          {mergeError || ''}
        </div>
        <div className="flex gap-3 items-center">
          <button
            onClick={onClose}
            className="px-4 py-2 rounded text-xs text-brand-muted hover:text-brand-text transition-colors"
          >
            Cancel
          </button>
          <button
            disabled={!allResolved || merging}
            onClick={handleMerge}
            className="px-4 py-2 rounded text-xs bg-brand-purple text-white font-semibold hover:bg-brand-purple/80 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          >
            {merging ? 'Merging…' : totalScenes === 0 ? 'Merge' : `Merge (${resolvedCount}/${totalScenes} reviewed)`}
          </button>
        </div>
      </div>
    </div>
  )
}

// ── Scene diff card ───────────────────────────────────────────────────────────

function SceneDiffCard({
  sd,
  resolution,
  onResolve,
}: {
  sd: SceneDiff
  resolution: SceneResolution | undefined
  onResolve: (res: SceneResolution) => void
}) {
  const [editing, setEditing]           = useState(false)
  const [manualContent, setManualContent] = useState('')

  const { canon, coauthor } = useMemo(() => extractTexts(sd.diff), [sd.diff])

  const label = sd.scene_id
    ? `scene · ${sd.scene_id.slice(0, 8)}…`
    : sd.git_path

  const isResolved = !!resolution
  const resLabel =
    resolution?.type === 'canon'    ? 'Keeping canon' :
    resolution?.type === 'coauthor' ? 'Using co-author' :
    resolution?.type === 'manual'   ? 'Custom edit saved' : null

  function startEdit() {
    setManualContent(coauthor)
    setEditing(true)
  }

  function saveEdit() {
    onResolve({ type: 'manual', manualContent })
    setEditing(false)
  }

  function cancelEdit() {
    setEditing(false)
  }

  return (
    <div className={`rounded-xl border overflow-hidden transition-colors ${
      isResolved ? 'border-brand-cyan/30' : 'border-brand-border'
    } bg-brand-bg-card`}>
      {/* Card header */}
      <div className="flex items-center justify-between px-4 py-2.5 bg-brand-bg/40 border-b border-brand-border/60">
        <div className="flex items-center gap-2">
          <SceneIcon />
          <span className="text-[10px] font-mono text-brand-muted">{label}</span>
          {sd.is_new && (
            <span className="text-[9px] font-bold uppercase tracking-wide px-1.5 py-0.5 rounded-full bg-green-400/10 text-green-400">
              new
            </span>
          )}
          {sd.is_deleted && (
            <span className="text-[9px] font-bold uppercase tracking-wide px-1.5 py-0.5 rounded-full bg-red-400/10 text-red-400">
              deleted
            </span>
          )}
        </div>
        {isResolved && (
          <span className="flex items-center gap-1 text-[10px] text-brand-cyan">
            <CheckIcon /> {resLabel}
          </span>
        )}
      </div>

      {/* Diff body */}
      <div className="p-4 max-h-96 overflow-y-auto">
        {sd.is_new ? (
          <pre className="text-sm text-green-300 bg-green-500/10 rounded p-3 whitespace-pre-wrap font-serif leading-relaxed">
            {coauthor || '(empty)'}
          </pre>
        ) : sd.is_deleted ? (
          <pre className="text-sm text-red-300 bg-red-500/10 rounded p-3 whitespace-pre-wrap font-serif leading-relaxed line-through decoration-red-400/50">
            {canon || '(empty)'}
          </pre>
        ) : editing ? (
          <textarea
            value={manualContent}
            onChange={e => setManualContent(e.target.value)}
            className="w-full h-56 bg-brand-bg border border-brand-cyan/40 rounded p-3 text-sm text-brand-text font-serif leading-relaxed resize-y focus:outline-none focus:border-brand-cyan"
          />
        ) : (
          <WordDiffView canon={canon} coauthor={coauthor} />
        )}
      </div>

      {/* Resolution buttons */}
      <div className="flex flex-wrap items-center gap-2 px-4 pb-3 pt-1">
        {sd.is_new ? (
          <>
            <ResBtn active={resolution?.type === 'coauthor'} color="green" onClick={() => onResolve({ type: 'coauthor' })}>
              ✓ Accept new scene
            </ResBtn>
            <ResBtn active={resolution?.type === 'canon'} color="red" onClick={() => onResolve({ type: 'canon' })}>
              ✗ Reject
            </ResBtn>
          </>
        ) : sd.is_deleted ? (
          <>
            <ResBtn active={resolution?.type === 'coauthor'} color="red" onClick={() => onResolve({ type: 'coauthor' })}>
              Accept deletion
            </ResBtn>
            <ResBtn active={resolution?.type === 'canon'} color="muted" onClick={() => onResolve({ type: 'canon' })}>
              Keep scene
            </ResBtn>
          </>
        ) : editing ? (
          <>
            <ResBtn active color="cyan" onClick={saveEdit}>Save edit ✓</ResBtn>
            <ResBtn active={false} color="muted" onClick={cancelEdit}>Cancel</ResBtn>
          </>
        ) : (
          <>
            <ResBtn active={resolution?.type === 'canon'} color="muted" onClick={() => onResolve({ type: 'canon' })}>
              ← Keep Canon
            </ResBtn>
            <ResBtn active={resolution?.type === 'coauthor'} color="green" onClick={() => onResolve({ type: 'coauthor' })}>
              Use Co-author →
            </ResBtn>
            <ResBtn active={resolution?.type === 'manual'} color="purple" onClick={startEdit}>
              Edit ✎
            </ResBtn>
          </>
        )}
      </div>
    </div>
  )
}

// ── Word-level diff renderer ───────────────────────────────────────────────────

function WordDiffView({ canon, coauthor }: { canon: string; coauthor: string }) {
  const diffs = useMemo(() => {
    const d = dmp.diff_main(canon, coauthor)
    dmp.diff_cleanupSemantic(d)
    return d
  }, [canon, coauthor])

  return (
    <div className="text-sm text-brand-text leading-relaxed font-serif whitespace-pre-wrap">
      {diffs.map(([op, text], i) => {
        if (op === 0)  return <span key={i}>{text}</span>
        if (op === -1) return <span key={i} className="bg-red-500/15 text-red-300 line-through decoration-red-400/50">{text}</span>
        return              <span key={i} className="bg-green-500/15 text-green-300">{text}</span>
      })}
    </div>
  )
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function sdKey(sd: SceneDiff) {
  return sd.scene_id || sd.git_path
}

function ResBtn({
  children, active, color, onClick,
}: {
  children: React.ReactNode
  active: boolean
  color: 'green' | 'red' | 'muted' | 'cyan' | 'purple'
  onClick: () => void
}) {
  const cls: Record<string, string> = {
    green:  active ? 'bg-green-500/20  text-green-400  ring-1 ring-green-400/40' : 'bg-green-500/8  text-green-400/70  hover:bg-green-500/15',
    red:    active ? 'bg-red-500/20    text-red-400    ring-1 ring-red-400/40'   : 'bg-red-500/8    text-red-400/70    hover:bg-red-500/15',
    muted:  active ? 'bg-brand-border/60 text-brand-text ring-1 ring-brand-border' : 'bg-brand-border/30 text-brand-muted hover:bg-brand-border/50',
    cyan:   active ? 'bg-brand-cyan/20 text-brand-cyan  ring-1 ring-brand-cyan/40' : 'bg-brand-cyan/8  text-brand-cyan/70  hover:bg-brand-cyan/15',
    purple: active ? 'bg-brand-purple/20 text-brand-purple ring-1 ring-brand-purple/40' : 'bg-brand-purple/8 text-brand-purple/70 hover:bg-brand-purple/15',
  }
  return (
    <button
      onClick={onClick}
      className={`px-3 py-1.5 rounded text-[10px] font-medium transition-colors ${cls[color]}`}
    >
      {children}
    </button>
  )
}

// ── Icons ─────────────────────────────────────────────────────────────────────

function XIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <path d="M3 3l10 10M13 3L3 13" />
    </svg>
  )
}

function CheckIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2.5 8.5l4 4 7-7" />
    </svg>
  )
}

function SceneIcon() {
  return (
    <svg className="w-3 h-3 text-brand-muted" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <rect x="2" y="3" width="12" height="10" rx="1.5" />
      <path d="M5 7h6M5 10h4" />
    </svg>
  )
}
