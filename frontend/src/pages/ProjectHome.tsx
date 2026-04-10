// ProjectHome — project overview page shown before entering the editor.
// Displays stats, quick-open links to wiki + editor, and project metadata.
import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'
import { api } from '@/services/api'
import type { Project, ProjectStats } from '@/services/api'

export default function ProjectHome() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const accessToken = useAuthStore((s) => s.accessToken)

  const [project, setProject] = useState<Project | null>(null)
  const [stats,   setStats]   = useState<ProjectStats | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!id || !accessToken) return
    let cancelled = false

    const load = async () => {
      try {
        const [p, s] = await Promise.all([
          api.projects.get(accessToken, id),
          api.projects.stats(accessToken, id),
        ])
        if (!cancelled) {
          setProject(p)
          setStats(s)
        }
      } catch {
        if (!cancelled) navigate('/dashboard', { replace: true })
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    load()
    return () => { cancelled = true }
  }, [id, accessToken])

  if (loading) {
    return (
      <div className="h-screen flex items-center justify-center bg-brand-bg">
        <svg className="animate-spin h-6 w-6 text-brand-cyan" viewBox="0 0 24 24" fill="none">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
        </svg>
      </div>
    )
  }

  if (!project) return null

  const lastUpdated = stats
    ? new Date(stats.last_updated_at).toLocaleDateString(undefined, { month: 'long', day: 'numeric', year: 'numeric' })
    : '—'

  return (
    <div className="min-h-screen bg-brand-bg flex flex-col">
      {/* Header */}
      <header className="h-14 flex items-center justify-between px-6 bg-brand-bg-card border-b border-brand-border shrink-0">
        <div className="flex items-center gap-3">
          <Link to="/dashboard" className="text-brand-muted hover:text-brand-text transition-colors">
            <BackIcon />
          </Link>
          <div className="flex items-center gap-2">
            <img src="/app-icon.png" alt="" className="w-5 h-5 opacity-80" />
            <span className="text-brand-cyan font-semibold tracking-wide">NexusTale</span>
          </div>
        </div>
        <Link to="/dashboard" className="text-sm text-brand-muted hover:text-brand-text transition-colors">
          Dashboard
        </Link>
      </header>

      <main className="flex-1 max-w-4xl w-full mx-auto px-6 py-12">
        {/* Project title + genres */}
        <div className="mb-10">
          <h1 className="text-3xl font-bold text-brand-text mb-2">{project.title}</h1>
          {project.description && (
            <p className="text-brand-muted text-base leading-relaxed mb-4 max-w-2xl">{project.description}</p>
          )}
          {project.genres.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {project.genres.map((g) => (
                <span key={g} className="px-2.5 py-0.5 rounded-full bg-brand-purple/20 text-brand-purple text-xs font-medium">
                  {g}
                </span>
              ))}
            </div>
          )}
        </div>

        {/* Stats row */}
        {stats && (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-10">
            <StatCard label="Words" value={stats.total_word_count.toLocaleString()} color="text-brand-cyan" />
            <StatCard label="Scenes" value={String(stats.scene_count)} color="text-brand-gold" />
            <StatCard label="Chapters" value={String(stats.chapter_count)} color="text-brand-purple" />
            <StatCard label="Last updated" value={lastUpdated} color="text-brand-text-muted" small />
          </div>
        )}

        {/* Quick-open cards */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <ActionCard
            title="Open Editor"
            description="Write and manage your manuscript — scenes, chapters, and acts."
            accent="cyan"
            onClick={() => navigate(`/projects/${id}/editor`)}
            icon={<EditorIcon />}
          />
          <ActionCard
            title="World Wiki"
            description="Entities, timeline, relationships, and the rules of your world."
            accent="purple"
            onClick={() => navigate(`/projects/${id}/wiki`)}
            icon={<WikiIcon />}
          />
        </div>
      </main>
    </div>
  )
}

// ── Sub-components ────────────────────────────────────────────────────────────

function StatCard({ label, value, color, small }: { label: string; value: string; color: string; small?: boolean }) {
  return (
    <div className="bg-brand-bg-card border border-brand-border rounded-xl px-5 py-4">
      <p className="text-brand-text-muted text-xs uppercase tracking-wider mb-1">{label}</p>
      <p className={`font-bold ${small ? 'text-sm' : 'text-2xl'} ${color}`}>{value}</p>
    </div>
  )
}

function ActionCard({
  title,
  description,
  accent,
  onClick,
  icon,
}: {
  title: string
  description: string
  accent: 'cyan' | 'purple'
  onClick: () => void
  icon: React.ReactNode
}) {
  const border = accent === 'cyan'
    ? 'hover:border-brand-cyan/40 hover:shadow-cyan-glow/20'
    : 'hover:border-brand-purple/40'
  const iconBg = accent === 'cyan' ? 'bg-brand-cyan/10 text-brand-cyan' : 'bg-brand-purple/10 text-brand-purple'

  return (
    <button
      onClick={onClick}
      className={`text-left bg-brand-bg-card border border-brand-border rounded-xl p-6 transition-all group ${border}`}
    >
      <div className={`w-10 h-10 rounded-lg flex items-center justify-center mb-4 ${iconBg}`}>
        {icon}
      </div>
      <h2 className="text-base font-semibold text-brand-text mb-1 group-hover:text-brand-cyan transition-colors">{title}</h2>
      <p className="text-brand-muted text-sm leading-relaxed">{description}</p>
    </button>
  )
}

// ── Icons ─────────────────────────────────────────────────────────────────────

function BackIcon() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M13 4l-6 6 6 6" />
    </svg>
  )
}

function EditorIcon() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 6h12M4 10h8M4 14h6" />
    </svg>
  )
}

function WikiIcon() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="10" cy="10" r="7" />
      <path d="M10 7v3l2 2" />
    </svg>
  )
}
