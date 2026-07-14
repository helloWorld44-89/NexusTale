import { useState, useMemo } from 'react'
import { api, type RenameCascadePreviewItem } from '@/services/api'
import { WordDiffView, ResBtn, extractTexts } from '@/components/shared/ProseDiff'

interface Props {
  token:      string
  projectId:  string
  entityId:   string
  oldName:    string
  newName:    string
  onDone:     (patchedCount: number) => void
  onClose:    () => void
}

const PAGE_SIZE = 20

export default function RenameCascadeModal({ token, projectId, entityId, oldName, newName, onDone, onClose }: Props) {
  const [items,       setItems]       = useState<RenameCascadePreviewItem[] | null>(null)
  const [loading,     setLoading]     = useState(true)
  const [fetchError,  setFetchError]  = useState<string | null>(null)
  const [resolutions, setResolutions] = useState<Map<string, 'apply' | 'skip'>>(new Map())
  const [applying,    setApplying]    = useState(false)
  const [applyError,  setApplyError]  = useState<string | null>(null)
  const [page,        setPage]        = useState(0)

  // Load preview on mount
  useState(() => {
    api.wiki.renameCascadePreview(token, projectId, entityId, oldName, newName)
      .then(data => { setItems(data); setLoading(false) })
      .catch(e => { setFetchError(e instanceof Error ? e.message : 'Preview failed'); setLoading(false) })
  })

  const totalScenes   = items?.length ?? 0
  const totalPages    = Math.max(1, Math.ceil(totalScenes / PAGE_SIZE))
  const pagedItems    = (items ?? []).slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE)
  const resolvedCount = resolutions.size
  const allResolved   = totalScenes === 0 || resolvedCount === totalScenes
  const applyCount    = [...resolutions.values()].filter(v => v === 'apply').length

  function resolve(sceneId: string, decision: 'apply' | 'skip') {
    setResolutions(prev => new Map(prev).set(sceneId, decision))
  }

  function acceptAll() {
    const m = new Map<string, 'apply' | 'skip'>()
    for (const item of items ?? []) m.set(item.scene_id, 'apply')
    setResolutions(m)
  }

  function skipAll() {
    const m = new Map<string, 'apply' | 'skip'>()
    for (const item of items ?? []) m.set(item.scene_id, 'skip')
    setResolutions(m)
  }

  async function handleApply() {
    setApplying(true)
    setApplyError(null)
    const approvedIds = [...resolutions.entries()]
      .filter(([, v]) => v === 'apply')
      .map(([id]) => id)
    try {
      const { patched_scenes } = await api.wiki.renameCascadeConfirm(
        token, projectId, entityId, oldName, newName, approvedIds,
      )
      onDone(patched_scenes)
    } catch (e) {
      setApplyError(e instanceof Error ? e.message : 'Apply failed')
    } finally {
      setApplying(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-black/85 flex flex-col">
      {/* Header */}
      <div className="shrink-0 flex items-center justify-between px-6 py-4 bg-brand-bg-card border-b border-brand-border">
        <div>
          <h2 className="text-sm font-semibold text-brand-text">
            Rename cascade: <span className="text-red-300">{oldName}</span> → <span className="text-green-300">{newName}</span>
          </h2>
          <p className="text-[10px] text-brand-muted mt-0.5">
            Review each scene and choose Apply or Skip. Only approved scenes will be updated.
          </p>
        </div>
        <button onClick={onClose} className="p-1.5 rounded hover:bg-brand-border/40 text-brand-muted hover:text-brand-text transition-colors">
          <XIcon />
        </button>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto px-6 py-5 space-y-4 max-w-5xl mx-auto w-full">
        {loading && <p className="text-xs text-brand-muted py-8 text-center">Loading preview…</p>}
        {fetchError && <p className="text-xs text-red-400 py-8 text-center">{fetchError}</p>}

        {items && totalScenes === 0 && (
          <div className="text-center py-16">
            <p className="text-sm text-brand-text font-medium mb-1">No occurrences found</p>
            <p className="text-xs text-brand-muted">The old name no longer appears in any scene.</p>
          </div>
        )}

        {items && totalScenes > 0 && (
          <>
            <div className="flex items-center justify-between">
              <div className="flex gap-2">
                <button onClick={acceptAll} className="px-3 py-1.5 rounded text-xs bg-green-500/10 text-green-400 hover:bg-green-500/20 transition-colors">
                  Apply All
                </button>
                <button onClick={skipAll} className="px-3 py-1.5 rounded text-xs bg-brand-border/40 text-brand-muted hover:bg-brand-border/60 transition-colors">
                  Skip All
                </button>
              </div>
              <span className="text-xs text-brand-muted">{resolvedCount} of {totalScenes} reviewed</span>
            </div>

            {pagedItems.map(item => (
              <RenameDiffCard
                key={item.scene_id}
                item={item}
                resolution={resolutions.get(item.scene_id)}
                onResolve={d => resolve(item.scene_id, d)}
              />
            ))}

            {totalPages > 1 && (
              <div className="flex items-center justify-center gap-3 pt-2">
                <button disabled={page === 0} onClick={() => setPage(p => p - 1)}
                  className="px-3 py-1.5 rounded text-xs bg-brand-border/40 text-brand-muted hover:bg-brand-border/60 disabled:opacity-40 transition-colors">
                  ← Prev
                </button>
                <span className="text-xs text-brand-muted">Page {page + 1} of {totalPages}</span>
                <button disabled={page >= totalPages - 1} onClick={() => setPage(p => p + 1)}
                  className="px-3 py-1.5 rounded text-xs bg-brand-border/40 text-brand-muted hover:bg-brand-border/60 disabled:opacity-40 transition-colors">
                  Next →
                </button>
              </div>
            )}
          </>
        )}
      </div>

      {/* Footer */}
      <div className="shrink-0 flex items-center justify-between px-6 py-4 bg-brand-bg-card border-t border-brand-border">
        <p className="text-xs text-red-400">{applyError || ''}</p>
        <div className="flex gap-3 items-center">
          <button onClick={onClose} className="px-4 py-2 rounded text-xs text-brand-muted hover:text-brand-text transition-colors">
            Cancel
          </button>
          <button
            disabled={!allResolved || applying || applyCount === 0}
            onClick={handleApply}
            className="px-4 py-2 rounded text-xs bg-brand-purple text-white font-semibold hover:bg-brand-purple/80 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          >
            {applying ? 'Applying…' : `Apply to ${applyCount} scene${applyCount !== 1 ? 's' : ''}`}
          </button>
        </div>
      </div>
    </div>
  )
}

function RenameDiffCard({
  item,
  resolution,
  onResolve,
}: {
  item: RenameCascadePreviewItem
  resolution: 'apply' | 'skip' | undefined
  onResolve: (d: 'apply' | 'skip') => void
}) {
  const { canon, coauthor } = useMemo(() => extractTexts(item.unified_diff), [item.unified_diff])

  return (
    <div className={`rounded-xl border overflow-hidden transition-colors ${resolution ? 'border-brand-cyan/30' : 'border-brand-border'} bg-brand-bg-card`}>
      <div className="flex items-center justify-between px-4 py-2.5 bg-brand-bg/40 border-b border-brand-border/60">
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-medium text-brand-text">{item.scene_title || 'Untitled scene'}</span>
          <span className="text-[10px] text-brand-muted">· {item.chapter_title}</span>
        </div>
        {resolution && (
          <span className={`text-[10px] font-medium ${resolution === 'apply' ? 'text-green-400' : 'text-brand-muted'}`}>
            {resolution === 'apply' ? '✓ Apply' : '– Skip'}
          </span>
        )}
      </div>
      <div className="p-4 max-h-80 overflow-y-auto">
        <WordDiffView canon={canon} coauthor={coauthor} />
      </div>
      <div className="flex gap-2 px-4 pb-3 pt-1">
        <ResBtn active={resolution === 'apply'} color="green" onClick={() => onResolve('apply')}>
          ✓ Apply
        </ResBtn>
        <ResBtn active={resolution === 'skip'} color="muted" onClick={() => onResolve('skip')}>
          Skip
        </ResBtn>
      </div>
    </div>
  )
}

function XIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <path d="M3 3l10 10M13 3L3 13" />
    </svg>
  )
}
