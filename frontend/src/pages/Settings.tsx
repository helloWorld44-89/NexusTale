// Settings — user account settings, starting with AI provider key management.
import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'
import { useThemeStore } from '@/app/store/themeStore'
import { api } from '@/services/api'
import type { APIKeyResponse } from '@/services/api'

const PROVIDERS = ['openai', 'anthropic', 'gemini', 'mistral', 'custom'] as const
type Provider = typeof PROVIDERS[number]

function SunIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <circle cx="10" cy="10" r="3.5" />
      <path d="M10 2v2M10 16v2M2 10h2M16 10h2M4.22 4.22l1.42 1.42M14.36 14.36l1.42 1.42M4.22 15.78l1.42-1.42M14.36 5.64l1.42-1.42" />
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
  openai:    'OpenAI',
  anthropic: 'Anthropic',
  gemini:    'Google Gemini',
  mistral:   'Mistral',
  custom:    'Custom (OpenAI-compatible)',
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
