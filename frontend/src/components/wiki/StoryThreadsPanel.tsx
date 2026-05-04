// StoryThreadsPanel — create, view, and manage story threads (arcs, mysteries, conflicts, world threads).
import { useState, useEffect, useCallback } from 'react'
import { api } from '@/services/api'
import type { StoryThread, ThreadType } from '@/services/api'

const THREAD_TYPES: { value: ThreadType; label: string; color: string; description: string }[] = [
  { value: 'arc',      label: 'Arc',      color: 'text-violet-400 bg-violet-900/40 border-violet-700', description: 'A character or thematic journey across the story' },
  { value: 'mystery',  label: 'Mystery',  color: 'text-amber-400 bg-amber-900/40 border-amber-700',   description: 'An unanswered question the reader is meant to follow' },
  { value: 'conflict', label: 'Conflict', color: 'text-red-400 bg-red-900/40 border-red-700',         description: 'An ongoing tension or struggle between forces' },
  { value: 'world',    label: 'World',    color: 'text-cyan-400 bg-cyan-900/40 border-cyan-700',      description: 'A world-building element that surfaces and recedes' },
]

function typeMeta(t: ThreadType) {
  return THREAD_TYPES.find((x) => x.value === t) ?? THREAD_TYPES[0]
}

interface Props {
  token: string
  projectId: string
}

interface CreateFormProps {
  token: string
  projectId: string
  onCreated: (t: StoryThread) => void
  onCancel: () => void
}

function CreateForm({ token, projectId, onCreated, onCancel }: CreateFormProps) {
  const [title, setTitle] = useState('')
  const [type, setType] = useState<ThreadType>('arc')
  const [saving, setSaving] = useState(false)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!title.trim()) return
    setSaving(true)
    try {
      const t = await api.threads.create(token, projectId, { title: title.trim(), type })
      onCreated(t)
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={submit} className="p-4 border-b border-gray-700 space-y-3">
      <input
        autoFocus
        className="w-full bg-gray-800 text-sm text-white rounded px-3 py-2 border border-gray-600 focus:outline-none focus:border-indigo-500"
        placeholder="Thread title…"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
      />
      <div className="flex flex-wrap gap-2">
        {THREAD_TYPES.map((tt) => (
          <button
            key={tt.value}
            type="button"
            onClick={() => setType(tt.value)}
            className={`text-xs px-2 py-1 rounded border ${type === tt.value ? tt.color : 'text-gray-400 bg-gray-800 border-gray-600'}`}
          >
            {tt.label}
          </button>
        ))}
      </div>
      <div className="flex gap-2 justify-end">
        <button type="button" onClick={onCancel} className="text-xs text-gray-400 hover:text-white px-3 py-1">
          Cancel
        </button>
        <button
          type="submit"
          disabled={saving || !title.trim()}
          className="text-xs bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white px-3 py-1 rounded"
        >
          {saving ? 'Creating…' : 'Create thread'}
        </button>
      </div>
    </form>
  )
}

interface ThreadDetailProps {
  thread: StoryThread
  token: string
  projectId: string
  onUpdated: (t: StoryThread) => void
  onDeleted: (id: string) => void
  onBack: () => void
}

function ThreadDetail({ thread, token, projectId, onUpdated, onDeleted, onBack }: ThreadDetailProps) {
  const [title, setTitle] = useState(thread.title)
  const [type, setType] = useState<ThreadType>(thread.type as ThreadType)
  const [notes, setNotes] = useState(thread.notes)
  const [saving, setSaving] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  const meta = typeMeta(type)
  const isClosed = !!thread.closed_at_scene_id

  async function save() {
    setSaving(true)
    try {
      const updated = await api.threads.update(token, projectId, thread.id, { title, type, notes })
      onUpdated(updated)
    } finally {
      setSaving(false)
    }
  }

  async function toggleClose() {
    const updated = await api.threads.update(token, projectId, thread.id, {
      closed_at_scene_id: isClosed ? null : 'manual',
    })
    onUpdated(updated)
  }

  async function deleteThread() {
    await api.threads.delete(token, projectId, thread.id)
    onDeleted(thread.id)
  }

  return (
    <div className="flex flex-col h-full overflow-y-auto">
      <div className="flex items-center gap-2 p-3 border-b border-gray-700">
        <button onClick={onBack} className="text-gray-400 hover:text-white text-xs">← Back</button>
        <span className={`text-xs px-2 py-0.5 rounded border ${meta.color}`}>{meta.label}</span>
        {isClosed && <span className="text-xs text-gray-500 ml-auto">Resolved</span>}
      </div>

      <div className="p-4 space-y-4 flex-1">
        {/* Title */}
        <div>
          <label className="block text-xs text-gray-400 mb-1">Title</label>
          <input
            className="w-full bg-gray-800 text-sm text-white rounded px-3 py-2 border border-gray-600 focus:outline-none focus:border-indigo-500"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onBlur={save}
          />
        </div>

        {/* Type */}
        <div>
          <label className="block text-xs text-gray-400 mb-1">Type</label>
          <div className="flex flex-wrap gap-2">
            {THREAD_TYPES.map((tt) => (
              <button
                key={tt.value}
                onClick={() => { setType(tt.value as ThreadType); save() }}
                className={`text-xs px-2 py-1 rounded border ${type === tt.value ? tt.color : 'text-gray-400 bg-gray-800 border-gray-600'}`}
                title={tt.description}
              >
                {tt.label}
              </button>
            ))}
          </div>
          <p className="text-xs text-gray-500 mt-1">{meta.description}</p>
        </div>

        {/* Notes */}
        <div>
          <label className="block text-xs text-gray-400 mb-1">Notes</label>
          <textarea
            rows={6}
            className="w-full bg-gray-800 text-sm text-white rounded px-3 py-2 border border-gray-600 focus:outline-none focus:border-indigo-500 resize-none"
            placeholder="Describe this thread: what opens it, key beats, how it might resolve…"
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            onBlur={save}
          />
        </div>

        {/* Opened / Closed scene IDs (read-only display) */}
        {(thread.opened_at_scene_id || thread.closed_at_scene_id) && (
          <div className="text-xs text-gray-500 space-y-1">
            {thread.opened_at_scene_id && <p>Opened in scene <span className="font-mono">{thread.opened_at_scene_id.slice(0, 8)}</span></p>}
            {thread.closed_at_scene_id && <p>Resolved in scene <span className="font-mono">{thread.closed_at_scene_id.slice(0, 8)}</span></p>}
          </div>
        )}

        {saving && <p className="text-xs text-gray-500">Saving…</p>}
      </div>

      {/* Footer actions */}
      <div className="p-4 border-t border-gray-700 flex items-center justify-between">
        <button
          onClick={toggleClose}
          className={`text-xs px-3 py-1 rounded border ${isClosed ? 'text-green-400 border-green-700 hover:bg-green-900/30' : 'text-gray-400 border-gray-600 hover:text-white hover:border-gray-400'}`}
        >
          {isClosed ? 'Re-open thread' : 'Mark resolved'}
        </button>

        {confirmDelete ? (
          <div className="flex items-center gap-2">
            <span className="text-xs text-gray-400">Delete?</span>
            <button onClick={deleteThread} className="text-xs text-red-400 hover:text-red-300 px-2 py-1 rounded border border-red-700">Yes</button>
            <button onClick={() => setConfirmDelete(false)} className="text-xs text-gray-400 hover:text-white px-2 py-1">No</button>
          </div>
        ) : (
          <button onClick={() => setConfirmDelete(true)} className="text-xs text-gray-500 hover:text-red-400">
            Delete
          </button>
        )}
      </div>
    </div>
  )
}

export default function StoryThreadsPanel({ token, projectId }: Props) {
  const [threads, setThreads] = useState<StoryThread[]>([])
  const [selected, setSelected] = useState<StoryThread | null>(null)
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [filter, setFilter] = useState<ThreadType | 'all'>('all')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await api.threads.list(token, projectId)
      setThreads(data ?? [])
    } finally {
      setLoading(false)
    }
  }, [token, projectId])

  useEffect(() => { load() }, [load])

  function handleCreated(t: StoryThread) {
    setThreads((prev) => [...prev, t])
    setCreating(false)
    setSelected(t)
  }

  function handleUpdated(t: StoryThread) {
    setThreads((prev) => prev.map((x) => (x.id === t.id ? t : x)))
    setSelected(t)
  }

  function handleDeleted(id: string) {
    setThreads((prev) => prev.filter((x) => x.id !== id))
    setSelected(null)
  }

  if (selected) {
    return (
      <ThreadDetail
        thread={selected}
        token={token}
        projectId={projectId}
        onUpdated={handleUpdated}
        onDeleted={handleDeleted}
        onBack={() => setSelected(null)}
      />
    )
  }

  const visible = filter === 'all' ? threads : threads.filter((t) => t.type === filter)
  const open   = visible.filter((t) => !t.closed_at_scene_id)
  const closed = visible.filter((t) => !!t.closed_at_scene_id)

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between p-3 border-b border-gray-700">
        <span className="text-xs font-medium text-gray-300">Story Threads</span>
        <button
          onClick={() => setCreating(true)}
          className="text-xs bg-indigo-600 hover:bg-indigo-500 text-white px-2 py-1 rounded"
        >
          + New
        </button>
      </div>

      {creating && (
        <CreateForm
          token={token}
          projectId={projectId}
          onCreated={handleCreated}
          onCancel={() => setCreating(false)}
        />
      )}

      {/* Type filter */}
      <div className="flex gap-1 px-3 py-2 flex-wrap">
        <button
          onClick={() => setFilter('all')}
          className={`text-xs px-2 py-0.5 rounded border ${filter === 'all' ? 'text-white bg-gray-700 border-gray-500' : 'text-gray-500 border-gray-700 hover:text-gray-300'}`}
        >
          All
        </button>
        {THREAD_TYPES.map((tt) => (
          <button
            key={tt.value}
            onClick={() => setFilter(tt.value)}
            className={`text-xs px-2 py-0.5 rounded border ${filter === tt.value ? tt.color : 'text-gray-500 border-gray-700 hover:text-gray-300'}`}
          >
            {tt.label}
          </button>
        ))}
      </div>

      {/* List */}
      <div className="flex-1 overflow-y-auto px-3 pb-4 space-y-4">
        {loading ? (
          <p className="text-xs text-gray-500 py-4 text-center">Loading…</p>
        ) : threads.length === 0 ? (
          <div className="text-center py-8 space-y-2">
            <p className="text-sm text-gray-400">No threads yet.</p>
            <p className="text-xs text-gray-500">Track arcs, mysteries, conflicts, and world elements that span multiple scenes.</p>
          </div>
        ) : (
          <>
            {open.length > 0 && (
              <div>
                <p className="text-xs text-gray-500 mb-2 uppercase tracking-wider">Open</p>
                <ul className="space-y-1">
                  {open.map((t) => <ThreadRow key={t.id} thread={t} onClick={() => setSelected(t)} />)}
                </ul>
              </div>
            )}
            {closed.length > 0 && (
              <div>
                <p className="text-xs text-gray-500 mb-2 uppercase tracking-wider">Resolved</p>
                <ul className="space-y-1">
                  {closed.map((t) => <ThreadRow key={t.id} thread={t} onClick={() => setSelected(t)} />)}
                </ul>
              </div>
            )}
            {visible.length === 0 && (
              <p className="text-xs text-gray-500 py-4 text-center">No {filter} threads.</p>
            )}
          </>
        )}
      </div>
    </div>
  )
}

function ThreadRow({ thread, onClick }: { thread: StoryThread; onClick: () => void }) {
  const meta = typeMeta(thread.type as ThreadType)
  const isClosed = !!thread.closed_at_scene_id
  return (
    <li>
      <button
        onClick={onClick}
        className={`w-full text-left px-3 py-2 rounded flex items-center gap-2 hover:bg-gray-700/60 ${isClosed ? 'opacity-50' : ''}`}
      >
        <span className={`text-xs px-1.5 py-0.5 rounded border flex-shrink-0 ${meta.color}`}>{meta.label}</span>
        <span className="text-sm text-gray-200 truncate">{thread.title}</span>
        {isClosed && <span className="ml-auto text-xs text-gray-600">✓</span>}
      </button>
    </li>
  )
}
