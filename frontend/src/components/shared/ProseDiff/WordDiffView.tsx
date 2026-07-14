import { useMemo } from 'react'
import DiffMatchPatch from 'diff-match-patch'

const dmp = new DiffMatchPatch()

export function WordDiffView({ canon, coauthor }: { canon: string; coauthor: string }) {
  const diffs = useMemo(() => {
    const d = dmp.diff_main(canon, coauthor)
    dmp.diff_cleanupSemantic(d)
    return d
  }, [canon, coauthor])

  return (
    <div className="text-sm text-brand-text leading-relaxed font-serif whitespace-pre-wrap">
      {diffs.map(([op, text], i) => {
        if (op === 0)  return <span key={i}>{text}</span>
        if (op === -1) return <span key={i} className="bg-red-500/15 text-red-300 line-through decoration-red-400/50">{text}</span>
        return              <span key={i} className="bg-green-500/15 text-green-300">{text}</span>
      })}
    </div>
  )
}
