// Hand-written fetch layer. All domain types are imported from the generated
// api-types.ts (produced by `npm run gen:api` from docs/openapi.yaml).
// Do not define response shapes here — update the spec and regenerate instead.
import type { components } from './api-types'

const BASE = '/api/v1'

// ── Re-export generated types for consumers ───────────────────────────────────

export type User                   = components['schemas']['UserResponse']
export type TokenPair              = components['schemas']['TokenPair']
export type AuthResponse           = components['schemas']['AuthResponse']
export type Project                = components['schemas']['ProjectResponse']
export type Act                    = components['schemas']['ActResponse']
export type Chapter                = components['schemas']['ChapterResponse']
export type Scene                  = components['schemas']['SceneResponse']
export type ChronicleEntry         = components['schemas']['ChronicleEntry']
export type NothingToChronicle     = components['schemas']['NothingToChronicle']
export type GitStatusResponse      = components['schemas']['GitStatusResponse']
export type TimelineInfo           = components['schemas']['TimelineInfo']
export type EchoResponse           = components['schemas']['EchoResponse']
export type CanonizeResult         = components['schemas']['CanonizeResult']
export type EntityType             = components['schemas']['EntityType']
export type WikiEntity             = components['schemas']['EntityResponse']
export type WikiRelationship       = components['schemas']['RelationshipResponse']
export type WikiGraph              = components['schemas']['WikiGraphResponse']
export type MagicRule              = components['schemas']['MagicRuleResponse']
export type WikiTimelineEvent      = components['schemas']['TimelineEventResponse']
export type AutolinkMatch          = components['schemas']['AutolinkMatch']
export type APIKeyResponse         = components['schemas']['APIKeyResponse']
export type ProjectStats           = components['schemas']['ProjectStats']

// ── Inline types (not yet in OpenAPI spec) ────────────────────────────────────

export interface AIUsageSummary {
  total_tokens:     number
  total_cost_usd:   number
  monthly_tokens:   number
  monthly_cost_usd: number
  calls_this_month: number
}

export interface ChapterSummaryResponse {
  chapter_id:  string
  branch_name: string
  ai_summary:  string
  stale:       boolean
}

export interface GuideStep {
  step_key:     string
  label:        string
  data:         Record<string, unknown>
  is_complete:  boolean
  completed_at?: string
}

export interface GuideProgress {
  steps:           GuideStep[]
  completed_count: number
  total_count:     number
}

export type NovelStructure      = components['schemas']['NovelStructureResponse']
export type StructureScore      = components['schemas']['StructureScoreEntry']
export type ProjectStructure    = components['schemas']['ProjectStructureResponse']

export interface ExportJob {
  id:           string
  project_id:   string
  format:       'markdown' | 'epub'
  status:       'pending' | 'processing' | 'done' | 'failed'
  download_url?: string  // presigned URL; only when status=done
  error_msg?:   string   // only when status=failed
  expires_at?:  string   // ISO-8601; only when status=done
  created_at:   string
}

export interface PromptResponse {
  id:             string
  project_id:     string
  name:           string
  category:       string       // 'prose' | 'workshop'
  content:        string       // style instruction appended to user turn
  system_content: string       // replaces default system prompt when non-empty
  sort_order:     number
  created_at:     string
  updated_at:     string
}

// ── Error class ───────────────────────────────────────────────────────────────

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

// ── Base fetch helper ─────────────────────────────────────────────────────────

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  token?: string,
  extraHeaders?: Record<string, string>,
): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers['Authorization'] = `Bearer ${token}`
  if (extraHeaders) Object.assign(headers, extraHeaders)

  const res = await fetch(`${BASE}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  if (!res.ok) {
    let message = res.statusText
    try {
      const data = await res.json()
      message = data.message ?? data.detail ?? message
    } catch {}
    throw new ApiError(res.status, message)
  }

  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return undefined as T
  }

  return res.json() as Promise<T>
}

// ── API client ────────────────────────────────────────────────────────────────

export const api = {
  auth: {
    register: (email: string, displayName: string, password: string) =>
      request<AuthResponse>('POST', '/auth/register', {
        email,
        display_name: displayName,
        password,
      }),

    login: (email: string, password: string) =>
      request<AuthResponse>('POST', '/auth/login', { email, password }),

    logout: (refreshToken: string, accessToken: string) =>
      request<void>('POST', '/auth/logout', { refresh_token: refreshToken }, accessToken),

    refresh: (refreshToken: string) =>
      request<TokenPair>('POST', '/auth/refresh', { refresh_token: refreshToken }),
  },

  projects: {
    list: (token: string) =>
      request<Project[]>('GET', '/projects', undefined, token),

    get: (token: string, id: string) =>
      request<Project>('GET', `/projects/${id}`, undefined, token),

    create: (token: string, title: string, description: string, genres: string[]) =>
      request<Project>('POST', '/projects', { title, description, genres }, token),

    stats: (token: string, id: string) =>
      request<ProjectStats>('GET', `/projects/${id}/stats`, undefined, token),
  },

  acts: {
    list: (token: string, projectId: string) =>
      request<Act[]>('GET', `/projects/${projectId}/acts`, undefined, token),

    create: (token: string, projectId: string, title: string, summary = '', sortOrder = 0) =>
      request<Act>('POST', `/projects/${projectId}/acts`, { title, summary, sort_order: sortOrder }, token),

    update: (token: string, projectId: string, actId: string, data: { title?: string; summary?: string; sort_order?: number }) =>
      request<Act>('PATCH', `/projects/${projectId}/acts/${actId}`, data, token),

    delete: (token: string, projectId: string, actId: string) =>
      request<void>('DELETE', `/projects/${projectId}/acts/${actId}`, undefined, token),
  },

  chapters: {
    list: (token: string, projectId: string, actId: string) =>
      request<Chapter[]>('GET', `/projects/${projectId}/acts/${actId}/chapters`, undefined, token),

    create: (token: string, projectId: string, actId: string, title: string, sortOrder: number) =>
      request<Chapter>('POST', `/projects/${projectId}/acts/${actId}/chapters`, { title, sort_order: sortOrder }, token),
  },

  scenes: {
    list: (token: string, chapterId: string) =>
      request<Scene[]>('GET', `/chapters/${chapterId}/scenes`, undefined, token),

    create: (token: string, chapterId: string, title: string, sortOrder: number) =>
      request<Scene>('POST', `/chapters/${chapterId}/scenes`, { title, sort_order: sortOrder }, token),

    update: (token: string, chapterId: string, sceneId: string, data: {
      content?: string
      title?: string
      pov?: string
      tense?: string
      tags?: string[]
      summary?: string
      summary_stale?: boolean
      sort_order?: number
    }, branch?: string) =>
      request<Scene>('PATCH', `/chapters/${chapterId}/scenes/${sceneId}`, data, token,
        branch ? { 'X-NexusTale-Branch': branch } : undefined),
  },

  git: {
    status: (token: string, projectId: string) =>
      request<GitStatusResponse>('GET', `/projects/${projectId}/git/status`, undefined, token),

    chronicle: (token: string, projectId: string, note: string) =>
      request<ChronicleEntry | NothingToChronicle>('POST', `/projects/${projectId}/git/chronicle`, { note }, token),

    lore: (token: string, projectId: string, page = 1, perPage = 20) =>
      request<ChronicleEntry[]>('GET', `/projects/${projectId}/git/lore?page=${page}&per_page=${perPage}`, undefined, token),

    echo: (token: string, projectId: string, from: string, to: string) =>
      request<EchoResponse>('GET', `/projects/${projectId}/git/echo?from=${from}&to=${to}`, undefined, token),

    timelines: (token: string, projectId: string) =>
      request<TimelineInfo[]>('GET', `/projects/${projectId}/git/timelines`, undefined, token),

    diverge: (token: string, projectId: string, timelineName: string, fromSha?: string) =>
      request<TimelineInfo[]>('POST', `/projects/${projectId}/git/timelines`, { timeline_name: timelineName, from_sha: fromSha }, token),

    travel: (token: string, projectId: string, timelineName: string) =>
      request<void>('POST', `/projects/${projectId}/git/timelines/${encodeURIComponent(timelineName)}/travel`, {}, token),

    canonize: (token: string, projectId: string, timelineName: string) =>
      request<CanonizeResult>('POST', `/projects/${projectId}/git/timelines/${encodeURIComponent(timelineName)}/canonize`, {}, token),
  },

  wiki: {
    listEntities: (token: string, projectId: string, type?: EntityType) => {
      const qs = type ? `?type=${type}` : ''
      return request<WikiEntity[]>('GET', `/projects/${projectId}/wiki/entities${qs}`, undefined, token)
    },

    getEntity: (token: string, projectId: string, entityId: string) =>
      request<WikiEntity>('GET', `/projects/${projectId}/wiki/entities/${entityId}`, undefined, token),

    createEntity: (token: string, projectId: string, data: { type: EntityType; name: string; summary?: string; attributes?: Record<string, string>; parent_entity_id?: string }) =>
      request<WikiEntity>('POST', `/projects/${projectId}/wiki/entities`, data, token),

    updateEntity: (token: string, projectId: string, entityId: string, data: { name?: string; summary?: string; attributes?: Record<string, string> }) =>
      request<WikiEntity>('PATCH', `/projects/${projectId}/wiki/entities/${entityId}`, data, token),

    deleteEntity: (token: string, projectId: string, entityId: string) =>
      request<void>('DELETE', `/projects/${projectId}/wiki/entities/${entityId}`, undefined, token),

    // Relationships — field names match the backend: from_entity_id / to_entity_id / type
    listRelationships: (token: string, projectId: string) =>
      request<WikiRelationship[]>('GET', `/projects/${projectId}/wiki/relationships`, undefined, token),

    createRelationship: (token: string, projectId: string, fromEntityId: string, toEntityId: string, type: string, description?: string) =>
      request<WikiRelationship>('POST', `/projects/${projectId}/wiki/relationships`, {
        from_entity_id: fromEntityId,
        to_entity_id:   toEntityId,
        type,
        description,
      }, token),

    deleteRelationship: (token: string, projectId: string, relationshipId: string) =>
      request<void>('DELETE', `/projects/${projectId}/wiki/relationships/${relationshipId}`, undefined, token),

    getGraph: (token: string, projectId: string) =>
      request<WikiGraph>('GET', `/projects/${projectId}/wiki/graph`, undefined, token),

    listMagicRules: (token: string, projectId: string) =>
      request<MagicRule[]>('GET', `/projects/${projectId}/wiki/magic-rules`, undefined, token),

    createMagicRule: (token: string, projectId: string, data: { name: string; description?: string }) =>
      request<MagicRule>('POST', `/projects/${projectId}/wiki/magic-rules`, data, token),

    updateMagicRule: (token: string, projectId: string, ruleId: string, data: { name?: string; description?: string }) =>
      request<MagicRule>('PATCH', `/projects/${projectId}/wiki/magic-rules/${ruleId}`, data, token),

    deleteMagicRule: (token: string, projectId: string, ruleId: string) =>
      request<void>('DELETE', `/projects/${projectId}/wiki/magic-rules/${ruleId}`, undefined, token),

    listTimeline: (token: string, projectId: string) =>
      request<WikiTimelineEvent[]>('GET', `/projects/${projectId}/wiki/timeline`, undefined, token),

    createTimelineEvent: (
      token: string,
      projectId: string,
      data: {
        name: string
        description?: string
        era?: string
        year?: number
        month?: number
        day?: number
        anchor_event_id?: string
        anchor_offset_year?: number
        anchor_offset_month?: number
        anchor_offset_day?: number
      },
    ) => request<WikiTimelineEvent>('POST', `/projects/${projectId}/wiki/timeline`, data, token),

    updateTimelineEvent: (
      token: string,
      projectId: string,
      eventId: string,
      data: {
        name?: string
        description?: string
        era?: string
        year?: number | null
        month?: number | null
        day?: number | null
      },
    ) => request<WikiTimelineEvent>('PATCH', `/projects/${projectId}/wiki/timeline/${eventId}`, data, token),

    deleteTimelineEvent: (token: string, projectId: string, eventId: string) =>
      request<void>('DELETE', `/projects/${projectId}/wiki/timeline/${eventId}`, undefined, token),

    autolink: (token: string, projectId: string, text: string) =>
      request<AutolinkMatch[]>('GET', `/projects/${projectId}/wiki/autolink?text=${encodeURIComponent(text)}`, undefined, token),
  },

  prompts: {
    list: (token: string, projectId: string) =>
      request<PromptResponse[]>('GET', `/projects/${projectId}/prompts`, undefined, token),

    create: (token: string, projectId: string, data: { name: string; category?: string; content?: string; system_content?: string; sort_order?: number }) =>
      request<PromptResponse>('POST', `/projects/${projectId}/prompts`, data, token),

    update: (token: string, projectId: string, promptId: string, data: { name?: string; category?: string; content?: string; system_content?: string; sort_order?: number }) =>
      request<PromptResponse>('PUT', `/projects/${projectId}/prompts/${promptId}`, data, token),

    delete: (token: string, projectId: string, promptId: string) =>
      request<void>('DELETE', `/projects/${projectId}/prompts/${promptId}`, undefined, token),
  },

  ai: {
    usage: (token: string, projectId: string) =>
      request<AIUsageSummary>('GET', `/projects/${projectId}/ai/usage`, undefined, token),

    /**
     * Stream a scene completion from POST /projects/:id/ai/complete.
     * Supports "continue" and "beat" modes.
     * Calls onDelta for each text chunk; resolves when [DONE] is received.
     */
    streamComplete: async (
      token: string,
      projectId: string,
      params: {
        sceneId?: string
        mode: 'continue' | 'beat'
        beat?: string
        instruction?: string
        promptId?: string
        branch?: string
      },
      onDelta: (text: string) => void,
      signal?: AbortSignal,
    ): Promise<void> => {
      const branchHeaders: Record<string, string> = params.branch
        ? { 'X-NexusTale-Branch': params.branch }
        : {}
      const res = await fetch(`${BASE}/projects/${projectId}/ai/complete`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
          ...branchHeaders,
        },
        body: JSON.stringify({
          scene_id:    params.sceneId ?? '',
          mode:        params.mode,
          beat:        params.beat ?? '',
          instruction: params.instruction ?? '',
          prompt_id:   params.promptId ?? '',
        }),
        signal,
      })

      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error((data as { message?: string }).message ?? `AI error ${res.status}`)
      }

      const reader = res.body?.getReader()
      if (!reader) throw new Error('No response body')

      const decoder = new TextDecoder()
      let buf = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buf += decoder.decode(value, { stream: true })
        const lines = buf.split('\n')
        buf = lines.pop() ?? ''
        for (const line of lines) {
          if (!line.startsWith('data: ')) continue
          const payload = line.slice(6)
          if (payload === '[DONE]') return
          try {
            const evt = JSON.parse(payload) as { delta?: string; error?: string }
            if (evt.error) throw new Error(evt.error)
            if (evt.delta) onDelta(evt.delta)
          } catch {
            // skip malformed chunks
          }
        }
      }
    },

    /**
     * Stream a chat response from POST /projects/:id/ai/chat.
     * Calls onDelta for each text chunk; resolves when [DONE] is received.
     * Rejects on network error or if the server returns a non-2xx status.
     */
    streamChat: async (
      token: string,
      projectId: string,
      messages: { role: string; content: string }[],
      sceneId: string | undefined,
      onDelta: (text: string) => void,
      signal?: AbortSignal,
      branch?: string,
    ): Promise<void> => {
      const branchHeaders: Record<string, string> = branch
        ? { 'X-NexusTale-Branch': branch }
        : {}
      const res = await fetch(`${BASE}/projects/${projectId}/ai/chat`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
          ...branchHeaders,
        },
        body: JSON.stringify({ messages, scene_id: sceneId ?? '' }),
        signal,
      })

      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error((data as { message?: string }).message ?? `AI error ${res.status}`)
      }

      const reader = res.body?.getReader()
      if (!reader) throw new Error('No response body')

      const decoder = new TextDecoder()
      let buf = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buf += decoder.decode(value, { stream: true })
        const lines = buf.split('\n')
        buf = lines.pop() ?? ''

        for (const line of lines) {
          if (!line.startsWith('data: ')) continue
          const payload = line.slice(6)
          if (payload === '[DONE]') return
          try {
            const evt = JSON.parse(payload) as { delta?: string; error?: string }
            if (evt.error) throw new Error(evt.error)
            if (evt.delta) onDelta(evt.delta)
          } catch {
            // skip malformed chunks
          }
        }
      }
    },
  },

  chapterSummaries: {
    get: (token: string, projectId: string, chapterId: string, branch?: string) =>
      request<ChapterSummaryResponse>(
        'GET', `/projects/${projectId}/chapters/${chapterId}/summary`, undefined, token,
        branch ? { 'X-NexusTale-Branch': branch } : undefined,
      ),

    regenerate: (token: string, projectId: string, chapterId: string, branch?: string) =>
      request<ChapterSummaryResponse>(
        'POST', `/projects/${projectId}/chapters/${chapterId}/summarize`, undefined, token,
        branch ? { 'X-NexusTale-Branch': branch } : undefined,
      ),
  },

  guide: {
    getProgress: (token: string, projectId: string): Promise<GuideProgress> =>
      request<GuideProgress>('GET', `/projects/${projectId}/guide`, undefined, token),

    saveStep: (token: string, projectId: string, step: string, data: Record<string, unknown>): Promise<GuideStep> =>
      request<GuideStep>('POST', `/projects/${projectId}/guide/${step}`, data, token),

    completeStep: (token: string, projectId: string, step: string, data: Record<string, unknown>): Promise<GuideStep> =>
      request<GuideStep>('POST', `/projects/${projectId}/guide/${step}/complete`, data, token),
  },

  structures: {
    /** Public catalog — no auth required. */
    list: (): Promise<NovelStructure[]> =>
      request<NovelStructure[]>('GET', '/novel-structures'),

    /** Run the scoring matrix. Returns ranked suggestions without persisting. */
    score: (token: string, projectId: string, answers: Record<string, string[]>): Promise<{ ranked: StructureScore[] }> =>
      request<{ ranked: StructureScore[] }>('POST', `/projects/${projectId}/guide/structure/score`, { answers }, token),

    /** Get the current structure selection for a project. */
    get: (token: string, projectId: string): Promise<ProjectStructure> =>
      request<ProjectStructure>('GET', `/projects/${projectId}/structure`, undefined, token),

    /** Set or clear the structure selection. Pass structure_id: null to clear. */
    update: (token: string, projectId: string, body: { structure_id?: string | null; structure_custom?: Record<string, unknown> | null }): Promise<ProjectStructure> =>
      request<ProjectStructure>('PUT', `/projects/${projectId}/structure`, body, token),
  },

  export: {
    /**
     * Trigger a markdown export — streams a zip and triggers browser download.
     * Uses raw fetch because the response is binary, not JSON.
     */
    downloadMarkdown: async (token: string, projectId: string, filename: string): Promise<void> => {
      const res = await fetch(`${BASE}/projects/${projectId}/export`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ format: 'markdown' }),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new ApiError(res.status, (data as { message?: string }).message ?? `Export failed ${res.status}`)
      }
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
    },

    /** Start an async EPUB export — returns the job ID immediately (HTTP 202). */
    startEpub: (token: string, projectId: string): Promise<{ job_id: string }> =>
      request<{ job_id: string }>('POST', `/projects/${projectId}/export`, { format: 'epub' }, token),

    /** Poll a single export job for status + presigned download URL. */
    getJob: (token: string, projectId: string, jobId: string): Promise<ExportJob> =>
      request<ExportJob>('GET', `/projects/${projectId}/export/${jobId}`, undefined, token),

    /** List the 20 most recent export jobs for the project. */
    listJobs: (token: string, projectId: string): Promise<{ jobs: ExportJob[] }> =>
      request<{ jobs: ExportJob[] }>('GET', `/projects/${projectId}/export`, undefined, token),
  },

  users: {
    me: (token: string) =>
      request<User>('GET', '/users/me', undefined, token),

    deleteMe: (token: string) =>
      request<void>('DELETE', '/users/me', undefined, token),
  },

  apiKeys: {
    list: (token: string) =>
      request<APIKeyResponse[]>('GET', '/users/me/api-keys', undefined, token),

    upsert: (token: string, provider: string, key: string) =>
      request<APIKeyResponse>('POST', '/users/me/api-keys', { provider, key }, token),

    delete: (token: string, provider: string) =>
      request<void>('DELETE', `/users/me/api-keys/${encodeURIComponent(provider)}`, undefined, token),
  },
}
