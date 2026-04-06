// ProjectExplorer — right sidebar: chapter/scene tree with inline creation.
import { useEffect, useRef, useState } from 'react'

interface SceneItem   { id: string; title: string }
interface ChapterItem { id: string; title: string; scenes: SceneItem[] }

interface ProjectExplorerProps {
  projectTitle: string
  chapters: ChapterItem[]
  selectedChapterId: string
  selectedSceneId: string
  onSelectScene: (chapterId: string, sceneId: string) => void
  onCreateChapter: (title: string) => Promise<void>
  onCreateScene: (chapterId: string, title: string) => Promise<void>
}

export default function ProjectExplorer({
  projectTitle,
  chapters,
  selectedChapterId,
  selectedSceneId,
  onSelectScene,
  onCreateChapter,
  onCreateScene,
}: ProjectExplorerProps) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(chapters.map((c) => [c.id, true]))
  )
  // Expand newly added chapters automatically.
  useEffect(() => {
    setExpanded((prev) => {
      const next = { ...prev }
      for (const c of chapters) if (!(c.id in next)) next[c.id] = true
      return next
    })
  }, [chapters])

  const [addingChapter, setAddingChapter] = useState(false)
  const [addingSceneFor, setAddingSceneFor] = useState<string | null>(null)

  const toggleChapter = (id: string) =>
    setExpanded((prev) => ({ ...prev, [id]: !prev[id] }))

  return (
    <div className="w-64 flex flex-col bg-brand-bg-card border-l border-brand-border shrink-0 overflow-hidden">

      {/* Header */}
      <div className="px-4 py-3 border-b border-brand-border flex items-center justify-between gap-2">
        <span className="text-xs font-semibold text-brand-muted uppercase tracking-wider truncate flex-1">
          {projectTitle}
        </span>
        <button
          title="New chapter"
          onClick={() => setAddingChapter(true)}
          className="text-brand-muted hover:text-brand-cyan transition-colors shrink-0"
        >
          <PlusIcon />
        </button>
      </div>

      {/* Tree */}
      <div className="flex-1 overflow-y-auto py-2 text-sm">

        {/* Inline new-chapter input */}
        {addingChapter && (
          <InlineInput
            placeholder="Chapter title…"
            onConfirm={async (title) => {
              await onCreateChapter(title)
              setAddingChapter(false)
            }}
            onCancel={() => setAddingChapter(false)}
          />
        )}

        {chapters.length === 0 && !addingChapter && (
          <p className="px-4 py-6 text-brand-muted/60 text-xs text-center">
            No chapters yet —{' '}
            <button onClick={() => setAddingChapter(true)} className="text-brand-cyan hover:underline">
              add one
            </button>
          </p>
        )}

        {chapters.map((chapter) => (
          <div key={chapter.id}>
            {/* Chapter row */}
            <div className="flex items-center group">
              <button
                onClick={() => toggleChapter(chapter.id)}
                className={`flex-1 flex items-center gap-2 px-3 py-1.5 text-left transition-colors min-w-0 ${
                  selectedChapterId === chapter.id
                    ? 'text-brand-text'
                    : 'text-brand-muted hover:text-brand-text'
                }`}
              >
                <ChevronIcon open={!!expanded[chapter.id]} />
                <FolderIcon open={!!expanded[chapter.id]} />
                <span className="truncate font-medium">{chapter.title}</span>
              </button>
              <button
                title="New scene"
                onClick={() => {
                  setExpanded((prev) => ({ ...prev, [chapter.id]: true }))
                  setAddingSceneFor(chapter.id)
                }}
                className="px-2 py-1.5 text-brand-muted opacity-0 group-hover:opacity-100 hover:text-brand-cyan transition-all shrink-0"
              >
                <PlusIcon />
              </button>
            </div>

            {/* Scenes */}
            {expanded[chapter.id] && (
              <div className="ml-6">
                {chapter.scenes.map((scene) => {
                  const active =
                    selectedChapterId === chapter.id && selectedSceneId === scene.id
                  return (
                    <button
                      key={scene.id}
                      onClick={() => onSelectScene(chapter.id, scene.id)}
                      className={`w-full flex items-center gap-2 px-3 py-1 text-left rounded-sm transition-colors ${
                        active
                          ? 'bg-brand-cyan/10 text-brand-cyan'
                          : 'text-brand-muted hover:text-brand-text hover:bg-brand-border/30'
                      }`}
                    >
                      <FileIcon />
                      <span className="truncate">{scene.title}</span>
                    </button>
                  )
                })}

                {/* Inline new-scene input */}
                {addingSceneFor === chapter.id && (
                  <InlineInput
                    placeholder="Scene title…"
                    onConfirm={async (title) => {
                      await onCreateScene(chapter.id, title)
                      setAddingSceneFor(null)
                    }}
                    onCancel={() => setAddingSceneFor(null)}
                  />
                )}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

// ── Inline input ──────────────────────────────────────────────────────────────

function InlineInput({
  placeholder,
  onConfirm,
  onCancel,
}: {
  placeholder: string
  onConfirm: (value: string) => Promise<void>
  onCancel: () => void
}) {
  const [value, setValue]   = useState('')
  const [busy, setBusy]     = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => { inputRef.current?.focus() }, [])

  const submit = async () => {
    const trimmed = value.trim()
    if (!trimmed || busy) return
    setBusy(true)
    try {
      await onConfirm(trimmed)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="px-3 py-1 flex items-center gap-1">
      <input
        ref={inputRef}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter')  submit()
          if (e.key === 'Escape') onCancel()
        }}
        placeholder={placeholder}
        disabled={busy}
        className="flex-1 min-w-0 bg-brand-bg-input border border-brand-cyan/40 rounded px-2 py-1 text-xs text-brand-text placeholder-brand-muted/60 focus:outline-none focus:border-brand-cyan"
      />
      <button
        onClick={submit}
        disabled={busy || !value.trim()}
        className="text-brand-cyan hover:text-brand-cyan/80 disabled:opacity-30 transition-colors"
        title="Confirm"
      >
        <CheckIcon />
      </button>
      <button
        onClick={onCancel}
        className="text-brand-muted hover:text-brand-text transition-colors"
        title="Cancel"
      >
        <XIcon />
      </button>
    </div>
  )
}

// ── Icons ─────────────────────────────────────────────────────────────────────

function ChevronIcon({ open }: { open: boolean }) {
  return (
    <svg className={`w-3 h-3 shrink-0 transition-transform ${open ? 'rotate-90' : ''}`} viewBox="0 0 16 16" fill="currentColor">
      <path d="M6 3l5 5-5 5" stroke="currentColor" strokeWidth="1.5" fill="none" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function FolderIcon({ open }: { open: boolean }) {
  return open ? (
    <svg className="w-3.5 h-3.5 shrink-0 text-brand-gold/70" viewBox="0 0 20 20" fill="currentColor">
      <path d="M2 6a2 2 0 012-2h4l2 2h6a2 2 0 012 2v1H2V6z" />
      <path d="M2 9h16v7a2 2 0 01-2 2H4a2 2 0 01-2-2V9z" />
    </svg>
  ) : (
    <svg className="w-3.5 h-3.5 shrink-0 text-brand-muted/60" viewBox="0 0 20 20" fill="currentColor">
      <path d="M2 6a2 2 0 012-2h4l2 2h6a2 2 0 012 2v7a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
    </svg>
  )
}

function FileIcon() {
  return (
    <svg className="w-3 h-3 shrink-0 opacity-50" viewBox="0 0 16 16" fill="currentColor">
      <path d="M4 0h5.5l4.5 4.5V15a1 1 0 01-1 1H4a1 1 0 01-1-1V1a1 1 0 011-1z" />
      <path d="M9 0v4a1 1 0 001 1h3.5" fill="none" stroke="currentColor" strokeWidth="0.75" />
    </svg>
  )
}

function PlusIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
      <path d="M8 3v10M3 8h10" />
    </svg>
  )
}

function CheckIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2.5 8l4 4 7-7" />
    </svg>
  )
}

function XIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
      <path d="M4 4l8 8M12 4l-8 8" />
    </svg>
  )
}
