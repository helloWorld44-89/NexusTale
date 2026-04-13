// ChatBar — AI assistant sidebar wired to POST /projects/:id/ai/chat (SSE streaming).
import { useState, useRef, useEffect, useCallback } from 'react'
import { api } from '@/services/api'

interface Message {
  id: string
  role: 'user' | 'assistant'
  text: string
  streaming?: boolean
}

interface ChatBarProps {
  token: string
  projectId: string
  sceneId?: string
  branch?: string
}

const GREETING: Message = {
  id: 'm0',
  role: 'assistant',
  text: "I'm Scribe, your AI co-author. Ask me anything about your story — characters, plot, worldbuilding, or prose suggestions.",
}

export default function ChatBar({ token, projectId, sceneId, branch }: ChatBarProps) {
  const [messages, setMessages]   = useState<Message[]>([GREETING])
  const [input, setInput]         = useState('')
  const [streaming, setStreaming] = useState(false)
  const bottomRef  = useRef<HTMLDivElement>(null)
  const abortRef   = useRef<AbortController | null>(null)

  // Scroll to bottom whenever messages change.
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const send = useCallback(async () => {
    const text = input.trim()
    if (!text || streaming) return

    setInput('')

    const userMsg: Message = { id: `u-${Date.now()}`, role: 'user', text }
    const assistantId = `a-${Date.now()}`
    const assistantMsg: Message = { id: assistantId, role: 'assistant', text: '', streaming: true }

    setMessages((prev) => [...prev, userMsg, assistantMsg])
    setStreaming(true)

    // Build history for the API: all prior non-streaming messages.
    const history = [...messages, userMsg].map((m) => ({
      role: m.role as string,
      content: m.text,
    }))

    abortRef.current = new AbortController()

    try {
      await api.ai.streamChat(
        token,
        projectId,
        history,
        sceneId,
        (delta) => {
          setMessages((prev) =>
            prev.map((m) =>
              m.id === assistantId ? { ...m, text: m.text + delta } : m
            )
          )
        },
        abortRef.current.signal,
        branch,
      )
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Something went wrong.'
      // If aborted by user, don't show an error.
      if ((err as Error).name !== 'AbortError') {
        setMessages((prev) =>
          prev.map((m) =>
            m.id === assistantId
              ? { ...m, text: m.text || msg, streaming: false }
              : m
          )
        )
      }
    } finally {
      setMessages((prev) =>
        prev.map((m) => (m.id === assistantId ? { ...m, streaming: false } : m))
      )
      setStreaming(false)
      abortRef.current = null
    }
  }, [input, streaming, messages, token, projectId, sceneId, branch])

  const handleKey = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      send()
    }
  }

  const stopStreaming = () => {
    abortRef.current?.abort()
  }

  return (
    <div className="w-72 flex flex-col bg-brand-bg-card border-r border-brand-border shrink-0">

      {/* Header */}
      <div className="px-4 py-3 border-b border-brand-border flex items-center justify-between">
        <p className="text-xs font-semibold text-brand-cyan uppercase tracking-wider">Scribe AI</p>
        {streaming && (
          <button
            onClick={stopStreaming}
            title="Stop generation"
            className="text-brand-muted text-xs hover:text-brand-text transition-colors"
          >
            Stop
          </button>
        )}
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-3 py-3 space-y-3 text-sm">
        {messages.map((m) => (
          <div
            key={m.id}
            className={`flex gap-2 ${m.role === 'user' ? 'flex-row-reverse' : ''}`}
          >
            {/* Avatar */}
            <div
              className={`w-6 h-6 rounded-full shrink-0 flex items-center justify-center text-xs font-bold mt-0.5 ${
                m.role === 'assistant'
                  ? 'bg-brand-purple/30 text-brand-purple'
                  : 'bg-brand-cyan/20 text-brand-cyan'
              }`}
            >
              {m.role === 'assistant' ? 'S' : 'U'}
            </div>

            {/* Bubble */}
            <div
              className={`max-w-[200px] rounded-lg px-3 py-2 leading-relaxed whitespace-pre-wrap ${
                m.role === 'assistant'
                  ? 'bg-brand-bg text-brand-muted'
                  : 'bg-brand-purple/20 text-brand-text'
              }`}
            >
              {m.text}
              {m.streaming && (
                <span className="inline-block w-1.5 h-3.5 ml-0.5 bg-brand-cyan/70 animate-pulse align-middle" />
              )}
            </div>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <div className="px-3 py-3 border-t border-brand-border">
        <div className="flex items-end gap-2 bg-brand-bg-input rounded-lg border border-brand-border px-3 py-2">
          <textarea
            rows={1}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKey}
            placeholder="Ask Scribe…"
            disabled={streaming}
            className="flex-1 resize-none bg-transparent text-brand-text text-sm placeholder:text-brand-muted focus:outline-none max-h-28 leading-relaxed disabled:opacity-50"
          />
          <button
            onClick={send}
            disabled={!input.trim() || streaming}
            className="shrink-0 p-1 rounded text-brand-cyan disabled:opacity-30 hover:bg-brand-cyan/10 transition-colors"
            title="Send (Enter)"
          >
            <SendIcon />
          </button>
        </div>
        <p className="text-brand-muted text-xs mt-1.5 text-center opacity-50">
          Enter to send · Shift+Enter for newline
        </p>
      </div>
    </div>
  )
}

function SendIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
      <path d="M10.894 2.553a1 1 0 00-1.788 0l-7 14a1 1 0 001.169 1.409l5-1.429A1 1 0 009 15.571V11a1 1 0 112 0v4.571a1 1 0 00.725.962l5 1.428a1 1 0 001.17-1.408l-7-14z" />
    </svg>
  )
}
