// TopBar — project breadcrumb + navigation + panel toggles + user menu.
declare const __APP_VERSION__: string

import { useState } from 'react'
import { Link } from 'react-router-dom'
import type { LeftPanel } from '@/pages/Editor'
import NotificationBell from './NotificationBell'

// ── Phase metadata ────────────────────────────────────────────────────────────

type ProjectPhase = 'drafting' | 'story_pass' | 'character_pass' | 'language_pass' | 'editorial_pass'

const PHASES: { value: ProjectPhase; label: string; description: string; color: string }[] = [
  { value: 'drafting',       label: 'Drafting',       description: 'Writing new content — no revision focus.',                      color: 'text-brand-muted' },
  { value: 'story_pass',     label: 'Story Pass',     description: 'Structural review — pacing, threads, promise/payoff.',          color: 'text-brand-purple' },
  { value: 'character_pass', label: 'Character Pass', description: 'Character consistency — motivation, voice, arc progression.',   color: 'text-brand-cyan' },
  { value: 'language_pass',  label: 'Language Pass',  description: 'Line editing — prose rhythm, word choice, sentence variety.',   color: 'text-amber-400' },
  { value: 'editorial_pass', label: 'Editorial Pass', description: 'Big-picture notes — chapter openings, act structure, POV.',     color: 'text-emerald-400' },
]

function phaseLabel(phase: string) {
  return PHASES.find((p) => p.value === phase)?.label ?? 'Drafting'
}

function phaseColor(phase: string) {
  return PHASES.find((p) => p.value === phase)?.color ?? 'text-brand-muted'
}

// ── Props ─────────────────────────────────────────────────────────────────────

interface TopBarProps {
  projectId:    string
  projectTitle: string
  projectPhase: string
  actTitle:     string   // empty when act layer is hidden (single default act)
  chapterTitle: string
  sceneTitle:   string
  displayName:  string
  token:        string
  leftPanel:    LeftPanel
  explorerOpen: boolean
  focusMode:    boolean
  onToggleChat:     () => void
  onToggleExplorer: () => void
  onToggleFocus:    () => void
  onLogout:         () => void
  onPhaseChange:    (phase: string) => void
}

export default function TopBar({
  projectId,
  projectTitle,
  projectPhase,
  actTitle,
  chapterTitle,
  sceneTitle,
  displayName,
  token,
  leftPanel,
  explorerOpen,
  focusMode,
  onToggleChat,
  onToggleExplorer,
  onToggleFocus,
  onLogout,
  onPhaseChange,
}: TopBarProps) {
  const [phaseOpen, setPhaseOpen] = useState(false)

  return (
    <header className="h-11 flex items-center justify-between px-3 bg-brand-bg-card border-b border-brand-border shrink-0 select-none gap-2">

      {/* Left: logo + app nav */}
      <div className="flex items-center gap-1 shrink-0">
        <Link
          to="/dashboard"
          className="flex items-center gap-1.5 px-1.5 py-1 rounded hover:bg-brand-border/40 transition-colors group"
          title="Dashboard"
        >
          <img src="/app-icon.png" alt="" className="w-4 h-4 opacity-80" />
          <span className="text-brand-cyan text-xs font-semibold tracking-wide">NexusTale</span>
          <span className="text-[9px] text-brand-muted/50 font-mono leading-none self-end mb-px">v{__APP_VERSION__}</span>
        </Link>

        <span className="text-brand-border/60 text-xs px-0.5">·</span>

        <Link
          to={`/projects/${projectId}`}
          className="flex items-center gap-1 px-1.5 py-1 rounded text-xs text-brand-muted hover:text-brand-text hover:bg-brand-border/40 transition-colors"
          title="Project Home"
        >
          <HomeIcon />
          <span className="hidden sm:inline">Home</span>
        </Link>

        <Link
          to={`/projects/${projectId}/wiki`}
          className="flex items-center gap-1 px-1.5 py-1 rounded text-xs text-brand-muted hover:text-brand-text hover:bg-brand-border/40 transition-colors"
          title="World Wiki"
        >
          <WikiIcon />
          <span className="hidden sm:inline">Wiki</span>
        </Link>

        <Link
          to={`/projects/${projectId}/guide`}
          className="flex items-center gap-1 px-1.5 py-1 rounded text-xs text-brand-muted hover:text-brand-text hover:bg-brand-border/40 transition-colors"
          title="Novel Guide"
        >
          <GuideIcon />
          <span className="hidden sm:inline">Guide</span>
        </Link>
      </div>

      {/* Center: breadcrumb + phase badge */}
      <div className="flex items-center gap-2 text-xs text-brand-muted overflow-hidden flex-1 justify-center">
        {/* Breadcrumb */}
        <div className="flex items-center gap-1 overflow-hidden">
          <span className="truncate max-w-[120px]">{projectTitle}</span>
          {actTitle && (
            <>
              <ChevronIcon />
              <span className="truncate max-w-[100px] text-brand-purple/70">{actTitle}</span>
            </>
          )}
          {chapterTitle && (
            <>
              <ChevronIcon />
              <span className="truncate max-w-[120px]">{chapterTitle}</span>
            </>
          )}
          {sceneTitle && (
            <>
              <ChevronIcon />
              <span className="text-brand-text truncate max-w-[120px]">{sceneTitle}</span>
            </>
          )}
        </div>

        {/* Phase badge — only shown when a project is loaded */}
        {projectTitle && (
          <div className="relative shrink-0">
            <button
              onClick={() => setPhaseOpen((v) => !v)}
              className={`flex items-center gap-1 px-2 py-0.5 rounded border border-brand-border/50 hover:border-brand-border transition-colors text-[10px] font-medium ${phaseColor(projectPhase)}`}
              title="Change writing phase"
            >
              <PhaseIcon />
              {phaseLabel(projectPhase)}
            </button>

            {phaseOpen && (
              <>
                {/* Backdrop */}
                <div
                  className="fixed inset-0 z-30"
                  onClick={() => setPhaseOpen(false)}
                />
                {/* Modal */}
                <div className="absolute left-1/2 -translate-x-1/2 top-full mt-1 z-40 w-72 bg-brand-bg-card border border-brand-border rounded-xl shadow-2xl p-3 space-y-1">
                  <p className="text-[10px] text-brand-muted uppercase tracking-wider px-1 pb-1">Writing Phase</p>
                  {PHASES.map((p) => (
                    <button
                      key={p.value}
                      onClick={() => { onPhaseChange(p.value); setPhaseOpen(false) }}
                      className={`w-full text-left px-3 py-2 rounded-lg transition-colors ${
                        projectPhase === p.value
                          ? 'bg-brand-border/40'
                          : 'hover:bg-brand-border/20'
                      }`}
                    >
                      <span className={`text-xs font-medium ${p.color}`}>{p.label}</span>
                      <p className="text-[11px] text-brand-muted mt-0.5">{p.description}</p>
                    </button>
                  ))}
                </div>
              </>
            )}
          </div>
        )}
      </div>

      {/* Right: panel toggles + user area */}
      <div className="flex items-center gap-1 shrink-0">
        {/* Panel toggles */}
        <ToggleButton active={leftPanel === 'chat'} title="Toggle Nexus AI" onClick={onToggleChat}>
          <ChatIcon />
        </ToggleButton>
        <ToggleButton active={explorerOpen} title="Toggle Project Explorer" onClick={onToggleExplorer}>
          <ExplorerIcon />
        </ToggleButton>
        <ToggleButton active={focusMode} title="Focus mode (F11)" onClick={onToggleFocus}>
          <FocusIcon />
        </ToggleButton>

        {/* Divider */}
        <span className="w-px h-4 bg-brand-border/60 mx-1" />

        {/* Notifications */}
        {token && <NotificationBell token={token} />}

        {/* Username chip */}
        {displayName && (
          <span className="text-xs text-brand-muted px-1.5 truncate max-w-[80px]" title={displayName}>
            {displayName}
          </span>
        )}

        {/* Feedback */}
        <a
          href="https://github.com/helloWorld44-89/NexusTale/issues"
          target="_blank"
          rel="noreferrer"
          title="Report a bug or give feedback"
          className="p-1.5 rounded text-brand-muted hover:text-brand-text hover:bg-brand-border/40 transition-colors"
        >
          <FeedbackIcon />
        </a>

        {/* Settings */}
        <Link
          to="/settings"
          title="Settings"
          className="p-1.5 rounded text-brand-muted hover:text-brand-text hover:bg-brand-border/40 transition-colors"
        >
          <SettingsIcon />
        </Link>

        {/* Logout */}
        <button
          onClick={onLogout}
          title="Sign out"
          className="p-1.5 rounded text-brand-muted hover:text-red-400 hover:bg-brand-border/40 transition-colors"
        >
          <LogoutIcon />
        </button>
      </div>
    </header>
  )
}

// ── sub-components ────────────────────────────────────────────────────────────

function ToggleButton({
  active, title, onClick, children,
}: {
  active: boolean; title: string; onClick: () => void; children: React.ReactNode
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

// ── icons ─────────────────────────────────────────────────────────────────────

function ChevronIcon() {
  return (
    <svg className="w-2.5 h-2.5 shrink-0 opacity-40" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M6 3l5 5-5 5" />
    </svg>
  )
}

function PhaseIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="8" cy="8" r="6" />
      <path d="M8 5v3l2 1.5" />
    </svg>
  )
}

function HomeIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2 6.5L8 2l6 4.5V14a1 1 0 01-1 1H3a1 1 0 01-1-1V6.5z" />
      <path d="M6 15V9h4v6" />
    </svg>
  )
}

function WikiIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 2h7l3 3v9a1 1 0 01-1 1H3a1 1 0 01-1-1V3a1 1 0 011-1z" />
      <path d="M10 2v3h3M5 7h6M5 10h4" />
    </svg>
  )
}

function GuideIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="8" cy="8" r="6" />
      <path d="M8 5v3l2 2" />
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

function FocusIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
      <path d="M7 3H3v4M13 3h4v4M7 17H3v-4M13 17h4v-4" />
    </svg>
  )
}

function FeedbackIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="10" cy="10" r="8" />
      <path d="M10 6v5" />
      <circle cx="10" cy="14" r="0.5" fill="currentColor" />
    </svg>
  )
}

function SettingsIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="10" cy="10" r="2.5" />
      <path d="M10 2v1.5M10 16.5V18M2 10h1.5M16.5 10H18M4.22 4.22l1.06 1.06M14.72 14.72l1.06 1.06M4.22 15.78l1.06-1.06M14.72 5.28l1.06-1.06" />
    </svg>
  )
}

function LogoutIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M13 10H3m0 0l3-3m-3 3l3 3" />
      <path d="M8 6V4a1 1 0 011-1h7a1 1 0 011 1v12a1 1 0 01-1 1H9a1 1 0 01-1-1v-2" />
    </svg>
  )
}
