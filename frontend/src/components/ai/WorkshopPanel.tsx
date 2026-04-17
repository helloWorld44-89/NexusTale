// WorkshopPanel — named, persistent AI chat sessions for deep story craft work.
// Left column: session list. Right column: active session chat.
// Messages are persisted to the backend after each exchange.

import { useState, useRef, useEffect, useCallback, useMemo } from 'react'
import { api } from '@/services/api'
import type { WorkshopSession, WorkshopMessage, ToolCallEvent } from '@/services/api'
import NexusThinking from './NexusThinking'

interface WorkshopPanelProps {
  token:              string
  projectId:          string
  sceneId?:           string
  branch?:            string
  onInsertToScene?:   (text: string) => void
  onToolWrite?:       (sceneId: string, chapterId: string) => void
  onStructureChange?: () => void
}

const WORKSHOP_INTRO: WorkshopMessage = {
  role:      'assistant',
  content:   "Workshop mode active. I'm focused on craft: structure, character, theme, pacing, and voice. What do you want to examine?",
  timestamp: new Date().toISOString(),
}

// ── Agent phases ──────────────────────────────────────────────────────────────
// idle     → no request in flight
// planning → round started, waiting for model response
// executing → model returned tool calls, tools are running
// replying → final text is streaming
type AgentPhase = 'idle' | 'planning' | 'executing' | 'replying'

// ── ChatMessage ───────────────────────────────────────────────────────────────
// Extends WorkshopMessage with optional agent-mode metadata.
// agentRunId groups tool events from the same send() call for display.
interface ChatMessage extends WorkshopMessage {
  toolEvent?: ToolCallEvent
  toolUndone?: boolean
  agentRunId?: string  // groups consecutive tool events under one run header
}

export default function WorkshopPanel({
  token,
  projectId,
  sceneId,
  branch,
  onInsertToScene,
  onToolWrite,
  onStructureChange,
}: WorkshopPanelProps) {
  const [sessions,         setSessions]         = useState<WorkshopSession[]>([])
  const [activeId,         setActiveId]         = useState<string | null>(null)
  const [messages,         setMessages]         = useState<ChatMessage[]>([WORKSHOP_INTRO])
  const [input,            setInput]            = useState('')
  const [streaming,        setStreaming]        = useState(false)
  const [agentPhase,       setAgentPhase]       = useState<AgentPhase>('idle')
  const [agentRound,       setAgentRound]       = useState(0)
  const [loading,          setLoading]          = useState(true)
  const [editingTitle,     setEditingTitle]     = useState(false)
  const [titleDraft,       setTitleDraft]       = useState('')
  const [creatingSession,  setCreatingSession]  = useState(false)
  const [toolsEnabled,     setToolsEnabled]     = useState(false)
  const [collapsed,        setCollapsed]        = useState<Set<string>>(new Set())

  const bottomRef   = useRef<HTMLDivElement>(null)
  const abortRef    = useRef<AbortController | null>(null)
  const titleRef    = useRef<HTMLInputElement>(null)
  const saveTimer   = useRef<ReturnType<typeof setTimeout> | null>(null)
  // Stable run ID for the current send(), so all tool events share one group.
  const runIdRef    = useRef<string>('')

  // ── load session list ──────────────────────────────────────────────────────

  useEffect(() => {
    let cancelled = false
    api.ai.workshop.list(token, projectId)
      .then((list) => {
        if (cancelled) return
        setSessions(list)
        if (list.length > 0) activateSession(list[0])
        else setLoading(false)
      })
      .catch(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, projectId])

  const activateSession = useCallback((session: WorkshopSession) => {
    setActiveId(session.id)
    const msgs = session.messages.length > 0 ? session.messages : [WORKSHOP_INTRO]
    setMessages(msgs)
    setLoading(false)
  }, [])

  const handleSelectSession = useCallback((session: WorkshopSession) => {
    if (streaming) return
    activateSession(session)
  }, [streaming, activateSession])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, agentPhase])

  // ── create new session ────────────────────────────────────────────────────

  const handleCreateSession = useCallback(async () => {
    if (creatingSession) return
    setCreatingSession(true)
    try {
      const session = await api.ai.workshop.create(token, projectId, 'New Session')
      setSessions((prev) => [session, ...prev])
      activateSession(session)
    } catch {
      // ignore
    } finally {
      setCreatingSession(false)
    }
  }, [token, projectId, creatingSession, activateSession])

  // ── send message ──────────────────────────────────────────────────────────

  const send = useCallback(async () => {
    const text = input.trim()
    if (!text || streaming || !activeId) return

    setInput('')

    // Assign a stable run ID for grouping any tool events this call produces.
    const currentRunId = toolsEnabled ? `run-${Date.now()}` : ''
    runIdRef.current = currentRunId

    const userMsg: ChatMessage = {
      role:      'user',
      content:   text,
      timestamp: new Date().toISOString(),
    }
    const assistantPlaceholder: ChatMessage = {
      role:      'assistant',
      content:   '',
      timestamp: new Date().toISOString(),
    }

    const nextMessages = [
      ...messages.filter((m) => m !== WORKSHOP_INTRO || messages.length > 1),
      userMsg,
      assistantPlaceholder,
    ]
    setMessages(nextMessages)
    setStreaming(true)
    if (toolsEnabled) setAgentPhase('planning')

    const history = nextMessages
      .filter((m) => (m.content !== '' || m.role !== 'assistant') && (m.role as string) !== 'system')
      .map((m) => ({ role: m.role as string, content: m.content }))

    abortRef.current = new AbortController()
    let fullResponse = ''

    try {
      await api.ai.workshop.streamChat(
        token,
        projectId,
        activeId,
        history,
        sceneId,
        (delta) => {
          // First delta means we're past the tool-use phase.
          setAgentPhase('replying')
          fullResponse += delta
          setMessages((prev) =>
            prev.map((m, i) =>
              i === prev.length - 1 && m.role === 'assistant'
                ? { ...m, content: m.content + delta }
                : m
            )
          )
        },
        abortRef.current.signal,
        branch,
        toolsEnabled,
        (evt) => {
          setAgentPhase('executing')
          const toolMsg: ChatMessage = {
            role:       'system' as ChatMessage['role'],
            content:    evt.result,
            timestamp:  new Date().toISOString(),
            toolEvent:  evt,
            toolUndone: false,
            agentRunId: runIdRef.current,
          }
          setMessages((prev) => {
            const copy = [...prev]
            copy.splice(copy.length - 1, 0, toolMsg)
            return copy
          })
          if (
            onToolWrite &&
            evt.scene_id &&
            evt.chapter_id &&
            (evt.tool === 'append_to_scene' || evt.tool === 'replace_scene_content')
          ) {
            onToolWrite(evt.scene_id, evt.chapter_id)
          }
        },
        (round) => {
          setAgentRound(round)
          setAgentPhase('planning')
        },
        // Pass 25 rounds for agent mode so large drafting tasks can complete.
        toolsEnabled ? 25 : undefined,
      )
    } catch (err) {
      if ((err as Error).name !== 'AbortError') {
        const msg = err instanceof Error ? err.message : 'Something went wrong.'
        setMessages((prev) =>
          prev.map((m, i) =>
            i === prev.length - 1 && m.role === 'assistant'
              ? { ...m, content: m.content || msg }
              : m
          )
        )
      }
    } finally {
      setStreaming(false)
      setAgentPhase('idle')
      setAgentRound(0)
      abortRef.current = null

      if (fullResponse && activeId) {
        const assistantMsg: ChatMessage = {
          role:      'assistant',
          content:   fullResponse,
          timestamp: new Date().toISOString(),
        }
        const persistedMessages = [
          ...messages.filter((m) => !(m === WORKSHOP_INTRO && messages.length === 1)),
          userMsg,
          assistantMsg,
        ]
        setMessages(persistedMessages)

        if (saveTimer.current) clearTimeout(saveTimer.current)
        saveTimer.current = setTimeout(() => {
          api.ai.workshop.update(token, projectId, activeId, { messages: persistedMessages })
            .then((updated) => {
              setSessions((prev) =>
                prev.map((s) => s.id === updated.id ? { ...s, updated_at: updated.updated_at } : s)
              )
            })
            .catch(() => {})
        }, 500)
      }
    }
  }, [input, streaming, activeId, messages, token, projectId, sceneId, branch, toolsEnabled, onToolWrite])

  const handleKey = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() }
  }

  // ── undo a tool action ────────────────────────────────────────────────────

  const handleUndo = useCallback(async (msgIndex: number, evt: ToolCallEvent) => {
    try {
      if (evt.tool === 'append_to_scene' || evt.tool === 'replace_scene_content') {
        if (evt.chapter_id && evt.scene_id) {
          await api.scenes.update(token, evt.chapter_id, evt.scene_id, {
            content: evt.before_content ?? '',
          })
          if (onToolWrite && evt.scene_id && evt.chapter_id) {
            onToolWrite(evt.scene_id, evt.chapter_id)
          }
        }
      } else if (evt.created_type === 'scene' && evt.created_id && evt.chapter_id) {
        await api.scenes.delete(token, evt.chapter_id, evt.created_id)
        onStructureChange?.()
      } else if (evt.created_type === 'chapter' && evt.created_id && evt.act_id && evt.project_id) {
        await api.chapters.delete(token, evt.project_id, evt.act_id, evt.created_id)
        onStructureChange?.()
      } else if (evt.created_type === 'act' && evt.created_id && evt.project_id) {
        await api.acts.delete(token, evt.project_id, evt.created_id)
        onStructureChange?.()
      }
      setMessages((prev) =>
        prev.map((m, i) => i === msgIndex ? { ...m, toolUndone: true } : m)
      )
    } catch {
      // leave button enabled so user can retry
    }
  }, [token, onToolWrite, onStructureChange])

  // ── toggle run collapse ───────────────────────────────────────────────────

  const toggleCollapse = (runId: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(runId)) next.delete(runId)
      else next.add(runId)
      return next
    })
  }

  // ── rename session ────────────────────────────────────────────────────────

  const startEditingTitle = () => {
    const session = sessions.find((s) => s.id === activeId)
    if (!session) return
    setTitleDraft(session.title)
    setEditingTitle(true)
    setTimeout(() => titleRef.current?.select(), 0)
  }

  const commitTitle = async () => {
    setEditingTitle(false)
    const newTitle = titleDraft.trim()
    if (!newTitle || !activeId) return
    const session = sessions.find((s) => s.id === activeId)
    if (!session || newTitle === session.title) return
    try {
      const updated = await api.ai.workshop.update(token, projectId, activeId, { title: newTitle })
      setSessions((prev) => prev.map((s) => s.id === activeId ? updated : s))
    } catch {
      // ignore
    }
  }

  // ── delete session ────────────────────────────────────────────────────────

  const handleDelete = useCallback(async (sessionId: string, e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      await api.ai.workshop.delete(token, projectId, sessionId)
      setSessions((prev) => {
        const remaining = prev.filter((s) => s.id !== sessionId)
        if (activeId === sessionId) {
          if (remaining.length > 0) activateSession(remaining[0])
          else { setActiveId(null); setMessages([WORKSHOP_INTRO]) }
        }
        return remaining
      })
    } catch { /* ignore */ }
  }, [token, projectId, activeId, activateSession])

  // ── Markdown export ───────────────────────────────────────────────────────

  const handleExport = useCallback(() => {
    const session = sessions.find((s) => s.id === activeId)
    if (!session) return
    const date = new Date(session.created_at).toLocaleDateString('en-US', {
      year: 'numeric', month: 'long', day: 'numeric',
    })
    const lines = [`# ${session.title}`, '', `**Date:** ${date}`, '', '---', '']
    for (const msg of messages) {
      if (msg === WORKSHOP_INTRO || (msg.role as string) === 'system') continue
      const label = msg.role === 'user' ? '**You:**' : '**Nexus:**'
      lines.push(label, '', msg.content, '', '---', '')
    }
    const blob = new Blob([lines.join('\n')], { type: 'text/markdown' })
    const url  = URL.createObjectURL(blob)
    const a    = document.createElement('a')
    a.href = url
    a.download = `${session.title.replace(/\s+/g, '-').toLowerCase()}.md`
    document.body.appendChild(a); a.click(); a.remove()
    URL.revokeObjectURL(url)
  }, [sessions, activeId, messages])

  // ── message rendering helpers ─────────────────────────────────────────────

  // Build a display list that groups consecutive tool-event messages sharing
  // the same agentRunId into a single AgentRunBlock entry.
  type DisplayItem =
    | { kind: 'msg';   msg: ChatMessage; idx: number }
    | { kind: 'run';   runId: string; events: Array<{ msg: ChatMessage; idx: number }> }

  const displayItems = useMemo((): DisplayItem[] => {
    const items: DisplayItem[] = []
    let i = 0
    while (i < messages.length) {
      const m = messages[i]
      const runId = (m as ChatMessage).agentRunId
      if ((m.role as string) === 'system' && runId) {
        // Collect all consecutive messages with the same runId.
        const group: Array<{ msg: ChatMessage; idx: number }> = []
        while (i < messages.length) {
          const cm = messages[i] as ChatMessage
          if ((cm.role as string) === 'system' && cm.agentRunId === runId) {
            group.push({ msg: cm, idx: i })
            i++
          } else break
        }
        items.push({ kind: 'run', runId, events: group })
      } else {
        items.push({ kind: 'msg', msg: m as ChatMessage, idx: i })
        i++
      }
    }
    return items
  }, [messages])

  // ── render ────────────────────────────────────────────────────────────────

  const activeSession   = sessions.find((s) => s.id === activeId)
  const agentIsWorking  = streaming && agentPhase !== 'replying'
  const inputPlaceholder = toolsEnabled
    ? 'Give Nexus a task: "Draft Act 2 — three chapters, noir tone, ~1500 words each"'
    : 'Analyze, question, brainstorm…'

  return (
    <div className="flex shrink-0 border-r border-brand-border bg-brand-bg-card" style={{ width: '420px' }}>

      {/* Session list sidebar */}
      <div className="w-36 flex flex-col border-r border-brand-border shrink-0">
        <div className="px-2 py-2 border-b border-brand-border">
          <button
            onClick={handleCreateSession}
            disabled={creatingSession}
            className="w-full flex items-center justify-center gap-1.5 py-1.5 rounded text-xs text-brand-cyan border border-brand-cyan/30 hover:bg-brand-cyan/10 transition-colors disabled:opacity-40"
          >
            <PlusIcon />
            New Session
          </button>
        </div>
        <div className="flex-1 overflow-y-auto py-1">
          {loading ? (
            <div className="flex items-center justify-center py-6">
              <SpinIcon className="w-4 h-4 text-brand-purple animate-spin" />
            </div>
          ) : sessions.length === 0 ? (
            <p className="text-brand-muted text-xs px-3 py-4 text-center leading-relaxed">
              No sessions yet. Create one to begin.
            </p>
          ) : (
            sessions.map((session) => (
              <SessionItem
                key={session.id}
                session={session}
                active={session.id === activeId}
                onClick={() => handleSelectSession(session)}
                onDelete={(e) => handleDelete(session.id, e)}
              />
            ))
          )}
        </div>
      </div>

      {/* Chat area */}
      <div className="flex flex-col flex-1 min-w-0">

        {/* Header */}
        <div className="px-3 py-2.5 border-b border-brand-border flex items-center gap-2 min-h-[44px]">
          <WorkshopIcon />
          {activeSession ? (
            editingTitle ? (
              <input
                ref={titleRef}
                value={titleDraft}
                onChange={(e) => setTitleDraft(e.target.value)}
                onBlur={commitTitle}
                onKeyDown={(e) => { if (e.key === 'Enter') commitTitle(); if (e.key === 'Escape') setEditingTitle(false) }}
                className="flex-1 bg-brand-bg-input border border-brand-border rounded px-2 py-0.5 text-xs text-brand-text focus:outline-none focus:border-brand-cyan"
              />
            ) : (
              <button
                onClick={startEditingTitle}
                className="flex-1 text-left text-xs font-semibold text-brand-text truncate hover:text-brand-cyan transition-colors"
                title="Click to rename"
              >
                {activeSession.title}
              </button>
            )
          ) : (
            <span className="text-xs font-semibold text-brand-purple uppercase tracking-wider">Workshop</span>
          )}

          <div className="flex items-center gap-1 shrink-0">
            <button
              onClick={() => setToolsEnabled((v) => !v)}
              title={toolsEnabled ? 'Agent writes ON — click to disable' : 'Agent writes OFF — click to enable'}
              className={`flex items-center gap-1 px-2 py-0.5 rounded text-[10px] border transition-colors ${
                toolsEnabled
                  ? 'bg-brand-purple/20 text-brand-purple border-brand-purple/40 hover:bg-brand-purple/30'
                  : 'text-brand-muted border-brand-border/50 hover:text-brand-text hover:border-brand-border'
              }`}
            >
              <ToolsIcon />
              {toolsEnabled ? 'Writes ON' : 'Writes OFF'}
            </button>
            {activeSession && (
              <button onClick={handleExport} title="Export to Markdown"
                className="p-1 rounded text-brand-muted hover:text-brand-cyan transition-colors">
                <ExportIcon />
              </button>
            )}
          </div>
        </div>

        {/* Agent mode notice bar */}
        {toolsEnabled && !streaming && (
          <div className="px-3 py-1.5 bg-brand-purple/10 border-b border-brand-purple/20 flex items-center gap-1.5">
            <ToolsIcon className="w-3 h-3 text-brand-purple shrink-0" />
            <span className="text-[10px] text-brand-purple/80">
              Agent mode — give Nexus a high-level task and it will write directly to your manuscript
            </span>
          </div>
        )}

        {/* Agent working status bar — replaces notice during active run */}
        {agentIsWorking && (
          <div className="px-3 py-1.5 bg-brand-purple/15 border-b border-brand-purple/30 flex items-center justify-between gap-2">
            <div className="flex items-center gap-1.5">
              <SpinIcon className="w-3 h-3 text-brand-purple animate-spin shrink-0" />
              <span className="text-[10px] text-brand-purple font-medium">
                {agentPhase === 'planning'
                  ? `Nexus is planning${agentRound > 1 ? ` (round ${agentRound})` : '…'}`
                  : 'Nexus is writing to your manuscript…'}
              </span>
            </div>
            <button
              onClick={() => abortRef.current?.abort()}
              className="shrink-0 flex items-center gap-1 px-2 py-0.5 rounded text-[10px] text-red-400 border border-red-400/30 hover:bg-red-400/10 transition-colors"
              title="Stop — already-written content is kept"
            >
              <StopIcon />
              Stop
            </button>
          </div>
        )}

        {/* Replying status bar */}
        {streaming && agentPhase === 'replying' && (
          <div className="px-3 py-1.5 bg-brand-bg border-b border-brand-border flex items-center justify-between gap-2">
            <div className="flex items-center gap-1.5">
              <SpinIcon className="w-3 h-3 text-brand-cyan animate-spin shrink-0" />
              <span className="text-[10px] text-brand-muted">Nexus is replying…</span>
            </div>
            <button
              onClick={() => abortRef.current?.abort()}
              className="shrink-0 text-brand-muted text-[10px] hover:text-brand-text transition-colors px-1"
            >
              Stop
            </button>
          </div>
        )}

        {/* No session selected */}
        {!activeSession && !loading && (
          <div className="flex-1 flex flex-col items-center justify-center gap-3 px-6 text-center">
            <WorkshopIcon className="w-8 h-8 text-brand-purple/40" />
            <p className="text-brand-muted text-sm">
              Create a session to start a focused craft discussion with Nexus.
            </p>
            <button
              onClick={handleCreateSession}
              disabled={creatingSession}
              className="px-4 py-1.5 rounded text-sm text-brand-cyan border border-brand-cyan/30 hover:bg-brand-cyan/10 transition-colors disabled:opacity-40"
            >
              New Session
            </button>
          </div>
        )}

        {/* Messages */}
        {(activeSession || loading) && (
          <>
            <div className="flex-1 overflow-y-auto px-3 py-3 space-y-3 text-sm">
              {loading ? (
                <div className="flex items-center justify-center py-8">
                  <SpinIcon className="w-4 h-4 text-brand-purple animate-spin" />
                </div>
              ) : (
                displayItems.map((item, di) => {
                  if (item.kind === 'run') {
                    const isCollapsed = collapsed.has(item.runId)
                    const actionCount = item.events.length
                    return (
                      <AgentRunBlock
                        key={item.runId}
                        runId={item.runId}
                        events={item.events}
                        collapsed={isCollapsed}
                        onToggle={() => toggleCollapse(item.runId)}
                        onUndo={(msgIdx, evt) => handleUndo(msgIdx, evt)}
                      />
                    )
                  }

                  const { msg: m, idx: i } = item
                  const isSystem   = (m.role as string) === 'system'
                  const isStreaming = i === messages.length - 1 && m.role === 'assistant' && streaming
                  const isIntro    = m === WORKSHOP_INTRO
                  const showInsert = m.role === 'assistant' && !isStreaming && !isIntro && onInsertToScene && m.content

                  // Ungrouped system message (legacy or non-agent tool event).
                  if (isSystem) {
                    return (
                      <div key={`sys-${di}`} className="flex items-center gap-1.5 py-0.5 px-2 rounded bg-brand-purple/10 border border-brand-purple/20">
                        <ToolsIcon className="w-3 h-3 text-brand-purple shrink-0" />
                        <span className="text-[10px] text-brand-purple/80 leading-relaxed">{m.content}</span>
                      </div>
                    )
                  }

                  return (
                    <div key={`msg-${i}`} className={`group flex gap-2 ${m.role === 'user' ? 'flex-row-reverse' : ''}`}>
                      <div className={`w-6 h-6 rounded-full shrink-0 flex items-center justify-center text-xs font-bold mt-0.5 ${
                        m.role === 'assistant' ? 'bg-brand-purple/30 text-brand-purple' : 'bg-brand-cyan/20 text-brand-cyan'
                      }`}>
                        {m.role === 'assistant' ? 'N' : 'U'}
                      </div>
                      <div className="flex flex-col gap-0.5 min-w-0">
                        <div className={`max-w-[230px] rounded-lg px-3 py-2 leading-relaxed whitespace-pre-wrap ${
                          m.role === 'assistant' ? 'bg-brand-bg text-brand-muted' : 'bg-brand-purple/20 text-brand-text'
                        }`}>
                          {isStreaming && !m.content
                            ? <NexusThinking agentMode={toolsEnabled} />
                            : m.content
                          }
                          {isStreaming && m.content && (
                            <span className="inline-block w-1.5 h-3.5 ml-0.5 bg-brand-cyan/70 animate-pulse align-middle" />
                          )}
                        </div>
                        {showInsert && (
                          <button
                            onClick={() => onInsertToScene(m.content)}
                            className="self-start opacity-0 group-hover:opacity-100 transition-opacity flex items-center gap-1 ml-1 text-[10px] text-brand-muted hover:text-brand-cyan"
                            title="Append to active scene"
                          >
                            <InsertIcon />
                            insert into scene
                          </button>
                        )}
                      </div>
                    </div>
                  )
                })
              )}
              <div ref={bottomRef} />
            </div>

            {/* Input */}
            <div className="px-3 py-3 border-t border-brand-border">
              <div className={`flex items-end gap-2 rounded-lg border px-3 py-2 transition-colors ${
                toolsEnabled
                  ? 'bg-brand-bg-input border-brand-purple/30 focus-within:border-brand-purple/60'
                  : 'bg-brand-bg-input border-brand-border'
              }`}>
                <textarea
                  rows={toolsEnabled ? 2 : 1}
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={handleKey}
                  placeholder={inputPlaceholder}
                  disabled={streaming || !activeSession}
                  className="flex-1 resize-none bg-transparent text-brand-text text-sm placeholder:text-brand-muted focus:outline-none max-h-28 leading-relaxed disabled:opacity-50"
                />
                <button
                  onClick={send}
                  disabled={!input.trim() || streaming || !activeSession}
                  title={toolsEnabled ? 'Execute agent task (Enter)' : 'Send (Enter)'}
                  className={`shrink-0 p-1 rounded disabled:opacity-30 transition-colors ${
                    toolsEnabled
                      ? 'text-brand-purple hover:bg-brand-purple/10'
                      : 'text-brand-cyan hover:bg-brand-cyan/10'
                  }`}
                >
                  {toolsEnabled ? <AgentSendIcon /> : <SendIcon />}
                </button>
              </div>
              <p className="text-brand-muted text-xs mt-1.5 text-center opacity-50">
                Enter to send · Shift+Enter for newline
              </p>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

// ── AgentRunBlock — collapsible group of tool event rows ──────────────────────

function AgentRunBlock({
  runId,
  events,
  collapsed,
  onToggle,
  onUndo,
}: {
  runId:     string
  events:    Array<{ msg: ChatMessage; idx: number }>
  collapsed: boolean
  onToggle:  () => void
  onUndo:    (msgIdx: number, evt: ToolCallEvent) => void
}) {
  const doneCount = events.filter((e) => e.msg.toolUndone).length
  const total     = events.length

  return (
    <div className="rounded border border-brand-purple/20 bg-brand-purple/5 overflow-hidden">
      {/* Header row */}
      <button
        onClick={onToggle}
        className="w-full flex items-center gap-2 px-2 py-1.5 text-left hover:bg-brand-purple/10 transition-colors"
      >
        <span className="text-brand-purple text-[10px]">{collapsed ? '▶' : '▼'}</span>
        <ToolsIcon className="w-3 h-3 text-brand-purple shrink-0" />
        <span className="flex-1 text-[10px] text-brand-purple/80 font-medium">
          Agent actions
        </span>
        <span className="text-[10px] text-brand-purple/60 tabular-nums">
          {doneCount > 0 ? `${total - doneCount}/${total} active` : `${total} action${total !== 1 ? 's' : ''}`}
        </span>
      </button>

      {/* Tool event rows */}
      {!collapsed && (
        <div className="px-2 pb-1.5 space-y-1">
          {events.map(({ msg, idx }) => (
            <ToolEventRow
              key={idx}
              evt={msg.toolEvent!}
              undone={msg.toolUndone ?? false}
              onUndo={() => onUndo(idx, msg.toolEvent!)}
            />
          ))}
        </div>
      )}
    </div>
  )
}

// ── ToolEventRow ──────────────────────────────────────────────────────────────

function toolLabel(evt: ToolCallEvent): string {
  if (evt.is_error) return `Error: ${evt.result}`
  switch (evt.tool) {
    case 'append_to_scene':       return 'Appended to scene'
    case 'replace_scene_content': return 'Replaced scene content'
    case 'create_scene':          return 'Created scene'
    case 'create_chapter':        return 'Created chapter'
    case 'create_act':            return 'Created act'
    default:                      return evt.result
  }
}

function canUndo(evt: ToolCallEvent): boolean {
  if (evt.is_error) return false
  if ((evt.tool === 'append_to_scene' || evt.tool === 'replace_scene_content') && evt.scene_id && evt.chapter_id) return true
  if (evt.created_type === 'scene'   && evt.created_id && evt.chapter_id) return true
  if (evt.created_type === 'chapter' && evt.created_id && evt.act_id && evt.project_id) return true
  if (evt.created_type === 'act'     && evt.created_id && evt.project_id) return true
  return false
}

function ToolEventRow({
  evt,
  undone,
  onUndo,
}: {
  evt:    ToolCallEvent
  undone: boolean
  onUndo: () => void
}) {
  const [undoing, setUndoing] = useState(false)

  const handleUndo = async () => {
    setUndoing(true)
    await onUndo()
    setUndoing(false)
  }

  return (
    <div className={`flex items-center gap-1.5 py-0.5 px-1.5 rounded ${
      evt.is_error ? 'opacity-70' : undone ? 'opacity-40' : ''
    }`}>
      <span className={`shrink-0 text-[10px] ${evt.is_error ? 'text-red-400' : 'text-brand-purple/60'}`}>↳</span>
      <span className={`flex-1 text-[10px] truncate leading-relaxed ${
        evt.is_error ? 'text-red-400' : undone ? 'text-brand-muted line-through' : 'text-brand-purple/80'
      }`}>
        {toolLabel(evt)}
      </span>
      {canUndo(evt) && !undone && (
        <button
          onClick={handleUndo}
          disabled={undoing}
          className="shrink-0 text-[10px] text-brand-muted hover:text-brand-text border border-brand-border/50 hover:border-brand-border rounded px-1.5 py-0.5 transition-colors disabled:opacity-40"
        >
          {undoing ? '…' : 'Undo'}
        </button>
      )}
      {undone && (
        <span className="shrink-0 text-[10px] text-brand-muted italic">undone</span>
      )}
    </div>
  )
}

// ── SessionItem ───────────────────────────────────────────────────────────────

function SessionItem({
  session, active, onClick, onDelete,
}: {
  session:  WorkshopSession
  active:   boolean
  onClick:  () => void
  onDelete: (e: React.MouseEvent) => void
}) {
  const [hovered, setHovered] = useState(false)
  const [now] = useState(Date.now)

  const relTime = useMemo(() => {
    const diff = now - new Date(session.updated_at).getTime()
    const mins = Math.floor(diff / 60_000)
    if (mins < 1) return 'just now'
    if (mins < 60) return `${mins}m ago`
    const hrs = Math.floor(mins / 60)
    if (hrs < 24) return `${hrs}h ago`
    return `${Math.floor(hrs / 24)}d ago`
  }, [now, session.updated_at])

  return (
    <div
      onClick={onClick}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      className={`relative flex flex-col px-2.5 py-2 mx-1 rounded cursor-pointer transition-colors group ${
        active
          ? 'bg-brand-purple/20 border-l-2 border-brand-purple pl-2'
          : 'hover:bg-brand-bg border-l-2 border-transparent'
      }`}
    >
      <span className={`text-xs font-medium truncate leading-snug ${active ? 'text-brand-text' : 'text-brand-muted'}`}>
        {session.title}
      </span>
      <span className="text-brand-muted text-[10px] mt-0.5 opacity-60">{relTime}</span>
      {hovered && (
        <button
          onClick={onDelete}
          title="Delete session"
          className="absolute right-1.5 top-1/2 -translate-y-1/2 p-0.5 rounded text-brand-muted hover:text-red-400 transition-colors"
        >
          <TrashIcon />
        </button>
      )}
    </div>
  )
}

// ── icons ─────────────────────────────────────────────────────────────────────

function WorkshopIcon({ className }: { className?: string }) {
  return (
    <svg className={className ?? 'w-4 h-4 text-brand-purple'} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="2" width="5" height="5" rx="1" />
      <rect x="9" y="2" width="5" height="5" rx="1" />
      <rect x="2" y="9" width="5" height="5" rx="1" />
      <path d="M9 11.5h5M11.5 9v5" />
    </svg>
  )
}

function PlusIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <path d="M6 2v8M2 6h8" />
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

function AgentSendIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2 8h12M10 4l4 4-4 4" />
      <circle cx="4" cy="8" r="1.5" fill="currentColor" stroke="none" />
    </svg>
  )
}

function StopIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 12 12" fill="currentColor">
      <rect x="2" y="2" width="8" height="8" rx="1" />
    </svg>
  )
}

function ExportIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 2v8M5 7l3 3 3-3" />
      <path d="M3 12h10" />
    </svg>
  )
}

function TrashIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 4h10M6 4V3h4v1M5 4l.5 9h5L11 4" />
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

function InsertIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M6 1v7M3 5l3 3 3-3" />
      <path d="M2 10h8" />
    </svg>
  )
}

function ToolsIcon({ className }: { className?: string }) {
  return (
    <svg className={className ?? 'w-3 h-3'} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9.5 2a3.5 3.5 0 013.34 4.54l-1.17-1.17-1.41 1.41 1.17 1.17A3.5 3.5 0 119.5 2z" />
      <path d="M6.5 7.5l-4 4a1 1 0 001.42 1.42l4-4" />
    </svg>
  )
}
