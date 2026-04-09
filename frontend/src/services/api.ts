const BASE = '/api/v1'

export interface User {
  id: string
  email: string
  display_name: string
  role: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
}

export interface AuthResponse {
  user: User
  tokens: TokenPair
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

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

// ── Domain types ─────────────────────────────────────────────────────────────

export interface Project {
  id: string
  owner_id: string
  title: string
  description: string
  genres: string[]
  archived: boolean
  created_at: string
  updated_at: string
}

export interface Chapter {
  id: string
  project_id: string
  title: string
  summary: string
  sort_order: number
  created_at: string
  updated_at: string
}

export interface Scene {
  id: string
  chapter_id: string
  title: string
  content: string
  pov: string
  tense: string
  tags: string[]
  summary: string
  summary_stale: boolean
  sort_order: number
  created_at: string
  updated_at: string
}

// ── Git / Chronicle types ─────────────────────────────────────────────────────

export interface ChronicleEntry {
  sha: string
  short_sha: string
  note: string
  author: string
  timestamp: string
}

export interface ChronicleResponse {
  sha: string
  short_sha: string
  note: string
  author: string
  timestamp: string
}

export interface NothingToChronicle {
  message: string
  last_chronicle: ChronicleEntry | null
}

export interface GitStatus {
  current_timeline: string
  last_chronicle: ChronicleEntry | null
  dirty: boolean
  modified_paths: string[]
}

export interface Timeline {
  name: string
  is_canon: boolean
  is_active: boolean
  last_chronicle: ChronicleEntry | null
}

export interface EchoResponse {
  from_sha: string
  to_sha: string
  diff: string
}

export interface CanonizeResponse {
  merged_timeline: string
  fast_forwarded: boolean
  head_sha: string
}

// ── Wiki types ────────────────────────────────────────────────────────────────

export type EntityType = 'character' | 'location' | 'faction' | 'item' | 'concept' | 'lore'

export interface WikiEntity {
  id: string
  project_id: string
  parent_entity_id: string | null
  type: EntityType
  name: string
  summary: string
  attributes: Record<string, string>
  created_at: string
  updated_at: string
}

export interface WikiRelationship {
  id: string
  source_id: string
  target_id: string
  label: string
  created_at: string
}

export interface WikiTimelineEvent {
  id: string
  project_id: string
  name: string
  description: string | null
  era: string | null
  year: number | null
  month: number | null
  day: number | null
  anchor_event_id: string | null
  anchor_offset_year: number | null
  anchor_offset_month: number | null
  anchor_offset_day: number | null
  created_at: string
  updated_at: string
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
      request<GitStatus>('GET', `/projects/${projectId}/git/status`, undefined, token),

    chronicle: (token: string, projectId: string, note: string) =>
      request<ChronicleResponse | NothingToChronicle>('POST', `/projects/${projectId}/git/chronicle`, { note }, token),

    lore: (token: string, projectId: string, page = 1, perPage = 20) =>
      request<ChronicleEntry[]>('GET', `/projects/${projectId}/git/lore?page=${page}&per_page=${perPage}`, undefined, token),

    echo: (token: string, projectId: string, from: string, to: string) =>
      request<EchoResponse>('GET', `/projects/${projectId}/git/echo?from=${from}&to=${to}`, undefined, token),

    timelines: (token: string, projectId: string) =>
      request<Timeline[]>('GET', `/projects/${projectId}/git/timelines`, undefined, token),

    diverge: (token: string, projectId: string, timelineName: string, fromSha?: string) =>
      request<Timeline[]>('POST', `/projects/${projectId}/git/timelines`, { timeline_name: timelineName, from_sha: fromSha }, token),

    travel: (token: string, projectId: string, timelineName: string) =>
      request<void>('POST', `/projects/${projectId}/git/timelines/${encodeURIComponent(timelineName)}/travel`, {}, token),

    canonize: (token: string, projectId: string, timelineName: string) =>
      request<CanonizeResponse>('POST', `/projects/${projectId}/git/timelines/${encodeURIComponent(timelineName)}/canonize`, {}, token),
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

    listRelationships: (token: string, projectId: string) =>
      request<WikiRelationship[]>('GET', `/projects/${projectId}/wiki/relationships`, undefined, token),

    createRelationship: (token: string, projectId: string, sourceId: string, targetId: string, label: string) =>
      request<WikiRelationship>('POST', `/projects/${projectId}/wiki/relationships`, { source_id: sourceId, target_id: targetId, label }, token),

    deleteRelationship: (token: string, projectId: string, relationshipId: string) =>
      request<void>('DELETE', `/projects/${projectId}/wiki/relationships/${relationshipId}`, undefined, token),

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
