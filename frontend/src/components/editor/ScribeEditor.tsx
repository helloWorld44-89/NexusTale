// ScribeEditor — centered writing area with TipTap editor, entity highlights,
// hover cards, Beat expansion toolbar, and annotation popover.
import { useRef, useState, useEffect, useCallback, forwardRef, useImperativeHandle } from 'react'
import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import BeatInput from './BeatInput'
import MentionsBar from './MentionsBar'
import EntityHoverCard from './EntityHoverCard'
import { EntityMentionExtension, mentionPluginKey } from './extensions/EntityMentionExtension'
import { plainToHTML, editorGetText, buildCharToPos, buildPosToChar } from './utils/editorUtils'
import { api, type Annotation, type MentionResponse } from '@/services/api'

export interface ScribeEditorHandle {
  jumpToAnnotation: (start: number, end: number) => void
}

interface ScribeEditorProps {
  sceneTitle:    string
  content:       string
  sceneSelected: boolean
  onChange:      (value: string) => void
  token?:        string
  projectId?:    string
  sceneId?:      string
  promptId?:     string | null
  branch?:       string
  projectPhase?: string
  onAnnotationCreated?: (ann: Annotation) => void
  onNavigateToEntity?:  (entityId: string) => void
}

// ── Popover / hover types ─────────────────────────────────────────────────────

interface PopoverState {
  start: number
  end:   number
  x:     number
  y:     number
}

interface HoverTarget {
  entityId:   string
  entityName: string
  entityType: string
  x:          number
  y:          number
}

const TYPE_OPTIONS = [
  { value: 'note',       label: 'Note',       color: 'text-yellow-400 bg-yellow-400/10 hover:bg-yellow-400/20' },
  { value: 'suggestion', label: 'Suggestion', color: 'text-blue-400   bg-blue-400/10   hover:bg-blue-400/20' },
  { value: 'question',   label: 'Question',   color: 'text-purple-400 bg-purple-400/10 hover:bg-purple-400/20' },
] as const

// ── Component ─────────────────────────────────────────────────────────────────

const ScribeEditor = forwardRef<ScribeEditorHandle, ScribeEditorProps>(function ScribeEditor(
  {
    sceneTitle, content, sceneSelected, onChange,
    token, projectId, sceneId, promptId, branch, projectPhase,
    onAnnotationCreated, onNavigateToEntity,
  },
  ref,
) {
  // Keep onChange stable in closures even if the prop identity changes.
  const onChangeRef = useRef(onChange)
  useEffect(() => { onChangeRef.current = onChange }, [onChange])

  // ── TipTap editor ─────────────────────────────────────────────────────────

  const editor = useEditor({
    extensions: [StarterKit, EntityMentionExtension],
    content: plainToHTML(content),
    onUpdate: ({ editor: ed }) => {
      onChangeRef.current(editorGetText(ed))
    },
    editable: sceneSelected,
  })

  // Sync external content changes (from AI tools, beat accept, etc.).
  useEffect(() => {
    if (!editor) return
    const current = editorGetText(editor)
    if (current === content) return
    editor.commands.setContent(plainToHTML(content), { emitUpdate: false })
  }, [content, editor])

  // Sync editability when sceneSelected changes.
  useEffect(() => {
    if (!editor) return
    editor.setEditable(sceneSelected)
  }, [editor, sceneSelected])

  // ── Mentions (shared with extension + MentionsBar) ────────────────────────

  const [mentions, setMentions] = useState<MentionResponse[]>([])

  const loadMentions = useCallback(async () => {
    if (!token || !projectId || !sceneId) return
    try {
      const res = await api.wiki.mentions.list(token, projectId, sceneId, branch ?? 'canon')
      setMentions(res.mentions ?? [])
    } catch {
      setMentions([])
    }
  }, [token, projectId, sceneId, branch])

  useEffect(() => { loadMentions() }, [loadMentions])

  // Push updated mentions into the decoration plugin.
  useEffect(() => {
    if (!editor) return
    editor.view.dispatch(
      editor.state.tr.setMeta(mentionPluginKey, mentions),
    )
  }, [editor, mentions])

  // ── jumpToAnnotation (plain-text offsets → PM selection) ──────────────────

  useImperativeHandle(ref, () => ({
    jumpToAnnotation(start: number, end: number) {
      if (!editor) return
      const charToPos = buildCharToPos(editor.state.doc)
      const from = charToPos(start)
      const to   = charToPos(end)
      editor.chain().focus().setTextSelection({ from, to }).run()
      editor.view.dispatch(editor.state.tr.scrollIntoView())
    },
  }))

  // ── Entity hover card ─────────────────────────────────────────────────────

  const [hoverTarget, setHoverTarget]   = useState<HoverTarget | null>(null)
  const hoverShowTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const hoverHideTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  function clearHoverTimers() {
    if (hoverShowTimer.current) clearTimeout(hoverShowTimer.current)
    if (hoverHideTimer.current) clearTimeout(hoverHideTimer.current)
  }

  function scheduleHide() {
    clearHoverTimers()
    hoverHideTimer.current = setTimeout(() => setHoverTarget(null), 150)
  }

  function handleEditorMouseOver(e: React.MouseEvent<HTMLDivElement>) {
    const span = (e.target as HTMLElement).closest('[data-entity-id]') as HTMLElement | null
    if (!span) return
    clearHoverTimers()
    const rect = span.getBoundingClientRect()
    hoverShowTimer.current = setTimeout(() => {
      setHoverTarget({
        entityId:   span.getAttribute('data-entity-id')!,
        entityName: span.getAttribute('data-entity-name')!,
        entityType: span.getAttribute('data-entity-type')!,
        x: rect.left,
        y: rect.top,
      })
    }, 400)
  }

  function handleEditorMouseOut(e: React.MouseEvent<HTMLDivElement>) {
    const related = e.relatedTarget as HTMLElement | null
    if (related?.closest('.entity-hover-card')) return
    clearHoverTimers()
    scheduleHide()
  }

  // ── Right-click suppress from editor decorations ──────────────────────────

  const [editorSuppressMenu, setEditorSuppressMenu] = useState<{
    x: number; y: number; mentionId: string
  } | null>(null)

  function handleEditorContextMenu(e: React.MouseEvent<HTMLDivElement>) {
    const span = (e.target as HTMLElement).closest('[data-mention-id]') as HTMLElement | null
    if (!span) return
    e.preventDefault()
    clearHoverTimers()
    setHoverTarget(null)
    setEditorSuppressMenu({
      x: e.clientX,
      y: e.clientY,
      mentionId: span.getAttribute('data-mention-id')!,
    })
  }

  async function handleSuppressFromEditor(mentionId: string) {
    setEditorSuppressMenu(null)
    setMentions(prev => prev.filter(m => m.id !== mentionId))
    try {
      await api.wiki.mentions.suppress(token!, projectId!, sceneId!, mentionId)
    } catch {
      loadMentions()
    }
  }

  // Suppress handlers for MentionsBar controlled mode.
  async function handleSuppressOne(mentionId: string) {
    setMentions(prev => prev.filter(m => m.id !== mentionId))
    try {
      await api.wiki.mentions.suppress(token!, projectId!, sceneId!, mentionId)
    } catch {
      loadMentions()
    }
  }

  async function handleSuppressAll() {
    setMentions([])
    try {
      await api.wiki.mentions.suppressAll(token!, projectId!, sceneId!, branch ?? 'canon')
    } catch {
      loadMentions()
    }
  }

  // ── Annotation popover ────────────────────────────────────────────────────

  const [popover, setPopover] = useState<PopoverState | null>(null)
  const [annType, setAnnType] = useState<'note' | 'suggestion' | 'question'>('note')
  const [annBody, setAnnBody] = useState('')
  const [saving,  setSaving]  = useState(false)

  function handleEditorMouseUp(e: React.MouseEvent<HTMLDivElement>) {
    if (!token || !projectId || !sceneId || !editor) return
    const { from, to } = editor.state.selection
    if (from === to) { setPopover(null); return }

    const posToChar = buildPosToChar(editor.state.doc)
    const start = posToChar(from)
    const end   = posToChar(to)

    const x = Math.min(e.clientX, window.innerWidth - 280)
    const y = Math.max(e.clientY - 140, 8)
    setPopover({ start, end, x, y })
    setAnnBody('')
  }

  function dismissPopover() {
    setPopover(null)
    setAnnBody('')
  }

  async function handleSaveAnnotation() {
    if (!popover || !token || !projectId || !sceneId || !annBody.trim()) return
    setSaving(true)
    try {
      const ann = await api.annotations.create(token, projectId, sceneId, {
        start_char: popover.start,
        end_char:   popover.end,
        body:       annBody.trim(),
        type:       annType,
      })
      onAnnotationCreated?.(ann)
      dismissPopover()
    } catch { /* ignore */ } finally {
      setSaving(false)
    }
  }

  const handleBeatAccept = (text: string) => {
    onChange(content + text)
  }

  // ── Render ────────────────────────────────────────────────────────────────

  return (
    <div data-tour="scribe-editor" className="flex-1 flex flex-col overflow-hidden bg-brand-bg">

      {/* Scene title strip */}
      <div className="px-8 pt-8 pb-3 max-w-3xl w-full mx-auto">
        <h1 className="text-xl font-semibold text-brand-text/80 tracking-tight">
          {sceneTitle}
        </h1>
      </div>

      {/* Writing surface */}
      <div className="flex-1 overflow-y-auto px-8 pb-4">
        <div className="max-w-3xl mx-auto h-full">
          {sceneSelected ? (
            <div
              className="w-full h-full"
              onMouseOver={handleEditorMouseOver}
              onMouseOut={handleEditorMouseOut}
              onMouseUp={handleEditorMouseUp}
              onContextMenu={handleEditorContextMenu}
            >
              <EditorContent
                editor={editor}
                className={[
                  'w-full h-full',
                  '[&_.ProseMirror]:outline-none',
                  '[&_.ProseMirror]:min-h-[60vh]',
                  '[&_.ProseMirror]:bg-transparent',
                  '[&_.ProseMirror]:text-brand-text',
                  '[&_.ProseMirror]:text-base',
                  '[&_.ProseMirror]:leading-8',
                  '[&_.ProseMirror]:font-serif',
                  '[&_.ProseMirror_p]:min-h-[2rem]',
                ].join(' ')}
              />
            </div>
          ) : (
            <div className="flex items-center justify-center h-full">
              <p className="text-brand-muted text-sm">Select a scene to start writing</p>
            </div>
          )}
        </div>
      </div>

      {/* Beat expansion toolbar */}
      {sceneSelected && token && projectId && sceneId && (
        <BeatInput
          token={token}
          projectId={projectId}
          sceneId={sceneId}
          promptId={promptId ?? null}
          branch={branch}
          projectPhase={projectPhase}
          onAccept={handleBeatAccept}
        />
      )}

      {/* Mentions bar — entity chips (controlled: synced with editor decorations) */}
      {sceneSelected && token && projectId && sceneId && (
        <MentionsBar
          token={token}
          projectId={projectId}
          sceneId={sceneId}
          branch={branch}
          controlled={{
            mentions,
            onSuppressOne: handleSuppressOne,
            onSuppressAll: handleSuppressAll,
          }}
          onNavigateToEntity={onNavigateToEntity}
        />
      )}

      {/* Entity hover card */}
      {hoverTarget && token && projectId && (
        <EntityHoverCard
          entityId={hoverTarget.entityId}
          entityName={hoverTarget.entityName}
          entityType={hoverTarget.entityType}
          x={hoverTarget.x}
          y={hoverTarget.y}
          token={token}
          projectId={projectId}
          onNavigate={(id) => { setHoverTarget(null); onNavigateToEntity?.(id) }}
          onMouseEnter={() => { clearHoverTimers() }}
          onMouseLeave={scheduleHide}
        />
      )}

      {/* Right-click suppress menu (from editor decoration) */}
      {editorSuppressMenu && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setEditorSuppressMenu(null)} />
          <div
            className="fixed z-50 bg-brand-bg-card border border-brand-border rounded-lg shadow-xl py-1 min-w-[160px]"
            style={{ left: editorSuppressMenu.x, top: editorSuppressMenu.y }}
          >
            <button
              onClick={() => handleSuppressFromEditor(editorSuppressMenu.mentionId)}
              className="w-full text-left px-3 py-1.5 text-xs text-brand-text hover:bg-brand-border/30 transition-colors"
            >
              Remove tag
            </button>
          </div>
        </>
      )}

      {/* Annotation creation popover */}
      {popover && (
        <div
          className="fixed z-40 w-64 bg-brand-bg-card border border-brand-border rounded-xl shadow-2xl p-3 space-y-2"
          style={{ left: popover.x, top: popover.y }}
        >
          <div className="flex gap-1.5">
            {TYPE_OPTIONS.map(opt => (
              <button
                key={opt.value}
                onClick={() => setAnnType(opt.value)}
                className={`flex-1 px-1.5 py-1 rounded text-[9px] font-semibold transition-colors ${opt.color} ${
                  annType === opt.value ? 'ring-1 ring-current' : 'opacity-60'
                }`}
              >
                {opt.label}
              </button>
            ))}
          </div>

          <textarea
            autoFocus
            value={annBody}
            onChange={e => setAnnBody(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) handleSaveAnnotation()
              if (e.key === 'Escape') dismissPopover()
            }}
            placeholder="Add a note…"
            rows={3}
            className="w-full bg-brand-bg border border-brand-border rounded px-2 py-1.5 text-xs text-brand-text resize-none focus:outline-none focus:border-brand-cyan/60 placeholder:text-brand-muted/50"
          />

          <div className="flex gap-1.5 justify-end">
            <button
              onClick={dismissPopover}
              className="px-2.5 py-1 rounded text-[10px] text-brand-muted hover:text-brand-text transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={handleSaveAnnotation}
              disabled={!annBody.trim() || saving}
              className="px-2.5 py-1 rounded text-[10px] bg-brand-cyan text-brand-bg font-semibold hover:bg-brand-cyan/80 disabled:opacity-40 transition-colors"
            >
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </div>
      )}

      {/* Click-away backdrop for annotation popover */}
      {popover && (
        <div className="fixed inset-0 z-30" onClick={dismissPopover} />
      )}
    </div>
  )
})

export default ScribeEditor
