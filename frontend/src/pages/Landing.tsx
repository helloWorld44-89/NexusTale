import { useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'

// ── Content ───────────────────────────────────────────────────────────────────

const FEATURES = [
  {
    icon: '⎇',
    title: 'Chronicle — save points for your novel',
    body: 'Branch into a "what if" without losing your draft. Every Chronicle is a named snapshot you can travel back to or diverge from at any time.',
  },
  {
    icon: '🗺',
    title: 'World wiki wired to the manuscript',
    body: 'One app, not five. Your entities, magic rules, and timeline live next to the prose — and Nexus reads them before every suggestion.',
  },
  {
    icon: '✦',
    title: 'AI that has read your whole story',
    body: "Nexus won't suggest a character who died in chapter 3. Context pins, chapter summaries, and a Workshop agent mode for craft-level editing.",
  },
  {
    icon: '↓',
    title: 'Always exportable',
    body: 'Download your manuscript as Markdown, Word, or EPUB any time. Your words are never locked in.',
  },
]

const LIMITATIONS = [
  {
    title: 'Alpha — expect rough edges',
    body: 'This is alpha software. Data is backed up daily, but no SLA is provided. Please report anything unexpected on GitHub.',
  },
  {
    title: 'Async collaboration only',
    body: 'Co-authors work on separate branches and propose changes via merge requests. There is no live co-editing.',
  },
  {
    title: 'AI requires your own API key',
    body: 'Nexus AI uses your own provider key (OpenAI, Anthropic) or a local Ollama instance. NexusTale does not supply AI compute.',
  },
  {
    title: 'Desktop browsers only',
    body: 'The editor and wiki panels are not adapted for small screens. Mobile support is planned for a later phase.',
  },
  {
    title: 'EPUB export queues briefly',
    body: 'EPUB output depends on a background worker — if the worker is busy the job may queue for a few seconds. Markdown and DOCX exports are instant.',
  },
]

// ── Component ─────────────────────────────────────────────────────────────────

export default function Landing() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const navigate = useNavigate()

  // Logged-in users skip the landing page
  useEffect(() => {
    if (isAuthenticated) navigate('/dashboard', { replace: true })
  }, [isAuthenticated, navigate])

  return (
    <div className="min-h-screen bg-brand-bg text-brand-text font-sans">

      {/* ── Header ─────────────────────────────────────────────────────── */}
      <header className="border-b border-brand-border px-6 py-4 flex items-center justify-between">
        <span className="text-brand-cyan font-bold text-lg tracking-tight">NexusTale</span>
        <nav className="flex items-center gap-4">
          <Link
            to="/login"
            className="text-sm text-brand-muted hover:text-brand-text transition-colors"
          >
            Sign in
          </Link>
          <Link
            to="/register"
            className="text-sm px-3 py-1.5 rounded-lg bg-brand-cyan/10 text-brand-cyan border border-brand-cyan/30 hover:bg-brand-cyan/20 transition-colors"
          >
            Register
          </Link>
        </nav>
      </header>

      <main className="max-w-3xl mx-auto px-6 py-16 space-y-20">

        {/* ── Hero ─────────────────────────────────────────────────────── */}
        <section className="space-y-5 text-center">
          <span className="inline-block text-xs font-semibold uppercase tracking-widest text-amber-400 border border-amber-400/30 bg-amber-400/10 rounded px-2 py-0.5">
            Alpha
          </span>
          <h1 className="text-4xl font-bold leading-tight text-brand-text">
            The writing tool built like a writer thinks
          </h1>
          <p className="text-brand-text-muted text-lg max-w-xl mx-auto leading-relaxed">
            Branching timelines, a living world wiki, and an AI that has read your whole
            manuscript before offering a single word.
          </p>
          <Link
            to="/register"
            className="mt-2 inline-flex items-center gap-2 px-5 py-2.5 rounded-lg bg-brand-cyan text-brand-bg font-semibold text-sm hover:opacity-90 transition-opacity shadow-cyan-glow"
          >
            Get started →
          </Link>
        </section>

        {/* ── Features ─────────────────────────────────────────────────── */}
        <section className="space-y-4">
          <h2 className="text-base font-semibold text-brand-text">What makes it different</h2>
          <div className="grid sm:grid-cols-2 gap-4">
            {FEATURES.map((f) => (
              <div
                key={f.title}
                className="border border-brand-border rounded-xl px-5 py-4 bg-brand-bg-card space-y-1.5"
              >
                <p className="text-xl">{f.icon}</p>
                <p className="text-sm font-medium text-brand-text">{f.title}</p>
                <p className="text-sm text-brand-text-muted leading-relaxed">{f.body}</p>
              </div>
            ))}
          </div>
        </section>

        {/* ── Known limitations ─────────────────────────────────────────── */}
        <section className="space-y-4">
          <h2 className="text-base font-semibold text-brand-text">What to expect in alpha</h2>
          <div className="space-y-3">
            {LIMITATIONS.map((item) => (
              <div
                key={item.title}
                className="border border-brand-border rounded-xl px-5 py-4 bg-brand-bg-card"
              >
                <p className="text-sm font-medium text-brand-text mb-1">{item.title}</p>
                <p className="text-sm text-brand-text-muted leading-relaxed">{item.body}</p>
              </div>
            ))}
          </div>
        </section>

      </main>

      {/* ── Footer ─────────────────────────────────────────────────────── */}
      <footer className="border-t border-brand-border px-6 py-6 flex items-center justify-between text-sm text-brand-muted">
        <span>NexusTale — alpha</span>
        <a
          href="https://github.com/helloWorld44-89/NexusTale/issues"
          target="_blank"
          rel="noreferrer"
          className="hover:text-brand-text transition-colors"
        >
          Report a bug →
        </a>
      </footer>
    </div>
  )
}
