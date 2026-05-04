// ScribeEditor — centered writing area with Beat expansion toolbar and annotation popover.
import { useRef, useState, forwardRef, useImperativeHandle } from 'react'
import BeatInput from './BeatInput'
import { api, type Annotation } from '@/services/api'

export interface ScribeEditorHandle {
  jumpToAnnotation: (start: number, end: number) => void
}

interface ScribeEditorProps {
  sceneTitle:  string
  content:     string
  sceneSelected: boolean
  onChange:    (value: string) => void
  // Beat/AI props — only provided when a scene is active
  token?:      string
  projectId?:  string
  sceneId?:    string
  promptId?:   string | null
  branch?:     string
  projectPhase?: string
  // Annotation callback — fired when user saves an annotation from the popover
  onAnnotationCreated?: (ann: Annotation) => void
}

// ── Popover state ─────────────────────────────────────────────────────────────

interface PopoverState {
  start:    number
  end:      number
  x:        number
  y:        number
}

const TYPE_OPTIONS = [
  { value: 'note',       label: 'Note',       color: 'text-yellow-400 bg-yellow-400/10 hover:bg-yellow-400/20' },
  { value: 'suggestion', label: 'Suggestion', color: 'text-blue-400   bg-blue-400/10   hover:bg-blue-400/20' },
  { value: 'question',   label: 'Question',   color: 'text-purple-400 bg-purple-400/10 hover:bg-purple-400/20' },
] as const

// ── Component ─────────────────────────────────────────────────────────────────

const ScribeEditor = forwardRef<ScribeEditorHandle, ScribeEditorProps>(function ScribeEditor(
  { sceneTitle, content, sceneSelected, onChange, token, projectId, sceneId, promptId, branch, projectPhase, onAnnotationCreated },
  ref,
) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  // Expose jump-to-annotation imperatively so AnnotationSidebar can drive selection.
  useImperativeHandle(ref, () => ({
    jumpToAnnotation(start: number, end: number) {
      const ta = textareaRef.current
      if (!ta) return
      ta.focus()
      ta.setSelectionRange(start, end)
      // Scroll the textarea so the selection is visible.
      const lineHeight = 32 // matches leading-8 (2rem)
      const charsPerLine = Math.max(1, Math.floor(ta.clientWidth / 9.6)) // rough monospace estimate
      const line = Math.floor(start / charsPerLine)
      ta.scrollTop = Math.max(0, line * lineHeight - ta.clientHeight / 2)
    },
  }))

  const [popover, setPopover]         = useState<PopoverState | null>(null)
  const [annType, setAnnType]         = useState<'note' | 'suggestion' | 'question'>('note')
  const [annBody, setAnnBody]         = useState('')
  const [saving, setSaving]           = useState(false)

  function handleMouseUp(e: React.MouseEvent<HTMLTextAreaElement>) {
    if (!token || !projectId || !sceneId) return
    const ta = e.currentTarget
    const start = ta.selectionStart
    const end   = ta.selectionEnd
    if (start === end) { setPopover(null); return }

    // Position popover above cursor, clamped to viewport.
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

  return (
    <div className="flex-1 flex flex-col overflow-hidden bg-brand-bg">

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
            <textarea
              ref={textareaRef}
              value={content}
              onChange={(e) => onChange(e.target.value)}
              onMouseUp={handleMouseUp}
              placeholder="Begin your scene…"
              spellCheck
              className="w-full h-full min-h-[60vh] resize-none bg-transparent text-brand-text text-base leading-8 placeholder:text-brand-muted/40 focus:outline-none font-serif"
            />
          ) : (
            <div className="flex items-center justify-center h-full">
              <p className="text-brand-muted text-sm">Select a scene to start writing</p>
            </div>
          )}
        </div>
      </div>

      {/* Beat expansion toolbar — only when scene is active and AI props provided */}
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

      {/* Annotation creation popover */}
      {popover && (
        <div
          className="fixed z-40 w-64 bg-brand-bg-card border border-brand-border rounded-xl shadow-2xl p-3 space-y-2"
          style={{ left: popover.x, top: popover.y }}
        >
          {/* Type selector */}
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

          {/* Body input */}
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

          {/* Actions */}
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

      {/* Click-away backdrop for popover */}
      {popover && (
        <div className="fixed inset-0 z-30" onClick={dismissPopover} />
      )}
    </div>
  )
})

export default ScribeEditor
