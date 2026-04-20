import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'
import { api } from '@/services/api'
import type { InvitePreview } from '@/services/api'

const ROLE_LABELS: Record<string, string> = {
  coauthor: 'Co-author',
  editor:   'Editor',
  reviewer: 'Reviewer',
}

const ROLE_DESCRIPTIONS: Record<string, string> = {
  coauthor: 'Can add new chapters and scenes, and request merges into the main manuscript.',
  editor:   'Can add content and leave suggestions on existing scenes.',
  reviewer: 'Can read the manuscript and leave inline notes and comments.',
}

export default function InviteAccept() {
  const { token } = useParams<{ token: string }>()
  const navigate = useNavigate()
  const { accessToken, isAuthenticated } = useAuthStore((s) => ({
    accessToken:     s.accessToken,
    isAuthenticated: s.isAuthenticated,
  }))

  const [preview,   setPreview]   = useState<InvitePreview | null>(null)
  const [loading,   setLoading]   = useState(true)
  const [accepting, setAccepting] = useState(false)
  const [error,     setError]     = useState<string | null>(null)
  const [done,      setDone]      = useState(false)

  useEffect(() => {
    if (!token) return
    api.collaboration.getInvitePreview(token)
      .then(setPreview)
      .catch((e) => setError(e.message ?? 'Invalid or expired invite link.'))
      .finally(() => setLoading(false))
  }, [token])

  const handleAccept = async () => {
    if (!token || !accessToken) return
    setAccepting(true)
    setError(null)
    try {
      const collab = await api.collaboration.acceptInvite(accessToken, token)
      setDone(true)
      setTimeout(() => navigate(`/projects/${collab.project_id}`, { replace: true }), 1800)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to accept invite.')
    } finally {
      setAccepting(false)
    }
  }

  const handleLogin = () => {
    navigate(`/login?redirect=/invites/${token}`, { replace: true })
  }

  if (loading) {
    return (
      <div className="min-h-screen bg-brand-bg flex items-center justify-center">
        <span className="text-brand-muted text-sm">Loading invite…</span>
      </div>
    )
  }

  if (error && !preview) {
    return (
      <div className="min-h-screen bg-brand-bg flex items-center justify-center">
        <div className="bg-brand-bg-card border border-brand-border rounded-xl p-8 max-w-sm w-full text-center space-y-3">
          <p className="text-red-400 font-medium">Invite unavailable</p>
          <p className="text-brand-muted text-sm">{error}</p>
          <button
            onClick={() => navigate('/dashboard')}
            className="text-brand-cyan text-sm hover:underline"
          >
            Go to dashboard
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-brand-bg flex items-center justify-center px-4">
      <div className="bg-brand-bg-card border border-brand-border rounded-xl p-8 max-w-sm w-full space-y-6">
        {/* Header */}
        <div className="text-center space-y-1">
          <div className="text-2xl font-bold text-brand-text">{preview?.project_title}</div>
          <p className="text-brand-muted text-sm">
            <span className="text-brand-text">{preview?.inviter_name}</span> invited you to collaborate
          </p>
        </div>

        {/* Role card */}
        <div className="bg-brand-bg rounded-lg border border-brand-border p-4 space-y-1">
          <div className="flex items-center gap-2">
            <span className="text-xs font-semibold uppercase tracking-wider text-brand-cyan">
              {ROLE_LABELS[preview?.role ?? ''] ?? preview?.role}
            </span>
          </div>
          <p className="text-brand-muted text-sm">
            {ROLE_DESCRIPTIONS[preview?.role ?? ''] ?? ''}
          </p>
        </div>

        {/* Expires */}
        <p className="text-brand-muted text-xs text-center">
          Expires {preview ? new Date(preview.expires_at).toLocaleDateString() : ''}
        </p>

        {/* Action */}
        {done ? (
          <div className="text-center text-brand-cyan text-sm font-medium">
            Joined! Redirecting…
          </div>
        ) : !isAuthenticated ? (
          <div className="space-y-3">
            <p className="text-brand-muted text-xs text-center">
              Sign in to your NexusTale account to accept this invite.
            </p>
            <button
              onClick={handleLogin}
              className="w-full py-2.5 rounded-lg bg-brand-cyan text-black font-semibold text-sm hover:bg-cyan-300 transition-colors"
            >
              Sign in to accept
            </button>
          </div>
        ) : (
          <div className="space-y-3">
            {error && <p className="text-red-400 text-sm text-center">{error}</p>}
            <button
              onClick={handleAccept}
              disabled={accepting}
              className="w-full py-2.5 rounded-lg bg-brand-cyan text-black font-semibold text-sm hover:bg-cyan-300 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {accepting ? 'Joining…' : 'Accept and join project'}
            </button>
            <button
              onClick={() => navigate('/dashboard')}
              className="w-full py-2 rounded-lg text-brand-muted text-sm hover:text-brand-text transition-colors"
            >
              Decline
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
