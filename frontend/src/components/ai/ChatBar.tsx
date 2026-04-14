// ChatBar — Nexus AI assistant sidebar wired to POST /projects/:id/ai/chat (SSE streaming).
// The Nexus intro message is shown only when at least one AI provider is configured.
import { useState, useRef, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
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

const NEXUS_INTRO: Message = {
  id: 'm-nexus-intro',
  role: 'assistant',
  text: "Signal acquired. I'm Nexus — your co-author and story intelligence. I've indexed your chapters, wiki, and every branch of your timeline.\n\nAsk me anything about your story, or tell me what to write next.",
}

const NO_CONNECTION_MSG: Message = {
  id: 'm-no-connection',
  role: 'assistant',
  text: "No AI connection configured. Add a provider key or Ollama URL in Settings to activate Nexus.",
}

export default function ChatBar({ token, projectId, sceneId, branch }: ChatBarProps) {
  const [messages, setMessages]     = useState<Message[]>([])
  const [input, setInput]           = useState('')
  const [streaming, setStreaming]   = useState(false)
  const [connected, setConnected]   = useState<boolean | null>(null) // null = checking
  const bottomRef  = useRef<HTMLDivElement>(null)
  const abortRef   = useRef<AbortController | null>(null)

  // On mount, check if any API keys are configured; show intro only if yes.
  useEffect(() => {
    if (!token) return
    api.apiKeys.list(token)
      .then((keys) => {
        const hasKeys = keys.length > 0
        setConnected(hasKeys)
        setMessages([hasKeys ? NEXUS_INTRO : NO_CONNECTION_MSG])
      })
      .catch(() => {
        setConnected(false)
        setMessages([NO_CONNECTION_MSG])
      })
  }, [token])

  // Scroll to bottom whenever messages change.
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const send = useCallback(async () => {
    const text = input.trim()
    if (!text || streaming) return

    setInput('')

    const userMsg: Message      = { id: `u-${Date.now()}`, role: 'user', text }
    const assistantId           = `a-${Date.now()}`
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

  const stopStreaming = () => { abortRef.current?.abort() }

  return (
    <div className="w-72 flex flex-col bg-brand-bg-card border-r border-brand-border shrink-0">

      {/* Header */}
      <div className="px-4 py-3 border-b border-brand-border flex items-center justify-between">
        <div className="flex items-center gap-2">
          <NexusLogo />
          <p className="text-xs font-semibold text-brand-cyan uppercase tracking-wider">Nexus</p>
        </div>
        <div className="flex items-center gap-2">
          {connected === false && (
            <Link
              to="/settings"
              className="text-xs text-brand-muted hover:text-brand-cyan transition-colors"
              title="Configure AI in Settings"
            >
              Configure
            </Link>
          )}
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
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-3 py-3 space-y-3 text-sm">
        {connected === null ? (
          <div className="flex items-center justify-center py-8">
            <SpinIcon className="w-4 h-4 text-brand-purple animate-spin" />
          </div>
        ) : (
          messages.map((m) => (
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
                {m.role === 'assistant' ? 'N' : 'U'}
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
          ))
        )}
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
            placeholder="Ask Nexus…"
            disabled={streaming || connected === false}
            className="flex-1 resize-none bg-transparent text-brand-text text-sm placeholder:text-brand-muted focus:outline-none max-h-28 leading-relaxed disabled:opacity-50"
          />
          <button
            onClick={send}
            disabled={!input.trim() || streaming || connected === false}
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

// ── icons ─────────────────────────────────────────────────────────────────────

function NexusLogo() {
  return (
    <svg className="w-4 h-4 text-brand-purple" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="8" cy="8" r="2" />
      <path d="M8 1v3M8 12v3M1 8h3M12 8h3M3.22 3.22l2.12 2.12M10.66 10.66l2.12 2.12M3.22 12.78l2.12-2.12M10.66 5.34l2.12-2.12" />
    </svg>
  )
}

function SendIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
      <path d="M10.894 2.553a1 1 0 00-1.788 0l-7 14a1 1 0 001.169 1.409l5-1.429A1 1 0 009 15.571V11a1 1 0 112 0v4.571a1 1 0 00.725.962l5 1.428a1 1 0 001.17-1.408l-7-14z" />
    </svg>
  )
}

function SpinIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
    </svg>
  )
}
