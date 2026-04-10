// ProjectExplorer — right sidebar: act/chapter/scene tree with inline creation.
//
// Act layer visibility rule: acts are hidden when the project has exactly one
// act whose title is "Act 1" (the auto-created default). Writers who want
// structure can rename or add acts and the layer appears automatically.
import { useEffect, useRef, useState } from 'react'

interface SceneItem   { id: string; title: string }
interface ChapterItem { id: string; title: string; scenes: SceneItem[] }
export interface ActItem { id: string; title: string; chapters: ChapterItem[] }

interface ProjectExplorerProps {
  projectTitle:      string
  acts:              ActItem[]
  selectedChapterId: string
  selectedSceneId:   string
  onSelectScene:     (chapterId: string, sceneId: string) => void
  onCreateAct:       (title: string) => Promise<void>
  onCreateChapter:   (actId: string, title: string) => Promise<void>
  onCreateScene:     (chapterId: string, title: string) => Promise<void>
}

// Hide the act layer when the project only has the silent default act.
function actsAreHidden(acts: ActItem[]): boolean {
  return acts.length === 1 && acts[0].title === 'Act 1'
}

export default function ProjectExplorer({
  projectTitle,
  acts,
  selectedChapterId,
  selectedSceneId,
  onSelectScene,
  onCreateAct,
  onCreateChapter,
  onCreateScene,
}: ProjectExplorerProps) {
  const hidden = actsAreHidden(acts)

  // Collapse state for acts and chapters keyed by id.
  const [expandedActs,     setExpandedActs]     = useState<Record<string, boolean>>({})
  const [expandedChapters, setExpandedChapters] = useState<Record<string, boolean>>({})

  // Auto-expand newly added acts/chapters.
  useEffect(() => {
    setExpandedActs((prev) => {
      const next = { ...prev }
      for (const a of acts) if (!(a.id in next)) next[a.id] = true
      return next
    })
  }, [acts])

  useEffect(() => {
    setExpandedChapters((prev) => {
      const next = { ...prev }
      for (const a of acts)
        for (const c of a.chapters) if (!(c.id in next)) next[c.id] = true
      return next
    })
  }, [acts])

  const [addingAct,        setAddingAct]        = useState(false)
  const [addingChapterFor, setAddingChapterFor] = useState<string | null>(null) // actId
  const [addingSceneFor,   setAddingSceneFor]   = useState<string | null>(null) // chapterId

  const toggleAct     = (id: string) => setExpandedActs((p) => ({ ...p, [id]: !p[id] }))
  const toggleChapter = (id: string) => setExpandedChapters((p) => ({ ...p, [id]: !p[id] }))

  // Flatten chapters when acts are hidden (single default act).
  const allChapters = acts.flatMap((a) => a.chapters)
  const defaultActId = acts[0]?.id ?? ''

  return (
    <div className="w-64 flex flex-col bg-brand-bg-card border-l border-brand-border shrink-0 overflow-hidden">

      {/* Header */}
      <div className="px-4 py-3 border-b border-brand-border flex items-center justify-between gap-2">
        <span className="text-xs font-semibold text-brand-muted uppercase tracking-wider truncate flex-1">
          {projectTitle}
        </span>
        <div className="flex items-center gap-1 shrink-0">
          {/* Show "New Act" button only when acts are visible */}
          {!hidden && (
            <button
              title="New act"
              onClick={() => setAddingAct(true)}
              className="text-brand-muted hover:text-brand-purple transition-colors"
            >
              <ActPlusIcon />
            </button>
          )}
          {/* New chapter always available — in hidden mode, adds to the default act */}
          <button
            title="New chapter"
            onClick={() => {
              if (hidden) {
                setAddingChapterFor(defaultActId)
              } else {
                // When acts are visible, open act input if no act selected yet.
                setAddingChapterFor(expandedActs[defaultActId] ? defaultActId : defaultActId)
              }
            }}
            className="text-brand-muted hover:text-brand-cyan transition-colors"
          >
            <PlusIcon />
          </button>
        </div>
      </div>

      {/* Tree */}
      <div className="flex-1 overflow-y-auto py-2 text-sm">

        {/* ── ACTS HIDDEN: flat chapter list ──────────────────────────────── */}
        {hidden && (
          <>
            {addingChapterFor && (
              <InlineInput
                placeholder="Chapter title…"
                onConfirm={async (title) => {
                  await onCreateChapter(defaultActId, title)
                  setAddingChapterFor(null)
                }}
                onCancel={() => setAddingChapterFor(null)}
              />
            )}

            {allChapters.length === 0 && !addingChapterFor && (
              <EmptyState label="No chapters yet" onAdd={() => setAddingChapterFor(defaultActId)} />
            )}

            {allChapters.map((chapter) => (
              <ChapterRow
                key={chapter.id}
                chapter={chapter}
                expanded={!!expandedChapters[chapter.id]}
                selectedChapterId={selectedChapterId}
                selectedSceneId={selectedSceneId}
                addingSceneFor={addingSceneFor}
                onToggle={() => toggleChapter(chapter.id)}
                onAddScene={() => {
                  setExpandedChapters((p) => ({ ...p, [chapter.id]: true }))
                  setAddingSceneFor(chapter.id)
                }}
                onSelectScene={onSelectScene}
                onCreateScene={onCreateScene}
                onCancelScene={() => setAddingSceneFor(null)}
              />
            ))}
          </>
        )}

        {/* ── ACTS VISIBLE: three-level tree ──────────────────────────────── */}
        {!hidden && (
          <>
            {/* Inline new-act input */}
            {addingAct && (
              <InlineInput
                placeholder="Act title…"
                onConfirm={async (title) => {
                  await onCreateAct(title)
                  setAddingAct(false)
                }}
                onCancel={() => setAddingAct(false)}
              />
            )}

            {acts.length === 0 && !addingAct && (
              <EmptyState label="No acts yet" onAdd={() => setAddingAct(true)} />
            )}

            {acts.map((act) => (
              <div key={act.id}>
                {/* Act row */}
                <div className="flex items-center group">
                  <button
                    onClick={() => toggleAct(act.id)}
                    className="flex-1 flex items-center gap-2 px-3 py-1.5 text-left transition-colors min-w-0 text-brand-muted hover:text-brand-text"
                  >
                    <ChevronIcon open={!!expandedActs[act.id]} />
                    <ActIcon />
                    <span className="truncate font-semibold text-brand-purple/80">{act.title}</span>
                  </button>
                  <button
                    title="New chapter in this act"
                    onClick={() => {
                      setExpandedActs((p) => ({ ...p, [act.id]: true }))
                      setAddingChapterFor(act.id)
                    }}
                    className="px-2 py-1.5 text-brand-muted opacity-0 group-hover:opacity-100 hover:text-brand-cyan transition-all shrink-0"
                  >
                    <PlusIcon />
                  </button>
                </div>

                {/* Chapters inside act */}
                {expandedActs[act.id] && (
                  <div className="ml-4">
                    {addingChapterFor === act.id && (
                      <InlineInput
                        placeholder="Chapter title…"
                        onConfirm={async (title) => {
                          await onCreateChapter(act.id, title)
                          setAddingChapterFor(null)
                        }}
                        onCancel={() => setAddingChapterFor(null)}
                      />
                    )}

                    {act.chapters.map((chapter) => (
                      <ChapterRow
                        key={chapter.id}
                        chapter={chapter}
                        expanded={!!expandedChapters[chapter.id]}
                        selectedChapterId={selectedChapterId}
                        selectedSceneId={selectedSceneId}
                        addingSceneFor={addingSceneFor}
                        onToggle={() => toggleChapter(chapter.id)}
                        onAddScene={() => {
                          setExpandedChapters((p) => ({ ...p, [chapter.id]: true }))
                          setAddingSceneFor(chapter.id)
                        }}
                        onSelectScene={onSelectScene}
                        onCreateScene={onCreateScene}
                        onCancelScene={() => setAddingSceneFor(null)}
                      />
                    ))}
                  </div>
                )}
              </div>
            ))}
          </>
        )}
      </div>
    </div>
  )
}

// ── ChapterRow ────────────────────────────────────────────────────────────────

function ChapterRow({
  chapter,
  expanded,
  selectedChapterId,
  selectedSceneId,
  addingSceneFor,
  onToggle,
  onAddScene,
  onSelectScene,
  onCreateScene,
  onCancelScene,
}: {
  chapter:           ChapterItem
  expanded:          boolean
  selectedChapterId: string
  selectedSceneId:   string
  addingSceneFor:    string | null
  onToggle:          () => void
  onAddScene:        () => void
  onSelectScene:     (chapterId: string, sceneId: string) => void
  onCreateScene:     (chapterId: string, title: string) => Promise<void>
  onCancelScene:     () => void
}) {
  return (
    <div>
      <div className="flex items-center group">
        <button
          onClick={onToggle}
          className={`flex-1 flex items-center gap-2 px-3 py-1.5 text-left transition-colors min-w-0 ${
            selectedChapterId === chapter.id ? 'text-brand-text' : 'text-brand-muted hover:text-brand-text'
          }`}
        >
          <ChevronIcon open={expanded} />
          <FolderIcon open={expanded} />
          <span className="truncate font-medium">{chapter.title}</span>
        </button>
        <button
          title="New scene"
          onClick={onAddScene}
          className="px-2 py-1.5 text-brand-muted opacity-0 group-hover:opacity-100 hover:text-brand-cyan transition-all shrink-0"
        >
          <PlusIcon />
        </button>
      </div>

      {expanded && (
        <div className="ml-6">
          {chapter.scenes.map((scene) => {
            const active = selectedChapterId === chapter.id && selectedSceneId === scene.id
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

          {addingSceneFor === chapter.id && (
            <InlineInput
              placeholder="Scene title…"
              onConfirm={async (title) => {
                await onCreateScene(chapter.id, title)
                onCancelScene()
              }}
              onCancel={onCancelScene}
            />
          )}
        </div>
      )}
    </div>
  )
}

// ── Empty state ───────────────────────────────────────────────────────────────

function EmptyState({ label, onAdd }: { label: string; onAdd: () => void }) {
  return (
    <p className="px-4 py-6 text-brand-muted/60 text-xs text-center">
      {label} —{' '}
      <button onClick={onAdd} className="text-brand-cyan hover:underline">
        add one
      </button>
    </p>
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
  const [value, setValue] = useState('')
  const [busy, setBusy]   = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => { inputRef.current?.focus() }, [])

  const submit = async () => {
    const trimmed = value.trim()
    if (!trimmed || busy) return
    setBusy(true)
    try { await onConfirm(trimmed) } finally { setBusy(false) }
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
      <button onClick={submit} disabled={busy || !value.trim()} className="text-brand-cyan hover:text-brand-cyan/80 disabled:opacity-30 transition-colors" title="Confirm">
        <CheckIcon />
      </button>
      <button onClick={onCancel} className="text-brand-muted hover:text-brand-text transition-colors" title="Cancel">
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

function ActIcon() {
  return (
    <svg className="w-3.5 h-3.5 shrink-0 text-brand-purple/60" viewBox="0 0 20 20" fill="currentColor">
      <path fillRule="evenodd" d="M3 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h8a1 1 0 110 2H4a1 1 0 01-1-1z" clipRule="evenodd" />
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

function ActPlusIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round">
      <path d="M2 4h12M2 8h8M2 12h5M13 10v6M10 13h6" />
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
