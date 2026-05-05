import { Extension } from '@tiptap/core'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import type { Node as PMNode } from '@tiptap/pm/model'
import type { MentionResponse } from '@/services/api'

export const mentionPluginKey = new PluginKey<DecorationSet>('entityMentions')

// These mirror MentionsBar TYPE_COLORS but as CSS rgb() values for inline styles.
const TYPE_COLORS: Record<string, string> = {
  character: 'rgb(0, 240, 255)',    // brand-cyan
  location:  'rgb(244, 201, 93)',   // brand-gold
  faction:   'rgb(159, 75, 255)',   // brand-purple
  item:      'rgb(52, 211, 153)',   // emerald-400
  concept:   'rgb(56, 189, 248)',   // sky-400
  lore:      'rgb(251, 113, 133)',  // rose-400
}

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function buildDecorations(doc: PMNode, mentions: MentionResponse[]): DecorationSet {
  if (!mentions.length) return DecorationSet.empty

  const decorations: Decoration[] = []

  for (const mention of mentions) {
    if (!mention.match_text) continue
    const color = TYPE_COLORS[mention.entity_type] ?? 'rgb(156, 163, 175)'
    const re = new RegExp(`\\b${escapeRegex(mention.match_text)}\\b`, 'gi')

    doc.descendants((node, pos) => {
      if (!node.isText || !node.text) return true
      re.lastIndex = 0
      let m: RegExpExecArray | null
      while ((m = re.exec(node.text)) !== null) {
        decorations.push(
          Decoration.inline(pos + m.index, pos + m.index + m[0].length, {
            class: 'entity-mention',
            style: `text-decoration: underline dotted ${color}; text-underline-offset: 3px; cursor: pointer;`,
            'data-mention-id':   mention.id,
            'data-entity-id':    mention.entity_id,
            'data-entity-name':  mention.entity_name,
            'data-entity-type':  mention.entity_type,
          }),
        )
      }
      return true
    })
  }

  return DecorationSet.create(doc, decorations)
}

// Dispatch a transaction with setMeta(mentionPluginKey, mentions) to update highlights.
export const EntityMentionExtension = Extension.create({
  name: 'entityMention',

  addProseMirrorPlugins() {
    return [
      new Plugin<DecorationSet>({
        key: mentionPluginKey,
        state: {
          init: (_, { doc }) => buildDecorations(doc, []),
          apply: (tr, old, _, newState) => {
            const next = tr.getMeta(mentionPluginKey) as MentionResponse[] | undefined
            if (next !== undefined) return buildDecorations(newState.doc, next)
            if (tr.docChanged) return old.map(tr.mapping, newState.doc)
            return old
          },
        },
        props: {
          decorations: (state) => mentionPluginKey.getState(state),
        },
      }),
    ]
  },
})
