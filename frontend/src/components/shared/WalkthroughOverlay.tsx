// WalkthroughOverlay — 6-step first-time editor tour.
// Uses a fixed spotlight ring (box-shadow technique) + tooltip bubble.
// No external dependencies; positions derived from data-tour attributes.
import { useEffect, useLayoutEffect, useState } from 'react'
import { TOUR_STEPS } from '@/hooks/useWalkthrough'

interface StepDef {
  target: string | null   // data-tour value, or null for welcome modal
  title:  string
  body:   string
  cta:    string
}

const STEPS: StepDef[] = [
  {
    target: null,
    title:  'Welcome to NexusTale',
    body:   "Let's take 60 seconds to show you around.",
    cta:    "Let's go →",
  },
  {
    target: 'scribe-editor',
    title:  'Your writing surface',
    body:   'This is where your story lives. Select a scene from the tree on the right to start writing.',
    cta:    'Next →',
  },
  {
    target: 'activity-bar',
    title:  'Your toolkit',
    body:   'These icons open your toolkit — AI chat, wiki, Workshop, Chronicle, context pins, and more.',
    cta:    'Next →',
  },
  {
    target: 'beat-input',
    title:  'Beat bar',
    body:   'The Beat bar sends your scene to the AI for a continuation suggestion. Accept, retry, or discard.',
    cta:    'Next →',
  },
  {
    target: 'mentions-bar',
    title:  'Entity tracking',
    body:   'As you write, NexusTale tracks which characters and places appear in each scene. Click a chip to jump to their wiki entry.',
    cta:    'Next →',
  },
  {
    target: 'chronicle-button',
    title:  'Chronicle',
    body:   'Chronicle saves a named snapshot of your manuscript — like a commit for your story. Use it after any meaningful session.',
    cta:    'Done',
  },
]

interface Rect { x: number; y: number; w: number; h: number }

const PAD = 8

interface WalkthroughOverlayProps {
  step: number
  onNext: () => void
  onSkip: () => void
}

export default function WalkthroughOverlay({ step, onNext, onSkip }: WalkthroughOverlayProps) {
  const def = STEPS[step] ?? STEPS[0]
  const [rect, setRect] = useState<Rect | null>(null)

  // Recompute spotlight position whenever step changes or window resizes.
  useLayoutEffect(() => {
    if (!def.target) { setRect(null); return }

    function compute() {
      const el = document.querySelector(`[data-tour="${def.target}"]`)
      if (!el) { setRect(null); return }
      const r = el.getBoundingClientRect()
      setRect({ x: r.left - PAD, y: r.top - PAD, w: r.width + PAD * 2, h: r.height + PAD * 2 })
    }

    compute()
    window.addEventListener('resize', compute)
    return () => window.removeEventListener('resize', compute)
  }, [step, def.target])

  // Advance on keyboard: Enter / Space / ArrowRight; Escape to skip.
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowRight') {
        e.preventDefault()
        onNext()
      }
      if (e.key === 'Escape') onSkip()
    }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [onNext, onSkip])

  // Step 0: welcome modal — no spotlight, centered card.
  if (!def.target) {
    return (
      <div className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/65">
        <div className="bg-brand-bg-card border border-brand-border rounded-2xl p-8 w-full max-w-sm shadow-2xl">
          <p className="text-[10px] text-brand-cyan uppercase tracking-widest font-semibold mb-3">
            {step + 1} / {TOUR_STEPS}
          </p>
          <h2 className="text-xl font-bold text-brand-text mb-2">{def.title}</h2>
          <p className="text-sm text-brand-muted leading-relaxed mb-6">{def.body}</p>
          <div className="flex items-center justify-between">
            <button
              onClick={onSkip}
              className="text-xs text-brand-muted hover:text-brand-text transition-colors"
            >
              Skip tour
            </button>
            <button
              onClick={onNext}
              className="px-5 py-2 rounded-lg bg-brand-cyan text-brand-bg text-sm font-semibold hover:bg-brand-cyan/90 transition-colors"
            >
              {def.cta}
            </button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <>
      {/* Dark backdrop — everything is dimmed */}
      <div className="fixed inset-0 z-[9997] pointer-events-none" style={{ background: 'rgba(0,0,0,0)' }} />

      {/* Spotlight ring — box-shadow creates the dark overlay with a bright "hole" */}
      {rect && (
        <div
          className="fixed z-[9998] rounded-lg pointer-events-none"
          style={{
            left:      rect.x,
            top:       rect.y,
            width:     rect.w,
            height:    rect.h,
            boxShadow: '0 0 0 9999px rgba(0,0,0,0.68)',
            outline:   '2px solid rgba(99,220,219,0.6)',
          }}
        />
      )}

      {/* Click-blocker backdrop (behind tooltip, above spotlight so user can't click through) */}
      <div className="fixed inset-0 z-[9999]" onClick={onSkip} />

      {/* Tooltip bubble */}
      <Tooltip def={def} rect={rect} step={step} onNext={onNext} onSkip={onSkip} />
    </>
  )
}

// ── Tooltip ───────────────────────────────────────────────────────────────────

function Tooltip({
  def, rect, step, onNext, onSkip,
}: {
  def:    StepDef
  rect:   Rect | null
  step:   number
  onNext: () => void
  onSkip: () => void
}) {
  const TOOLTIP_W = 280
  const TOOLTIP_H = 180  // conservative estimate; real height varies by content

  let style: React.CSSProperties = {}

  if (rect) {
    const viewW = window.innerWidth
    const viewH = window.innerHeight
    const spaceBelow = viewH - (rect.y + rect.h)
    const spaceAbove = rect.y

    const left = Math.max(8, Math.min(rect.x + rect.w / 2 - TOOLTIP_W / 2, viewW - TOOLTIP_W - 8))

    let top: number
    if (spaceBelow >= TOOLTIP_H + 16 || spaceBelow >= spaceAbove) {
      top = rect.y + rect.h + 12
    } else {
      top = rect.y - TOOLTIP_H - 12
    }
    // Clamp so the tooltip never escapes the viewport vertically.
    top = Math.max(8, Math.min(top, viewH - TOOLTIP_H - 8))

    style = { left, top, width: TOOLTIP_W }
  } else {
    // No rect: fall back to bottom-center
    style = {
      left:      '50%',
      bottom:    24,
      transform: 'translateX(-50%)',
      width:     TOOLTIP_W,
    }
  }

  return (
    <div
      className="fixed z-[10000] bg-brand-bg-card border border-brand-cyan/40 rounded-xl shadow-2xl p-4 pointer-events-auto"
      style={style}
      onClick={(e) => e.stopPropagation()}
    >
      <p className="text-[9px] text-brand-cyan uppercase tracking-widest font-semibold mb-1.5">
        {step + 1} / {TOUR_STEPS}
      </p>
      <p className="text-sm font-semibold text-brand-text mb-1">{def.title}</p>
      <p className="text-xs text-brand-muted leading-relaxed mb-4">{def.body}</p>
      <div className="flex items-center justify-between">
        <button
          onClick={onSkip}
          className="text-[11px] text-brand-muted hover:text-brand-text transition-colors"
        >
          Skip
        </button>
        <button
          onClick={onNext}
          className="px-4 py-1.5 rounded-lg bg-brand-cyan text-brand-bg text-xs font-semibold hover:bg-brand-cyan/90 transition-colors"
        >
          {def.cta}
        </button>
      </div>
    </div>
  )
}
