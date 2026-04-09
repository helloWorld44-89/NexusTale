// SceneMetadataPanel — collapsible drawer showing POV, tense, tags, and summary
// for the active scene. Edits are saved via debounced PATCH on blur/change.
import { useState, useEffect, useRef } from 'react'
import { api } from '@/services/api'
import type { Scene } from '@/services/api'

interface SceneMetadataPanelProps {
  token: string
  projectId: string
  chapterId: string
  scene: Scene
  onUpdate: (updated: Scene) => void
}

const SAVE_DELAY_MS = 800

export default function SceneMetadataPanel({
  token,
  projectId,
  chapterId,
  scene,
  onUpdate,
}: SceneMetadataPanelProps) {
  const [open, setOpen]       = useState(false)
  const [pov, setPov]         = useState(scene.pov ?? '')
  const [tense, setTense]     = useState(scene.tense ?? '')
  const [tags, setTags]       = useState((scene.tags ?? []).join(', '))
  const [summary, setSummary] = useState(scene.summary ?? '')
  const saveTimer             = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Sync when the active scene changes.
  useEffect(() => {
    setPov(scene.pov ?? '')
    setTense(scene.tense ?? '')
    setTags((scene.tags ?? []).join(', '))
    setSummary(scene.summary ?? '')
  }, [scene.id])

  const scheduleSave = (patch: Parameters<typeof api.scenes.update>[4]) => {
    if (saveTimer.current) clearTimeout(saveTimer.current)
    saveTimer.current = setTimeout(async () => {
      try {
        const updated = await api.scenes.update(token, projectId, chapterId, scene.id, patch)
        onUpdate(updated)
      } catch {}
    }, SAVE_DELAY_MS)
  }

  const handlePovChange = (v: string) => {
    setPov(v)
    scheduleSave({ pov: v })
  }

  const handleTenseChange = (v: string) => {
    setTense(v)
    scheduleSave({ tense: v })
  }

  const handleTagsBlur = () => {
    const parsed = tags.split(',').map((t) => t.trim()).filter(Boolean)
    scheduleSave({ tags: parsed })
  }

  const handleSummaryBlur = () => {
    scheduleSave({ summary, summary_stale: false })
  }

  const wordCount = scene.word_count ?? 0

  return (
    <div className="border-t border-brand-border bg-brand-bg-card shrink-0">
      {/* Header row — always visible */}
      <button
        onClick={() => setOpen((v) => !v)}
        className="w-full flex items-center justify-between px-4 py-1.5 text-xs text-brand-text-muted hover:text-brand-text transition-colors"
      >
        <div className="flex items-center gap-3">
          <span className="font-medium uppercase tracking-wider">Scene Details</span>
          {wordCount > 0 && (
            <span className="text-brand-cyan">{wordCount.toLocaleString()} words</span>
          )}
          {pov && <span className="text-brand-gold">POV: {pov}</span>}
          {tense && <span className="text-brand-purple">{tense}</span>}
        </div>
        <ChevronIcon open={open} />
      </button>

      {/* Expandable form */}
      {open && (
        <div className="px-4 pb-4 grid grid-cols-2 gap-3 text-xs">
          {/* POV */}
          <div className="flex flex-col gap-1">
            <label className="text-brand-text-muted uppercase tracking-wider">POV Character</label>
            <input
              type="text"
              value={pov}
              onChange={(e) => handlePovChange(e.target.value)}
              placeholder="e.g. Kira Solenne"
              className="bg-brand-bg border border-brand-border rounded px-2 py-1 text-brand-text placeholder:text-brand-text-muted/50 focus:outline-none focus:border-brand-cyan"
            />
          </div>

          {/* Tense */}
          <div className="flex flex-col gap-1">
            <label className="text-brand-text-muted uppercase tracking-wider">Tense</label>
            <select
              value={tense}
              onChange={(e) => handleTenseChange(e.target.value)}
              className="bg-brand-bg border border-brand-border rounded px-2 py-1 text-brand-text focus:outline-none focus:border-brand-cyan"
            >
              <option value="">—</option>
              <option value="past">Past</option>
              <option value="present">Present</option>
              <option value="future">Future</option>
            </select>
          </div>

          {/* Tags — full width */}
          <div className="col-span-2 flex flex-col gap-1">
            <label className="text-brand-text-muted uppercase tracking-wider">Tags (comma-separated)</label>
            <input
              type="text"
              value={tags}
              onChange={(e) => setTags(e.target.value)}
              onBlur={handleTagsBlur}
              placeholder="e.g. action, revelation, chapter-opener"
              className="bg-brand-bg border border-brand-border rounded px-2 py-1 text-brand-text placeholder:text-brand-text-muted/50 focus:outline-none focus:border-brand-cyan"
            />
          </div>

          {/* Summary — full width */}
          <div className="col-span-2 flex flex-col gap-1">
            <label className="text-brand-text-muted uppercase tracking-wider">Scene Summary</label>
            <textarea
              rows={3}
              value={summary}
              onChange={(e) => setSummary(e.target.value)}
              onBlur={handleSummaryBlur}
              placeholder="One-paragraph summary for your own reference…"
              className="bg-brand-bg border border-brand-border rounded px-2 py-1 text-brand-text placeholder:text-brand-text-muted/50 focus:outline-none focus:border-brand-cyan resize-none"
            />
          </div>
        </div>
      )}
    </div>
  )
}

function ChevronIcon({ open }: { open: boolean }) {
  return (
    <svg
      className={`w-3 h-3 transition-transform ${open ? 'rotate-180' : ''}`}
      viewBox="0 0 12 12"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
    >
      <path d="M2 4l4 4 4-4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}
