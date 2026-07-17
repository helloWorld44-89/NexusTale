// Small hand-drawn SVG glyphs for the map palette toolbar (DOM rendering —
// the canvas itself renders symbols via Konva primitives, see MapStudio.tsx's
// renderSymbolShape, since react-konva can't host arbitrary DOM <svg> nodes).
// Same inline-icon-component convention as WikiHub.tsx's ImageIcon/XIcon/etc.

export function MapIcon({ type, className = 'w-5 h-5' }: { type: string; className?: string }) {
  const common = {
    className,
    viewBox: '0 0 24 24',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 1.5,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
  }

  switch (type) {
    case 'mountain':
      return <svg {...common}><path d="M3 19l6-11 4 6 2-3 6 8H3z" /></svg>
    case 'lake':
      return <svg {...common}><ellipse cx="12" cy="13" rx="8" ry="5" /><path d="M6 9c1.5-2 4-3 6-3s4.5 1 6 3" /></svg>
    case 'river':
      return <svg {...common}><path d="M4 5c3 2 1 5 4 7s1 5 4 7 1-5-1-7-1-5-4-7-3 0-3 0z" /></svg>
    case 'ocean':
      return <svg {...common}><path d="M3 10c2-2 4-2 6 0s4 2 6 0 4-2 6 0" /><path d="M3 15c2-2 4-2 6 0s4 2 6 0 4-2 6 0" /></svg>
    case 'forest':
      return <svg {...common}><path d="M8 15l3-7 3 7H8z" /><path d="M13 17l3-8 3 8h-6z" /><path d="M11 15v4M16 17v3" /></svg>
    case 'desert':
      return <svg {...common}><circle cx="17" cy="6" r="2.5" /><path d="M3 17c2-3 5-3 7 0s5 3 7 0 4-2 4-2" /></svg>
    case 'swamp':
      return <svg {...common}><path d="M4 16c2-1 3 1 5 0s3 1 5 0 3 1 5 0" /><circle cx="8" cy="10" r="0.5" fill="currentColor" /><circle cx="14" cy="9" r="0.5" fill="currentColor" /></svg>
    case 'city':
      return <svg {...common}><rect x="4" y="10" width="4" height="9" /><rect x="10" y="6" width="4" height="13" /><rect x="16" y="12" width="4" height="7" /></svg>
    case 'star':
      return <svg {...common}><path d="M12 3l2.2 6.3H21l-5.3 3.9 2 6.4L12 15.8 6.3 19.6l2-6.4L3 9.3h6.8z" /></svg>
    case 'planet':
      return <svg {...common}><circle cx="12" cy="12" r="5" /><ellipse cx="12" cy="12" rx="9" ry="2.5" /></svg>
    case 'moon':
      return <svg {...common}><path d="M15 4a8 8 0 100 16 8 8 0 01-6-16z" /></svg>
    case 'nebula':
      return <svg {...common}><circle cx="9" cy="10" r="4" /><circle cx="15" cy="9" r="3" /><circle cx="13" cy="14" r="4" /></svg>
    case 'asteroid_field':
      return <svg {...common}><circle cx="6" cy="8" r="1.5" /><circle cx="12" cy="14" r="2" /><circle cx="18" cy="7" r="1" /><circle cx="16" cy="17" r="1.3" /><circle cx="9" cy="17" r="1" /></svg>
    case 'space_station':
      return <svg {...common}><circle cx="12" cy="12" r="3" /><path d="M12 3v4M12 17v4M3 12h4M17 12h4" /></svg>
    case 'wormhole':
      return <svg {...common}><circle cx="12" cy="12" r="3" /><circle cx="12" cy="12" r="6" opacity="0.6" /><circle cx="12" cy="12" r="9" opacity="0.3" /></svg>
    case 'black_hole':
      return <svg {...common}><circle cx="12" cy="12" r="3" fill="currentColor" /><ellipse cx="12" cy="12" rx="9" ry="3" /></svg>
    default:
      return <svg {...common}><circle cx="12" cy="12" r="6" /></svg>
  }
}
