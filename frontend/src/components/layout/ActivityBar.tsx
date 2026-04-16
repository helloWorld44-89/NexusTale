// ActivityBar — thin icon strip on the far left (VSCode-style).
import type { LeftPanel } from '@/pages/Editor'

interface ActivityBarProps {
  activePanel:      LeftPanel
  onToggleChat:     () => void
  onToggleGit:      () => void
  onToggleWiki:     () => void
  onToggleContext:  () => void
  onToggleWorkshop: () => void
}

export default function ActivityBar({ activePanel, onToggleChat, onToggleGit, onToggleWiki, onToggleContext, onToggleWorkshop }: ActivityBarProps) {
  return (
    <div className="w-12 flex flex-col items-center py-2 gap-1 bg-brand-bg border-r border-brand-border shrink-0">
      <ActivityButton
        active={activePanel === 'chat'}
        title="AI Chat (Ctrl+Shift+C)"
        onClick={onToggleChat}
      >
        <ChatIcon />
      </ActivityButton>

      <ActivityButton
        active={activePanel === 'context'}
        title="Context Pins"
        onClick={onToggleContext}
      >
        <PinIcon />
      </ActivityButton>

      <ActivityButton
        active={activePanel === 'workshop'}
        title="Workshop"
        onClick={onToggleWorkshop}
      >
        <WorkshopIcon />
      </ActivityButton>

      <ActivityButton
        active={activePanel === 'git'}
        title="Chronicle (Git)"
        onClick={onToggleGit}
      >
        <BranchIcon />
      </ActivityButton>

      <ActivityButton
        active={activePanel === 'wiki'}
        title="World Wiki"
        onClick={onToggleWiki}
      >
        <BookIcon />
      </ActivityButton>
    </div>
  )
}

function ActivityButton({
  active,
  title,
  onClick,
  children,
}: {
  active: boolean
  title: string
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      title={title}
      onClick={onClick}
      className={`w-9 h-9 flex items-center justify-center rounded transition-colors relative ${
        active
          ? 'text-brand-cyan'
          : 'text-brand-muted hover:text-brand-text'
      }`}
    >
      {active && (
        <span className="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-6 bg-brand-cyan rounded-r" />
      )}
      {children}
    </button>
  )
}

function ChatIcon() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 20 20" fill="currentColor">
      <path d="M2 5a2 2 0 012-2h12a2 2 0 012 2v7a2 2 0 01-2 2H6l-4 3V5z" />
    </svg>
  )
}

function PinIcon() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 2l-1.5 5.5L16 9l-6 9-1.5-5.5L3 11l9-9z" />
    </svg>
  )
}

function WorkshopIcon() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="3" width="6" height="6" rx="1" />
      <rect x="11" y="3" width="6" height="6" rx="1" />
      <rect x="3" y="11" width="6" height="6" rx="1" />
      <path d="M11 14h6M14 11v6" />
    </svg>
  )
}

function BranchIcon() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="5" cy="4" r="2" />
      <circle cx="5" cy="16" r="2" />
      <circle cx="15" cy="7" r="2" />
      <path d="M5 6v8M5 6C5 9 15 9 15 9" />
    </svg>
  )
}

function BookIcon() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 3h9a2 2 0 012 2v10a2 2 0 01-2 2H4a1 1 0 01-1-1V4a1 1 0 011-1z" />
      <path d="M13 3v14M7 7h3M7 10h3" />
    </svg>
  )
}
