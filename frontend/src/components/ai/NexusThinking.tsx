// NexusThinking — cycling sci-fi/fantasy annotations shown while Nexus processes.
// Appears in chat bubbles and beat-input when the AI is working but no tokens
// have arrived yet, replacing the generic blinking-cursor empty state.
import { useState, useEffect, useRef } from 'react'

const GENERAL_PHRASES = [
  "Scanning the narrative timestream…",
  "Consulting the arcane codex…",
  "Attuning to story frequencies…",
  "Tracing the threads of your tale…",
  "Cross-referencing the chronicles…",
  "Reading the ley lines of your plot…",
  "Decoding character resonance…",
  "Divining the next story beat…",
  "Aligning the constellation of events…",
  "Channeling the word-forge…",
  "Parsing the deep narrative structure…",
  "Summoning context from the aether…",
  "Mapping the web of consequences…",
  "Calibrating prose harmonics…",
  "Communing with your story's spirit…",
  "Seeking signal through the static…",
  "Unlocking the hidden plot chamber…",
  "Triangulating the arc of fate…",
]

const AGENT_PHRASES = [
  "Architecting the manuscript…",
  "Forging new chapters in the crucible…",
  "Inscribing to the grand chronicle…",
  "Weaving worlds from raw aether…",
  "Drafting at the speed of thought…",
  "Etching new lore into the ledger…",
  "Compiling the narrative matrix…",
  "Laying the structural foundations…",
  "Assembling the story scaffolding…",
  "Transmuting ideas into prose…",
]

interface NexusThinkingProps {
  /** Use more action-oriented phrases for agent/write mode. */
  agentMode?: boolean
  className?: string
}

export default function NexusThinking({ agentMode = false, className }: NexusThinkingProps) {
  const phrases  = agentMode ? AGENT_PHRASES : GENERAL_PHRASES
  // Start at a random index so repeated invocations don't always show the same phrase.
  const [idx, setIdx]       = useState(() => Math.floor(Math.random() * phrases.length))
  const [visible, setVisible] = useState(true)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    // Cycle: visible for 2.2 s → fade out 0.3 s → advance → fade in.
    const cycle = () => {
      setVisible(false)
      timerRef.current = setTimeout(() => {
        setIdx((i) => (i + 1) % phrases.length)
        setVisible(true)
        timerRef.current = setTimeout(cycle, 2200)
      }, 300)
    }
    timerRef.current = setTimeout(cycle, 2200)
    return () => { if (timerRef.current) clearTimeout(timerRef.current) }
  }, [phrases.length])

  return (
    <span
      className={`inline-flex items-center gap-1 text-[11px] italic transition-opacity duration-300 ${
        visible ? 'opacity-100' : 'opacity-0'
      } ${className ?? 'text-brand-purple/60'}`}
    >
      <OrbIcon />
      {phrases[idx]}
    </span>
  )
}

function OrbIcon() {
  return (
    <svg className="w-2.5 h-2.5 shrink-0 animate-pulse" viewBox="0 0 10 10" fill="currentColor">
      <circle cx="5" cy="5" r="2.5" opacity="0.7" />
      <circle cx="5" cy="5" r="4" fill="none" stroke="currentColor" strokeWidth="0.8" opacity="0.4" />
    </svg>
  )
}
