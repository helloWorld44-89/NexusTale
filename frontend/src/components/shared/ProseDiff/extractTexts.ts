// Reconstructs before (canon) and after (coauthor/patched) text from a unified diff.
export function extractTexts(unifiedDiff: string): { canon: string; coauthor: string } {
  const canonLines: string[] = []
  const coauthorLines: string[] = []

  for (const line of unifiedDiff.split('\n')) {
    if (
      line.startsWith('--- ') || line.startsWith('+++ ') ||
      line.startsWith('@@ ') || line.startsWith('diff ') ||
      line.startsWith('index ') || line.startsWith('new file') ||
      line.startsWith('deleted file') || line.startsWith('\\ No newline')
    ) continue

    if (line.startsWith('-')) {
      canonLines.push(line.slice(1))
    } else if (line.startsWith('+')) {
      coauthorLines.push(line.slice(1))
    } else {
      const content = line.startsWith(' ') ? line.slice(1) : line
      canonLines.push(content)
      coauthorLines.push(content)
    }
  }

  return {
    canon:   canonLines.join('\n').trim(),
    coauthor: coauthorLines.join('\n').trim(),
  }
}
