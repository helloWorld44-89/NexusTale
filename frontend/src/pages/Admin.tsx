import { useEffect, useState } from 'react'
import { Link, Navigate } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'
import { api } from '@/services/api'
import type { AdminStats, AdminUser, AdminAIUsageRow } from '@/services/api'

const ROLES  = ['author', 'admin', 'viewer']
const PLANS  = ['free', 'scribe', 'chronicler', 'studio']

export default function Admin() {
  const user        = useAuthStore((s) => s.user)
  const accessToken = useAuthStore((s) => s.accessToken)

  const [stats,     setStats]     = useState<AdminStats | null>(null)
  const [users,     setUsers]     = useState<AdminUser[]>([])
  const [aiUsage,   setAIUsage]   = useState<AdminAIUsageRow[]>([])
  const [loading,   setLoading]   = useState(true)
  const [error,     setError]     = useState<string | null>(null)
  const [saving,    setSaving]    = useState<string | null>(null) // user id being saved
  const [tab,       setTab]       = useState<'users' | 'ai'>('users')
  const [userOffset, setUserOffset] = useState(0)
  const PAGE = 50

  if (!user || user.role !== 'admin') return <Navigate to="/dashboard" replace />

  useEffect(() => {
    if (!accessToken) return
    setLoading(true)
    Promise.all([
      api.admin.stats(accessToken),
      api.admin.listUsers(accessToken, PAGE, userOffset),
      api.admin.aiUsage(accessToken),
    ])
      .then(([s, u, a]) => { setStats(s); setUsers(u); setAIUsage(a) })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load admin data'))
      .finally(() => setLoading(false))
  }, [accessToken, userOffset])

  const handleUpdate = async (userId: string, field: 'role' | 'plan', value: string) => {
    if (!accessToken) return
    setSaving(userId + field)
    try {
      await api.admin.updateUser(accessToken, userId, { [field]: value })
      setUsers((prev) => prev.map((u) => u.id === userId ? { ...u, [field]: value } : u))
    } catch (e) {
      alert(e instanceof Error ? e.message : 'Update failed')
    } finally {
      setSaving(null)
    }
  }

  return (
    <div className="min-h-screen bg-brand-bg text-brand-text">
      {/* Header */}
      <header className="border-b border-brand-border px-6 py-3 flex items-center gap-4">
        <Link to="/dashboard" className="text-brand-muted hover:text-brand-text text-sm transition-colors">← Dashboard</Link>
        <span className="text-brand-cyan font-semibold">Admin</span>
      </header>

      <div className="max-w-6xl mx-auto px-6 py-8">
        {error && <p className="text-red-400 mb-4">{error}</p>}

        {/* Stat cards */}
        {stats && (
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 mb-8">
            {[
              { label: 'Users',      value: stats.total_users.toLocaleString() },
              { label: 'Projects',   value: stats.total_projects.toLocaleString() },
              { label: 'Scenes',     value: stats.total_scenes.toLocaleString() },
              { label: 'AI Calls',   value: stats.total_ai_calls.toLocaleString() },
              { label: 'Tokens',     value: stats.total_tokens.toLocaleString() },
              { label: 'AI Cost',    value: `$${stats.total_cost_usd.toFixed(2)}` },
            ].map(({ label, value }) => (
              <div key={label} className="bg-brand-bg-card border border-brand-border rounded-lg px-4 py-3">
                <p className="text-xs text-brand-muted mb-1">{label}</p>
                <p className="text-lg font-semibold text-brand-text tabular-nums">{value}</p>
              </div>
            ))}
          </div>
        )}

        {/* Tab nav */}
        <div className="flex gap-2 mb-6 border-b border-brand-border">
          {(['users', 'ai'] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                tab === t
                  ? 'border-brand-cyan text-brand-cyan'
                  : 'border-transparent text-brand-muted hover:text-brand-text'
              }`}
            >
              {t === 'users' ? 'Users' : 'AI Usage (30d)'}
            </button>
          ))}
        </div>

        {loading && <p className="text-brand-muted text-sm">Loading…</p>}

        {/* Users table */}
        {!loading && tab === 'users' && (
          <div>
            <div className="overflow-x-auto rounded-lg border border-brand-border">
              <table className="w-full text-sm">
                <thead className="bg-brand-bg-card border-b border-brand-border">
                  <tr>
                    {['Email', 'Name', 'Role', 'Plan', 'Projects', 'Joined'].map((h) => (
                      <th key={h} className="text-left px-4 py-2.5 text-brand-muted font-medium text-xs uppercase tracking-wider">{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody className="divide-y divide-brand-border/40">
                  {users.map((u) => (
                    <tr key={u.id} className="hover:bg-brand-bg-card/50 transition-colors">
                      <td className="px-4 py-2.5 text-brand-text font-mono text-xs">{u.email}</td>
                      <td className="px-4 py-2.5 text-brand-text">{u.display_name}</td>
                      <td className="px-4 py-2.5">
                        <select
                          value={u.role}
                          disabled={saving === u.id + 'role'}
                          onChange={(e) => handleUpdate(u.id, 'role', e.target.value)}
                          className="bg-brand-bg border border-brand-border rounded px-2 py-0.5 text-xs text-brand-text focus:outline-none focus:border-brand-purple disabled:opacity-50"
                        >
                          {ROLES.map((r) => <option key={r} value={r}>{r}</option>)}
                        </select>
                      </td>
                      <td className="px-4 py-2.5">
                        <select
                          value={u.plan}
                          disabled={saving === u.id + 'plan'}
                          onChange={(e) => handleUpdate(u.id, 'plan', e.target.value)}
                          className="bg-brand-bg border border-brand-border rounded px-2 py-0.5 text-xs text-brand-text focus:outline-none focus:border-brand-purple disabled:opacity-50"
                        >
                          {PLANS.map((p) => <option key={p} value={p}>{p}</option>)}
                        </select>
                      </td>
                      <td className="px-4 py-2.5 text-brand-muted tabular-nums">{u.project_count}</td>
                      <td className="px-4 py-2.5 text-brand-muted text-xs">{u.created_at.slice(0, 10)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            <div className="flex gap-2 mt-4 justify-end">
              <button
                disabled={userOffset === 0}
                onClick={() => setUserOffset((o) => Math.max(0, o - PAGE))}
                className="px-3 py-1 text-xs rounded border border-brand-border text-brand-muted hover:text-brand-text disabled:opacity-30 transition-colors"
              >
                ← Prev
              </button>
              <span className="text-xs text-brand-muted self-center">
                {userOffset + 1}–{userOffset + users.length}
              </span>
              <button
                disabled={users.length < PAGE}
                onClick={() => setUserOffset((o) => o + PAGE)}
                className="px-3 py-1 text-xs rounded border border-brand-border text-brand-muted hover:text-brand-text disabled:opacity-30 transition-colors"
              >
                Next →
              </button>
            </div>
          </div>
        )}

        {/* AI Usage table */}
        {!loading && tab === 'ai' && (
          <div className="overflow-x-auto rounded-lg border border-brand-border">
            <table className="w-full text-sm">
              <thead className="bg-brand-bg-card border-b border-brand-border">
                <tr>
                  {['Email', 'Name', 'Calls', 'Tokens', 'Cost (USD)'].map((h) => (
                    <th key={h} className="text-left px-4 py-2.5 text-brand-muted font-medium text-xs uppercase tracking-wider">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-brand-border/40">
                {aiUsage.map((row) => (
                  <tr key={row.user_id} className="hover:bg-brand-bg-card/50 transition-colors">
                    <td className="px-4 py-2.5 text-brand-text font-mono text-xs">{row.email}</td>
                    <td className="px-4 py-2.5 text-brand-text">{row.display_name}</td>
                    <td className="px-4 py-2.5 text-brand-muted tabular-nums">{row.call_count.toLocaleString()}</td>
                    <td className="px-4 py-2.5 text-brand-muted tabular-nums">{row.total_tokens.toLocaleString()}</td>
                    <td className="px-4 py-2.5 text-brand-muted tabular-nums">${row.total_cost_usd.toFixed(4)}</td>
                  </tr>
                ))}
                {aiUsage.length === 0 && (
                  <tr><td colSpan={5} className="px-4 py-6 text-center text-brand-muted text-xs">No AI calls in the last 30 days.</td></tr>
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
