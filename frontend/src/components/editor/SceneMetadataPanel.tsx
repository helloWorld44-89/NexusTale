// SceneMetadataPanel — collapsible drawer showing POV, tense, tags, summary,
// and writing style selector for the active scene.
import { useState, useEffect, useRef } from 'react'
import { api } from '@/services/api'
import type { Scene, PromptResponse, PortableStyle, SceneAttributes } from '@/services/api'

interface SceneMetadataPanelProps {
  token:           string
  chapterId:       string
  projectId:       string
  scene:           Scene
  selectedPromptId: string | null
  onUpdate:        (updated: Scene) => void
  onPromptChange:  (promptId: string | null) => void
}

const SAVE_DELAY_MS = 800

export default function SceneMetadataPanel({
  token,
  chapterId,
  projectId,
  scene,
  selectedPromptId,
  onUpdate,
  onPromptChange,
}: SceneMetadataPanelProps) {
  const [open, setOpen]         = useState(false)
  const [pov, setPov]           = useState(scene.pov ?? '')
  const [tense, setTense]       = useState(scene.tense ?? '')
  const [tags, setTags]         = useState((scene.tags ?? []).join(', '))
  const [summary, setSummary]   = useState(scene.summary ?? '')
  const [structureOpen, setStructureOpen] = useState(false)
  const [sceneAttrs, setSceneAttrs] = useState<SceneAttributes>(() => extractSceneAttrs(scene))
  const [prompts, setPrompts]         = useState<PromptResponse[]>([])
  const [importStatus, setImportStatus] = useState<string | null>(null)
  const saveTimer                       = useRef<ReturnType<typeof setTimeout> | null>(null)
  const importInputRef                  = useRef<HTMLInputElement | null>(null)

  // Sync fields when the active scene changes.
  useEffect(() => {
    setPov(scene.pov ?? '')
    setTense(scene.tense ?? '')
    setTags((scene.tags ?? []).join(', '))
    setSummary(scene.summary ?? '')
    setSceneAttrs(extractSceneAttrs(scene))
  }, [scene.id])

  // Load writing style presets when the panel opens.
  useEffect(() => {
    if (!open) return
    api.prompts.list(token, projectId)
      .then(setPrompts)
      .catch(() => {})
  }, [open, token, projectId])

  const saveAttrs = (attrs: SceneAttributes) => {
    if (saveTimer.current) clearTimeout(saveTimer.current)
    saveTimer.current = setTimeout(async () => {
      try {
        const updated = await api.scenes.update(token, chapterId, scene.id, { attributes: attrs })
        onUpdate(updated)
      } catch {}
    }, SAVE_DELAY_MS)
  }

  const handleRoleChange = (role: SceneAttributes['scene_role']) => {
    const next = { ...sceneAttrs, scene_role: sceneAttrs.scene_role === role ? undefined : role }
    setSceneAttrs(next)
    saveAttrs(next)
  }

  const handleAttrBlur = (key: keyof Omit<SceneAttributes, 'scene_role'>, value: string) => {
    const next = { ...sceneAttrs, [key]: value || undefined }
    setSceneAttrs(next)
    saveAttrs(next)
  }

  const scheduleSave = (patch: Parameters<typeof api.scenes.update>[3]) => {
    if (saveTimer.current) clearTimeout(saveTimer.current)
    saveTimer.current = setTimeout(async () => {
      try {
        const updated = await api.scenes.update(token, chapterId, scene.id, patch)
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

  const handleExport = async () => {
    try {
      const data = await api.prompts.export(token, projectId)
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
      const url  = URL.createObjectURL(blob)
      const a    = document.createElement('a')
      a.href     = url
      a.download = `nexustale-styles.json`
      a.click()
      URL.revokeObjectURL(url)
    } catch {
      setImportStatus('Export failed')
    }
  }

  const handleImportFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    e.target.value = '' // allow re-import of same file
    try {
      const text = await file.text()
      const json = JSON.parse(text) as { styles?: PortableStyle[] }
      const styles = json.styles
      if (!Array.isArray(styles) || styles.length === 0) {
        setImportStatus('No styles found in file')
        return
      }
      const result = await api.prompts.import(token, projectId, styles)
      setImportStatus(`Imported ${result.imported}${result.skipped ? `, ${result.skipped} skipped` : ''}`)
      // Reload the prompts list so new styles appear immediately.
      const updated = await api.prompts.list(token, projectId)
      setPrompts(updated)
    } catch {
      setImportStatus('Import failed — invalid file')
    }
    setTimeout(() => setImportStatus(null), 4000)
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
          {sceneAttrs.scene_role && (
            <span className={`px-1.5 py-0.5 rounded text-[10px] font-semibold ${ROLE_STYLES[sceneAttrs.scene_role]}`}>
              {sceneAttrs.scene_role}
            </span>
          )}
          {selectedPromptId && prompts.length > 0 && (
            <span className="text-brand-green text-[10px] px-1.5 py-0.5 rounded border border-brand-green/40">
              {prompts.find((p) => p.id === selectedPromptId)?.name ?? 'Style'}
            </span>
          )}
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

          {/* Writing style — full width */}
          <div className="col-span-2 flex flex-col gap-1">
            <div className="flex items-center justify-between">
              <label className="text-brand-text-muted uppercase tracking-wider">Writing Style</label>
              <div className="flex items-center gap-2">
                <button
                  onClick={handleExport}
                  title="Export styles as JSON"
                  className="text-brand-muted hover:text-brand-text transition-colors text-[10px]"
                >
                  ↓ Export
                </button>
                <button
                  onClick={() => importInputRef.current?.click()}
                  title="Import styles from JSON"
                  className="text-brand-muted hover:text-brand-text transition-colors text-[10px]"
                >
                  ↑ Import
                </button>
                <input
                  ref={importInputRef}
                  type="file"
                  accept=".json,application/json"
                  className="hidden"
                  onChange={handleImportFile}
                />
              </div>
            </div>
            {importStatus && (
              <p className="text-[10px] text-brand-cyan">{importStatus}</p>
            )}
            <select
              value={selectedPromptId ?? ''}
              onChange={(e) => onPromptChange(e.target.value || null)}
              className="bg-brand-bg border border-brand-border rounded px-2 py-1 text-brand-text focus:outline-none focus:border-brand-cyan"
            >
              <option value="">Default (NexusTale)</option>
              {prompts.map((p) => (
                <option key={p.id} value={p.id}>{p.name}</option>
              ))}
            </select>
            {prompts.length === 0 && (
              <p className="text-brand-text-muted/60 text-[10px]">
                No styles yet — create one or import a styles file.
              </p>
            )}
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

          {/* Scene Structure — collapsible */}
          <div className="col-span-2 border border-brand-border rounded overflow-hidden">
            <button
              type="button"
              onClick={() => setStructureOpen((v) => !v)}
              className="w-full flex items-center justify-between px-3 py-1.5 text-left hover:bg-brand-border/20 transition-colors"
            >
              <div className="flex items-center gap-2">
                <span className="text-brand-text-muted uppercase tracking-wider">Scene Structure</span>
                {sceneAttrs.scene_role && (
                  <span className={`px-1.5 py-0.5 rounded text-[10px] font-semibold ${ROLE_STYLES[sceneAttrs.scene_role]}`}>
                    {sceneAttrs.scene_role}
                  </span>
                )}
              </div>
              <ChevronIcon open={structureOpen} />
            </button>
            {structureOpen && (
              <div className="px-3 pb-3 border-t border-brand-border space-y-3 pt-3">
                {/* Role selector */}
                <div className="flex flex-col gap-1">
                  <label className="text-brand-text-muted uppercase tracking-wider">Structural Role</label>
                  <div className="grid grid-cols-4 gap-1">
                    {SCENE_ROLES.map((r) => (
                      <button
                        key={r.value}
                        type="button"
                        onClick={() => handleRoleChange(r.value)}
                        title={r.hint}
                        className={`py-1 rounded text-[10px] font-medium border transition-colors ${
                          sceneAttrs.scene_role === r.value
                            ? ROLE_STYLES[r.value]
                            : 'border-brand-border text-brand-muted hover:text-brand-text'
                        }`}
                      >
                        {r.label}
                      </button>
                    ))}
                  </div>
                </div>
                {/* Goal / Conflict / Outcome */}
                {SCENE_ATTR_FIELDS.map(({ key, label, placeholder }) => (
                  <div key={key} className="flex flex-col gap-1">
                    <label className="text-brand-text-muted uppercase tracking-wider">{label}</label>
                    <textarea
                      rows={2}
                      defaultValue={sceneAttrs[key] ?? ''}
                      onBlur={(e) => handleAttrBlur(key, e.target.value)}
                      placeholder={placeholder}
                      className="bg-brand-bg border border-brand-border rounded px-2 py-1 text-brand-text placeholder:text-brand-text-muted/50 focus:outline-none focus:border-brand-cyan resize-none"
                    />
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

// ── Scene structure helpers ───────────────────────────────────────────────────

type SceneRole = NonNullable<SceneAttributes['scene_role']>

const SCENE_ROLES: { value: SceneRole; label: string; hint: string }[] = [
  { value: 'setup',       label: 'Setup',       hint: 'Establishes situation, character, or stakes' },
  { value: 'development', label: 'Development', hint: 'Advances the story — conflict deepens or changes' },
  { value: 'resolution',  label: 'Resolution',  hint: 'A thread closes or a question is answered' },
  { value: 'transition',  label: 'Transition',  hint: 'Moves time, place, or perspective' },
]

const ROLE_STYLES: Record<SceneRole, string> = {
  setup:       'bg-sky-500/20 text-sky-400 border border-sky-500/30',
  development: 'bg-amber-500/20 text-amber-400 border border-amber-500/30',
  resolution:  'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30',
  transition:  'bg-brand-muted/20 text-brand-muted border border-brand-muted/30',
}

const SCENE_ATTR_FIELDS: { key: keyof Omit<SceneAttributes, 'scene_role'>; label: string; placeholder: string }[] = [
  { key: 'scene_goal',     label: 'Scene Goal',     placeholder: 'What is the POV character trying to achieve?' },
  { key: 'scene_conflict', label: 'Conflict',       placeholder: 'What is in the way? (internal, external, or both)' },
  { key: 'scene_outcome',  label: 'Outcome',        placeholder: 'What actually happens? (fill after drafting)' },
]

function extractSceneAttrs(scene: Scene): SceneAttributes {
  const a = (scene as Scene & { attributes?: SceneAttributes }).attributes
  return {
    scene_role:     a?.scene_role,
    scene_goal:     a?.scene_goal     ?? '',
    scene_conflict: a?.scene_conflict ?? '',
    scene_outcome:  a?.scene_outcome  ?? '',
  }
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
