// BeatInput — inline beat expansion UI embedded below ScribeEditor.
// Writer types a 1-sentence story beat; pressing Generate streams 2–3 paragraphs
// of prose into the append area. Accept inserts into scene; Discard removes it.
import { useState, useRef, useCallback } from 'react'
import { api } from '@/services/api'

interface BeatInputProps {
  token:      string
  projectId:  string
  sceneId:    string
  promptId:   string | null
  onAccept:   (text: string) => void  // append generated text to scene content
}

export default function BeatInput({ token, projectId, sceneId, promptId, onAccept }: BeatInputProps) {
  const [open, setOpen]           = useState(false)
  const [beat, setBeat]           = useState('')
  const [generated, setGenerated] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [error, setError]         = useState<string | null>(null)
  const abortRef                  = useRef<AbortController | null>(null)

  const generate = useCallback(async () => {
    const text = beat.trim()
    if (!text || streaming) return

    setGenerated('')
    setError(null)
    setStreaming(true)
    abortRef.current = new AbortController()

    try {
      await api.ai.streamComplete(
        token,
        projectId,
        {
          sceneId,
          mode:     'beat',
          beat:     text,
          promptId: promptId ?? undefined,
        },
        (delta) => setGenerated((prev) => prev + delta),
        abortRef.current.signal,
      )
    } catch (err) {
      if ((err as Error).name !== 'AbortError') {
        setError(err instanceof Error ? err.message : 'Generation failed.')
      }
    } finally {
      setStreaming(false)
      abortRef.current = null
    }
  }, [beat, streaming, token, projectId, sceneId, promptId])

  const handleKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      generate()
    }
    if (e.key === 'Escape') {
      discard()
    }
  }

  const accept = () => {
    if (!generated) return
    onAccept('\n\n' + generated)
    discard()
  }

  const discard = () => {
    abortRef.current?.abort()
    setGenerated('')
    setBeat('')
    setError(null)
    setStreaming(false)
    setOpen(false)
  }

  const retry = () => {
    setGenerated('')
    setError(null)
    generate()
  }

  if (!open) {
    return (
      <div className="flex justify-center py-2 border-t border-brand-border/40">
        <button
          onClick={() => setOpen(true)}
          className="flex items-center gap-1.5 px-3 py-1 rounded text-xs text-brand-purple hover:text-brand-text hover:bg-brand-purple/10 border border-brand-purple/30 hover:border-brand-purple/60 transition-all"
          title="Expand a story beat into prose"
        >
          <BeatIcon />
          Beat →
        </button>
      </div>
    )
  }

  return (
    <div className="border-t border-brand-border/40 bg-brand-bg-card/50 px-8 py-3 max-w-3xl mx-auto w-full">
      {/* Beat input row */}
      <div className="flex items-center gap-2 mb-2">
        <input
          type="text"
          value={beat}
          onChange={(e) => setBeat(e.target.value)}
          onKeyDown={handleKey}
          placeholder="What happens next? e.g. 'Kira finds the door already ajar'"
          disabled={streaming}
          autoFocus
          className="flex-1 bg-brand-bg border border-brand-border rounded px-3 py-1.5 text-sm text-brand-text placeholder:text-brand-muted/50 focus:outline-none focus:border-brand-purple disabled:opacity-50"
        />
        {!streaming ? (
          <button
            onClick={generate}
            disabled={!beat.trim()}
            className="px-3 py-1.5 rounded text-xs font-medium bg-brand-purple/20 text-brand-purple border border-brand-purple/40 hover:bg-brand-purple/30 disabled:opacity-30 transition-colors"
          >
            Generate
          </button>
        ) : (
          <button
            onClick={() => abortRef.current?.abort()}
            className="px-3 py-1.5 rounded text-xs font-medium bg-brand-bg border border-brand-border text-brand-muted hover:text-brand-text transition-colors"
          >
            Stop
          </button>
        )}
        <button
          onClick={discard}
          className="p-1.5 rounded text-brand-muted hover:text-brand-text transition-colors"
          title="Close beat input (Esc)"
        >
          <CloseIcon />
        </button>
      </div>

      {/* Generated preview */}
      {(generated || streaming) && (
        <div className="rounded border border-brand-border bg-brand-bg px-4 py-3 text-sm text-brand-text leading-7 whitespace-pre-wrap font-serif mb-2 relative">
          {generated}
          {streaming && (
            <span className="inline-block w-1.5 h-4 ml-0.5 bg-brand-purple/70 animate-pulse align-middle" />
          )}
        </div>
      )}

      {error && (
        <p className="text-xs text-red-400 mb-2">{error}</p>
      )}

      {/* Actions — shown after generation completes */}
      {generated && !streaming && (
        <div className="flex gap-2">
          <button
            onClick={accept}
            className="px-3 py-1 rounded text-xs font-medium bg-brand-cyan/20 text-brand-cyan border border-brand-cyan/40 hover:bg-brand-cyan/30 transition-colors"
          >
            Accept
          </button>
          <button
            onClick={retry}
            className="px-3 py-1 rounded text-xs font-medium bg-brand-bg border border-brand-border text-brand-muted hover:text-brand-text transition-colors"
          >
            Retry
          </button>
          <button
            onClick={discard}
            className="px-3 py-1 rounded text-xs font-medium bg-brand-bg border border-brand-border text-brand-muted hover:text-brand-text transition-colors"
          >
            Discard
          </button>
        </div>
      )}
    </div>
  )
}

function BeatIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2 8h3l2-5 3 10 2-5h2" />
    </svg>
  )
}

function CloseIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
      <path d="M2 2l10 10M12 2L2 12" />
    </svg>
  )
}
