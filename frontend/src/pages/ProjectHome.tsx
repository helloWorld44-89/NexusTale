// ProjectHome — project overview page shown before entering the editor.
// Displays stats, quick-open links to wiki + editor, and project metadata.
import { useEffect, useRef, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'
import { api } from '@/services/api'
import type { Project, ProjectStats, AIUsageSummary, ExportJob, ProjectStructure } from '@/services/api'

export default function ProjectHome() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const accessToken = useAuthStore((s) => s.accessToken)

  const [project,    setProject]    = useState<Project | null>(null)
  const [stats,      setStats]      = useState<ProjectStats | null>(null)
  const [usage,      setUsage]      = useState<AIUsageSummary | null>(null)
  const [structure,  setStructure]  = useState<ProjectStructure | null>(null)
  const [loading,    setLoading]    = useState(true)
  const [epubJobId,  setEpubJobId]  = useState<string | null>(null)
  const [epubJob,    setEpubJob]    = useState<ExportJob | null>(null)
  const [exporting,  setExporting]  = useState(false)
  const [exportErr,  setExportErr]  = useState<string | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    if (!id || !accessToken) return
    let cancelled = false

    const load = async () => {
      try {
        const [p, s, u, st] = await Promise.all([
          api.projects.get(accessToken, id),
          api.projects.stats(accessToken, id),
          api.ai.usage(accessToken, id).catch(() => null),
          api.structures.get(accessToken, id).catch(() => null),
        ])
        if (!cancelled) {
          setProject(p)
          setStats(s)
          setUsage(u)
          setStructure(st)
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

  // Poll for EPUB job completion every 3 seconds.
  useEffect(() => {
    if (!epubJobId || !accessToken || !id) return
    if (pollRef.current) clearInterval(pollRef.current)

    pollRef.current = setInterval(async () => {
      try {
        const job = await api.export.getJob(accessToken, id, epubJobId)
        setEpubJob(job)
        if (job.status === 'done' || job.status === 'failed') {
          clearInterval(pollRef.current!)
          pollRef.current = null
          setExporting(false)
        }
      } catch {
        clearInterval(pollRef.current!)
        pollRef.current = null
        setExporting(false)
      }
    }, 3000)

    return () => {
      if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null }
    }
  }, [epubJobId, accessToken, id])

  const handleMarkdownExport = async () => {
    if (!accessToken || !id || !project) return
    setExporting(true)
    setExportErr(null)
    try {
      await api.export.downloadMarkdown(accessToken, id, `${project.title}.zip`)
    } catch (e) {
      setExportErr(e instanceof Error ? e.message : 'Export failed')
    } finally {
      setExporting(false)
    }
  }

  const handleEpubExport = async () => {
    if (!accessToken || !id) return
    setExporting(true)
    setExportErr(null)
    setEpubJob(null)
    try {
      const { job_id } = await api.export.startEpub(accessToken, id)
      setEpubJobId(job_id)
      setEpubJob({ id: job_id, project_id: id, format: 'epub', status: 'pending', created_at: new Date().toISOString() })
    } catch (e) {
      setExportErr(e instanceof Error ? e.message : 'Export failed')
      setExporting(false)
    }
  }

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
          {/* Structure badge — only shown when a named structure is selected */}
          {structure?.structure_name && (
            <div className="mt-3">
              <Link
                to={`/projects/${id}/guide?step=structure`}
                className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full border border-brand-cyan/30 bg-brand-cyan/5 text-brand-cyan text-xs font-medium hover:bg-brand-cyan/10 transition-colors"
              >
                <svg className="w-3 h-3 opacity-70" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M2 4h12M2 8h8M2 12h5" />
                </svg>
                {structure.structure_name}
              </Link>
            </div>
          )}
        </div>

        {/* Stats row */}
        {stats && (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-4">
            <StatCard label="Words" value={stats.total_word_count.toLocaleString()} color="text-brand-cyan" />
            <StatCard label="Scenes" value={String(stats.scene_count)} color="text-brand-gold" />
            <StatCard label="Chapters" value={String(stats.chapter_count)} color="text-brand-purple" />
            <StatCard label="Last updated" value={lastUpdated} color="text-brand-text-muted" small />
          </div>
        )}

        {/* AI usage row — only shown when at least one AI call has been made */}
        {usage && usage.total_tokens > 0 && (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-10">
            <StatCard
              label="AI tokens (total)"
              value={usage.total_tokens.toLocaleString()}
              color="text-brand-purple"
            />
            <StatCard
              label="AI tokens (month)"
              value={usage.monthly_tokens.toLocaleString()}
              color="text-brand-purple"
            />
            <StatCard
              label="AI calls (month)"
              value={String(usage.calls_this_month)}
              color="text-brand-text-muted"
            />
            <StatCard
              label="AI cost (month)"
              value={usage.monthly_cost_usd > 0 ? `$${usage.monthly_cost_usd.toFixed(4)}` : '$0.00'}
              color="text-brand-text-muted"
              small
            />
          </div>
        )}
        {usage && usage.total_tokens === 0 && <div className="mb-10" />}

        {/* Quick-open cards */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-10">
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
          <ActionCard
            title="Novel Guide"
            description="Step-by-step wizard: premise, characters, world, outline, first scene."
            accent="gold"
            onClick={() => navigate(`/projects/${id}/guide`)}
            icon={<GuideIcon />}
          />
        </div>

        {/* Export panel */}
        <div className="bg-brand-bg-card border border-brand-border rounded-xl p-6">
          <h2 className="text-sm font-semibold text-brand-text mb-4 flex items-center gap-2">
            <ExportIcon />
            Export Manuscript
          </h2>
          <div className="flex flex-wrap gap-3">
            <button
              onClick={handleMarkdownExport}
              disabled={exporting}
              className="px-4 py-2 rounded-lg bg-brand-cyan/10 text-brand-cyan text-sm font-medium hover:bg-brand-cyan/20 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {exporting && !epubJobId ? 'Exporting…' : 'Download Markdown (.zip)'}
            </button>
            <button
              onClick={handleEpubExport}
              disabled={exporting}
              className="px-4 py-2 rounded-lg bg-brand-purple/10 text-brand-purple text-sm font-medium hover:bg-brand-purple/20 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {exporting && epubJobId ? 'Generating EPUB…' : 'Export EPUB'}
            </button>
          </div>

          {exportErr && (
            <p className="mt-3 text-sm text-red-400">{exportErr}</p>
          )}

          {epubJob && (
            <div className="mt-4">
              <EpubJobStatus job={epubJob} />
            </div>
          )}
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
  accent: 'cyan' | 'purple' | 'gold'
  onClick: () => void
  icon: React.ReactNode
}) {
  const border =
    accent === 'cyan'   ? 'hover:border-brand-cyan/40 hover:shadow-cyan-glow/20' :
    accent === 'purple' ? 'hover:border-brand-purple/40' :
                          'hover:border-brand-gold/40'
  const iconBg =
    accent === 'cyan'   ? 'bg-brand-cyan/10 text-brand-cyan' :
    accent === 'purple' ? 'bg-brand-purple/10 text-brand-purple' :
                          'bg-brand-gold/10 text-brand-gold'

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

function EpubJobStatus({ job }: { job: ExportJob }) {
  const statusColor =
    job.status === 'done'       ? 'text-green-400' :
    job.status === 'failed'     ? 'text-red-400'   :
    job.status === 'processing' ? 'text-brand-cyan' :
                                  'text-brand-muted'

  const label =
    job.status === 'done'       ? 'EPUB ready' :
    job.status === 'failed'     ? 'Export failed' :
    job.status === 'processing' ? 'Building EPUB…' :
                                  'Queued…'

  return (
    <div className="flex items-center gap-3 text-sm">
      {(job.status === 'pending' || job.status === 'processing') && (
        <svg className="animate-spin h-4 w-4 text-brand-cyan" viewBox="0 0 24 24" fill="none">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
        </svg>
      )}
      <span className={statusColor}>{label}</span>
      {job.status === 'done' && job.download_url && (
        <a
          href={job.download_url}
          download
          className="px-3 py-1 rounded bg-brand-purple/20 text-brand-purple text-xs font-medium hover:bg-brand-purple/30 transition-colors"
        >
          Download EPUB
        </a>
      )}
      {job.status === 'failed' && job.error_msg && (
        <span className="text-red-400/70 text-xs">{job.error_msg}</span>
      )}
      {job.status === 'done' && job.expires_at && (
        <span className="text-brand-muted text-xs">
          Expires {new Date(job.expires_at).toLocaleDateString()}
        </span>
      )}
    </div>
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

function GuideIcon() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 4h12M4 8h8M4 12h10M4 16h6" />
    </svg>
  )
}

function ExportIcon() {
  return (
    <svg className="w-4 h-4 text-brand-muted" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M10 3v9M6 8l4 4 4-4" />
      <path d="M4 14v1a2 2 0 002 2h8a2 2 0 002-2v-1" />
    </svg>
  )
}
