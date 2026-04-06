// StatusBar — bottom strip: word count, location, save status.
interface StatusBarProps {
  wordCount: number
  chapterTitle: string
  sceneTitle: string
}

export default function StatusBar({ wordCount, chapterTitle, sceneTitle }: StatusBarProps) {
  return (
    <footer className="h-7 flex items-center justify-between px-4 bg-brand-bg-card border-t border-brand-border shrink-0 select-none text-xs text-brand-muted">

      {/* Left: location */}
      <div className="flex items-center gap-2">
        {chapterTitle && (
          <>
            <span>{chapterTitle}</span>
            {sceneTitle && (
              <>
                <span className="opacity-40">›</span>
                <span className="text-brand-text/70">{sceneTitle}</span>
              </>
            )}
          </>
        )}
      </div>

      {/* Right: word count + saved indicator */}
      <div className="flex items-center gap-4">
        <span>{wordCount.toLocaleString()} {wordCount === 1 ? 'word' : 'words'}</span>
        <span className="flex items-center gap-1 text-green-400/70">
          <svg className="w-2.5 h-2.5" viewBox="0 0 10 10" fill="currentColor">
            <circle cx="5" cy="5" r="5" />
          </svg>
          Saved
        </span>
      </div>
    </footer>
  )
}
