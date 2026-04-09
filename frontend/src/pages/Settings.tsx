// Settings — user account settings, starting with AI provider key management.
import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'
import { api } from '@/services/api'
import type { APIKeyResponse } from '@/services/api'

const PROVIDERS = ['openai', 'anthropic', 'gemini', 'mistral', 'custom'] as const
type Provider = typeof PROVIDERS[number]

const PROVIDER_LABELS: Record<Provider, string> = {
  openai:    'OpenAI',
  anthropic: 'Anthropic',
  gemini:    'Google Gemini',
  mistral:   'Mistral',
  custom:    'Custom (OpenAI-compatible)',
}

export default function Settings() {
  const accessToken = useAuthStore((s) => s.accessToken)

  const [keys, setKeys]           = useState<APIKeyResponse[]>([])
  const [loadingKeys, setLoading] = useState(true)
  const [error, setError]         = useState<string | null>(null)

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
      .then(setKeys)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }, [accessToken])

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
    } catch {}
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
            <div className="mb-6 space-y-2">
              {keys.map((k) => (
                <div
                  key={k.id}
                  className="flex items-center justify-between px-4 py-3 rounded-lg border border-brand-border bg-brand-bg-card"
                >
                  <div>
                    <span className="text-sm font-medium text-brand-text">
                      {PROVIDER_LABELS[k.provider as Provider] ?? k.provider}
                    </span>
                    <span className="ml-3 text-xs text-brand-text-muted font-mono">
                      ••••{k.key_hint}
                    </span>
                  </div>
                  <button
                    onClick={() => handleDelete(k.provider)}
                    className="text-xs text-red-400 hover:text-red-300 transition-colors"
                  >
                    Remove
                  </button>
                </div>
              ))}
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
      </main>
    </div>
  )
}
