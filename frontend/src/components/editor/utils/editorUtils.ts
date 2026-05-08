import type { Editor } from '@tiptap/react'
import type { Node as PMNode } from '@tiptap/pm/model'

// Converts plain text (textarea-style) → TipTap HTML.
// '\n\n' → paragraph break; '\n' → <br> within a paragraph.
export function plainToHTML(text: string): string {
  if (!text) return '<p></p>'
  return text
    .split('\n\n')
    .map(block =>
      `<p>${block
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/\n/g, '<br>')
      }</p>`,
    )
    .join('')
}

// Inverse of plainToHTML — get the full plain-text value from TipTap.
// Serialises editor content to plain text that round-trips through plainToHTML.
// '\n\n' between block nodes (paragraph break), '\n' for hardBreak (line break).
// The blockSeparator MUST match the paragraph delimiter in plainToHTML so that
// editorGetText(editor) === content whenever the editor was initialised from
// plainToHTML(content) — otherwise the content sync guard always fails and
// setContent() fires on every render, potentially clobbering in-progress edits.
export function editorGetText(editor: Editor): string {
  return editor.getText({
    blockSeparator: '\n\n',
    textSerializers: {
      hardBreak: () => '\n',
    },
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
