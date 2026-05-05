// MentionsBar — thin chip row below ScribeEditor showing auto-detected wiki
// entity mentions in the active scene. Chips are type-colored; right-click
// suppresses a tag; "Clear all" suppresses every mention on this scene.
// When a `controlled` prop is provided the parent manages mention state and
// API calls; otherwise the component self-loads.
import { useState, useEffect, useCallback } from 'react'
import { api, type MentionResponse } from '@/services/api'

const TYPE_COLORS: Record<string, string> = {
  character: 'text-brand-cyan   bg-brand-cyan/10   border-brand-cyan/20',
  location:  'text-brand-gold   bg-brand-gold/10   border-brand-gold/20',
  faction:   'text-brand-purple bg-brand-purple/10 border-brand-purple/20',
  item:      'text-emerald-400  bg-emerald-400/10  border-emerald-400/20',
  concept:   'text-sky-400      bg-sky-400/10      border-sky-400/20',
  lore:      'text-rose-400     bg-rose-400/10     border-rose-400/20',
}

interface ControlledMentions {
  mentions:      MentionResponse[]
  onSuppressOne: (id: string) => void
  onSuppressAll: () => void
}

interface MentionsBarProps {
  token:     string
  projectId: string
  sceneId:   string
  branch?:   string
  // Provide to opt into parent-controlled mode (no self-loading).
  controlled?: ControlledMentions
  onNavigateToEntity?: (entityId: string) => void
}

export default function MentionsBar({
  token, projectId, sceneId, branch = 'canon', controlled, onNavigateToEntity,
}: MentionsBarProps) {
  const [selfMentions, setSelfMentions]     = useState<MentionResponse[]>([])
  const [contextMenu, setContextMenu]       = useState<{ x: number; y: number; mentionId: string } | null>(null)

  const isControlled = controlled !== undefined
  const mentions     = isControlled ? controlled.mentions : selfMentions

  const load = useCallback(async () => {
    if (isControlled) return
    try {
      const res = await api.wiki.mentions.list(token, projectId, sceneId, branch)
      setSelfMentions(res.mentions ?? [])
    } catch {
      setSelfMentions([])
    }
  }, [token, projectId, sceneId, branch, isControlled])

  useEffect(() => {
    load()
  }, [load])

  function handleContextMenu(e: React.MouseEvent, mentionId: string) {
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY, mentionId })
  }

  async function handleSuppress(mentionId: string) {
    setContextMenu(null)
    if (isControlled) {
      controlled.onSuppressOne(mentionId)
      return
    }
    setSelfMentions((prev) => prev.filter((m) => m.id !== mentionId))
    try {
      await api.wiki.mentions.suppress(token, projectId, sceneId, mentionId)
    } catch {
      load()
    }
  }

  async function handleClearAll() {
    if (isControlled) {
      controlled.onSuppressAll()
      return
    }
    setSelfMentions([])
    try {
      await api.wiki.mentions.suppressAll(token, projectId, sceneId, branch)
    } catch {
      load()
    }
  }

  if (mentions.length === 0) return null

  return (
    <>
      <div className="flex items-center gap-1.5 px-8 py-1.5 border-t border-brand-border/40 bg-brand-bg flex-wrap">
        <span className="text-[10px] text-brand-muted uppercase tracking-wider font-semibold shrink-0 mr-0.5">
          In this scene
        </span>
        {mentions.map((m) => (
          <button
            key={m.id}
            title={`${m.entity_type} — right-click to remove tag`}
            onContextMenu={(e) => handleContextMenu(e, m.id)}
            onClick={() => onNavigateToEntity?.(m.entity_id)}
            className={`px-2 py-0.5 rounded-full text-[11px] font-medium border transition-opacity hover:opacity-80 ${TYPE_COLORS[m.entity_type] ?? 'text-brand-muted bg-brand-border/40 border-brand-border'}`}
          >
            {m.match_text}
          </button>
        ))}
        <button
          onClick={handleClearAll}
          className="ml-auto text-[10px] text-brand-muted hover:text-brand-text transition-colors shrink-0"
          title="Suppress all entity tags on this scene"
        >
          Clear all
        </button>
      </div>

      {contextMenu && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setContextMenu(null)} />
          <div
            className="fixed z-50 bg-brand-bg-card border border-brand-border rounded-lg shadow-xl py-1 min-w-[160px]"
            style={{ left: contextMenu.x, top: contextMenu.y }}
          >
            <button
              onClick={() => handleSuppress(contextMenu.mentionId)}
              className="w-full text-left px-3 py-1.5 text-xs text-brand-text hover:bg-brand-border/30 transition-colors"
            >
              Remove tag
            </button>
          </div>
        </>
      )}
    </>
  )
}
