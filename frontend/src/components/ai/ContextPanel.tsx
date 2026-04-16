// ContextPanel — writer-curated AI context pins.
// Writers can pin wiki entities, chapters, or scenes so Nexus always has
// specific information in scope during every AI call in this session.
import { useState, useEffect, useCallback } from 'react'
import { api } from '@/services/api'
import type { ContextPin, ContextPinType, ContextPinMode, WikiEntity, Chapter } from '@/services/api'

interface ContextPanelProps {
  token:     string
  projectId: string
}

// ── pin type badge colours ────────────────────────────────────────────────────
const TYPE_BADGE: Record<ContextPinType, string> = {
  entity:  'bg-brand-cyan/15 text-brand-cyan',
  chapter: 'bg-brand-purple/15 text-brand-purple',
  scene:   'bg-brand-gold/15 text-brand-gold',
}

const TYPE_LABEL: Record<ContextPinType, string> = {
  entity:  'Entity',
  chapter: 'Chapter',
  scene:   'Scene',
}

// ── search tab ────────────────────────────────────────────────────────────────
type SearchTab = ContextPinType

// ── helpers ───────────────────────────────────────────────────────────────────

function PinBadge({ type }: { type: ContextPinType }) {
  return (
    <span className={`text-[10px] font-semibold px-1.5 py-0.5 rounded uppercase tracking-wide ${TYPE_BADGE[type]}`}>
      {TYPE_LABEL[type]}
    </span>
  )
}

function ModeToggle({
  mode,
  onChange,
}: {
  mode: ContextPinMode
  onChange: (m: ContextPinMode) => void
}) {
  return (
    <button
      onClick={() => onChange(mode === 'summary' ? 'full' : 'summary')}
      title={mode === 'summary' ? 'Switch to full content' : 'Switch to summary'}
      className="text-[10px] px-1.5 py-0.5 rounded border border-brand-border text-brand-muted hover:text-brand-text transition-colors"
    >
      {mode === 'summary' ? 'sum' : 'full'}
    </button>
  )
}

// ── main component ────────────────────────────────────────────────────────────

export default function ContextPanel({ token, projectId }: ContextPanelProps) {
  const [pins,       setPins]       = useState<ContextPin[]>([])
  const [loading,    setLoading]    = useState(true)
  const [error,      setError]      = useState<string | null>(null)
  const [searchTab,  setSearchTab]  = useState<SearchTab>('entity')
  const [searchQ,    setSearchQ]    = useState('')
  const [adding,     setAdding]     = useState(false)

  // Searchable candidate lists
  const [entities,  setEntities]  = useState<WikiEntity[]>([])
  const [chapters,  setChapters]  = useState<Chapter[]>([])
  const [scenes,    setScenes]    = useState<{ id: string; title: string; chapterTitle: string }[]>([])
  const [listsLoaded, setListsLoaded] = useState(false)

  // ── load pins ───────────────────────────────────────────────────────────────

  const loadPins = useCallback(async () => {
    try {
      const data = await api.ai.contextPins.list(token, projectId)
      setPins(data)
    } catch {
      setError('Failed to load context pins')
    } finally {
      setLoading(false)
    }
  }, [token, projectId])

  useEffect(() => { loadPins() }, [loadPins])

  // ── load candidate lists (deferred until "Add" is opened) ──────────────────

  const loadCandidates = useCallback(async () => {
    if (listsLoaded) return
    try {
      const [rawEntities, rawActs] = await Promise.all([
        api.wiki.listEntities(token, projectId),
        api.acts.list(token, projectId),
      ])
      setEntities(rawEntities)

      // Flatten acts → chapters, and chapters → scenes
      const allChapters: Chapter[] = []
      const allScenes: { id: string; title: string; chapterTitle: string }[] = []

      for (const act of rawActs) {
        const chs = await api.chapters.list(token, projectId, act.id)
        for (const ch of chs) {
          allChapters.push(ch)
          const scs = await api.scenes.list(token, ch.id)
          for (const sc of scs) {
            allScenes.push({ id: sc.id, title: sc.title || 'Untitled scene', chapterTitle: ch.title })
          }
        }
      }

      setChapters(allChapters)
      setScenes(allScenes)
      setListsLoaded(true)
    } catch {
      // Non-fatal — search will just be empty
    }
  }, [token, projectId, listsLoaded])

  const handleOpenAdd = () => {
    setAdding(true)
    loadCandidates()
  }

  // ── pin actions ─────────────────────────────────────────────────────────────

  const handleAdd = async (pinType: ContextPinType, refId: string) => {
    try {
      const pin = await api.ai.contextPins.create(token, projectId, pinType, refId, 'summary')
      setPins((prev) => {
        // Upsert: replace if ref_id+type already present (server did ON CONFLICT update)
        const without = prev.filter((p) => !(p.pin_type === pin.pin_type && p.ref_id === pin.ref_id))
        return [...without, pin]
      })
    } catch {
      // ignore — don't interrupt the UX for a failed pin
    }
  }

  const handleDelete = async (pinId: string) => {
    try {
      await api.ai.contextPins.delete(token, projectId, pinId)
      setPins((prev) => prev.filter((p) => p.id !== pinId))
    } catch {
      setError('Failed to remove pin')
    }
  }

  const handleModeToggle = async (pin: ContextPin, newMode: ContextPinMode) => {
    // Re-create with the new mode (server uses ON CONFLICT DO UPDATE)
    try {
      const updated = await api.ai.contextPins.create(token, projectId, pin.pin_type, pin.ref_id, newMode)
      setPins((prev) => prev.map((p) => (p.id === pin.id ? updated : p)))
    } catch {
      setError('Failed to update pin mode')
    }
  }

  // ── filtered search results ─────────────────────────────────────────────────

  const q = searchQ.toLowerCase()

  const filteredEntities = entities.filter(
    (e) => !q || e.name.toLowerCase().includes(q) || e.type.toLowerCase().includes(q),
  )
  const filteredChapters = chapters.filter(
    (c) => !q || c.title.toLowerCase().includes(q),
  )
  const filteredScenes = scenes.filter(
    (s) => !q || s.title.toLowerCase().includes(q) || s.chapterTitle.toLowerCase().includes(q),
  )

  const isPinned = (pinType: ContextPinType, refId: string) =>
    pins.some((p) => p.pin_type === pinType && p.ref_id === refId)

  // ── render ──────────────────────────────────────────────────────────────────

  if (loading) {
    return (
      <div className="flex items-center justify-center h-24 text-brand-muted text-sm">
        Loading…
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full text-sm text-brand-text overflow-hidden">

      {/* ── header ── */}
      <div className="px-3 pt-3 pb-2 border-b border-brand-border shrink-0">
        <div className="flex items-center justify-between">
          <span className="font-semibold text-brand-text">Context Pins</span>
          {!adding && (
            <button
              onClick={handleOpenAdd}
              className="text-brand-cyan hover:text-brand-cyan/80 text-xs font-medium transition-colors"
            >
              + Add
            </button>
          )}
          {adding && (
            <button
              onClick={() => { setAdding(false); setSearchQ('') }}
              className="text-brand-muted hover:text-brand-text text-xs transition-colors"
            >
              Done
            </button>
          )}
        </div>
        <p className="text-[11px] text-brand-muted mt-0.5">
          Pinned items are injected into every Nexus AI call.
        </p>
      </div>

      {error && (
        <div className="mx-3 mt-2 text-xs text-red-400">{error}</div>
      )}

      {/* ── pin list ── */}
      <div className="flex-1 overflow-y-auto">
        {!adding && (
          <div className="px-3 py-2 space-y-1">
            {pins.length === 0 && (
              <p className="text-brand-muted text-xs py-4 text-center">
                No pins yet. Click "Add" to pin entities, chapters, or scenes.
              </p>
            )}
            {pins.map((pin) => (
              <div
                key={pin.id}
                className="flex items-center gap-2 py-1.5 border-b border-brand-border/40 last:border-0"
              >
                <PinBadge type={pin.pin_type} />
                <span className="flex-1 truncate text-xs text-brand-text" title={pin.label}>
                  {pin.label || pin.ref_id}
                </span>
                <ModeToggle
                  mode={pin.include_mode}
                  onChange={(m) => handleModeToggle(pin, m)}
                />
                <button
                  onClick={() => handleDelete(pin.id)}
                  title="Remove pin"
                  className="text-brand-muted hover:text-red-400 transition-colors ml-1 shrink-0"
                >
                  <XIcon />
                </button>
              </div>
            ))}
          </div>
        )}

        {/* ── add pane ── */}
        {adding && (
          <div className="flex flex-col h-full">
            {/* type tabs */}
            <div className="flex border-b border-brand-border shrink-0">
              {(['entity', 'chapter', 'scene'] as SearchTab[]).map((tab) => (
                <button
                  key={tab}
                  onClick={() => { setSearchTab(tab); setSearchQ('') }}
                  className={`flex-1 py-1.5 text-xs font-medium capitalize transition-colors ${
                    searchTab === tab
                      ? 'text-brand-cyan border-b-2 border-brand-cyan'
                      : 'text-brand-muted hover:text-brand-text'
                  }`}
                >
                  {tab}
                </button>
              ))}
            </div>

            {/* search input */}
            <div className="px-3 py-2 shrink-0">
              <input
                type="text"
                placeholder={`Search ${searchTab}s…`}
                value={searchQ}
                onChange={(e) => setSearchQ(e.target.value)}
                className="w-full bg-brand-surface border border-brand-border rounded px-2 py-1 text-xs text-brand-text placeholder:text-brand-muted focus:outline-none focus:border-brand-cyan"
              />
            </div>

            {/* search results */}
            <div className="flex-1 overflow-y-auto px-3 pb-3 space-y-0.5">
              {searchTab === 'entity' && filteredEntities.map((e) => (
                <SearchRow
                  key={e.id}
                  label={e.name}
                  sub={e.type}
                  pinned={isPinned('entity', e.id)}
                  onAdd={() => handleAdd('entity', e.id)}
                />
              ))}

              {searchTab === 'chapter' && filteredChapters.map((c) => (
                <SearchRow
                  key={c.id}
                  label={c.title}
                  pinned={isPinned('chapter', c.id)}
                  onAdd={() => handleAdd('chapter', c.id)}
                />
              ))}

              {searchTab === 'scene' && filteredScenes.map((s) => (
                <SearchRow
                  key={s.id}
                  label={s.title}
                  sub={s.chapterTitle}
                  pinned={isPinned('scene', s.id)}
                  onAdd={() => handleAdd('scene', s.id)}
                />
              ))}

              {searchTab === 'entity'  && filteredEntities.length  === 0 && <Empty />}
              {searchTab === 'chapter' && filteredChapters.length  === 0 && <Empty />}
              {searchTab === 'scene'   && filteredScenes.length    === 0 && <Empty />}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// ── sub-components ────────────────────────────────────────────────────────────

function SearchRow({
  label,
  sub,
  pinned,
  onAdd,
}: {
  label: string
  sub?: string
  pinned: boolean
  onAdd: () => void
}) {
  return (
    <div className="flex items-center gap-2 py-1.5 border-b border-brand-border/30 last:border-0">
      <div className="flex-1 min-w-0">
        <div className="text-xs text-brand-text truncate">{label}</div>
        {sub && <div className="text-[10px] text-brand-muted truncate capitalize">{sub}</div>}
      </div>
      <button
        onClick={onAdd}
        disabled={pinned}
        title={pinned ? 'Already pinned' : 'Pin this'}
        className={`shrink-0 text-xs px-2 py-0.5 rounded transition-colors ${
          pinned
            ? 'text-brand-muted cursor-default'
            : 'text-brand-cyan hover:text-brand-cyan/80'
        }`}
      >
        {pinned ? '✓' : '+'}
      </button>
    </div>
  )
}

function Empty() {
  return <p className="text-xs text-brand-muted text-center py-4">Nothing found</p>
}

function XIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
      <path fillRule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clipRule="evenodd" />
    </svg>
  )
}
