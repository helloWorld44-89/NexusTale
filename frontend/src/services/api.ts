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
): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers['Authorization'] = `Bearer ${token}`

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
  },

  chapters: {
    list: (token: string, projectId: string) =>
      request<Chapter[]>('GET', `/projects/${projectId}/chapters`, undefined, token),

    create: (token: string, projectId: string, title: string, sortOrder: number) =>
      request<Chapter>('POST', `/projects/${projectId}/chapters`, { title, sort_order: sortOrder }, token),
  },

  scenes: {
    list: (token: string, projectId: string, chapterId: string) =>
      request<Scene[]>('GET', `/projects/${projectId}/chapters/${chapterId}/scenes`, undefined, token),

    create: (token: string, projectId: string, chapterId: string, title: string, sortOrder: number) =>
      request<Scene>('POST', `/projects/${projectId}/chapters/${chapterId}/scenes`, { title, sort_order: sortOrder }, token),

    update: (token: string, projectId: string, chapterId: string, sceneId: string, data: { content?: string; title?: string }) =>
      request<Scene>('PATCH', `/projects/${projectId}/chapters/${chapterId}/scenes/${sceneId}`, data, token),
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
  },
}
