export function ResBtn({
  children, active, color, onClick, disabled,
}: {
  children: React.ReactNode
  active: boolean
  color: 'green' | 'red' | 'muted' | 'cyan' | 'purple'
  onClick: () => void
  disabled?: boolean
}) {
  const cls: Record<string, string> = {
    green:  active ? 'bg-green-500/20  text-green-400  ring-1 ring-green-400/40' : 'bg-green-500/8  text-green-400/70  hover:bg-green-500/15',
    red:    active ? 'bg-red-500/20    text-red-400    ring-1 ring-red-400/40'   : 'bg-red-500/8    text-red-400/70    hover:bg-red-500/15',
    muted:  active ? 'bg-brand-border/60 text-brand-text ring-1 ring-brand-border' : 'bg-brand-border/30 text-brand-muted hover:bg-brand-border/50',
    cyan:   active ? 'bg-brand-cyan/20 text-brand-cyan  ring-1 ring-brand-cyan/40' : 'bg-brand-cyan/8  text-brand-cyan/70  hover:bg-brand-cyan/15',
    purple: active ? 'bg-brand-purple/20 text-brand-purple ring-1 ring-brand-purple/40' : 'bg-brand-purple/8 text-brand-purple/70 hover:bg-brand-purple/15',
  }
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={`px-3 py-1.5 rounded text-[10px] font-medium transition-colors disabled:opacity-40 ${cls[color]}`}
    >
      {children}
    </button>
  )
}
