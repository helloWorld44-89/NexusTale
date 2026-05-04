// MagicRulePanel — list and detail view for magic systems in a project wiki.
import { useState, useEffect, useCallback } from 'react'
import { api } from '@/services/api'
import type { MagicRule, MagicRuleAttributes } from '@/services/api'

const CLARITY_OPTIONS: { value: MagicRuleAttributes['rules_clarity']; label: string; hint: string }[] = [
  { value: 'defined',    label: 'Defined',    hint: 'Rules are explicit — readers and characters know the limits' },
  { value: 'mysterious', label: 'Mysterious', hint: 'Rules are hidden — power feels unknowable and vast' },
  { value: 'mixed',      label: 'Mixed',      hint: 'Some rules are clear; others remain unknown' },
]

const ATTR_FIELDS: { key: keyof Omit<MagicRuleAttributes, 'rules_clarity'>; label: string; hint: string; prominent?: boolean }[] = [
  { key: 'limitations',   label: 'Limitations & Costs',  hint: 'What the magic cannot do and what it costs to use', prominent: true },
  { key: 'powers',        label: 'What it can do',        hint: 'Abilities and scope of the system' },
  { key: 'cost',          label: 'Cost to use',           hint: 'Price paid by the user — fatigue, sacrifice, resource drain' },
  { key: 'source',        label: 'Source & Mechanics',    hint: 'Where the power comes from and how it works internally' },
  { key: 'accessibility', label: 'Who can access it',     hint: 'Conditions for gaining or using this ability' },
]

interface Props {
  token: string
  projectId: string
}

export default function MagicRulePanel({ token, projectId }: Props) {
  const [rules, setRules] = useState<MagicRule[]>([])
  const [selected, setSelected] = useState<MagicRule | null>(null)
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await api.wiki.listMagicRules(token, projectId)
      setRules(data)
    } finally {
      setLoading(false)
    }
  }, [token, projectId])

  useEffect(() => { load() }, [load])

  function handleCreated(r: MagicRule) {
    setRules((prev) => [...prev, r].sort((a, b) => a.name.localeCompare(b.name)))
    setSelected(r)
    setCreating(false)
  }

  function handleUpdated(r: MagicRule) {
    setRules((prev) => prev.map((x) => x.id === r.id ? r : x))
    setSelected(r)
  }

  function handleDeleted(id: string) {
    setRules((prev) => prev.filter((x) => x.id !== id))
    setSelected(null)
  }

  return (
    <div className="flex gap-6 h-full">
      {/* Sidebar list */}
      <aside className="w-56 shrink-0 flex flex-col gap-2">
        <button
          onClick={() => setCreating(true)}
          className="w-full py-2 rounded-lg border border-dashed border-brand-border text-brand-muted text-xs hover:text-brand-text hover:border-brand-purple/50 transition-colors flex items-center justify-center gap-1.5"
        >
          <PlusIcon /> New magic system
        </button>

        {loading ? (
          <p className="text-xs text-brand-muted px-1 pt-1">Loading…</p>
        ) : rules.length === 0 ? (
          <p className="text-xs text-brand-muted px-1 pt-1">No magic systems yet.</p>
        ) : (
          <ul className="space-y-0.5">
            {rules.map((r) => (
              <li key={r.id}>
                <button
                  onClick={() => setSelected(r)}
                  className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors ${
                    selected?.id === r.id
                      ? 'bg-brand-purple/15 text-brand-text'
                      : 'text-brand-muted hover:bg-brand-border/30 hover:text-brand-text'
                  }`}
                >
                  <span className="line-clamp-1">{r.name}</span>
                  {(r.attributes as MagicRuleAttributes)?.rules_clarity && (
                    <span className="text-[10px] text-brand-muted capitalize">
                      {(r.attributes as MagicRuleAttributes).rules_clarity}
                    </span>
                  )}
                </button>
              </li>
            ))}
          </ul>
        )}
      </aside>

      {/* Detail pane */}
      <div className="flex-1 min-w-0">
        {creating ? (
          <CreateForm
            token={token}
            projectId={projectId}
            onCreated={handleCreated}
            onCancel={() => setCreating(false)}
          />
        ) : selected ? (
          <RuleDetail
            key={selected.id}
            rule={selected}
            token={token}
            projectId={projectId}
            onUpdated={handleUpdated}
            onDeleted={handleDeleted}
          />
        ) : (
          <div className="flex items-center justify-center h-48 text-brand-muted text-sm">
            Select a magic system or create a new one.
          </div>
        )}
      </div>
    </div>
  )
}

// ── Detail / edit ─────────────────────────────────────────────────────────────

function RuleDetail({
  rule,
  token,
  projectId,
  onUpdated,
  onDeleted,
}: {
  rule: MagicRule
  token: string
  projectId: string
  onUpdated: (r: MagicRule) => void
  onDeleted: (id: string) => void
}) {
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState(rule.name)
  const [description, setDescription] = useState(rule.description)
  const [attrs, setAttrs] = useState<MagicRuleAttributes>((rule.attributes as MagicRuleAttributes) ?? {})
  const [saving, setSaving] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  useEffect(() => {
    setName(rule.name)
    setDescription(rule.description)
    setAttrs((rule.attributes as MagicRuleAttributes) ?? {})
    setEditing(false)
  }, [rule.id])

  async function save() {
    setSaving(true)
    try {
      const updated = await api.wiki.updateMagicRule(token, projectId, rule.id, {
        name: name.trim(),
        description: description.trim(),
        attributes: attrs,
      })
      onUpdated(updated)
      setEditing(false)
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete() {
    await api.wiki.deleteMagicRule(token, projectId, rule.id)
    onDeleted(rule.id)
  }

  function setAttr(key: keyof MagicRuleAttributes, value: string) {
    setAttrs((prev) => ({ ...prev, [key]: value }))
  }

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-start justify-between gap-3">
        {editing ? (
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="input-field text-lg font-bold flex-1"
            autoFocus
          />
        ) : (
          <h2 className="text-lg font-bold text-brand-text">{rule.name}</h2>
        )}
        <div className="flex gap-2 shrink-0">
          {editing ? (
            <>
              <button onClick={() => setEditing(false)} className="text-xs text-brand-muted hover:text-brand-text px-2 py-1 rounded border border-brand-border transition-colors">
                Cancel
              </button>
              <button onClick={save} disabled={saving} className="text-xs bg-brand-gradient text-brand-bg px-3 py-1 rounded font-semibold disabled:opacity-50 transition-opacity">
                {saving ? 'Saving…' : 'Save'}
              </button>
            </>
          ) : (
            <button onClick={() => setEditing(true)} className="text-xs text-brand-muted hover:text-brand-text px-2 py-1 rounded border border-brand-border transition-colors">
              Edit
            </button>
          )}
        </div>
      </div>

      {/* Rules clarity spectrum */}
      <div>
        <label className="block text-xs font-semibold text-brand-muted uppercase tracking-wider mb-2">Clarity Spectrum</label>
        <div className="flex gap-2">
          {CLARITY_OPTIONS.map((opt) => (
            <button
              key={opt.value}
              type="button"
              disabled={!editing}
              onClick={() => editing && setAttr('rules_clarity', attrs.rules_clarity === opt.value ? '' : opt.value!)}
              title={opt.hint}
              className={`flex-1 py-1.5 rounded text-xs font-medium border transition-colors ${
                attrs.rules_clarity === opt.value
                  ? 'bg-brand-purple/20 text-brand-purple border-brand-purple/40'
                  : editing
                  ? 'border-brand-border text-brand-muted hover:text-brand-text'
                  : 'border-brand-border/40 text-brand-muted/50 cursor-default'
              }`}
            >
              {opt.label}
            </button>
          ))}
        </div>
        {attrs.rules_clarity && (
          <p className="mt-1 text-[11px] text-brand-muted">
            {CLARITY_OPTIONS.find((o) => o.value === attrs.rules_clarity)?.hint}
          </p>
        )}
      </div>

      {/* Structured fields — Limitations first (most important) */}
      <div className="space-y-3">
        {ATTR_FIELDS.map(({ key, label, hint, prominent }) => (
          <div key={key}>
            <label className={`block text-xs font-semibold uppercase tracking-wider mb-1 ${prominent ? 'text-brand-cyan' : 'text-brand-muted'}`}>
              {label}
              {prominent && <span className="ml-1 text-[9px] normal-case font-normal opacity-70">(most important)</span>}
            </label>
            {editing ? (
              <textarea
                value={attrs[key] ?? ''}
                onChange={(e) => setAttr(key, e.target.value)}
                placeholder={hint}
                rows={3}
                className="input-field resize-none w-full text-sm"
              />
            ) : (
              <p className={`text-sm whitespace-pre-wrap ${attrs[key] ? 'text-brand-text' : 'text-brand-muted italic'}`}>
                {attrs[key] || `No ${label.toLowerCase()} specified.`}
              </p>
            )}
          </div>
        ))}
      </div>

      {/* Freeform description */}
      <div>
        <label className="block text-xs font-semibold text-brand-muted uppercase tracking-wider mb-1">Notes</label>
        {editing ? (
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Any additional notes…"
            rows={4}
            className="input-field resize-none w-full text-sm"
          />
        ) : (
          <p className={`text-sm whitespace-pre-wrap ${description ? 'text-brand-text' : 'text-brand-muted italic'}`}>
            {description || 'No notes.'}
          </p>
        )}
      </div>

      {/* Delete */}
      {!editing && (
        <div className="pt-2 border-t border-brand-border">
          {confirmDelete ? (
            <div className="flex items-center gap-3">
              <span className="text-xs text-brand-muted">Delete this magic system?</span>
              <button onClick={handleDelete} className="text-xs text-red-400 hover:text-red-300 transition-colors">Confirm</button>
              <button onClick={() => setConfirmDelete(false)} className="text-xs text-brand-muted hover:text-brand-text transition-colors">Cancel</button>
            </div>
          ) : (
            <button onClick={() => setConfirmDelete(true)} className="text-xs text-brand-muted hover:text-red-400 transition-colors">
              Delete magic system
            </button>
          )}
        </div>
      )}
    </div>
  )
}

// ── Create form ───────────────────────────────────────────────────────────────

function CreateForm({
  token,
  projectId,
  onCreated,
  onCancel,
}: {
  token: string
  projectId: string
  onCreated: (r: MagicRule) => void
  onCancel: () => void
}) {
  const [name, setName] = useState('')
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    setSubmitting(true)
    try {
      const rule = await api.wiki.createMagicRule(token, projectId, { name: name.trim() })
      onCreated(rule)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4 max-w-sm">
      <h3 className="text-sm font-semibold text-brand-text">New Magic System</h3>
      <div>
        <label className="block text-xs font-medium text-brand-muted uppercase tracking-wider mb-1.5">Name *</label>
        <input
          autoFocus
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. Allomancy, The Force, Sympathy"
          className="input-field w-full"
        />
      </div>
      <p className="text-xs text-brand-muted">Add details (limitations, cost, source) after creating.</p>
      <div className="flex gap-2">
        <button type="button" onClick={onCancel} className="flex-1 py-2 rounded-lg border border-brand-border text-brand-muted text-sm hover:text-brand-text transition-colors">
          Cancel
        </button>
        <button type="submit" disabled={submitting || !name.trim()} className="flex-1 py-2 rounded-lg bg-brand-gradient text-brand-bg text-sm font-semibold disabled:opacity-50">
          {submitting ? 'Creating…' : 'Create'}
        </button>
      </div>
    </form>
  )
}

function PlusIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <path d="M6 1v10M1 6h10" />
    </svg>
  )
}
