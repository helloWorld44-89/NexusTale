// Guide — 6-step novel wizard that scaffolds a project from premise to first scene.
// Steps 1-5 are backed by guide_steps on the server; Step 3.5 (Structure) is
// an optional frontend-only step whose result lands in projects.structure_id.
import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate, useSearchParams, Link } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'
import { api } from '@/services/api'
import type { GuideProgress, ProjectStructure } from '@/services/api'
import { StructureStep } from '@/components/guide/StructureStep'

// ── Step definitions ──────────────────────────────────────────────────────────

// Backend guide_steps keys — never include 'structure'.
type StepKey = 'premise' | 'characters' | 'world' | 'outline' | 'first_scene'

// Display step adds the optional structure step between world and outline.
type DisplayStep = StepKey | 'structure'

const DISPLAY_STEPS: DisplayStep[] = ['premise', 'characters', 'world', 'structure', 'outline', 'first_scene']

const DISPLAY_LABELS: Record<DisplayStep, string> = {
  premise:     'Premise',
  characters:  'Core Characters',
  world:       'World Basics',
  structure:   'Story Structure',
  outline:     'Chapter Outline',
  first_scene: 'First Scene',
}

// ── per-step default data ─────────────────────────────────────────────────────
function defaultData(step: StepKey): Record<string, unknown> {
  switch (step) {
    case 'premise':     return { logline: '', theme: '', genres: [] }
    case 'characters':  return { characters: [{ name: '', role: 'protagonist', description: '' }] }
    case 'world':       return { setting: '', locations: [{ name: '', description: '' }], magic_systems: [] }
    case 'outline':     return { chapters: [{ title: 'Chapter 1', summary: '' }] }
    case 'first_scene': return { title: 'Opening Scene', content: '' }
  }
}

export default function Guide() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const accessToken = useAuthStore((s) => s.accessToken)

  const [progress,          setProgress]          = useState<GuideProgress | null>(null)
  const [projectStructure,  setProjectStructure]  = useState<ProjectStructure | null>(null)
  const [activeStep,        setActiveStep]        = useState<DisplayStep>('premise')
  const [stepData,          setStepData]          = useState<Record<StepKey, Record<string, unknown>>>({
    premise:     defaultData('premise'),
    characters:  defaultData('characters'),
    world:       defaultData('world'),
    outline:     defaultData('outline'),
    first_scene: defaultData('first_scene'),
  })
  const [loading,      setLoading]      = useState(true)
  const [saving,       setSaving]       = useState(false)
  const [completing,   setCompleting]   = useState(false)
  const [error,        setError]        = useState<string | null>(null)

  // Load guide progress and current structure selection on mount.
  useEffect(() => {
    if (!id || !accessToken) return
    Promise.all([
      api.guide.getProgress(accessToken, id),
      api.structures.get(accessToken, id),
    ])
      .then(([p, s]) => {
        setProgress(p)
        setProjectStructure(s)
        // Merge saved step data into local state.
        const merged = { ...stepData }
        for (const step of p.steps) {
          if (step.data && Object.keys(step.data).length > 0) {
            merged[step.step_key as StepKey] = step.data as Record<string, unknown>
          }
        }
        setStepData(merged)
        // If a ?step= query param is present, honour it directly.
        const stepParam = searchParams.get('step')
        if (stepParam && DISPLAY_STEPS.includes(stepParam as DisplayStep)) {
          setActiveStep(stepParam as DisplayStep)
        } else {
          // Resume at the first incomplete backend step, inserting structure if world is done but outline isn't.
          const firstIncomplete = p.steps.find((step) => !step.is_complete)
          if (firstIncomplete) {
            const key = firstIncomplete.step_key as StepKey
            // If world is complete and structure not yet chosen, land on structure step.
            const worldDone   = p.steps.find((step) => step.step_key === 'world')?.is_complete ?? false
            const outlineDone = p.steps.find((step) => step.step_key === 'outline')?.is_complete ?? false
            if (worldDone && !outlineDone && !s.structure_id) {
              setActiveStep('structure')
            } else {
              setActiveStep(key)
            }
          }
        }
      })
      .catch(() => navigate(`/projects/${id}`, { replace: true }))
      .finally(() => setLoading(false))
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, accessToken])

  const updateField = useCallback((key: string, value: unknown) => {
    if (activeStep === 'structure') return
    setStepData((prev) => ({
      ...prev,
      [activeStep]: { ...prev[activeStep as StepKey], [key]: value },
    }))
  }, [activeStep])

  const handleSave = async () => {
    if (!id || !accessToken || activeStep === 'structure') return
    setSaving(true)
    try {
      await api.guide.saveStep(accessToken, id, activeStep, stepData[activeStep as StepKey])
    } catch { /* non-fatal autosave */ } finally {
      setSaving(false)
    }
  }

  const handleComplete = async () => {
    if (!id || !accessToken || activeStep === 'structure') return
    setCompleting(true)
    setError(null)
    try {
      const updated = await api.guide.completeStep(
        accessToken, id, activeStep as StepKey, stepData[activeStep as StepKey],
      )
      setProgress((prev) => {
        if (!prev) return prev
        return {
          ...prev,
          steps: prev.steps.map((s) => s.step_key === activeStep ? { ...s, ...updated } : s),
          completed_count: prev.steps.filter((s) => s.step_key === activeStep ? true : s.is_complete).length,
        }
      })
      advanceStep()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to complete step')
    } finally {
      setCompleting(false)
    }
  }

  // Advance to the next display step.
  const advanceStep = () => {
    const idx = DISPLAY_STEPS.indexOf(activeStep)
    if (idx < DISPLAY_STEPS.length - 1) setActiveStep(DISPLAY_STEPS[idx + 1])
  }

  const handleSkip = () => {
    if (activeStep === 'structure') { advanceStep(); return }
    advanceStep()
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

  // All five backend steps complete = wizard done (structure is optional, ignored here).
  const allComplete = progress ? progress.completed_count === progress.total_count : false

  // For the regular step form: look up backend progress.
  const currentStepInfo = activeStep !== 'structure'
    ? progress?.steps.find((s) => s.step_key === activeStep)
    : null

  // ── Sidebar step list ───────────────────────────────────────────────────────
  // Build a merged display list: inject the optional structure step and derive
  // its is_complete / label from projectStructure.
  const structureComplete = !!(projectStructure?.structure_id || projectStructure?.structure_custom)

  const displayStepList = DISPLAY_STEPS.map((key, i) => {
    if (key === 'structure') {
      return {
        step_key:   'structure' as const,
        label:      DISPLAY_LABELS.structure,
        is_complete: structureComplete,
        optional:   true,
        index:      i,
      }
    }
    const backendStep = progress?.steps.find((s) => s.step_key === key)
    return {
      step_key:   key,
      label:      DISPLAY_LABELS[key],
      is_complete: backendStep?.is_complete ?? false,
      optional:   false,
      index:      i,
    }
  })

  return (
    <div className="min-h-screen bg-brand-bg flex flex-col">
      {/* Header */}
      <header className="h-14 flex items-center justify-between px-6 bg-brand-bg-card border-b border-brand-border shrink-0">
        <div className="flex items-center gap-3">
          <Link to={`/projects/${id}`} className="text-brand-muted hover:text-brand-text transition-colors">
            <BackIcon />
          </Link>
          <div className="flex items-center gap-2">
            <img src="/app-icon.png" alt="" className="w-5 h-5 opacity-80" />
            <span className="text-brand-cyan font-semibold tracking-wide">NexusTale</span>
          </div>
          <span className="text-brand-muted/40">/</span>
          <span className="text-sm text-brand-text-muted">Novel Guide</span>
        </div>
        <Link to={`/projects/${id}`} className="text-sm text-brand-muted hover:text-brand-text transition-colors">
          Back to project
        </Link>
      </header>

      <div className="flex flex-1 max-w-5xl w-full mx-auto px-6 py-10 gap-10">
        {/* Sidebar — step list */}
        <nav className="w-52 shrink-0">
          <p className="text-xs uppercase tracking-widest text-brand-muted mb-4">Steps</p>
          {/* Progress bar — counts only the 5 backend steps */}
          {progress && (
            <div className="mb-6">
              <div className="h-1.5 rounded-full bg-brand-border overflow-hidden">
                <div
                  className="h-full bg-brand-cyan rounded-full transition-all"
                  style={{ width: `${(progress.completed_count / progress.total_count) * 100}%` }}
                />
              </div>
              <p className="text-xs text-brand-muted mt-1.5">
                {progress.completed_count} of {progress.total_count} complete
              </p>
            </div>
          )}
          <ol className="space-y-1">
            {displayStepList.map((s) => (
              <li key={s.step_key}>
                <button
                  onClick={() => setActiveStep(s.step_key as DisplayStep)}
                  className={`w-full text-left flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors
                    ${activeStep === s.step_key
                      ? 'bg-brand-cyan/10 text-brand-cyan'
                      : 'text-brand-muted hover:text-brand-text hover:bg-brand-bg-card'}`}
                >
                  <span className={`w-5 h-5 rounded-full flex items-center justify-center text-xs font-semibold shrink-0
                    ${s.is_complete
                      ? 'bg-emerald-500/20 text-emerald-400'
                      : activeStep === s.step_key
                        ? 'bg-brand-cyan/20 text-brand-cyan'
                        : 'bg-brand-border text-brand-muted'}`}
                  >
                    {s.is_complete ? '✓' : s.index + 1}
                  </span>
                  <span className="flex-1 min-w-0">
                    {s.label}
                    {s.optional && (
                      <span className="block text-[10px] text-brand-muted/60 leading-none mt-0.5">optional</span>
                    )}
                  </span>
                </button>
              </li>
            ))}
          </ol>
        </nav>

        {/* Main — active step form */}
        <main className="flex-1 min-w-0">
          {allComplete ? (
            <AllDone projectId={id!} />
          ) : activeStep === 'structure' ? (
            /* ── Structure step — its own self-contained component ── */
            <div>
              <div className="mb-6">
                <h1 className="text-2xl font-bold text-brand-text mb-1">Story Structure</h1>
                <p className="text-sm text-brand-muted">
                  Pick a template, get a personalised recommendation, or define your own rules.
                  This step is entirely optional.
                </p>
              </div>
              <div className="bg-brand-bg-card border border-brand-border rounded-xl p-6">
                <StructureStep
                  token={accessToken!}
                  projectId={id!}
                  onComplete={(s) => { setProjectStructure(s); advanceStep() }}
                  onSkip={advanceStep}
                />
              </div>
            </div>
          ) : (
            /* ── Regular wizard steps ── */
            <>
              <div className="mb-6">
                <h1 className="text-2xl font-bold text-brand-text mb-1">
                  {currentStepInfo?.label ?? DISPLAY_LABELS[activeStep as StepKey]}
                </h1>
                <StepHint step={activeStep as StepKey} />
              </div>

              <div className="bg-brand-bg-card border border-brand-border rounded-xl p-6 mb-6">
                <StepForm
                  step={activeStep as StepKey}
                  data={stepData[activeStep as StepKey]}
                  onChange={updateField}
                  onBlur={handleSave}
                />
              </div>

              {error && <p className="text-sm text-red-400 mb-4">{error}</p>}

              <div className="flex items-center gap-3">
                <button
                  onClick={handleComplete}
                  disabled={completing}
                  className="px-5 py-2.5 rounded-lg bg-brand-cyan text-brand-bg text-sm font-semibold hover:bg-brand-cyan/80 disabled:opacity-50 transition-colors"
                >
                  {completing ? 'Saving…' : currentStepInfo?.is_complete ? 'Update Step' : 'Complete Step'}
                </button>
                <button
                  onClick={handleSkip}
                  disabled={DISPLAY_STEPS.indexOf(activeStep) === DISPLAY_STEPS.length - 1}
                  className="px-4 py-2.5 rounded-lg border border-brand-border text-brand-muted text-sm hover:text-brand-text hover:border-brand-cyan/30 disabled:opacity-30 transition-colors"
                >
                  Skip for now
                </button>
                {saving && <span className="text-xs text-brand-muted">Saving…</span>}
              </div>
            </>
          )}
        </main>
      </div>
    </div>
  )
}

// ── Step hint text ────────────────────────────────────────────────────────────

function StepHint({ step }: { step: StepKey }) {
  const hints: Record<StepKey, string> = {
    premise:     'Capture the core idea of your story — a one-sentence hook, the central theme, and the genre.',
    characters:  'Name the key people in your story. You can add more later from the Wiki.',
    world:       'Describe where your story takes place. Add key locations and any magic or special systems.',
    outline:     'Sketch the chapters you plan to write. Titles and a brief summary are enough to start.',
    first_scene: 'Write or paste the opening of your story. Even a rough draft helps you start.',
  }
  return <p className="text-sm text-brand-muted">{hints[step]}</p>
}

// ── Step form components ──────────────────────────────────────────────────────

function StepForm({
  step,
  data,
  onChange,
  onBlur,
}: {
  step: StepKey
  data: Record<string, unknown>
  onChange: (key: string, value: unknown) => void
  onBlur: () => void
}) {
  switch (step) {
    case 'premise':     return <PremiseForm     data={data} onChange={onChange} onBlur={onBlur} />
    case 'characters':  return <CharactersForm  data={data} onChange={onChange} onBlur={onBlur} />
    case 'world':       return <WorldForm       data={data} onChange={onChange} onBlur={onBlur} />
    case 'outline':     return <OutlineForm     data={data} onChange={onChange} onBlur={onBlur} />
    case 'first_scene': return <FirstSceneForm  data={data} onChange={onChange} onBlur={onBlur} />
  }
}

type FormProps = {
  data: Record<string, unknown>
  onChange: (key: string, value: unknown) => void
  onBlur: () => void
}

function PremiseForm({ data, onChange, onBlur }: FormProps) {
  const genres = (data.genres as string[] | undefined) ?? []

  const toggleGenre = (g: string) => {
    onChange('genres', genres.includes(g) ? genres.filter((x) => x !== g) : [...genres, g])
  }

  const GENRE_OPTIONS = ['Fantasy', 'Sci-Fi', 'Horror', 'Romance', 'Thriller', 'Mystery', 'Historical', 'Literary']

  return (
    <div className="space-y-5">
      <Field label="Logline" hint="One sentence: who wants what, why it's hard, what's at stake.">
        <textarea
          rows={3}
          value={(data.logline as string) ?? ''}
          onChange={(e) => onChange('logline', e.target.value)}
          onBlur={onBlur}
          placeholder="A disgraced knight must recover a stolen relic before an ancient curse destroys the realm."
          className={inputCls + ' resize-none'}
        />
      </Field>
      <Field label="Central Theme" hint="What does the story ultimately explore?">
        <input
          type="text"
          value={(data.theme as string) ?? ''}
          onChange={(e) => onChange('theme', e.target.value)}
          onBlur={onBlur}
          placeholder="Redemption through sacrifice"
          className={inputCls}
        />
      </Field>
      <Field label="Genres">
        <div className="flex flex-wrap gap-2 mt-1">
          {GENRE_OPTIONS.map((g) => (
            <button
              key={g}
              type="button"
              onClick={() => { toggleGenre(g); onBlur() }}
              className={`px-3 py-1 rounded-full text-xs font-medium transition-colors
                ${genres.includes(g)
                  ? 'bg-brand-purple/30 text-brand-purple border border-brand-purple/40'
                  : 'bg-brand-bg border border-brand-border text-brand-muted hover:border-brand-cyan/30 hover:text-brand-text'}`}
            >
              {g}
            </button>
          ))}
        </div>
      </Field>
    </div>
  )
}

function CharactersForm({ data, onChange, onBlur }: FormProps) {
  type CharEntry = { name: string; role: string; description: string }
  const chars: CharEntry[] = (data.characters as CharEntry[] | undefined) ?? []

  const update = (i: number, field: keyof CharEntry, val: string) => {
    const updated = chars.map((c, idx) => idx === i ? { ...c, [field]: val } : c)
    onChange('characters', updated)
  }

  const add = () => {
    onChange('characters', [...chars, { name: '', role: 'supporting', description: '' }])
  }

  const remove = (i: number) => {
    onChange('characters', chars.filter((_, idx) => idx !== i))
    onBlur()
  }

  return (
    <div className="space-y-4">
      {chars.map((ch, i) => (
        <div key={i} className="grid grid-cols-1 sm:grid-cols-3 gap-3 pb-4 border-b border-brand-border last:border-0">
          <input
            type="text"
            value={ch.name}
            onChange={(e) => update(i, 'name', e.target.value)}
            onBlur={onBlur}
            placeholder="Character name"
            className={inputCls}
          />
          <select
            value={ch.role}
            onChange={(e) => { update(i, 'role', e.target.value); onBlur() }}
            className={inputCls}
          >
            <option value="protagonist">Protagonist</option>
            <option value="antagonist">Antagonist</option>
            <option value="supporting">Supporting</option>
            <option value="mentor">Mentor</option>
            <option value="love_interest">Love interest</option>
          </select>
          <div className="flex gap-2">
            <input
              type="text"
              value={ch.description}
              onChange={(e) => update(i, 'description', e.target.value)}
              onBlur={onBlur}
              placeholder="Brief description"
              className={inputCls + ' flex-1'}
            />
            {chars.length > 1 && (
              <button onClick={() => remove(i)} className="text-brand-muted hover:text-red-400 transition-colors px-1">
                ×
              </button>
            )}
          </div>
        </div>
      ))}
      <button
        type="button"
        onClick={add}
        className="text-sm text-brand-cyan hover:text-brand-cyan/80 transition-colors"
      >
        + Add character
      </button>
    </div>
  )
}

function WorldForm({ data, onChange, onBlur }: FormProps) {
  type LocEntry = { name: string; description: string }
  type MagicEntry = { name: string; description: string }
  const locations: LocEntry[] = (data.locations as LocEntry[] | undefined) ?? []
  const magicSystems: MagicEntry[] = (data.magic_systems as MagicEntry[] | undefined) ?? []

  return (
    <div className="space-y-5">
      <Field label="Setting" hint="Where and when does this story take place?">
        <textarea
          rows={2}
          value={(data.setting as string) ?? ''}
          onChange={(e) => onChange('setting', e.target.value)}
          onBlur={onBlur}
          placeholder="A frozen empire at the edge of a dying star system, three centuries after the Collapse."
          className={inputCls + ' resize-none'}
        />
      </Field>

      <Field label="Key Locations" hint="Places your characters will visit or that shape the world.">
        <div className="space-y-2">
          {locations.map((loc, i) => (
            <div key={i} className="flex gap-2">
              <input
                type="text"
                value={loc.name}
                onChange={(e) => {
                  const updated = locations.map((l, idx) => idx === i ? { ...l, name: e.target.value } : l)
                  onChange('locations', updated)
                }}
                onBlur={onBlur}
                placeholder="Name"
                className={inputCls + ' w-40'}
              />
              <input
                type="text"
                value={loc.description}
                onChange={(e) => {
                  const updated = locations.map((l, idx) => idx === i ? { ...l, description: e.target.value } : l)
                  onChange('locations', updated)
                }}
                onBlur={onBlur}
                placeholder="Brief description"
                className={inputCls + ' flex-1'}
              />
              {locations.length > 1 && (
                <button
                  onClick={() => { onChange('locations', locations.filter((_, idx) => idx !== i)); onBlur() }}
                  className="text-brand-muted hover:text-red-400 transition-colors px-1"
                >×</button>
              )}
            </div>
          ))}
          <button
            type="button"
            onClick={() => onChange('locations', [...locations, { name: '', description: '' }])}
            className="text-sm text-brand-cyan hover:text-brand-cyan/80 transition-colors"
          >
            + Add location
          </button>
        </div>
      </Field>

      <Field label="Magic / Special Systems" hint="Leave empty for stories without magic or special tech rules.">
        <div className="space-y-2">
          {magicSystems.map((ms, i) => (
            <div key={i} className="flex gap-2">
              <input
                type="text"
                value={ms.name}
                onChange={(e) => {
                  const updated = magicSystems.map((m, idx) => idx === i ? { ...m, name: e.target.value } : m)
                  onChange('magic_systems', updated)
                }}
                onBlur={onBlur}
                placeholder="System name"
                className={inputCls + ' w-40'}
              />
              <input
                type="text"
                value={ms.description}
                onChange={(e) => {
                  const updated = magicSystems.map((m, idx) => idx === i ? { ...m, description: e.target.value } : m)
                  onChange('magic_systems', updated)
                }}
                onBlur={onBlur}
                placeholder="Core rule or description"
                className={inputCls + ' flex-1'}
              />
              <button
                onClick={() => { onChange('magic_systems', magicSystems.filter((_, idx) => idx !== i)); onBlur() }}
                className="text-brand-muted hover:text-red-400 transition-colors px-1"
              >×</button>
            </div>
          ))}
          <button
            type="button"
            onClick={() => onChange('magic_systems', [...magicSystems, { name: '', description: '' }])}
            className="text-sm text-brand-cyan hover:text-brand-cyan/80 transition-colors"
          >
            + Add system
          </button>
        </div>
      </Field>
    </div>
  )
}

function OutlineForm({ data, onChange, onBlur }: FormProps) {
  type ChEntry = { title: string; summary: string }
  const chapters: ChEntry[] = (data.chapters as ChEntry[] | undefined) ?? []

  return (
    <div className="space-y-3">
      {chapters.map((ch, i) => (
        <div key={i} className="flex gap-2 items-start">
          <span className="text-xs text-brand-muted mt-2.5 w-6 shrink-0 text-right">{i + 1}.</span>
          <input
            type="text"
            value={ch.title}
            onChange={(e) => {
              const updated = chapters.map((c, idx) => idx === i ? { ...c, title: e.target.value } : c)
              onChange('chapters', updated)
            }}
            onBlur={onBlur}
            placeholder="Chapter title"
            className={inputCls + ' w-48'}
          />
          <input
            type="text"
            value={ch.summary}
            onChange={(e) => {
              const updated = chapters.map((c, idx) => idx === i ? { ...c, summary: e.target.value } : c)
              onChange('chapters', updated)
            }}
            onBlur={onBlur}
            placeholder="What happens? (optional)"
            className={inputCls + ' flex-1'}
          />
          {chapters.length > 1 && (
            <button
              onClick={() => { onChange('chapters', chapters.filter((_, idx) => idx !== i)); onBlur() }}
              className="text-brand-muted hover:text-red-400 transition-colors px-1 mt-2"
            >×</button>
          )}
        </div>
      ))}
      <button
        type="button"
        onClick={() => onChange('chapters', [...chapters, { title: `Chapter ${chapters.length + 1}`, summary: '' }])}
        className="text-sm text-brand-cyan hover:text-brand-cyan/80 transition-colors ml-8"
      >
        + Add chapter
      </button>
    </div>
  )
}

function FirstSceneForm({ data, onChange, onBlur }: FormProps) {
  return (
    <div className="space-y-4">
      <Field label="Scene Title">
        <input
          type="text"
          value={(data.title as string) ?? ''}
          onChange={(e) => onChange('title', e.target.value)}
          onBlur={onBlur}
          placeholder="Opening Scene"
          className={inputCls}
        />
      </Field>
      <Field label="Content" hint="Write freely — this is your first draft. You can edit it in the editor.">
        <textarea
          rows={12}
          value={(data.content as string) ?? ''}
          onChange={(e) => onChange('content', e.target.value)}
          onBlur={onBlur}
          placeholder="The day everything changed began like any other…"
          className={inputCls + ' resize-y font-serif'}
        />
      </Field>
    </div>
  )
}

// ── All done screen ───────────────────────────────────────────────────────────

function AllDone({ projectId }: { projectId: string }) {
  const navigate = useNavigate()
  return (
    <div className="flex flex-col items-center justify-center py-20 text-center">
      <div className="w-16 h-16 rounded-full bg-emerald-500/10 flex items-center justify-center mb-6">
        <svg className="w-8 h-8 text-emerald-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M20 6L9 17l-5-5" />
        </svg>
      </div>
      <h2 className="text-2xl font-bold text-brand-text mb-3">Your novel is scaffolded!</h2>
      <p className="text-brand-muted max-w-md mb-8">
        Characters, locations, and chapters have been added to your project. Head to the editor to start writing.
      </p>
      <div className="flex gap-4">
        <button
          onClick={() => navigate(`/projects/${projectId}/editor`)}
          className="px-6 py-2.5 rounded-lg bg-brand-cyan text-brand-bg text-sm font-semibold hover:bg-brand-cyan/80 transition-colors"
        >
          Open Editor
        </button>
        <button
          onClick={() => navigate(`/projects/${projectId}`)}
          className="px-5 py-2.5 rounded-lg border border-brand-border text-brand-muted text-sm hover:text-brand-text hover:border-brand-cyan/30 transition-colors"
        >
          Back to Project
        </button>
      </div>
    </div>
  )
}

// ── Shared UI primitives ──────────────────────────────────────────────────────

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-xs font-medium text-brand-text-muted uppercase tracking-wider">{label}</label>
      {hint && <p className="text-xs text-brand-muted -mt-1">{hint}</p>}
      {children}
    </div>
  )
}

const inputCls = 'bg-brand-bg border border-brand-border rounded-lg px-3 py-2 text-sm text-brand-text placeholder:text-brand-muted/40 focus:outline-none focus:border-brand-cyan transition-colors'

function BackIcon() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M13 4l-6 6 6 6" />
    </svg>
  )
}
