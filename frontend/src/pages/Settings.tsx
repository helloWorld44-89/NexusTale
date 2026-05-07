// Settings — user account settings, starting with AI provider key management.
import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'
import { useThemeStore } from '@/app/store/themeStore'
import { api } from '@/services/api'
import type { APIKeyResponse } from '@/services/api'

const PROVIDERS = ['openai', 'anthropic', 'openrouter', 'gemini', 'groq', 'deepseek', 'mistral', 'custom'] as const
type Provider = typeof PROVIDERS[number]

function SunIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <circle cx="10" cy="10" r="3.5" />
      <path d="M10 2v2M10 16v2M2 10h2M16 10h2M4.22 4.22l1.42 1.42M14.36 14.36l1.42 1.42M4.22 15.78l1.42-1.42M14.36 5.64l1.42-1.42" />
    </svg>
  )
}

function InfoIcon() {
  return (
    <svg className="w-5 h-5 shrink-0 text-brand-muted group-hover:text-brand-cyan transition-colors" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="10" cy="10" r="8" />
      <path d="M10 9v5" />
      <circle cx="10" cy="6.5" r="0.5" fill="currentColor" />
    </svg>
  )
}

function GitHubIcon() {
  return (
    <svg className="w-5 h-5 shrink-0 text-brand-muted group-hover:text-brand-cyan transition-colors" viewBox="0 0 20 20" fill="currentColor">
      <path fillRule="evenodd" d="M10 0C4.477 0 0 4.477 0 10c0 4.418 2.865 8.166 6.839 9.489.5.092.682-.217.682-.482 0-.237-.009-.868-.013-1.703-2.782.604-3.369-1.34-3.369-1.34-.454-1.154-1.11-1.462-1.11-1.462-.908-.62.069-.608.069-.608 1.003.07 1.531 1.03 1.531 1.03.892 1.529 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.11-4.555-4.943 0-1.091.39-1.984 1.029-2.683-.103-.253-.446-1.27.098-2.647 0 0 .84-.269 2.75 1.025A9.578 9.578 0 0110 4.836a9.59 9.59 0 012.504.337c1.909-1.294 2.747-1.025 2.747-1.025.546 1.377.203 2.394.1 2.647.641.699 1.028 1.592 1.028 2.683 0 3.842-2.339 4.687-4.566 4.935.359.309.678.919.678 1.852 0 1.336-.012 2.415-.012 2.743 0 .267.18.578.688.48C17.138 18.163 20 14.418 20 10c0-5.523-4.477-10-10-10z" clipRule="evenodd" />
    </svg>
  )
}

function MoonIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <path d="M17.5 12.5A7.5 7.5 0 0 1 7.5 2.5a7.5 7.5 0 1 0 10 10z" />
    </svg>
  )
}

const PROVIDER_LABELS: Record<Provider, string> = {
  openai:      'OpenAI',
  anthropic:   'Anthropic',
  openrouter:  'OpenRouter',
  gemini:      'Google Gemini',
  groq:        'Groq',
  deepseek:    'DeepSeek',
  mistral:     'Mistral',
  custom:      'Custom (OpenAI-compatible)',
}

export default function Settings() {
  const navigate    = useNavigate()
  const { accessToken, user, logout } = useAuthStore((s) => ({
    accessToken: s.accessToken,
    user:        s.user,
    logout:      s.logout,
  }))
  const { theme, toggleTheme } = useThemeStore((s) => ({ theme: s.theme, toggleTheme: s.toggleTheme }))

  const [keys, setKeys]           = useState<APIKeyResponse[]>([])
  const [loadingKeys, setLoading] = useState(true)
  const [error, setError]         = useState<string | null>(null)

  // Per-provider test state: provider → { testing, result }
  const [testStates, setTestStates] = useState<Record<string, {
    testing: boolean
    result?: { ok: boolean; models?: string[]; error?: string }
  }>>({})

  // Ollama URL form state
  const [ollamaURL,          setOllamaURL]          = useState('')
  const [savingOllama,       setSavingOllama]        = useState(false)
  const [ollamaErr,          setOllamaErr]           = useState<string | null>(null)
  const [ollamaOk,           setOllamaOk]            = useState(false)

  // Ollama model selection state
  const [ollamaModelSaving,  setOllamaModelSaving]  = useState(false)
  const [ollamaModelOk,      setOllamaModelOk]      = useState(false)
  const [ollamaModelErr,     setOllamaModelErr]     = useState<string | null>(null)

  // OpenRouter model selection state
  const [orModelSaving, setOrModelSaving] = useState(false)
  const [orModelOk,     setOrModelOk]     = useState(false)
  const [orModelErr,    setOrModelErr]    = useState<string | null>(null)

  // Gemini model selection state
  const [geminiModelSaving, setGeminiModelSaving] = useState(false)
  const [geminiModelOk,     setGeminiModelOk]     = useState(false)
  const [geminiModelErr,    setGeminiModelErr]    = useState<string | null>(null)

  // Groq model selection state
  const [groqModelSaving, setGroqModelSaving] = useState(false)
  const [groqModelOk,     setGroqModelOk]     = useState(false)
  const [groqModelErr,    setGroqModelErr]    = useState<string | null>(null)

  // DeepSeek model selection state
  const [dsModelSaving, setDsModelSaving] = useState(false)
  const [dsModelOk,     setDsModelOk]     = useState(false)
  const [dsModelErr,    setDsModelErr]    = useState<string | null>(null)

  // Danger zone state
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const [confirmEmail, setConfirmEmail]         = useState('')
  const [deleting, setDeleting]                 = useState(false)
  const [deleteError, setDeleteError]           = useState<string | null>(null)

  const handleDeleteAccount = async () => {
    if (!accessToken || confirmEmail !== user?.email) return
    setDeleting(true)
    setDeleteError(null)
    try {
      await api.users.deleteMe(accessToken)
      await logout()
      navigate('/login', { replace: true })
    } catch (e: unknown) {
      setDeleteError(e instanceof Error ? e.message : 'Failed to delete account')
      setDeleting(false)
    }
  }

  // Add-key form state
  const [provider, setProvider]   = useState<Provider>('openai')
  const [keyValue, setKeyValue]   = useState('')
  const [saving, setSaving]       = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [saveOk, setSaveOk]       = useState(false)

  useEffect(() => {
    if (!accessToken) return
    setLoading(true)
    api.apiKeys.list(accessToken)
      .then((ks) => {
        setKeys(ks)
        // Pre-populate Ollama URL field if one is already stored.
        // The hint is the last 4 chars; we can't recover the full URL so we
        // just show a placeholder indicating something is saved.
        const ollamaKey = ks.find((k) => k.provider === 'ollama')
        if (ollamaKey) setOllamaURL('')   // can't recover full URL; show empty + hint
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }, [accessToken])

  const handleSaveOllamaURL = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!accessToken || !ollamaURL.trim()) return
    setSavingOllama(true)
    setOllamaErr(null)
    setOllamaOk(false)
    try {
      const saved = await api.apiKeys.upsert(accessToken, 'ollama', ollamaURL.trim())
      setKeys((prev) => {
        const filtered = prev.filter((k) => k.provider !== 'ollama')
        return [...filtered, saved].sort((a, b) => a.provider.localeCompare(b.provider))
      })
      setOllamaURL('')
      setOllamaOk(true)
      setTimeout(() => setOllamaOk(false), 2500)
    } catch (e: unknown) {
      setOllamaErr(e instanceof Error ? e.message : 'Failed to save URL')
    } finally {
      setSavingOllama(false)
    }
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!accessToken || !keyValue.trim()) return
    setSaving(true)
    setSaveError(null)
    setSaveOk(false)
    try {
      const saved = await api.apiKeys.upsert(accessToken, provider, keyValue.trim())
      setKeys((prev) => {
        const filtered = prev.filter((k) => k.provider !== provider)
        return [...filtered, saved].sort((a, b) => a.provider.localeCompare(b.provider))
      })
      setKeyValue('')
      setSaveOk(true)
      setTimeout(() => setSaveOk(false), 2500)
    } catch (e: unknown) {
      setSaveError(e instanceof Error ? e.message : 'Failed to save key')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (p: string) => {
    if (!accessToken) return
    try {
      await api.apiKeys.delete(accessToken, p)
      setKeys((prev) => prev.filter((k) => k.provider !== p))
      setTestStates((prev) => { const n = { ...prev }; delete n[p]; return n })
    } catch {}
  }

  const handleSetOllamaModel = async (model: string) => {
    if (!accessToken) return
    setOllamaModelSaving(true)
    setOllamaModelOk(false)
    setOllamaModelErr(null)
    try {
      const saved = await api.apiKeys.upsert(accessToken, 'ollama_model', model)
      setKeys((prev) => {
        const filtered = prev.filter((k) => k.provider !== 'ollama_model')
        return [...filtered, saved].sort((a, b) => a.provider.localeCompare(b.provider))
      })
      setOllamaModelOk(true)
      setTimeout(() => setOllamaModelOk(false), 2500)
    } catch (e: unknown) {
      setOllamaModelErr(e instanceof Error ? e.message : 'Failed to save model')
    } finally {
      setOllamaModelSaving(false)
    }
  }

  const handleSetOpenRouterModel = async (model: string) => {
    if (!accessToken) return
    setOrModelSaving(true)
    setOrModelOk(false)
    setOrModelErr(null)
    try {
      const saved = await api.apiKeys.upsert(accessToken, 'openrouter_model', model)
      setKeys((prev) => {
        const filtered = prev.filter((k) => k.provider !== 'openrouter_model')
        return [...filtered, saved].sort((a, b) => a.provider.localeCompare(b.provider))
      })
      setOrModelOk(true)
      setTimeout(() => setOrModelOk(false), 2500)
    } catch (e: unknown) {
      setOrModelErr(e instanceof Error ? e.message : 'Failed to save model')
    } finally {
      setOrModelSaving(false)
    }
  }

  const handleSetGeminiModel = async (model: string) => {
    if (!accessToken) return
    setGeminiModelSaving(true)
    setGeminiModelOk(false)
    setGeminiModelErr(null)
    try {
      const saved = await api.apiKeys.upsert(accessToken, 'gemini_model', model)
      setKeys((prev) => {
        const filtered = prev.filter((k) => k.provider !== 'gemini_model')
        return [...filtered, saved].sort((a, b) => a.provider.localeCompare(b.provider))
      })
      setGeminiModelOk(true)
      setTimeout(() => setGeminiModelOk(false), 2500)
    } catch (e: unknown) {
      setGeminiModelErr(e instanceof Error ? e.message : 'Failed to save model')
    } finally {
      setGeminiModelSaving(false)
    }
  }

  const handleSetGroqModel = async (model: string) => {
    if (!accessToken) return
    setGroqModelSaving(true); setGroqModelOk(false); setGroqModelErr(null)
    try {
      const saved = await api.apiKeys.upsert(accessToken, 'groq_model', model)
      setKeys((prev) => [...prev.filter((k) => k.provider !== 'groq_model'), saved].sort((a, b) => a.provider.localeCompare(b.provider)))
      setGroqModelOk(true); setTimeout(() => setGroqModelOk(false), 2500)
    } catch (e: unknown) { setGroqModelErr(e instanceof Error ? e.message : 'Failed to save model') }
    finally { setGroqModelSaving(false) }
  }

  const handleSetDeepSeekModel = async (model: string) => {
    if (!accessToken) return
    setDsModelSaving(true); setDsModelOk(false); setDsModelErr(null)
    try {
      const saved = await api.apiKeys.upsert(accessToken, 'deepseek_model', model)
      setKeys((prev) => [...prev.filter((k) => k.provider !== 'deepseek_model'), saved].sort((a, b) => a.provider.localeCompare(b.provider)))
      setDsModelOk(true); setTimeout(() => setDsModelOk(false), 2500)
    } catch (e: unknown) { setDsModelErr(e instanceof Error ? e.message : 'Failed to save model') }
    finally { setDsModelSaving(false) }
  }

  const handleTestConnection = async (provider: string) => {
    if (!accessToken) return
    setTestStates((prev) => ({ ...prev, [provider]: { testing: true } }))
    try {
      const result = await api.testConnection(accessToken, provider)
      setTestStates((prev) => ({ ...prev, [provider]: { testing: false, result } }))
    } catch (e: unknown) {
      setTestStates((prev) => ({
        ...prev,
        [provider]: { testing: false, result: { ok: false, error: e instanceof Error ? e.message : 'Request failed' } },
      }))
    }
  }

  return (
    <div className="min-h-screen bg-brand-bg text-brand-text font-sans">
      {/* Header */}
      <header className="border-b border-brand-border px-6 py-4 flex items-center gap-4">
        <Link to="/dashboard" className="text-brand-muted hover:text-brand-text transition-colors text-sm">
          ← Back
        </Link>
        <span className="text-brand-cyan font-semibold">NexusTale</span>
        <span className="text-brand-muted/40">/</span>
        <span className="text-sm text-brand-text-muted">Settings</span>
      </header>

      <main className="max-w-2xl mx-auto px-6 py-10 space-y-10">

        {/* AI Provider Keys */}
        <section>
          <h2 className="text-lg font-semibold text-brand-text mb-1">AI Provider Keys</h2>
          <p className="text-sm text-brand-text-muted mb-6">
            Keys are encrypted at rest on the server. Only the last 4 characters are shown here.
            Your raw key is never returned in any API response.
          </p>

          {/* Stored keys list */}
          {loadingKeys ? (
            <p className="text-sm text-brand-text-muted">Loading…</p>
          ) : error ? (
            <p className="text-sm text-red-400">{error}</p>
          ) : keys.length > 0 ? (
            <div className="mb-6 space-y-3">
              {keys.filter((k) => !['ollama', 'ollama_model', 'openrouter_model', 'gemini_model', 'groq_model', 'deepseek_model'].includes(k.provider)).map((k) => {
                const ts = testStates[k.provider]
                return (
                  <div key={k.id} className="rounded-lg border border-brand-border bg-brand-bg-card overflow-hidden">
                    <div className="flex items-center justify-between px-4 py-3">
                      <div>
                        <span className="text-sm font-medium text-brand-text">
                          {PROVIDER_LABELS[k.provider as Provider] ?? k.provider}
                        </span>
                        <span className="ml-3 text-xs text-brand-text-muted font-mono">
                          ••••{k.key_hint}
                        </span>
                      </div>
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => handleTestConnection(k.provider)}
                          disabled={ts?.testing}
                          className="text-xs text-brand-cyan hover:text-brand-cyan/80 border border-brand-cyan/30 hover:border-brand-cyan/60 px-2 py-0.5 rounded transition-colors disabled:opacity-50"
                        >
                          {ts?.testing ? 'Testing…' : 'Test'}
                        </button>
                        <button
                          onClick={() => handleDelete(k.provider)}
                          className="text-xs text-red-400 hover:text-red-300 transition-colors"
                        >
                          Remove
                        </button>
                      </div>
                    </div>
                    {ts?.result && (
                      <div className={`px-4 py-2 border-t text-xs ${ts.result.ok ? 'border-emerald-500/20 bg-emerald-500/5' : 'border-red-500/20 bg-red-500/5'}`}>
                        {ts.result.ok ? (
                          <div className="space-y-0.5">
                            <p className="text-emerald-400 font-medium">Connected</p>
                            {ts.result.models && ts.result.models.length > 0 && (
                              <p className="text-brand-muted">{ts.result.models.slice(0, 5).join(', ')}{ts.result.models.length > 5 ? ` +${ts.result.models.length - 5} more` : ''}</p>
                            )}
                          </div>
                        ) : (
                          <p className="text-red-400">{ts.result.error}</p>
                        )}
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          ) : (
            <p className="text-sm text-brand-text-muted mb-6">No keys stored yet.</p>
          )}

          {/* Add / update form */}
          <form onSubmit={handleSave} className="space-y-4 border border-brand-border rounded-xl p-5 bg-brand-bg-card">
            <h3 className="text-sm font-medium text-brand-text">Add or replace a key</h3>

            <div className="flex flex-col gap-1.5">
              <label className="text-xs text-brand-text-muted uppercase tracking-wider">Provider</label>
              <select
                value={provider}
                onChange={(e) => setProvider(e.target.value as Provider)}
                className="bg-brand-bg border border-brand-border rounded-lg px-3 py-2 text-sm text-brand-text focus:outline-none focus:border-brand-cyan"
              >
                {PROVIDERS.map((p) => (
                  <option key={p} value={p}>{PROVIDER_LABELS[p]}</option>
                ))}
              </select>
            </div>

            <div className="flex flex-col gap-1.5">
              <label className="text-xs text-brand-text-muted uppercase tracking-wider">API Key</label>
              <input
                type="password"
                autoComplete="new-password"
                value={keyValue}
                onChange={(e) => setKeyValue(e.target.value)}
                placeholder="sk-…"
                className="bg-brand-bg border border-brand-border rounded-lg px-3 py-2 text-sm text-brand-text placeholder:text-brand-text-muted/40 focus:outline-none focus:border-brand-cyan font-mono"
              />
            </div>

            {saveError && <p className="text-xs text-red-400">{saveError}</p>}
            {saveOk    && <p className="text-xs text-emerald-400">Key saved.</p>}

            <button
              type="submit"
              disabled={saving || !keyValue.trim()}
              className="px-4 py-2 rounded-lg bg-brand-cyan text-brand-bg text-sm font-semibold hover:bg-brand-cyan/80 disabled:opacity-50 transition-colors"
            >
              {saving ? 'Saving…' : 'Save Key'}
            </button>
          </form>
        </section>

        {/* OpenRouter */}
        {keys.find((k) => k.provider === 'openrouter') && (() => {
          const orKey      = keys.find((k) => k.provider === 'openrouter')!
          const orModelKey = keys.find((k) => k.provider === 'openrouter_model')
          const ts         = testStates['openrouter']
          const modelList  = ts?.result?.ok ? (ts.result.models ?? []) : []
          return (
            <section>
              <h2 className="text-lg font-semibold text-brand-text mb-1">OpenRouter</h2>
              <p className="text-sm text-brand-text-muted mb-4">
                Route AI calls through OpenRouter to access Anthropic, OpenAI, Google, and open models
                with a single key. Models use the <span className="font-mono text-brand-text">provider/model-name</span> format.
              </p>
              <div className="rounded-lg border border-brand-border bg-brand-bg-card overflow-hidden">
                <div className="flex items-center justify-between px-4 py-3">
                  <div>
                    <span className="text-sm font-medium text-brand-text">OpenRouter key</span>
                    <span className="ml-3 text-xs text-brand-text-muted font-mono">
                      ••••{orKey.key_hint}
                    </span>
                    {orModelKey && (
                      <span className="ml-3 text-xs text-brand-purple font-mono" title="Active model">
                        model: ••••{orModelKey.key_hint}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleTestConnection('openrouter')}
                      disabled={ts?.testing}
                      className="text-xs text-brand-cyan hover:text-brand-cyan/80 border border-brand-cyan/30 hover:border-brand-cyan/60 px-2 py-0.5 rounded transition-colors disabled:opacity-50"
                    >
                      {ts?.testing ? 'Testing…' : 'Test Connection'}
                    </button>
                    <button
                      onClick={() => handleDelete('openrouter')}
                      className="text-xs text-red-400 hover:text-red-300 transition-colors"
                    >
                      Remove
                    </button>
                  </div>
                </div>

                {ts?.result && (
                  <div className={`px-4 py-3 border-t text-xs ${ts.result.ok ? 'border-emerald-500/20 bg-emerald-500/5' : 'border-red-500/20 bg-red-500/5'}`}>
                    {ts.result.ok ? (
                      <div className="space-y-2">
                        <p className="text-emerald-400 font-medium">Connected — click a model to activate it:</p>
                        {orModelErr && <p className="text-red-400">{orModelErr}</p>}
                        {orModelOk  && <p className="text-emerald-400">Model saved.</p>}
                        <div className="space-y-1">
                          {modelList.map((m) => (
                            <button
                              key={m}
                              onClick={() => handleSetOpenRouterModel(m)}
                              disabled={orModelSaving}
                              className="block w-full text-left font-mono text-brand-text hover:text-brand-cyan hover:bg-brand-cyan/5 px-2 py-1 rounded transition-colors disabled:opacity-50"
                            >
                              {m}
                            </button>
                          ))}
                        </div>
                      </div>
                    ) : (
                      <p className="text-red-400">{ts.result.error}</p>
                    )}
                  </div>
                )}
              </div>
            </section>
          )
        })()}

        {/* Gemini */}
        {keys.find((k) => k.provider === 'gemini') && (() => {
          const geminiKey      = keys.find((k) => k.provider === 'gemini')!
          const geminiModelKey = keys.find((k) => k.provider === 'gemini_model')
          const ts             = testStates['gemini']
          const modelList      = ts?.result?.ok ? (ts.result.models ?? []) : []
          return (
            <section>
              <h2 className="text-lg font-semibold text-brand-text mb-1">Google Gemini</h2>
              <p className="text-sm text-brand-text-muted mb-4">
                Gemini 2.5 Flash is the recommended default — fast, generous free tier, and the
                best-supported model on Gemini's OpenAI-compatible endpoint. Select a model below
                after clicking Test Connection.
              </p>
              <div className="rounded-lg border border-brand-border bg-brand-bg-card overflow-hidden">
                <div className="flex items-center justify-between px-4 py-3">
                  <div>
                    <span className="text-sm font-medium text-brand-text">Gemini key</span>
                    <span className="ml-3 text-xs text-brand-text-muted font-mono">
                      ••••{geminiKey.key_hint}
                    </span>
                    {geminiModelKey && (
                      <span className="ml-3 text-xs text-brand-purple font-mono" title="Active model">
                        model: ••••{geminiModelKey.key_hint}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleTestConnection('gemini')}
                      disabled={ts?.testing}
                      className="text-xs text-brand-cyan hover:text-brand-cyan/80 border border-brand-cyan/30 hover:border-brand-cyan/60 px-2 py-0.5 rounded transition-colors disabled:opacity-50"
                    >
                      {ts?.testing ? 'Testing…' : 'Test Connection'}
                    </button>
                    <button
                      onClick={() => handleDelete('gemini')}
                      className="text-xs text-red-400 hover:text-red-300 transition-colors"
                    >
                      Remove
                    </button>
                  </div>
                </div>

                {ts?.result && (
                  <div className={`px-4 py-3 border-t text-xs ${ts.result.ok ? 'border-emerald-500/20 bg-emerald-500/5' : 'border-red-500/20 bg-red-500/5'}`}>
                    {ts.result.ok ? (
                      <div className="space-y-2">
                        <p className="text-emerald-400 font-medium">Connected — click a model to activate it:</p>
                        {geminiModelErr && <p className="text-red-400">{geminiModelErr}</p>}
                        {geminiModelOk  && <p className="text-emerald-400">Model saved.</p>}
                        <div className="space-y-1">
                          {modelList.map((m) => (
                            <button
                              key={m}
                              onClick={() => handleSetGeminiModel(m)}
                              disabled={geminiModelSaving}
                              className="block w-full text-left font-mono text-brand-text hover:text-brand-cyan hover:bg-brand-cyan/5 px-2 py-1 rounded transition-colors disabled:opacity-50"
                            >
                              {m}
                            </button>
                          ))}
                        </div>
                      </div>
                    ) : (
                      <p className="text-red-400">{ts.result.error}</p>
                    )}
                  </div>
                )}
              </div>
            </section>
          )
        })()}

        {/* Groq */}
        {keys.find((k) => k.provider === 'groq') && (() => {
          const groqKey      = keys.find((k) => k.provider === 'groq')!
          const groqModelKey = keys.find((k) => k.provider === 'groq_model')
          const ts           = testStates['groq']
          const modelList    = ts?.result?.ok ? (ts.result.models ?? []) : []
          return (
            <section>
              <h2 className="text-lg font-semibold text-brand-text mb-1">Groq</h2>
              <p className="text-sm text-brand-text-muted mb-4">
                Fastest inference available — Beat mode responds near-instantly.
                Free tier with generous daily limits. Default model is <span className="font-mono text-brand-text">llama-3.1-70b-versatile</span>.
              </p>
              <div className="rounded-lg border border-brand-border bg-brand-bg-card overflow-hidden">
                <div className="flex items-center justify-between px-4 py-3">
                  <div>
                    <span className="text-sm font-medium text-brand-text">Groq key</span>
                    <span className="ml-3 text-xs text-brand-text-muted font-mono">••••{groqKey.key_hint}</span>
                    {groqModelKey && <span className="ml-3 text-xs text-brand-purple font-mono">model: ••••{groqModelKey.key_hint}</span>}
                  </div>
                  <div className="flex items-center gap-2">
                    <button onClick={() => handleTestConnection('groq')} disabled={ts?.testing} className="text-xs text-brand-cyan hover:text-brand-cyan/80 border border-brand-cyan/30 hover:border-brand-cyan/60 px-2 py-0.5 rounded transition-colors disabled:opacity-50">
                      {ts?.testing ? 'Testing…' : 'Test Connection'}
                    </button>
                    <button onClick={() => handleDelete('groq')} className="text-xs text-red-400 hover:text-red-300 transition-colors">Remove</button>
                  </div>
                </div>
                {ts?.result && (
                  <div className={`px-4 py-3 border-t text-xs ${ts.result.ok ? 'border-emerald-500/20 bg-emerald-500/5' : 'border-red-500/20 bg-red-500/5'}`}>
                    {ts.result.ok ? (
                      <div className="space-y-2">
                        <p className="text-emerald-400 font-medium">Connected — click a model to activate it:</p>
                        {groqModelErr && <p className="text-red-400">{groqModelErr}</p>}
                        {groqModelOk  && <p className="text-emerald-400">Model saved.</p>}
                        <div className="space-y-1">
                          {modelList.map((m) => (
                            <button key={m} onClick={() => handleSetGroqModel(m)} disabled={groqModelSaving} className="block w-full text-left font-mono text-brand-text hover:text-brand-cyan hover:bg-brand-cyan/5 px-2 py-1 rounded transition-colors disabled:opacity-50">{m}</button>
                          ))}
                        </div>
                      </div>
                    ) : <p className="text-red-400">{ts.result.error}</p>}
                  </div>
                )}
              </div>
            </section>
          )
        })()}

        {/* DeepSeek */}
        {keys.find((k) => k.provider === 'deepseek') && (() => {
          const dsKey      = keys.find((k) => k.provider === 'deepseek')!
          const dsModelKey = keys.find((k) => k.provider === 'deepseek_model')
          const ts         = testStates['deepseek']
          const modelList  = ts?.result?.ok ? (ts.result.models ?? []) : []
          return (
            <section>
              <h2 className="text-lg font-semibold text-brand-text mb-1">DeepSeek</h2>
              <p className="text-sm text-brand-text-muted mb-3">
                GPT-4o-class quality at roughly 3% of the cost. Default model is <span className="font-mono text-brand-text">deepseek-chat</span>.
                Switch to <span className="font-mono text-brand-text">deepseek-reasoner</span> for chain-of-thought tasks (non-streaming).
              </p>
              <div className="flex items-start gap-2 mb-4 rounded-lg border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-xs text-amber-300">
                <span className="shrink-0 mt-0.5">⚠</span>
                <span>DeepSeek servers are operated by a Chinese company. Writers with data-sensitivity concerns should use Anthropic, OpenAI, Gemini, or Ollama instead.</span>
              </div>
              <div className="rounded-lg border border-brand-border bg-brand-bg-card overflow-hidden">
                <div className="flex items-center justify-between px-4 py-3">
                  <div>
                    <span className="text-sm font-medium text-brand-text">DeepSeek key</span>
                    <span className="ml-3 text-xs text-brand-text-muted font-mono">••••{dsKey.key_hint}</span>
                    {dsModelKey && <span className="ml-3 text-xs text-brand-purple font-mono">model: ••••{dsModelKey.key_hint}</span>}
                  </div>
                  <div className="flex items-center gap-2">
                    <button onClick={() => handleTestConnection('deepseek')} disabled={ts?.testing} className="text-xs text-brand-cyan hover:text-brand-cyan/80 border border-brand-cyan/30 hover:border-brand-cyan/60 px-2 py-0.5 rounded transition-colors disabled:opacity-50">
                      {ts?.testing ? 'Testing…' : 'Test Connection'}
                    </button>
                    <button onClick={() => handleDelete('deepseek')} className="text-xs text-red-400 hover:text-red-300 transition-colors">Remove</button>
                  </div>
                </div>
                {ts?.result && (
                  <div className={`px-4 py-3 border-t text-xs ${ts.result.ok ? 'border-emerald-500/20 bg-emerald-500/5' : 'border-red-500/20 bg-red-500/5'}`}>
                    {ts.result.ok ? (
                      <div className="space-y-2">
                        <p className="text-emerald-400 font-medium">Connected — click a model to activate it:</p>
                        {dsModelErr && <p className="text-red-400">{dsModelErr}</p>}
                        {dsModelOk  && <p className="text-emerald-400">Model saved.</p>}
                        <div className="space-y-1">
                          {modelList.map((m) => (
                            <button key={m} onClick={() => handleSetDeepSeekModel(m)} disabled={dsModelSaving} className="block w-full text-left font-mono text-brand-text hover:text-brand-cyan hover:bg-brand-cyan/5 px-2 py-1 rounded transition-colors disabled:opacity-50">{m}</button>
                          ))}
                        </div>
                      </div>
                    ) : <p className="text-red-400">{ts.result.error}</p>}
                  </div>
                )}
              </div>
            </section>
          )
        })()}

        {/* Ollama (local AI) */}
        <section>
          <h2 className="text-lg font-semibold text-brand-text mb-1">Local AI (Ollama)</h2>
          <p className="text-sm text-brand-text-muted mb-6">
            Set the base URL of your Ollama instance. Used when no cloud API key is configured,
            or when you explicitly choose <span className="font-mono text-brand-text">ollama</span> as
            the provider. Required when the app runs in Docker — <span className="font-mono text-brand-text">localhost</span> won't
            reach the host machine.
          </p>

          {/* Show stored hint + Test Connection + model selector if URL is saved */}
          {keys.find((k) => k.provider === 'ollama') && (() => {
            const ollamaKey    = keys.find((k) => k.provider === 'ollama')!
            const modelKey     = keys.find((k) => k.provider === 'ollama_model')
            const ts           = testStates['ollama']
            const modelList    = ts?.result?.ok ? (ts.result.models ?? []) : []
            return (
              <div className="mb-4 rounded-lg border border-brand-border bg-brand-bg-card overflow-hidden">
                {/* URL row */}
                <div className="flex items-center justify-between px-4 py-3">
                  <div>
                    <span className="text-sm font-medium text-brand-text">Ollama URL</span>
                    <span className="ml-3 text-xs text-brand-text-muted font-mono">
                      ••••{ollamaKey.key_hint}
                    </span>
                    {modelKey && (
                      <span className="ml-3 text-xs text-brand-purple font-mono" title="Active model">
                        model: ••••{modelKey.key_hint}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleTestConnection('ollama')}
                      disabled={ts?.testing}
                      className="text-xs text-brand-cyan hover:text-brand-cyan/80 border border-brand-cyan/30 hover:border-brand-cyan/60 px-2 py-0.5 rounded transition-colors disabled:opacity-50"
                    >
                      {ts?.testing ? 'Testing…' : 'Test Connection'}
                    </button>
                    <button
                      onClick={() => handleDelete('ollama')}
                      className="text-xs text-red-400 hover:text-red-300 transition-colors"
                    >
                      Remove
                    </button>
                  </div>
                </div>

                {/* Test result panel */}
                {ts?.result && (
                  <div className={`px-4 py-3 border-t text-xs ${ts.result.ok ? 'border-emerald-500/20 bg-emerald-500/5' : 'border-red-500/20 bg-red-500/5'}`}>
                    {ts.result.ok ? (
                      <div className="space-y-2">
                        <p className="text-emerald-400 font-medium">Connected — click a model to activate it:</p>
                        {ollamaModelErr && <p className="text-red-400">{ollamaModelErr}</p>}
                        {ollamaModelOk  && <p className="text-emerald-400">Model saved.</p>}
                        <div className="space-y-1">
                          {modelList.map((m) => (
                            <button
                              key={m}
                              onClick={() => handleSetOllamaModel(m)}
                              disabled={ollamaModelSaving}
                              className="block w-full text-left font-mono text-brand-text hover:text-brand-cyan hover:bg-brand-cyan/5 px-2 py-1 rounded transition-colors disabled:opacity-50"
                            >
                              {m}
                            </button>
                          ))}
                        </div>
                      </div>
                    ) : (
                      <p className="text-red-400">{ts.result.error}</p>
                    )}
                  </div>
                )}
              </div>
            )
          })()}

          <form onSubmit={handleSaveOllamaURL} className="space-y-4 border border-brand-border rounded-xl p-5 bg-brand-bg-card">
            <h3 className="text-sm font-medium text-brand-text">Set Ollama base URL</h3>

            <div className="flex flex-col gap-1.5">
              <label className="text-xs text-brand-text-muted uppercase tracking-wider">Base URL</label>
              <input
                type="url"
                value={ollamaURL}
                onChange={(e) => setOllamaURL(e.target.value)}
                placeholder="http://host.docker.internal:11434"
                className="bg-brand-bg border border-brand-border rounded-lg px-3 py-2 text-sm text-brand-text placeholder:text-brand-text-muted/40 focus:outline-none focus:border-brand-cyan font-mono"
              />
              <p className="text-xs text-brand-text-muted">
                Examples: <span className="font-mono">http://host.docker.internal:11434</span> (Docker Desktop),{' '}
                <span className="font-mono">http://192.168.1.10:11434</span> (LAN IP)
              </p>
            </div>

            {ollamaErr && <p className="text-xs text-red-400">{ollamaErr}</p>}
            {ollamaOk  && <p className="text-xs text-emerald-400">URL saved.</p>}

            <button
              type="submit"
              disabled={savingOllama || !ollamaURL.trim()}
              className="px-4 py-2 rounded-lg bg-brand-cyan text-brand-bg text-sm font-semibold hover:bg-brand-cyan/80 disabled:opacity-50 transition-colors"
            >
              {savingOllama ? 'Saving…' : 'Save URL'}
            </button>
          </form>
        </section>

        {/* Appearance */}
        <section>
          <h2 className="text-lg font-semibold text-brand-text mb-1">Appearance</h2>
          <p className="text-sm text-brand-text-muted mb-4">Choose your preferred colour scheme.</p>
          <div className="flex items-center justify-between border border-brand-border rounded-xl px-5 py-4 bg-brand-bg-card">
            <div>
              <p className="text-sm font-medium text-brand-text">
                {theme === 'dark' ? 'Dark mode' : 'Light mode'}
              </p>
              <p className="text-xs text-brand-text-muted mt-0.5">Saved automatically.</p>
            </div>
            <button
              onClick={toggleTheme}
              className="flex items-center gap-2 px-4 py-2 rounded-lg border border-brand-border text-sm text-brand-text hover:border-brand-cyan/40 hover:text-brand-cyan transition-colors"
            >
              {theme === 'dark' ? <SunIcon /> : <MoonIcon />}
              Switch to {theme === 'dark' ? 'light' : 'dark'}
            </button>
          </div>
        </section>

        {/* Feedback */}
        <section>
          <h2 className="text-lg font-semibold text-brand-text mb-1">Feedback &amp; Support</h2>
          <p className="text-sm text-brand-text-muted mb-4">
            NexusTale is in early alpha. Bug reports and feature ideas help a lot.
          </p>
          <div className="space-y-3">
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
                  Report bugs, request features, or share feedback
                </p>
              </div>
            </a>
            <Link
              to="/about"
              className="flex items-center gap-3 border border-brand-border rounded-xl px-5 py-4 bg-brand-bg-card hover:border-brand-cyan/40 transition-colors group"
            >
              <InfoIcon />
              <div>
                <p className="text-sm font-medium text-brand-text group-hover:text-brand-cyan transition-colors">
                  Known limitations
                </p>
                <p className="text-xs text-brand-text-muted mt-0.5">
                  What to expect during the alpha period
                </p>
              </div>
            </Link>
          </div>
        </section>

        {/* Editor Walkthrough */}
        <section>
          <h2 className="text-lg font-semibold text-brand-text mb-1">Editor Walkthrough</h2>
          <p className="text-sm text-brand-text-muted mb-4">
            Replay the 6-step tour that introduces the editor surfaces.
          </p>
          <button
            onClick={() => {
              localStorage.removeItem('nexustale_tour_done')
              window.history.back()
            }}
            className="px-4 py-2 rounded-lg border border-brand-border text-brand-muted text-sm font-medium hover:text-brand-text hover:border-brand-cyan/40 transition-colors"
          >
            Restart walkthrough
          </button>
        </section>

        {/* Danger Zone */}
        <section>
          <h2 className="text-lg font-semibold text-red-400 mb-1">Danger Zone</h2>
          <p className="text-sm text-brand-text-muted mb-4">
            Permanently delete your account and all your stories, wiki data, and settings.
            This cannot be undone.
          </p>
          <div className="border border-red-500/30 rounded-xl p-5 bg-red-500/5">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-brand-text">Delete account</p>
                <p className="text-xs text-brand-text-muted mt-0.5">All projects, scenes, and wiki data will be permanently removed.</p>
              </div>
              <button
                onClick={() => setShowDeleteDialog(true)}
                className="px-4 py-2 rounded-lg border border-red-500/50 text-red-400 text-sm font-medium hover:bg-red-500/10 transition-colors shrink-0 ml-6"
              >
                Delete account
              </button>
            </div>
          </div>
        </section>
      </main>

      {/* Version footer */}
      <p className="text-center text-xs text-brand-muted/50 py-6">
        NexusTale {import.meta.env.VITE_APP_VERSION ?? 'dev'}
      </p>

      {/* Delete account confirm dialog */}
      {showDeleteDialog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm px-4">
          <div className="bg-brand-bg-card border border-red-500/30 rounded-2xl p-8 w-full max-w-md shadow-card">
            <h2 className="text-xl font-bold text-red-400 mb-2">Delete account</h2>
            <p className="text-sm text-brand-text-muted mb-6">
              This will permanently delete all your stories, chapters, scenes, wiki data, and AI keys.
              Type your email address to confirm.
            </p>

            <div className="flex flex-col gap-1.5 mb-4">
              <label className="text-xs text-brand-text-muted uppercase tracking-wider">
                Your email: <span className="text-brand-text font-mono">{user?.email}</span>
              </label>
              <input
                autoFocus
                type="email"
                value={confirmEmail}
                onChange={(e) => setConfirmEmail(e.target.value)}
                placeholder={user?.email}
                className="bg-brand-bg border border-brand-border rounded-lg px-3 py-2 text-sm text-brand-text placeholder:text-brand-text-muted/40 focus:outline-none focus:border-red-500/60 font-mono"
              />
            </div>

            {deleteError && <p className="text-xs text-red-400 mb-3">{deleteError}</p>}

            <div className="flex gap-3">
              <button
                onClick={() => { setShowDeleteDialog(false); setConfirmEmail(''); setDeleteError(null) }}
                className="flex-1 py-2.5 rounded-lg border border-brand-border text-brand-muted hover:text-brand-text hover:border-brand-cyan/40 transition-colors text-sm font-medium"
              >
                Cancel
              </button>
              <button
                onClick={handleDeleteAccount}
                disabled={deleting || confirmEmail !== user?.email}
                className="flex-1 py-2.5 rounded-lg bg-red-600 text-white text-sm font-semibold hover:bg-red-500 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
              >
                {deleting ? 'Deleting…' : 'Delete forever'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
