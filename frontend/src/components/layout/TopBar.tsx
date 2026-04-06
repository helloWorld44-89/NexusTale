// TopBar — project breadcrumb + panel toggles.
import type { LeftPanel } from '@/pages/Editor'

interface TopBarProps {
  projectTitle: string
  chapterTitle: string
  sceneTitle: string
  leftPanel: LeftPanel
  explorerOpen: boolean
  onToggleChat: () => void
  onToggleExplorer: () => void
}

export default function TopBar({
  projectTitle,
  chapterTitle,
  sceneTitle,
  leftPanel,
  explorerOpen,
  onToggleChat,
  onToggleExplorer,
}: TopBarProps) {
  return (
    <header className="h-11 flex items-center justify-between px-3 bg-brand-bg-card border-b border-brand-border shrink-0 select-none">

      {/* Left: logo + app name */}
      <div className="flex items-center gap-2 w-48">
        <img src="/app-icon.png" alt="" className="w-5 h-5 opacity-80" />
        <span className="text-brand-cyan text-sm font-semibold tracking-wide">NexusTale</span>
      </div>

      {/* Center: breadcrumb */}
      <div className="flex items-center gap-1.5 text-sm text-brand-muted overflow-hidden">
        <span className="truncate max-w-[160px]">{projectTitle}</span>
        {chapterTitle && (
          <>
            <ChevronIcon />
            <span className="truncate max-w-[160px]">{chapterTitle}</span>
          </>
        )}
        {sceneTitle && (
          <>
            <ChevronIcon />
            <span className="text-brand-text truncate max-w-[160px]">{sceneTitle}</span>
          </>
        )}
      </div>

      {/* Right: panel toggles */}
      <div className="flex items-center gap-1 w-48 justify-end">
        <ToggleButton
          active={leftPanel === 'chat'}
          title="Toggle AI Chat"
          onClick={onToggleChat}
        >
          <ChatIcon />
        </ToggleButton>
        <ToggleButton
          active={explorerOpen}
          title="Toggle Project Explorer"
          onClick={onToggleExplorer}
        >
          <ExplorerIcon />
        </ToggleButton>
      </div>
    </header>
  )
}

function ToggleButton({
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
      className={`p-1.5 rounded transition-colors ${
        active
          ? 'text-brand-cyan bg-brand-cyan/10'
          : 'text-brand-muted hover:text-brand-text hover:bg-brand-border/40'
      }`}
    >
      {children}
    </button>
  )
}

function ChevronIcon() {
  return (
    <svg className="w-3 h-3 shrink-0 opacity-40" viewBox="0 0 16 16" fill="currentColor">
      <path d="M6 3l5 5-5 5" stroke="currentColor" strokeWidth="1.5" fill="none" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function ChatIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
      <path d="M2 5a2 2 0 012-2h12a2 2 0 012 2v7a2 2 0 01-2 2H6l-4 3V5z" />
    </svg>
  )
}

function ExplorerIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
      <path d="M2 4a1 1 0 011-1h4l2 2h7a1 1 0 011 1v9a1 1 0 01-1 1H3a1 1 0 01-1-1V4z" />
    </svg>
  )
}
