import { useEffect, useState } from 'react'
import { api } from '@/services/api'
import type { CollaboratorResponse, InviteResponse, CollabRole } from '@/services/api'

interface Props {
  projectId:   string
  ownerId:     string
  currentUser: string   // userID of the logged-in user
  token:       string
}

const ROLE_LABELS: Record<CollabRole, string> = {
  coauthor: 'Co-author',
  editor:   'Editor',
  reviewer: 'Reviewer',
}

const ROLE_COLORS: Record<CollabRole, string> = {
  coauthor: 'text-brand-cyan',
  editor:   'text-brand-purple',
  reviewer: 'text-amber-400',
}

export default function CollaboratorsPanel({ projectId, ownerId, currentUser, token }: Props) {
  const isOwner = currentUser === ownerId

  const [collabs,  setCollabs]  = useState<CollaboratorResponse[]>([])
  const [invites,  setInvites]  = useState<InviteResponse[]>([])
  const [loading,  setLoading]  = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [email,    setEmail]    = useState('')
  const [role,     setRole]     = useState<CollabRole>('reviewer')
  const [sending,  setSending]  = useState(false)
  const [formErr,  setFormErr]  = useState<string | null>(null)
  const [newToken, setNewToken] = useState<string | null>(null)   // token to display after invite
  const [copied,   setCopied]   = useState(false)

  const load = async () => {
    try {
      const [c, i] = await Promise.all([
        api.collaboration.listCollaborators(token, projectId),
        isOwner ? api.collaboration.listInvites(token, projectId) : Promise.resolve([]),
      ])
      setCollabs(c)
      setInvites(i)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [projectId])

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!email.trim()) return
    setSending(true)
    setFormErr(null)
    setNewToken(null)
    try {
      const inv = await api.collaboration.invite(token, projectId, email.trim(), role)
      setNewToken(inv.token)
      setEmail('')
      setInvites((p) => [inv, ...p])
    } catch (err: unknown) {
      setFormErr(err instanceof Error ? err.message : 'Failed to send invite.')
    } finally {
      setSending(false)
    }
  }

  const handleRemove = async (userId: string) => {
    if (!confirm('Remove this collaborator?')) return
    try {
      await api.collaboration.removeCollaborator(token, projectId, userId)
      setCollabs((p) => p.filter((c) => c.user_id !== userId))
    } catch {
      // ignore
    }
  }

  const handleCopy = (tok: string) => {
    const url = `${window.location.origin}/invites/${tok}`
    navigator.clipboard.writeText(url).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  const inviteURL = (tok: string) => `${window.location.origin}/invites/${tok}`

  if (loading) return null

  return (
    <div className="bg-brand-bg-card border border-brand-border rounded-xl p-6 space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold text-brand-text">Collaborators</h2>
        {isOwner && !showForm && (
          <button
            onClick={() => setShowForm(true)}
            className="text-sm text-brand-cyan hover:underline"
          >
            + Invite
          </button>
        )}
      </div>

      {/* Invite form */}
      {isOwner && showForm && (
        <form onSubmit={handleInvite} className="space-y-3 pb-2 border-b border-brand-border">
          <div className="flex gap-2">
            <input
              type="email"
              placeholder="Email address"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="flex-1 bg-brand-bg border border-brand-border rounded-lg px-3 py-2 text-sm text-brand-text placeholder-brand-muted focus:outline-none focus:border-brand-cyan"
            />
            <select
              value={role}
              onChange={(e) => setRole(e.target.value as CollabRole)}
              className="bg-brand-bg border border-brand-border rounded-lg px-3 py-2 text-sm text-brand-text focus:outline-none focus:border-brand-cyan"
            >
              <option value="reviewer">Reviewer</option>
              <option value="editor">Editor</option>
              <option value="coauthor">Co-author</option>
            </select>
          </div>

          {formErr && <p className="text-red-400 text-xs">{formErr}</p>}

          {newToken && (
            <div className="bg-brand-bg border border-brand-border rounded-lg p-3 space-y-1">
              <p className="text-xs text-brand-muted">Share this invite link:</p>
              <div className="flex items-center gap-2">
                <code className="text-xs text-brand-cyan flex-1 break-all">{inviteURL(newToken)}</code>
                <button
                  type="button"
                  onClick={() => handleCopy(newToken)}
                  className="text-xs text-brand-muted hover:text-brand-text shrink-0"
                >
                  {copied ? 'Copied!' : 'Copy'}
                </button>
              </div>
            </div>
          )}

          <div className="flex gap-2">
            <button
              type="submit"
              disabled={sending || !email.trim()}
              className="px-4 py-1.5 bg-brand-cyan text-black rounded-lg text-sm font-medium hover:bg-cyan-300 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {sending ? 'Sending…' : 'Send invite'}
            </button>
            <button
              type="button"
              onClick={() => { setShowForm(false); setNewToken(null); setFormErr(null) }}
              className="px-4 py-1.5 text-brand-muted text-sm hover:text-brand-text"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {/* Collaborator list */}
      {collabs.length === 0 && invites.length === 0 ? (
        <p className="text-brand-muted text-sm">No collaborators yet.</p>
      ) : (
        <ul className="space-y-2">
          {collabs.map((c) => (
            <li key={c.user_id} className="flex items-center justify-between">
              <div>
                <span className="text-sm text-brand-text">{c.display_name}</span>
                <span className="text-xs text-brand-muted ml-2">{c.email}</span>
              </div>
              <div className="flex items-center gap-3">
                <span className={`text-xs font-medium ${ROLE_COLORS[c.role]}`}>
                  {ROLE_LABELS[c.role]}
                </span>
                {isOwner && (
                  <button
                    onClick={() => handleRemove(c.user_id)}
                    className="text-xs text-brand-muted hover:text-red-400 transition-colors"
                  >
                    Remove
                  </button>
                )}
              </div>
            </li>
          ))}

          {/* Pending invites (owner only) */}
          {isOwner && invites.map((inv) => (
            <li key={inv.id} className="flex items-center justify-between opacity-60">
              <div>
                <span className="text-sm text-brand-text">{inv.email}</span>
                <span className="text-xs text-brand-muted ml-2">pending</span>
              </div>
              <div className="flex items-center gap-3">
                <span className={`text-xs font-medium ${ROLE_COLORS[inv.role]}`}>
                  {ROLE_LABELS[inv.role]}
                </span>
                <button
                  onClick={() => handleCopy(inv.token)}
                  className="text-xs text-brand-muted hover:text-brand-text"
                >
                  Copy link
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
