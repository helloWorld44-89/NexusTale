import { useEffect, useState } from 'react'
import { api, type WikiEntity } from '@/services/api'

interface EntityHoverCardProps {
  entityId:    string
  entityName:  string
  entityType:  string
  x:           number
  y:           number
  token:       string
  projectId:   string
  onNavigate:  (entityId: string) => void
  onMouseEnter: () => void
  onMouseLeave: () => void
}

const TYPE_LABEL: Record<string, string> = {
  character: 'Character',
  location:  'Location',
  faction:   'Faction',
  item:      'Item',
  concept:   'Concept',
  lore:      'Lore',
}

const TYPE_COLOR: Record<string, string> = {
  character: 'text-brand-cyan   bg-brand-cyan/10',
  location:  'text-brand-gold   bg-brand-gold/10',
  faction:   'text-brand-purple bg-brand-purple/10',
  item:      'text-emerald-400  bg-emerald-400/10',
  concept:   'text-sky-400      bg-sky-400/10',
  lore:      'text-rose-400     bg-rose-400/10',
}

export default function EntityHoverCard({
  entityId, entityName, entityType, x, y,
  token, projectId, onNavigate, onMouseEnter, onMouseLeave,
}: EntityHoverCardProps) {
  const [entity, setEntity] = useState<WikiEntity | null>(null)

  useEffect(() => {
    let cancelled = false
    setEntity(null)
    api.wiki.getEntity(token, projectId, entityId)
      .then(e => { if (!cancelled) setEntity(e) })
      .catch(() => {})
    return () => { cancelled = true }
  }, [entityId, token, projectId])

  // Flip below the viewport edge if the card would overflow the top.
  const top = y < 160 ? y + 36 : y - 8

  const colorCls = TYPE_COLOR[entityType] ?? 'text-brand-muted bg-brand-border/40'

  return (
    <div
      className="entity-hover-card fixed z-50 w-52 bg-brand-bg-card border border-brand-border rounded-xl shadow-2xl p-3 space-y-2"
      style={{ left: Math.min(x, window.innerWidth - 224), top }}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
    >
      <div className="flex items-start justify-between gap-2">
        <span className="font-semibold text-brand-text text-[13px] leading-snug">{entityName}</span>
        <span className={`shrink-0 text-[9px] font-semibold uppercase tracking-wide px-1.5 py-0.5 rounded ${colorCls}`}>
          {TYPE_LABEL[entityType] ?? entityType}
        </span>
      </div>

      {entity ? (
        <>
          {entity.image_url && (
            <img
              src={entity.image_url}
              alt={entity.name}
              className="w-full h-16 object-cover rounded-lg"
            />
          )}
          {entity.summary && (
            <p className="text-brand-muted text-[11px] leading-relaxed line-clamp-3">{entity.summary}</p>
          )}
          {!entity.summary && !entity.image_url && (
            <p className="text-brand-muted/50 text-[11px] italic">No description yet.</p>
          )}
        </>
      ) : (
        <p className="text-brand-muted/50 text-[11px]">Loading…</p>
      )}

      <button
        onClick={() => onNavigate(entityId)}
        className="text-brand-cyan text-[11px] hover:underline transition-colors"
      >
        Open in Wiki →
      </button>
    </div>
  )
}
