import { useState, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'
import { api } from '@/services/api'
import type { ImportPreviewTree, ImportPreviewChapter, ImportPreviewScene } from '@/services/api'

export default function ImportManuscript() {
  const accessToken = useAuthStore((s) => s.accessToken)
  const navigate    = useNavigate()

  const [step,        setStep]        = useState<'upload' | 'preview' | 'confirming'>('upload')
  const [dragging,    setDragging]    = useState(false)
  const [parsing,     setParsing]     = useState(false)
  const [error,       setError]       = useState<string | null>(null)
  const [tree,        setTree]        = useState<ImportPreviewTree | null>(null)
  const [format,      setFormat]      = useState('')
  const [confirming,  setConfirming]  = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  const accepted = '.md,.markdown,.txt,.docx'

  // ── file handling ────────────────────────────────────────────────────────────

  async function handleFile(file: File) {
    if (!accessToken) return
    setParsing(true)
    setError(null)
    try {
      const result = await api.import.preview(accessToken, file)
      setTree(result.tree)
      setFormat(result.format)
      setStep('preview')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Parse failed')
    } finally {
      setParsing(false)
    }
  }

  function onFileInput(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (file) handleFile(file)
  }

  function onDrop(e: React.DragEvent) {
    e.preventDefault()
    setDragging(false)
    const file = e.dataTransfer.files[0]
    if (file) handleFile(file)
  }

  // ── tree editing ─────────────────────────────────────────────────────────────

  function setProjectTitle(t: string) {
    setTree(prev => prev ? { ...prev, project_title: t } : prev)
  }

  function setChapterTitle(ci: number, t: string) {
    setTree(prev => {
      if (!prev) return prev
      const chapters = [...prev.chapters]
      chapters[ci] = { ...chapters[ci], title: t }
      return { ...prev, chapters }
    })
  }

  function setSceneTitle(ci: number, si: number, t: string) {
    setTree(prev => {
      if (!prev) return prev
      const chapters = [...prev.chapters]
      const scenes = [...chapters[ci].scenes]
      scenes[si] = { ...scenes[si], title: t }
      chapters[ci] = { ...chapters[ci], scenes }
      return { ...prev, chapters }
    })
  }

  function mergeSceneDown(ci: number, si: number) {
    setTree(prev => {
      if (!prev) return prev
      const chapters = [...prev.chapters]
      const scenes = [...chapters[ci].scenes]
      if (si >= scenes.length - 1) return prev
      const merged: ImportPreviewScene = {
        title:   scenes[si].title,
        content: scenes[si].content + '\n\n' + scenes[si + 1].content,
      }
      scenes.splice(si, 2, merged)
      chapters[ci] = { ...chapters[ci], scenes }
      return { ...prev, chapters }
    })
  }

  function removeScene(ci: number, si: number) {
    setTree(prev => {
      if (!prev) return prev
      const chapters = [...prev.chapters]
      const scenes = chapters[ci].scenes.filter((_, i) => i !== si)
      chapters[ci] = { ...chapters[ci], scenes }
      return { ...prev, chapters }
    })
  }

  // ── confirm ──────────────────────────────────────────────────────────────────

  async function handleConfirm() {
    if (!accessToken || !tree) return
    setConfirming(true)
    setError(null)
    try {
      const result = await api.import.confirm(accessToken, tree)
      navigate(`/projects/${result.project_id}`)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Import failed')
      setConfirming(false)
    }
  }

  // ── render ───────────────────────────────────────────────────────────────────

  return (
    <div className="min-h-screen bg-brand-bg text-brand-text font-sans">
      <header className="border-b border-brand-border px-6 py-3 flex items-center gap-4">
        <button onClick={() => navigate('/dashboard')} className="text-brand-muted hover:text-brand-text text-sm transition-colors">← Dashboard</button>
        <span className="text-brand-cyan font-semibold">Import Manuscript</span>
        {format && <span className="text-xs text-brand-muted bg-brand-bg-card border border-brand-border rounded px-2 py-0.5 font-mono">{format}</span>}
      </header>

      <div className="max-w-3xl mx-auto px-6 py-8">
        {/* Step 1 — Upload */}
        {step === 'upload' && (
          <div className="space-y-6">
            <div>
              <h1 className="text-xl font-bold text-brand-text mb-1">Import a manuscript</h1>
              <p className="text-sm text-brand-muted">
                Upload a <span className="font-mono">.md</span>, <span className="font-mono">.txt</span>, or{' '}
                <span className="font-mono">.docx</span> file. NexusTale will parse it into chapters and scenes.
                You can review and adjust the structure before creating the project.
              </p>
            </div>

            {/* Drop zone */}
            <div
              onDragOver={(e) => { e.preventDefault(); setDragging(true) }}
              onDragLeave={() => setDragging(false)}
              onDrop={onDrop}
              onClick={() => fileRef.current?.click()}
              className={`border-2 border-dashed rounded-xl p-12 flex flex-col items-center gap-3 cursor-pointer transition-colors ${
                dragging ? 'border-brand-cyan bg-brand-cyan/5' : 'border-brand-border hover:border-brand-purple/60 hover:bg-brand-purple/5'
              }`}
            >
              <UploadIcon />
              <p className="text-sm text-brand-text font-medium">
                {parsing ? 'Parsing…' : 'Drop your manuscript here or click to browse'}
              </p>
              <p className="text-xs text-brand-muted">Supported: .md · .txt · .docx · max 10 MiB</p>
              <input ref={fileRef} type="file" accept={accepted} className="hidden" onChange={onFileInput} />
            </div>

            {error && <p className="text-sm text-red-400">{error}</p>}

            <div className="rounded-lg border border-brand-border bg-brand-bg-card p-4 text-xs text-brand-muted space-y-1">
              <p className="font-semibold text-brand-text mb-2">Structure inference rules</p>
              <p><span className="font-mono text-brand-purple"># Heading</span> → chapter · <span className="font-mono text-brand-purple">## Heading</span> → scene</p>
              <p><span className="font-mono text-brand-muted"># # #</span> or <span className="font-mono text-brand-muted">***</span> → scene break</p>
              <p>Word headings: Heading 1 → chapter · Heading 2 → scene</p>
              <p>Plain text: two blank lines → chapter break</p>
            </div>
          </div>
        )}

        {/* Step 2 — Preview */}
        {step === 'preview' && tree && (
          <div className="space-y-6">
            <div>
              <h1 className="text-xl font-bold text-brand-text mb-1">Review structure</h1>
              <p className="text-sm text-brand-muted">
                Edit titles, merge scenes, or remove items before creating the project.
                Scene content is preserved as-is.
              </p>
            </div>

            {/* Project title */}
            <div>
              <label className="block text-xs font-semibold text-brand-muted uppercase tracking-wider mb-1">Project title</label>
              <input
                value={tree.project_title}
                onChange={(e) => setProjectTitle(e.target.value)}
                className="w-full bg-brand-bg-card border border-brand-border rounded px-3 py-2 text-sm text-brand-text focus:outline-none focus:border-brand-purple"
              />
            </div>

            {/* Chapter / scene tree */}
            <div className="space-y-3">
              {tree.chapters.map((ch, ci) => (
                <ChapterCard
                  key={ci}
                  chapter={ch}
                  chapterIndex={ci}
                  onTitleChange={(t) => setChapterTitle(ci, t)}
                  onSceneTitleChange={(si, t) => setSceneTitle(ci, si, t)}
                  onMergeDown={(si) => mergeSceneDown(ci, si)}
                  onRemoveScene={(si) => removeScene(ci, si)}
                />
              ))}
            </div>

            {error && <p className="text-sm text-red-400">{error}</p>}

            {/* Summary */}
            <div className="flex items-center justify-between pt-4 border-t border-brand-border">
              <p className="text-xs text-brand-muted">
                {tree.chapters.length} chapter{tree.chapters.length !== 1 ? 's' : ''} ·{' '}
                {tree.chapters.reduce((n, c) => n + c.scenes.length, 0)} scene{tree.chapters.reduce((n, c) => n + c.scenes.length, 0) !== 1 ? 's' : ''}
              </p>
              <div className="flex gap-3">
                <button
                  onClick={() => setStep('upload')}
                  className="px-4 py-2 rounded text-sm text-brand-muted hover:text-brand-text transition-colors"
                >
                  ← Re-upload
                </button>
                <button
                  disabled={confirming || !tree.project_title.trim()}
                  onClick={handleConfirm}
                  className="px-5 py-2 rounded text-sm font-semibold bg-brand-gradient text-brand-bg hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed transition-opacity"
                >
                  {confirming ? 'Creating project…' : 'Create project →'}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// ── Chapter card ──────────────────────────────────────────────────────────────

function ChapterCard({
  chapter, chapterIndex: _chapterIndex, onTitleChange, onSceneTitleChange, onMergeDown, onRemoveScene,
}: {
  chapter: ImportPreviewChapter
  chapterIndex: number
  onTitleChange: (t: string) => void
  onSceneTitleChange: (si: number, t: string) => void
  onMergeDown: (si: number) => void
  onRemoveScene: (si: number) => void
}) {
  const [open, setOpen] = useState(true)

  return (
    <div className="rounded-lg border border-brand-border bg-brand-bg-card overflow-hidden">
      {/* Chapter header */}
      <div className="flex items-center gap-2 px-3 py-2 bg-brand-bg/60 border-b border-brand-border/60">
        <button onClick={() => setOpen(o => !o)} className="text-brand-muted hover:text-brand-text transition-colors text-xs">
          {open ? '▾' : '▸'}
        </button>
        <input
          value={chapter.title}
          onChange={(e) => onTitleChange(e.target.value)}
          className="flex-1 bg-transparent text-sm font-semibold text-brand-text focus:outline-none"
          placeholder="Chapter title"
        />
        <span className="text-[10px] text-brand-muted">{chapter.scenes.length} scene{chapter.scenes.length !== 1 ? 's' : ''}</span>
      </div>

      {/* Scenes */}
      {open && (
        <div className="divide-y divide-brand-border/30">
          {chapter.scenes.map((sc, si) => (
            <div key={si} className="flex items-start gap-2 px-3 py-2">
              <div className="flex-1 min-w-0">
                <input
                  value={sc.title}
                  onChange={(e) => onSceneTitleChange(si, e.target.value)}
                  className="w-full bg-transparent text-xs text-brand-text focus:outline-none mb-1"
                  placeholder={`Scene ${si + 1}`}
                />
                <p className="text-[10px] text-brand-muted truncate">
                  {sc.content ? sc.content.slice(0, 80) + (sc.content.length > 80 ? '…' : '') : '(empty)'}
                </p>
              </div>
              <div className="flex gap-1 shrink-0 mt-0.5">
                {si < chapter.scenes.length - 1 && (
                  <button
                    onClick={() => onMergeDown(si)}
                    title="Merge with next scene"
                    className="text-[10px] text-brand-muted hover:text-brand-text px-1.5 py-0.5 rounded border border-brand-border/60 hover:border-brand-border transition-colors"
                  >
                    Merge ↓
                  </button>
                )}
                <button
                  onClick={() => onRemoveScene(si)}
                  title="Remove scene"
                  className="text-[10px] text-red-400/70 hover:text-red-400 px-1.5 py-0.5 rounded border border-red-500/20 hover:border-red-500/40 transition-colors"
                >
                  ✕
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ── Icon ──────────────────────────────────────────────────────────────────────

function UploadIcon() {
  return (
    <svg className="w-8 h-8 text-brand-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 16V4m0 0L8 8m4-4 4 4" />
      <path d="M3 16v2a3 3 0 0 0 3 3h12a3 3 0 0 0 3-3v-2" />
    </svg>
  )
}
