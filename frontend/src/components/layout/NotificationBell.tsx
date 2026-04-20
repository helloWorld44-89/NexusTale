import { useEffect, useRef, useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type Notification } from '@/services/api'

const POLL_INTERVAL_MS = 60_000

interface Props {
  token: string
}

export default function NotificationBell({ token }: Props) {
  const [notifications, setNotifications] = useState<Notification[]>([])
  const [open, setOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()

  const unreadCount = notifications.filter((n) => n.read_at === null).length

  const fetchNotifications = useCallback(async () => {
    try {
      const data = await api.notifications.list(token)
      setNotifications(data)
    } catch {
      // silently ignore poll failures
    }
  }, [token])

  // Initial fetch + 60 s polling
  useEffect(() => {
    fetchNotifications()
    const interval = setInterval(fetchNotifications, POLL_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [fetchNotifications])

  // Close dropdown on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    if (open) document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [open])

  async function handleMarkRead(n: Notification) {
    if (n.read_at !== null) return
    await api.notifications.markRead(token, n.id).catch(() => {})
    setNotifications((prev) =>
      prev.map((x) => (x.id === n.id ? { ...x, read_at: new Date().toISOString() } : x))
    )
    navigateTo(n)
    setOpen(false)
  }

  async function handleMarkAllRead() {
    await api.notifications.markAllRead(token).catch(() => {})
    setNotifications((prev) => prev.map((n) => ({ ...n, read_at: n.read_at ?? new Date().toISOString() })))
  }

  function navigateTo(n: Notification) {
    if (!n.project_id) return
    switch (n.type) {
      case 'invite_received': {
        const inviteToken = n.payload.invite_token as string | undefined
        if (inviteToken) navigate(`/invites/${inviteToken}`)
        else navigate(`/projects/${n.project_id}`)
        break
      }
      case 'export_complete':
        navigate(`/projects/${n.project_id}`)
        break
      case 'mr_opened':
      case 'mr_approved':
      case 'mr_rejected':
      case 'mr_merged':
      case 'annotation_added':
      case 'summary_ready':
        navigate(`/projects/${n.project_id}`)
        break
    }
  }

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={() => setOpen((v) => !v)}
        title="Notifications"
        className={`relative p-1.5 rounded transition-colors ${
          open
            ? 'text-brand-cyan bg-brand-cyan/10'
            : 'text-brand-muted hover:text-brand-text hover:bg-brand-border/40'
        }`}
      >
        <BellIcon />
        {unreadCount > 0 && (
          <span className="absolute -top-0.5 -right-0.5 min-w-[14px] h-[14px] px-0.5 rounded-full bg-brand-cyan text-brand-bg text-[9px] font-bold flex items-center justify-center leading-none">
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        )}
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-1 w-80 bg-brand-bg-card border border-brand-border rounded-lg shadow-xl z-50 overflow-hidden">
          {/* Header */}
          <div className="flex items-center justify-between px-3 py-2 border-b border-brand-border">
            <span className="text-xs font-semibold text-brand-text">Notifications</span>
            {unreadCount > 0 && (
              <button
                onClick={handleMarkAllRead}
                className="text-xs text-brand-cyan hover:text-brand-cyan/70 transition-colors"
              >
                Mark all read
              </button>
            )}
          </div>

          {/* List */}
          <div className="max-h-80 overflow-y-auto">
            {notifications.length === 0 ? (
              <p className="px-3 py-4 text-xs text-brand-muted text-center">No notifications</p>
            ) : (
              notifications.map((n) => (
                <NotificationRow key={n.id} notification={n} onClick={() => handleMarkRead(n)} />
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}

// ── Row ───────────────────────────────────────────────────────────────────────

function NotificationRow({ notification: n, onClick }: { notification: Notification; onClick: () => void }) {
  const isUnread = n.read_at === null
  const { copy, icon } = renderNotification(n)

  return (
    <button
      onClick={onClick}
      className={`w-full flex items-start gap-2.5 px-3 py-2.5 text-left hover:bg-brand-border/30 transition-colors border-b border-brand-border/40 last:border-0 ${
        isUnread ? 'bg-brand-cyan/5' : ''
      }`}
    >
      <span className={`mt-0.5 shrink-0 ${isUnread ? 'text-brand-cyan' : 'text-brand-muted'}`}>
        {icon}
      </span>
      <div className="flex-1 min-w-0">
        <p className={`text-xs leading-snug ${isUnread ? 'text-brand-text' : 'text-brand-muted'}`}>
          {copy}
        </p>
        <p className="text-[10px] text-brand-muted/60 mt-0.5">{relativeTime(n.created_at)}</p>
      </div>
      {isUnread && <span className="w-1.5 h-1.5 rounded-full bg-brand-cyan shrink-0 mt-1" />}
    </button>
  )
}

// ── display helpers ───────────────────────────────────────────────────────────

function renderNotification(n: Notification): { copy: string; icon: React.ReactNode } {
  const p = n.payload
  switch (n.type) {
    case 'invite_received':
      return {
        copy: `${p.inviter_name ?? 'Someone'} invited you to "${p.project_title ?? 'a project'}" as ${p.role ?? 'collaborator'}`,
        icon: <InviteIcon />,
      }
    case 'mr_opened':
      return { copy: `New merge request in "${p.project_title ?? 'a project'}"`, icon: <MRIcon /> }
    case 'mr_approved':
      return { copy: `Your merge request in "${p.project_title ?? 'a project'}" was approved`, icon: <MRIcon /> }
    case 'mr_rejected':
      return { copy: `Your merge request in "${p.project_title ?? 'a project'}" was rejected`, icon: <MRIcon /> }
    case 'mr_merged':
      return { copy: `Merge request merged in "${p.project_title ?? 'a project'}"`, icon: <MRIcon /> }
    case 'annotation_added':
      return { copy: `New annotation in "${p.project_title ?? 'a project'}"`, icon: <AnnotationIcon /> }
    case 'export_complete':
      return { copy: `Export ready: ${p.format ?? 'file'} for "${p.project_title ?? 'a project'}"`, icon: <ExportIcon /> }
    case 'summary_ready':
      return { copy: `Chapter summary updated in "${p.project_title ?? 'a project'}"`, icon: <SummaryIcon /> }
    default:
      return { copy: 'New notification', icon: <BellIcon /> }
  }
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins  = Math.floor(diff / 60_000)
  const hours = Math.floor(diff / 3_600_000)
  const days  = Math.floor(diff / 86_400_000)
  if (mins < 1)   return 'just now'
  if (mins < 60)  return `${mins}m ago`
  if (hours < 24) return `${hours}h ago`
  return `${days}d ago`
}

// ── icons ─────────────────────────────────────────────────────────────────────

function BellIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M10 2a6 6 0 00-6 6v3l-1.5 2.5h15L16 11V8a6 6 0 00-6-6z" />
      <path d="M8 16a2 2 0 004 0" />
    </svg>
  )
}

function InviteIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="6" cy="5" r="2.5" />
      <path d="M1 14c0-2.76 2.24-5 5-5h1" />
      <path d="M12 10v4M10 12h4" />
    </svg>
  )
}

function MRIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="4" cy="4" r="1.5" />
      <circle cx="12" cy="12" r="1.5" />
      <path d="M4 5.5v5a3 3 0 003 3h1.5" />
      <path d="M10 9l2 2-2 2" />
    </svg>
  )
}

function AnnotationIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2 3h12a1 1 0 011 1v7a1 1 0 01-1 1H5l-3 2V4a1 1 0 011-1z" />
    </svg>
  )
}

function ExportIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 2v8M5 7l3 3 3-3" />
      <path d="M3 12h10" />
    </svg>
  )
}

function SummaryIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 4h10M3 7h7M3 10h5" />
    </svg>
  )
}
