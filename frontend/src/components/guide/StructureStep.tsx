// StructureStep — the optional Step 3.5 in the novel guide wizard.
// Four paths: questionnaire → scored recommendation, browse template grid,
// freeform (custom rules), or skip entirely.
import { useEffect, useState } from 'react'
import { api } from '@/services/api'
import type { NovelStructure, StructureScore, ProjectStructure } from '@/services/api'

// ── Question definitions (must match score.go answer keys) ───────────────────

type Question = {
  key: string
  text: string
  multi: boolean
  options: { value: string; label: string }[]
}

const QUESTIONS: Question[] = [
  {
    key: 'q1', text: 'What reading experience do you want to create?', multi: true,
    options: [
      { value: 'fast_paced',   label: 'Fast-paced / page-turner' },
      { value: 'suspenseful',  label: 'Suspenseful / tense' },
      { value: 'emotional',    label: 'Emotional / introspective' },
      { value: 'epic',         label: 'Epic / mythic' },
      { value: 'cozy',         label: 'Cozy / low-stakes' },
      { value: 'experimental', label: 'Experimental / unconventional' },
    ],
  },
  {
    key: 'q2', text: 'What primarily drives your story?', multi: false,
    options: [
      { value: 'external_plot',            label: 'External plot and events' },
      { value: 'character_transformation', label: 'Character transformation' },
      { value: 'mystery',                  label: 'A mystery or question' },
      { value: 'theme_contrast',           label: 'Theme, mood, or contrast' },
      { value: 'plan_mission',             label: 'A plan, mission, or objective' },
    ],
  },
  {
    key: 'q3', text: 'How central is conflict?', multi: false,
    options: [
      { value: 'one_central',     label: 'One clear central conflict' },
      { value: 'many_escalating', label: 'Many escalating conflicts' },
      { value: 'constant_crises', label: 'Ongoing crises with little downtime' },
      { value: 'minimal',         label: 'Minimal or indirect conflict' },
    ],
  },
  {
    key: 'q4', text: 'How do stakes change over time?', multi: false,
    options: [
      { value: 'gradual',           label: 'Gradual escalation' },
      { value: 'constant_pressure', label: 'Constant high pressure' },
      { value: 'episodic',          label: 'Episodic challenges' },
      { value: 'mostly_internal',   label: 'Mostly internal or emotional' },
    ],
  },
  {
    key: 'q5', text: 'How much does the protagonist change?', multi: false,
    options: [
      { value: 'fundamental_transformation', label: 'Fundamental personal transformation' },
      { value: 'moderate_growth',            label: 'Moderate growth' },
      { value: 'little_change',              label: 'Little to no change' },
      { value: 'moral_decline',              label: 'Moral or psychological decline' },
    ],
  },
  {
    key: 'q6', text: 'What matters more — success or transformation?', multi: false,
    options: [
      { value: 'transformation', label: 'Transformation' },
      { value: 'success',        label: 'Success' },
      { value: 'both',           label: 'Both equally' },
    ],
  },
  {
    key: 'q7', text: 'How should the story begin?', multi: false,
    options: [
      { value: 'calm_setup',        label: 'Calm setup, ordinary life first' },
      { value: 'immediate_action',  label: 'Immediate action' },
      { value: 'already_in_motion', label: 'Story already in motion' },
      { value: 'ordinary_life',     label: 'Ordinary life disrupted by an event' },
    ],
  },
  {
    key: 'q8', text: 'How should the story end?', multi: false,
    options: [
      { value: 'clean',       label: 'Clean resolution' },
      { value: 'bittersweet', label: 'Bittersweet resolution' },
      { value: 'ambiguous',   label: 'Ambiguous ending' },
      { value: 'tragic',      label: 'Tragic or inevitable outcome' },
    ],
  },
  {
    key: 'q9', text: 'How much structural guidance do you want?', multi: false,
    options: [
      { value: 'very_clear', label: 'Very clear acts and beats' },
      { value: 'loose',      label: 'Loose guidance' },
      { value: 'freestyle',  label: 'Minimal — I\'ll define my own structure' },
    ],
  },
  {
    key: 'q10', text: 'Are you intentionally borrowing from any structure?', multi: true,
    options: [
      { value: 'three_act',     label: 'Three-Act Structure' },
      { value: 'heros_journey', label: "Hero's Journey" },
      { value: 'mystery',       label: 'Mystery / Investigation' },
      { value: 'heist',         label: 'Heist' },
      { value: 'save_the_cat',  label: 'Save the Cat' },
      { value: 'other',         label: 'Other / Custom' },
    ],
  },
]

// ── Types ────────────────────────────────────────────────────────────────────

type Path = 'choose' | 'questionnaire' | 'browse' | 'freeform' | 'result'

type FreeformData = {
  acts:     string
  midpoint: string
  ending:   string
  rules:    string
}

type Props = {
  token:     string
  projectId: string
  /** Called after a successful structure selection or explicit freeform completion. */
  onComplete: (structure: ProjectStructure) => void
  /** Called when the user skips the step entirely. */
  onSkip: () => void
}

// ── Component ────────────────────────────────────────────────────────────────

export function StructureStep({ token, projectId, onComplete, onSkip }: Props) {
  const [path,       setPath]       = useState<Path>('choose')
  const [structures, setStructures] = useState<NovelStructure[]>([])
  const [answers,    setAnswers]    = useState<Record<string, string[]>>({})
  const [qIndex,     setQIndex]     = useState(0)
  const [ranked,     setRanked]     = useState<StructureScore[]>([])
  const [freeform,   setFreeform]   = useState<FreeformData>({ acts: '', midpoint: '', ending: '', rules: '' })
  const [expanded,   setExpanded]   = useState<string | null>(null)
  const [scoring,    setScoring]    = useState(false)
  const [saving,     setSaving]     = useState(false)
  const [error,      setError]      = useState<string | null>(null)

  // Load structure catalog once.
  useEffect(() => {
    api.structures.list().then(setStructures).catch(() => {/* non-fatal */})
  }, [])

  // ── answer helpers ──────────────────────────────────────────────────────────

  const toggleAnswer = (qKey: string, value: string, multi: boolean) => {
    setAnswers((prev) => {
      const current = prev[qKey] ?? []
      if (multi) {
        return {
          ...prev,
          [qKey]: current.includes(value) ? current.filter((v) => v !== value) : [...current, value],
        }
      }
      return { ...prev, [qKey]: [value] }
    })
  }

  const currentQ = QUESTIONS[qIndex]

  // ── submit handlers ─────────────────────────────────────────────────────────

  const handleScore = async () => {
    setScoring(true)
    setError(null)
    try {
      const { ranked: r } = await api.structures.score(token, projectId, answers)
      setRanked(r)
      setPath('result')
    } catch {
      setError('Scoring failed — please try again.')
    } finally {
      setScoring(false)
    }
  }

  const handleSelectStructure = async (structureId: string) => {
    setSaving(true)
    setError(null)
    try {
      const updated = await api.structures.update(token, projectId, { structure_id: structureId })
      onComplete(updated)
    } catch {
      setError('Failed to save structure — please try again.')
      setSaving(false)
    }
  }

  const handleFreeformComplete = async () => {
    const rules = freeform.rules.trim()
      ? freeform.rules.split('\n').map((r) => r.trim()).filter(Boolean)
      : []
    const custom = {
      acts:     freeform.acts     || undefined,
      midpoint: freeform.midpoint || undefined,
      ending:   freeform.ending   || undefined,
      rules:    rules.length > 0 ? rules : undefined,
    }
    setSaving(true)
    setError(null)
    try {
      const updated = await api.structures.update(token, projectId, {
        structure_id: null,
        structure_custom: custom,
      })
      onComplete(updated)
    } catch {
      setError('Failed to save — please try again.')
      setSaving(false)
    }
  }

  // ── renders ─────────────────────────────────────────────────────────────────

  if (path === 'choose') {
    return (
      <div className="space-y-4">
        <p className="text-sm text-brand-muted">
          A story structure is a tool, not a requirement. Choose how you'd like to approach it — or skip entirely.
        </p>
        <div className="grid gap-3">
          {[
            { label: 'Answer a few questions',   sub: 'Get a personalised recommendation based on your answers.',  action: () => { setPath('questionnaire'); setQIndex(0) } },
            { label: 'Browse templates',          sub: 'Pick a named structure from the full catalog.',               action: () => setPath('browse') },
            { label: 'Define my own structure',   sub: 'Tell the AI your acts, midpoint, and custom rules.',          action: () => setPath('freeform') },
            { label: 'Skip — no structure',       sub: 'Write completely freeform. You can add a structure later.',   action: onSkip },
          ].map(({ label, sub, action }) => (
            <button
              key={label}
              onClick={action}
              className="w-full text-left px-5 py-4 rounded-xl border border-brand-border bg-brand-bg hover:border-brand-cyan/40 hover:bg-brand-bg-card transition-colors group"
            >
              <p className="text-sm font-medium text-brand-text group-hover:text-brand-cyan transition-colors">{label}</p>
              <p className="text-xs text-brand-muted mt-0.5">{sub}</p>
            </button>
          ))}
        </div>
      </div>
    )
  }

  if (path === 'questionnaire') {
    const q      = currentQ
    const sel    = answers[q.key] ?? []
    const isLast = qIndex === QUESTIONS.length - 1
    const answeredCount = Object.keys(answers).filter((k) => (answers[k]?.length ?? 0) > 0).length

    return (
      <div className="space-y-5">
        {/* Progress */}
        <div className="flex items-center gap-3">
          <div className="flex-1 h-1.5 rounded-full bg-brand-border overflow-hidden">
            <div
              className="h-full bg-brand-cyan rounded-full transition-all"
              style={{ width: `${((qIndex + 1) / QUESTIONS.length) * 100}%` }}
            />
          </div>
          <span className="text-xs text-brand-muted shrink-0">{qIndex + 1} / {QUESTIONS.length}</span>
        </div>

        {/* Question */}
        <div>
          <p className="text-sm font-medium text-brand-text mb-3">{q.text}</p>
          {q.multi && <p className="text-xs text-brand-muted mb-3">Select all that apply.</p>}
          <div className="space-y-2">
            {q.options.map((opt) => {
              const active = sel.includes(opt.value)
              return (
                <button
                  key={opt.value}
                  onClick={() => toggleAnswer(q.key, opt.value, q.multi)}
                  className={`w-full text-left px-4 py-2.5 rounded-lg border text-sm transition-colors
                    ${active
                      ? 'border-brand-cyan/60 bg-brand-cyan/10 text-brand-cyan'
                      : 'border-brand-border bg-brand-bg text-brand-muted hover:border-brand-cyan/30 hover:text-brand-text'}`}
                >
                  {opt.label}
                </button>
              )
            })}
          </div>
        </div>

        {/* Navigation */}
        <div className="flex items-center gap-3 pt-1">
          {isLast ? (
            <button
              onClick={handleScore}
              disabled={scoring || answeredCount === 0}
              className="px-5 py-2.5 rounded-lg bg-brand-cyan text-brand-bg text-sm font-semibold hover:bg-brand-cyan/80 disabled:opacity-50 transition-colors"
            >
              {scoring ? 'Analysing…' : 'Get recommendation'}
            </button>
          ) : (
            <button
              onClick={() => setQIndex((i) => i + 1)}
              className="px-5 py-2.5 rounded-lg bg-brand-cyan text-brand-bg text-sm font-semibold hover:bg-brand-cyan/80 transition-colors"
            >
              Next
            </button>
          )}
          <button
            onClick={() => isLast ? handleScore() : setQIndex((i) => i + 1)}
            className="px-4 py-2.5 rounded-lg border border-brand-border text-brand-muted text-sm hover:text-brand-text hover:border-brand-cyan/30 transition-colors"
          >
            {isLast ? (scoring ? '…' : 'Skip & score') : 'Skip question'}
          </button>
          {qIndex > 0 && (
            <button
              onClick={() => setQIndex((i) => i - 1)}
              className="text-xs text-brand-muted hover:text-brand-text transition-colors ml-auto"
            >
              ← Back
            </button>
          )}
        </div>
        {error && <p className="text-sm text-red-400">{error}</p>}
        <button
          onClick={() => setPath('choose')}
          className="text-xs text-brand-muted hover:text-brand-text transition-colors"
        >
          ← Back to options
        </button>
      </div>
    )
  }

  if (path === 'result') {
    const primary   = ranked.filter((r) => !r.is_secondary)
    const secondary = ranked.filter((r) => r.is_secondary)

    return (
      <div className="space-y-5">
        {ranked.length === 0 ? (
          <div className="text-center py-8">
            <p className="text-brand-text font-medium mb-2">Freeform recommended</p>
            <p className="text-sm text-brand-muted mb-6">Your answers suggest a unique story. Define your own structure below, or skip.</p>
            <div className="flex gap-3 justify-center">
              <button onClick={() => setPath('freeform')} className={btnPrimary}>Define my own</button>
              <button onClick={onSkip} className={btnSecondary}>Skip</button>
            </div>
          </div>
        ) : (
          <>
            <div>
              <p className="text-xs uppercase tracking-widest text-brand-muted mb-3">Best match</p>
              {primary.map((r) => (
                <ResultCard
                  key={r.structure_id}
                  result={r}
                  structures={structures}
                  onUse={() => handleSelectStructure(r.structure_id)}
                  saving={saving}
                />
              ))}
            </div>
            {secondary.length > 0 && (
              <div>
                <p className="text-xs uppercase tracking-widest text-brand-muted mb-3">Could also borrow from</p>
                {secondary.map((r) => (
                  <ResultCard
                    key={r.structure_id}
                    result={r}
                    structures={structures}
                    onUse={() => handleSelectStructure(r.structure_id)}
                    saving={saving}
                  />
                ))}
              </div>
            )}
            <div className="flex gap-3 pt-1">
              <button onClick={() => setPath('browse')} className={btnSecondary}>Browse all templates</button>
              <button onClick={onSkip} className="text-xs text-brand-muted hover:text-brand-text transition-colors px-2">Continue without structure</button>
            </div>
          </>
        )}
        {error && <p className="text-sm text-red-400">{error}</p>}
      </div>
    )
  }

  if (path === 'browse') {
    return (
      <div className="space-y-4">
        <p className="text-xs uppercase tracking-widest text-brand-muted">All templates</p>
        <div className="space-y-2">
          {structures.map((s) => {
            const open = expanded === s.id
            return (
              <div key={s.id} className="border border-brand-border rounded-xl overflow-hidden">
                <button
                  onClick={() => setExpanded(open ? null : s.id)}
                  className="w-full flex items-center justify-between px-5 py-3.5 bg-brand-bg-card hover:bg-brand-bg transition-colors text-left"
                >
                  <div>
                    <p className="text-sm font-medium text-brand-text">{s.name}</p>
                    <p className="text-xs text-brand-muted mt-0.5 line-clamp-1">{s.description}</p>
                  </div>
                  <ChevronIcon open={open} />
                </button>
                {open && (
                  <div className="px-5 pb-4 bg-brand-bg border-t border-brand-border space-y-3">
                    <p className="text-xs text-brand-muted pt-3">{s.description}</p>
                    <div className="grid grid-cols-2 gap-3 text-xs">
                      <div>
                        <p className="text-emerald-400 font-medium mb-1">Strengths</p>
                        <p className="text-brand-muted">{s.strengths}</p>
                      </div>
                      <div>
                        <p className="text-amber-400 font-medium mb-1">Risks</p>
                        <p className="text-brand-muted">{s.risks}</p>
                      </div>
                    </div>
                    {s.phases.length > 0 && (
                      <div className="space-y-1">
                        <p className="text-xs text-brand-muted font-medium">Phases</p>
                        <ol className="space-y-1">
                          {s.phases.map((ph, i) => (
                            <li key={i} className="text-xs text-brand-muted flex gap-2">
                              <span className="shrink-0 text-brand-cyan/60">{i + 1}.</span>
                              <span className="font-medium text-brand-text-muted">{ph.name}</span>
                            </li>
                          ))}
                        </ol>
                      </div>
                    )}
                    <button
                      onClick={() => handleSelectStructure(s.id)}
                      disabled={saving}
                      className={btnPrimary + ' mt-2'}
                    >
                      {saving ? 'Saving…' : 'Use this structure'}
                    </button>
                  </div>
                )}
              </div>
            )
          })}
        </div>
        {error && <p className="text-sm text-red-400">{error}</p>}
        <div className="flex gap-3 pt-2">
          <button onClick={() => setPath('freeform')} className={btnSecondary}>Define my own instead</button>
          <button onClick={onSkip} className="text-xs text-brand-muted hover:text-brand-text transition-colors px-2">None of these fit — skip</button>
        </div>
        <button onClick={() => setPath('choose')} className="text-xs text-brand-muted hover:text-brand-text transition-colors">← Back</button>
      </div>
    )
  }

  // path === 'freeform'
  return (
    <div className="space-y-5">
      <p className="text-sm text-brand-muted">
        Define your own story structure. These details are optional — fill in only what helps you plan.
        The AI will treat them as guidance, not rigid rules.
      </p>
      <div className="space-y-4">
        {[
          { key: 'acts'     as const, label: 'How many acts or phases?',         placeholder: 'e.g. 3 acts, or: Setup / Rising / Climax / Fallout' },
          { key: 'midpoint' as const, label: 'What marks the midpoint?',          placeholder: 'e.g. The protagonist discovers the conspiracy' },
          { key: 'ending'   as const, label: 'What changes by the end?',          placeholder: 'e.g. The hero accepts their past and moves on' },
        ].map(({ key, label, placeholder }) => (
          <div key={key} className="space-y-1.5">
            <label className="text-xs font-medium text-brand-text-muted uppercase tracking-wider">{label}</label>
            <input
              type="text"
              value={freeform[key]}
              onChange={(e) => setFreeform((p) => ({ ...p, [key]: e.target.value }))}
              placeholder={placeholder}
              className={inputCls}
            />
          </div>
        ))}
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-brand-text-muted uppercase tracking-wider">Rules for the AI</label>
          <p className="text-xs text-brand-muted">One rule per line. The AI will follow these when suggesting beats or continuing scenes.</p>
          <textarea
            rows={4}
            value={freeform.rules}
            onChange={(e) => setFreeform((p) => ({ ...p, rules: e.target.value }))}
            placeholder={"Avoid flashbacks\nEvery chapter ends on a question\nThe antagonist must remain sympathetic"}
            className={inputCls + ' resize-none'}
          />
        </div>
      </div>
      {error && <p className="text-sm text-red-400">{error}</p>}
      <div className="flex gap-3">
        <button onClick={handleFreeformComplete} disabled={saving} className={btnPrimary}>
          {saving ? 'Saving…' : 'Save my structure'}
        </button>
        <button onClick={onSkip} className={btnSecondary}>Skip for now</button>
      </div>
      <button onClick={() => setPath('choose')} className="text-xs text-brand-muted hover:text-brand-text transition-colors">← Back</button>
    </div>
  )
}

// ── Sub-components ────────────────────────────────────────────────────────────

function ResultCard({
  result,
  structures,
  onUse,
  saving,
}: {
  result:     StructureScore
  structures: NovelStructure[]
  onUse:      () => void
  saving:     boolean
}) {
  const detail = structures.find((s) => s.id === result.structure_id)
  const [open, setOpen] = useState(false)

  return (
    <div className={`border rounded-xl mb-3 overflow-hidden ${result.is_secondary ? 'border-brand-border' : 'border-brand-cyan/40'}`}>
      <div className="px-5 py-4 bg-brand-bg-card">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="text-sm font-semibold text-brand-text">{result.name}</p>
            {detail && <p className="text-xs text-brand-muted mt-0.5 line-clamp-2">{detail.description}</p>}
          </div>
          <span className="text-xs text-brand-cyan font-mono shrink-0">{result.score} pts</span>
        </div>
        <div className="flex items-center gap-3 mt-3">
          <button onClick={onUse} disabled={saving} className={btnPrimary + ' text-xs py-1.5'}>
            {saving ? 'Saving…' : 'Use this'}
          </button>
          {detail && (
            <button
              onClick={() => setOpen((v) => !v)}
              className="text-xs text-brand-muted hover:text-brand-text transition-colors"
            >
              {open ? 'Hide details' : 'See details'}
            </button>
          )}
        </div>
      </div>
      {open && detail && (
        <div className="px-5 pb-4 bg-brand-bg border-t border-brand-border space-y-3 text-xs">
          <div className="grid grid-cols-2 gap-3 pt-3">
            <div>
              <p className="text-emerald-400 font-medium mb-1">Strengths</p>
              <p className="text-brand-muted">{detail.strengths}</p>
            </div>
            <div>
              <p className="text-amber-400 font-medium mb-1">Risks</p>
              <p className="text-brand-muted">{detail.risks}</p>
            </div>
          </div>
          {detail.phases.length > 0 && (
            <ol className="space-y-1">
              {detail.phases.map((ph, i) => (
                <li key={i} className="flex gap-2 text-brand-muted">
                  <span className="shrink-0 text-brand-cyan/60">{i + 1}.</span>
                  <span>{ph.name}</span>
                </li>
              ))}
            </ol>
          )}
        </div>
      )}
    </div>
  )
}

function ChevronIcon({ open }: { open: boolean }) {
  return (
    <svg
      className={`w-4 h-4 text-brand-muted transition-transform ${open ? 'rotate-180' : ''}`}
      viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5"
      strokeLinecap="round" strokeLinejoin="round"
    >
      <path d="M5 8l5 5 5-5" />
    </svg>
  )
}

// ── Shared style shortcuts ────────────────────────────────────────────────────

const btnPrimary   = 'px-4 py-2 rounded-lg bg-brand-cyan text-brand-bg text-sm font-semibold hover:bg-brand-cyan/80 disabled:opacity-50 transition-colors'
const btnSecondary = 'px-4 py-2 rounded-lg border border-brand-border text-brand-muted text-sm hover:text-brand-text hover:border-brand-cyan/30 transition-colors'
const inputCls     = 'w-full bg-brand-bg border border-brand-border rounded-lg px-3 py-2 text-sm text-brand-text placeholder:text-brand-muted/40 focus:outline-none focus:border-brand-cyan transition-colors'
