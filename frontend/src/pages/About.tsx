import { Link } from 'react-router-dom'

const LIMITATIONS = [
  {
    title: 'Async collaboration only',
    body: 'Collaborators work on separate branches and propose changes via merge requests. There is no live co-editing — two people cannot type in the same scene at the same time.',
  },
  {
    title: 'No mobile optimisation',
    body: 'NexusTale is designed for desktop browsers. The editor and wiki panels are not adapted for small screens.',
  },
  {
    title: 'AI requires your own API key',
    body: 'Nexus AI, Beat, Workshop, and all AI features use your own provider key (OpenAI, Anthropic, etc.) or a local Ollama instance. NexusTale does not supply AI compute.',
  },
  {
    title: 'Exports are best-effort',
    body: 'Markdown and DOCX exports are stable. EPUB output depends on a background worker and MinIO storage — if the worker is busy the job may queue for a few seconds.',
  },
  {
    title: 'Alpha — bugs expected',
    body: 'This is invite-only alpha software. Data is backed up daily, but no SLA is provided. Please report anything unexpected on GitHub.',
  },
]

export default function About() {
  return (
    <div className="min-h-screen bg-brand-bg text-brand-text font-sans">
      <header className="border-b border-brand-border px-6 py-4 flex items-center gap-4">
        <Link to="/dashboard" className="text-brand-muted hover:text-brand-text transition-colors text-sm">
          ← Back
        </Link>
        <span className="text-brand-cyan font-semibold">NexusTale</span>
        <span className="text-brand-muted/40">/</span>
        <span className="text-sm text-brand-muted">Alpha — Known Limitations</span>
      </header>

      <main className="max-w-2xl mx-auto px-6 py-10 space-y-10">

        <section>
          <div className="flex items-center gap-3 mb-2">
            <span className="text-xs font-semibold uppercase tracking-widest text-amber-400 border border-amber-400/30 bg-amber-400/10 rounded px-2 py-0.5">
              Alpha
            </span>
            <h1 className="text-2xl font-bold text-brand-text">NexusTale</h1>
          </div>
          <p className="text-brand-text-muted text-sm leading-relaxed">
            Thank you for being an early tester. The list below covers the current known limitations
            so you know what to expect during the alpha period.
          </p>
        </section>

        <section className="space-y-4">
          <h2 className="text-base font-semibold text-brand-text">Known limitations</h2>
          {LIMITATIONS.map((item) => (
            <div key={item.title} className="border border-brand-border rounded-xl px-5 py-4 bg-brand-bg-card">
              <p className="text-sm font-medium text-brand-text mb-1">{item.title}</p>
              <p className="text-sm text-brand-text-muted leading-relaxed">{item.body}</p>
            </div>
          ))}
        </section>

        <section>
          <h2 className="text-base font-semibold text-brand-text mb-3">Report a bug or share feedback</h2>
          <a
            href="https://github.com/helloWorld44-89/NexusTale/issues"
            target="_blank"
            rel="noreferrer"
            className="flex items-center gap-3 border border-brand-border rounded-xl px-5 py-4 bg-brand-bg-card hover:border-brand-cyan/40 transition-colors group"
          >
            <GitHubIcon />
            <div>
              <p className="text-sm font-medium text-brand-text group-hover:text-brand-cyan transition-colors">
                Open a GitHub issue
              </p>
              <p className="text-xs text-brand-text-muted mt-0.5">
                github.com/helloWorld44-89/NexusTale/issues
              </p>
            </div>
          </a>
        </section>

      </main>
    </div>
  )
}

function GitHubIcon() {
  return (
    <svg className="w-5 h-5 shrink-0 text-brand-muted group-hover:text-brand-cyan transition-colors" viewBox="0 0 20 20" fill="currentColor">
      <path fillRule="evenodd" d="M10 0C4.477 0 0 4.477 0 10c0 4.418 2.865 8.166 6.839 9.489.5.092.682-.217.682-.482 0-.237-.009-.868-.013-1.703-2.782.604-3.369-1.34-3.369-1.34-.454-1.154-1.11-1.462-1.11-1.462-.908-.62.069-.608.069-.608 1.003.07 1.531 1.03 1.531 1.03.892 1.529 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.11-4.555-4.943 0-1.091.39-1.984 1.029-2.683-.103-.253-.446-1.27.098-2.647 0 0 .84-.269 2.75 1.025A9.578 9.578 0 0110 4.836a9.59 9.59 0 012.504.337c1.909-1.294 2.747-1.025 2.747-1.025.546 1.377.203 2.394.1 2.647.641.699 1.028 1.592 1.028 2.683 0 3.842-2.339 4.687-4.566 4.935.359.309.678.919.678 1.852 0 1.336-.012 2.415-.012 2.743 0 .267.18.578.688.48C17.138 18.163 20 14.418 20 10c0-5.523-4.477-10-10-10z" clipRule="evenodd" />
    </svg>
  )
}
