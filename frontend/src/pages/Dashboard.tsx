import { useEffect, useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'
import { useProjectStore } from '@/app/store/projectStore'
import { ApiError } from '@/services/api'
import type { Project } from '@/services/api'

export default function Dashboard() {
  const navigate   = useNavigate()
  const { user, accessToken, logout } = useAuthStore((s) => ({ user: s.user, accessToken: s.accessToken, logout: s.logout }))
  const { projects, loading, error, fetch, create } = useProjectStore()

  const [showCreate, setShowCreate] = useState(false)

  useEffect(() => {
    if (accessToken) fetch(accessToken)
  }, [accessToken])

  const handleLogout = async () => {
    await logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="min-h-screen bg-brand-bg flex flex-col">
      {/* Header */}
      <header className="h-14 flex items-center justify-between px-6 bg-brand-bg-card border-b border-brand-border shrink-0">
        <div className="flex items-center gap-2.5">
          <img src="/app-icon.png" alt="" className="w-5 h-5 opacity-80" />
          <span className="text-brand-cyan font-semibold tracking-wide">NexusTale</span>
        </div>
        <div className="flex items-center gap-4">
          {user && (
            <span className="text-brand-muted text-sm">{user.display_name}</span>
          )}
          <Link to="/settings" className="text-sm text-brand-muted hover:text-brand-text transition-colors">
            Settings
          </Link>
          <button onClick={handleLogout} className="text-sm text-brand-muted hover:text-brand-text transition-colors">
            Sign out
          </button>
        </div>
      </header>

      {/* Content */}
      <main className="flex-1 max-w-6xl w-full mx-auto px-6 py-10">
        {/* Page heading */}
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold text-brand-text">Your Stories</h1>
            <p className="text-brand-muted text-sm mt-1">
              {projects.length === 0 && !loading ? 'No stories yet — create your first.' : `${projects.length} project${projects.length === 1 ? '' : 's'}`}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Link
              to="/import"
              className="flex items-center gap-1.5 px-3 py-2 rounded-lg border border-brand-border text-brand-muted text-sm hover:text-brand-text hover:border-brand-purple/60 transition-colors"
            >
              <ImportIcon />
              Import
            </Link>
            <button
              onClick={() => setShowCreate(true)}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-brand-gradient text-brand-bg text-sm font-semibold hover:opacity-90 hover:shadow-cyan-glow transition-all"
            >
              <PlusIcon />
              New Story
            </button>
          </div>
        </div>

        {/* Error */}
        {error && (
          <div className="mb-6 rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3">
            <p className="text-red-400 text-sm">{error}</p>
          </div>
        )}

        {/* Loading */}
        {loading && (
          <div className="flex items-center justify-center py-24">
            <SpinnerIcon className="w-6 h-6 text-brand-cyan animate-spin" />
          </div>
        )}

        {/* Project grid */}
        {!loading && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {projects.map((p) => (
              <ProjectCard key={p.id} project={p} onClick={() => navigate(`/projects/${p.id}`)} />
            ))}
          </div>
        )}
      </main>

      {/* Create modal */}
      {showCreate && (
        <CreateProjectModal
          token={accessToken!}
          onCreate={create}
          onCreated={(p) => navigate(`/projects/${p.id}`)}
          onClose={() => setShowCreate(false)}
        />
      )}
    </div>
  )
}

// ── Project card ──────────────────────────────────────────────────────────────

function ProjectCard({ project, onClick }: { project: Project; onClick: () => void }) {
  const date = new Date(project.created_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })

  return (
    <button
      onClick={onClick}
      className="text-left bg-brand-bg-card border border-brand-border rounded-xl p-5 hover:border-brand-cyan/40 hover:shadow-cyan-glow/20 transition-all group"
    >
      <div className="flex items-start justify-between gap-2 mb-3">
        <h2 className="text-base font-semibold text-brand-text group-hover:text-brand-cyan transition-colors line-clamp-2">
          {project.title}
        </h2>
        <ArrowIcon />
      </div>

      {project.description && (
        <p className="text-brand-muted text-sm leading-relaxed line-clamp-2 mb-3">
          {project.description}
        </p>
      )}

      {project.genres.length > 0 && (
        <div className="flex flex-wrap gap-1.5 mb-3">
          {project.genres.slice(0, 4).map((g) => (
            <span key={g} className="px-2 py-0.5 rounded-full bg-brand-purple/20 text-brand-purple text-xs font-medium">
              {g}
            </span>
          ))}
        </div>
      )}

      <p className="text-brand-muted/60 text-xs mt-auto">{date}</p>
    </button>
  )
}

// ── Create project modal ──────────────────────────────────────────────────────

interface CreateProjectModalProps {
  token: string
  onCreate: (token: string, title: string, description: string, genres: string[]) => Promise<Project>
  onCreated: (project: Project) => void
  onClose: () => void
}

function CreateProjectModal({ token, onCreate, onCreated, onClose }: CreateProjectModalProps) {
  const [title, setTitle]       = useState('')
  const [description, setDesc]  = useState('')
  const [genreInput, setGenre]  = useState('')
  const [genres, setGenres]     = useState<string[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [error, setError]       = useState<string | null>(null)

  const addGenre = () => {
    const g = genreInput.trim().toLowerCase()
    if (g && !genres.includes(g)) setGenres((prev) => [...prev, g])
    setGenre('')
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) return
    setSubmitting(true)
    setError(null)
    try {
      const project = await onCreate(token, title.trim(), description.trim(), genres)
      onCreated(project)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to create project')
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm px-4">
      <div className="bg-brand-bg-card border border-brand-border rounded-2xl p-8 w-full max-w-md shadow-card">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-bold text-brand-text">New Story</h2>
          <button onClick={onClose} className="text-brand-muted hover:text-brand-text transition-colors">
            <CloseIcon />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-5">
          <div className="space-y-1.5">
            <label className="block text-xs font-medium text-brand-muted uppercase tracking-wider">Title *</label>
            <input
              autoFocus
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="e.g. The Last Starship"
              className="input-field"
              maxLength={200}
            />
          </div>

          <div className="space-y-1.5">
            <label className="block text-xs font-medium text-brand-muted uppercase tracking-wider">Description</label>
            <textarea
              value={description}
              onChange={(e) => setDesc(e.target.value)}
              placeholder="A brief summary of your story…"
              rows={3}
              className="input-field resize-none"
            />
          </div>

          <div className="space-y-1.5">
            <label className="block text-xs font-medium text-brand-muted uppercase tracking-wider">Genres</label>
            <div className="flex gap-2">
              <input
                value={genreInput}
                onChange={(e) => setGenre(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addGenre() } }}
                placeholder="sci-fi, fantasy…"
                className="input-field"
              />
              <button type="button" onClick={addGenre} className="px-3 py-2 rounded-lg border border-brand-border text-brand-muted hover:text-brand-text hover:border-brand-cyan/40 transition-colors text-sm shrink-0">
                Add
              </button>
            </div>
            {genres.length > 0 && (
              <div className="flex flex-wrap gap-1.5 pt-1">
                {genres.map((g) => (
                  <span key={g} className="flex items-center gap-1 px-2.5 py-0.5 rounded-full bg-brand-purple/20 text-brand-purple text-xs font-medium">
                    {g}
                    <button type="button" onClick={() => setGenres((prev) => prev.filter((x) => x !== g))} className="opacity-60 hover:opacity-100">×</button>
                  </span>
                ))}
              </div>
            )}
          </div>

          {error && (
            <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3">
              <p className="text-red-400 text-sm">{error}</p>
            </div>
          )}

          <div className="flex gap-3 pt-1">
            <button type="button" onClick={onClose} className="flex-1 py-2.5 rounded-lg border border-brand-border text-brand-muted hover:text-brand-text hover:border-brand-cyan/40 transition-colors text-sm font-medium">
              Cancel
            </button>
            <button type="submit" disabled={submitting || !title.trim()} className="flex-1 py-2.5 rounded-lg bg-brand-gradient text-brand-bg text-sm font-semibold hover:opacity-90 hover:shadow-cyan-glow transition-all disabled:opacity-50 disabled:cursor-not-allowed">
              {submitting ? 'Creating…' : 'Create Story'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ── Icons ─────────────────────────────────────────────────────────────────────

function PlusIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
      <path d="M8 3v10M3 8h10" />
    </svg>
  )
}

function ImportIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 10V3M5 7l3 3 3-3" />
      <path d="M2 12v1a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1v-1" />
    </svg>
  )
}

function ArrowIcon() {
  return (
    <svg className="w-4 h-4 shrink-0 text-brand-muted/40 group-hover:text-brand-cyan/60 transition-colors" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 8h10M9 4l4 4-4 4" />
    </svg>
  )
}

function CloseIcon() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <path d="M5 5l10 10M15 5L5 15" />
    </svg>
  )
}

function SpinnerIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
    </svg>
  )
}
