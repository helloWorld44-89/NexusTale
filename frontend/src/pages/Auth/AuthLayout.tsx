// AuthLayout wraps both Login and Register pages.
// Left panel: branding with logo and ambient background.
// Right panel: the form slot.

interface AuthLayoutProps {
  children: React.ReactNode
}

export default function AuthLayout({ children }: AuthLayoutProps) {
  return (
    <div className="min-h-screen flex bg-brand-bg">

      {/* ── Left brand panel (hidden on small screens) ── */}
      <div className="hidden lg:flex lg:w-[45%] relative flex-col items-center justify-center overflow-hidden">

        {/* Ambient background layers */}
        <div className="absolute inset-0 bg-brand-bg" />
        <div
          className="absolute inset-0 opacity-20"
          style={{
            background:
              'radial-gradient(ellipse 80% 60% at 50% 40%, #9F4BFF 0%, transparent 70%)',
          }}
        />
        <div
          className="absolute inset-0 opacity-15"
          style={{
            background:
              'radial-gradient(ellipse 60% 40% at 30% 70%, #00F0FF 0%, transparent 60%)',
          }}
        />
        {/* Subtle grid pattern */}
        <div
          className="absolute inset-0 opacity-5"
          style={{
            backgroundImage:
              'linear-gradient(rgba(0,240,255,0.5) 1px, transparent 1px), linear-gradient(90deg, rgba(0,240,255,0.5) 1px, transparent 1px)',
            backgroundSize: '40px 40px',
          }}
        />

        {/* Content */}
        <div className="relative z-10 flex flex-col items-center text-center px-12 gap-8">
          <img
            src="/logo-dark.png"
            alt="NexusTale"
            className="w-52 animate-float drop-shadow-[0_0_30px_rgba(0,240,255,0.4)]"
          />

          <div className="space-y-3">
            <h1 className="text-3xl font-bold text-brand-text tracking-tight">
              Where worlds are written.
            </h1>
            <p className="text-brand-muted text-base leading-relaxed max-w-xs">
              Craft your story with AI-assisted writing, a living world wiki, and
              version control built for novelists.
            </p>
          </div>

          {/* Feature pills */}
          <div className="flex flex-wrap justify-center gap-2 mt-2">
            {['AI Co-author', 'World Wiki', 'Git Versioning', 'Sci-fi & Fantasy'].map((f) => (
              <span
                key={f}
                className="px-3 py-1 rounded-full text-xs font-medium border border-brand-border text-brand-muted bg-brand-bg-card"
              >
                {f}
              </span>
            ))}
          </div>
        </div>

        {/* Bottom gradient fade */}
        <div className="absolute bottom-0 left-0 right-0 h-24 bg-gradient-to-t from-brand-bg to-transparent" />
      </div>

      {/* ── Right form panel ── */}
      <div className="flex-1 flex flex-col items-center justify-center px-6 py-12 relative">

        {/* Mobile-only logo */}
        <div className="lg:hidden mb-8">
          <img src="/logo-dark.png" alt="NexusTale" className="w-32 mx-auto" />
        </div>

        {children}

        <p className="mt-8 text-brand-muted text-xs text-center">
          © {new Date().getFullYear()} NexusTale. Open source.
        </p>
      </div>
    </div>
  )
}
