import { create } from 'zustand'
import { api, type Project } from '@/services/api'

interface ProjectState {
  projects: Project[]
  loading: boolean
  error: string | null

  fetch: (token: string) => Promise<void>
  create: (token: string, title: string, description: string, genres: string[]) => Promise<Project>
}

export const useProjectStore = create<ProjectState>()((set) => ({
  projects: [],
  loading: false,
  error: null,

  fetch: async (token) => {
    set({ loading: true, error: null })
    try {
      const projects = await api.projects.list(token)
      set({ projects: projects ?? [], loading: false })
    } catch (e) {
      set({ loading: false, error: e instanceof Error ? e.message : 'Failed to load projects' })
    }
  },

  create: async (token, title, description, genres) => {
    const project = await api.projects.create(token, title, description, genres)
    set((s) => ({ projects: [project, ...s.projects] }))
    return project
  },
}))
