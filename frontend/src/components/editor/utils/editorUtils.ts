import type { Editor } from '@tiptap/react'
import type { Node as PMNode } from '@tiptap/pm/model'

// Converts Markdown (or plain text) → TipTap HTML.
// '\n\n' → paragraph break; '\n' → <br>.
// Inline: ***text*** → bold+italic, **text** → bold, *text* → italic, `text` → code.
// Plain text is valid Markdown (no markers), so old stored content loads unchanged.
export function markdownToHTML(text: string): string {
  if (!text) return '<p></p>'
  return text
    .split('\n\n')
    .map(block =>
      `<p>${block
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/\*\*\*(.+?)\*\*\*/gs, '<strong><em>$1</em></strong>')
        .replace(/\*\*(.+?)\*\*/gs, '<strong>$1</strong>')
        .replace(/\*(.+?)\*/gs, '<em>$1</em>')
        .replace(/`(.+?)`/gs, '<code>$1</code>')
        .replace(/\n/g, '<br>')
      }</p>`,
    )
    .join('')
}

// Keep plainToHTML as an alias so other callers (if any) aren't broken.
export const plainToHTML = markdownToHTML

// Serialise a TipTap editor document to a Markdown string.
// Paragraphs separated by '\n\n'; hard breaks become '\n'.
// Bold → **text**, Italic → *text*, Bold+Italic → ***text***, Code → `text`.
// Round-trips correctly with markdownToHTML — the sync guard in ScribeEditor
// relies on editorGetMarkdown(editor) === content after setContent(markdownToHTML(content)).
export function editorGetMarkdown(editor: Editor): string {
  const blocks: string[] = []
  editor.state.doc.forEach(blockNode => {
    const parts: string[] = []
    blockNode.forEach(inlineNode => {
      if (inlineNode.type.name === 'hardBreak') {
        parts.push('\n')
        return
      }
      const raw      = inlineNode.text ?? ''
      const hasBold   = inlineNode.marks.some(m => m.type.name === 'bold')
      const hasItalic = inlineNode.marks.some(m => m.type.name === 'italic')
      const hasCode   = inlineNode.marks.some(m => m.type.name === 'code')
      if (hasCode)                   parts.push(`\`${raw}\``)
      else if (hasBold && hasItalic) parts.push(`***${raw}***`)
      else if (hasBold)              parts.push(`**${raw}**`)
      else if (hasItalic)            parts.push(`*${raw}*`)
      else                           parts.push(raw)
    })
    blocks.push(parts.join(''))
  })
  return blocks.join('\n\n')
}

// editorGetText kept for any external callers; prefer editorGetMarkdown in ScribeEditor.
export function editorGetText(editor: Editor): string {
  return editor.getText({
    blockSeparator: '\n\n',
    textSerializers: { hardBreak: () => '\n' },
  })
}

// Returns a function that maps plain-text char offset → ProseMirror position.
// '\n\n' between paragraphs consumes 2 plain-text chars with no PM equivalent.
// Used by jumpToAnnotation to convert stored plain-text offsets → editor selection.
export function buildCharToPos(doc: PMNode): (charOffset: number) => number {
  const map = new Map<number, number>()
  let charOffset = 0
  let firstBlock = true

  doc.forEach((blockNode, blockOffset) => {
    if (!firstBlock) charOffset += 2   // skip the \n\n between paragraphs
    firstBlock = false

    blockNode.forEach((inlineNode, inlineOffset) => {
      if (inlineNode.isText && inlineNode.text) {
        for (let i = 0; i < inlineNode.text.length; i++) {
          map.set(charOffset++, blockOffset + 1 + inlineOffset + i)
        }
      } else if (inlineNode.type.name === 'hardBreak') {
        map.set(charOffset++, blockOffset + 1 + inlineOffset)
      }
    })
  })

  return (offset: number) => map.get(offset) ?? doc.content.size - 1
}

// Inverse of buildCharToPos: ProseMirror position → plain-text char offset.
// Used when creating annotations to convert the TipTap selection back to
// the plain-text offsets stored on the server.
export function buildPosToChar(doc: PMNode): (pmPos: number) => number {
  const map = new Map<number, number>()
  let charOffset = 0
  let firstBlock = true

  doc.forEach((blockNode, blockOffset) => {
    if (!firstBlock) charOffset += 2
    firstBlock = false

    blockNode.forEach((inlineNode, inlineOffset) => {
      if (inlineNode.isText && inlineNode.text) {
        for (let i = 0; i < inlineNode.text.length; i++) {
          map.set(blockOffset + 1 + inlineOffset + i, charOffset++)
        }
      } else if (inlineNode.type.name === 'hardBreak') {
        map.set(blockOffset + 1 + inlineOffset, charOffset++)
      }
    })
  })

  return (pos: number) => map.get(pos) ?? 0
}
