// Editor.tsx — main writing shell, wired to real API.
import { useState, useCallback, useEffect, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'
import { api } from '@/services/api'
import type { Chapter, Scene } from '@/services/api'
import TopBar from '@/components/layout/TopBar'
import StatusBar from '@/components/layout/StatusBar'
import ChatBar from '@/components/ai/ChatBar'
import GitPanel from '@/components/editor/GitPanel'
import WikiPanel from '@/components/wiki/WikiPanel'
import ScribeEditor from '@/components/editor/ScribeEditor'
import ProjectExplorer from '@/components/project/ProjectExplorer'
import ActivityBar from '@/components/layout/ActivityBar'

export type LeftPanel = 'chat' | 'git' | 'wiki' | 'none'

// Chapter augmented with its scenes for the explorer tree.
interface ChapterWithScenes extends Chapter {
  scenes: Scene[]
}

const AUTOSAVE_MS = 1500

export default function Editor() {
  const { id: projectId } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const accessToken = useAuthStore((s) => s.accessToken)

  const [projectTitle,       setProjectTitle]       = useState('')
  const [chapters,           setChapters]           = useState<ChapterWithScenes[]>([])
  const [selectedChapterId,  setSelectedChapterId]  = useState<string | null>(null)
  const [selectedSceneId,    setSelectedSceneId]    = useState<string | null>(null)
  const [sceneContents,      setSceneContents]      = useState<Record<string, string>>({})
  const [sceneToChapter,     setSceneToChapter]     = useState<Record<string, string>>({})
  const [loading,            setLoading]            = useState(true)
  const [leftPanel,          setLeftPanel]          = useState<LeftPanel>('chat')
  const [explorerOpen,       setExplorerOpen]       = useState(true)

  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // ── Load project data ───────────────────────────────────────────────────────

  useEffect(() => {
    if (!projectId || !accessToken) return

    let cancelled = false

    const load = async () => {
      try {
        // Load project + chapters in parallel.
        const [project, rawChapters] = await Promise.all([
          api.projects.get(accessToken, projectId),
          api.chapters.list(accessToken, projectId),
        ])

        if (cancelled) return

        // Sort chapters by sort_order, then load all their scenes in parallel.
        const sorted = [...rawChapters].sort((a, b) => a.sort_order - b.sort_order)
        const sceneLists = await Promise.all(
          sorted.map((ch) => api.scenes.list(accessToken, projectId, ch.id))
        )

        if (cancelled) return

        const contents: Record<string, string> = {}
        const toChapter: Record<string, string> = {}
        const withScenes: ChapterWithScenes[] = sorted.map((ch, i) => {
          const scenes = [...(sceneLists[i] ?? [])].sort((a, b) => a.sort_order - b.sort_order)
          for (const sc of scenes) {
            contents[sc.id] = sc.content
            toChapter[sc.id] = ch.id
          }
          return { ...ch, scenes }
        })

        setProjectTitle(project.title)
        setChapters(withScenes)
        setSceneContents(contents)
        setSceneToChapter(toChapter)

        // Default selection: first chapter's first scene.
        const firstCh = withScenes[0]
        const firstSc = firstCh?.scenes[0]
        if (firstCh) setSelectedChapterId(firstCh.id)
        if (firstSc) setSelectedSceneId(firstSc.id)
      } catch {
        if (!cancelled) navigate('/dashboard', { replace: true })
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    load()
    return () => { cancelled = true }
  }, [projectId, accessToken])

  // ── Content editing + auto-save ─────────────────────────────────────────────

  const handleContentChange = useCallback((sceneId: string, value: string) => {
    setSceneContents((prev) => ({ ...prev, [sceneId]: value }))

    if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
    saveTimerRef.current = setTimeout(() => {
      const chapterId = sceneToChapter[sceneId]
      if (!chapterId || !projectId || !accessToken) return
      api.scenes.update(accessToken, projectId, chapterId, sceneId, { content: value })
        .catch(() => {}) // silent — user will retry on next keystroke
    }, AUTOSAVE_MS)
  }, [accessToken, projectId, sceneToChapter])

  const handleSelectScene = useCallback((chapterId: string, sceneId: string) => {
    setSelectedChapterId(chapterId)
    setSelectedSceneId(sceneId)
  }, [])

  const handleCreateChapter = useCallback(async (title: string) => {
    if (!projectId || !accessToken) return
    const sortOrder = chapters.length + 1
    const chapter = await api.chapters.create(accessToken, projectId, title, sortOrder)
    const newChapter: ChapterWithScenes = { ...chapter, scenes: [] }
    setChapters((prev) => [...prev, newChapter])
    setSelectedChapterId(chapter.id)
    setSelectedSceneId(null)
    // Expand the new chapter in the explorer (handled via default-open in explorer).
  }, [projectId, accessToken, chapters.length])

  const handleCreateScene = useCallback(async (chapterId: string, title: string) => {
    if (!projectId || !accessToken) return
    const chapter = chapters.find((c) => c.id === chapterId)
    const sortOrder = (chapter?.scenes.length ?? 0) + 1
    const scene = await api.scenes.create(accessToken, projectId, chapterId, title, sortOrder)
    setSceneContents((prev) => ({ ...prev, [scene.id]: '' }))
    setSceneToChapter((prev) => ({ ...prev, [scene.id]: chapterId }))
    setChapters((prev) =>
      prev.map((c) => c.id === chapterId ? { ...c, scenes: [...c.scenes, scene] } : c)
    )
    setSelectedChapterId(chapterId)
    setSelectedSceneId(scene.id)
  }, [projectId, accessToken, chapters])

  const toggleLeftPanel = useCallback((panel: LeftPanel) => {
    setLeftPanel((prev) => (prev === panel ? 'none' : panel))
  }, [])

  // ── Derived display values ──────────────────────────────────────────────────

  const activeChapter = chapters.find((c) => c.id === selectedChapterId)
  const activeScene   = activeChapter?.scenes.find((s) => s.id === selectedSceneId)
  const content       = activeScene ? (sceneContents[activeScene.id] ?? '') : ''
  const wordCount     = content.trim() === '' ? 0 : content.trim().split(/\s+/).length

  // ── Loading state ───────────────────────────────────────────────────────────

  if (loading) {
    return (
      <div className="h-screen flex items-center justify-center bg-brand-bg">
        <svg className="animate-spin h-6 w-6 text-brand-cyan" viewBox="0 0 24 24" fill="none">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
        </svg>
      </div>
    )
  }

  return (
    <div className="h-screen flex flex-col bg-brand-bg overflow-hidden font-sans">
      <TopBar
        projectTitle={projectTitle}
        chapterTitle={activeChapter?.title ?? ''}
        sceneTitle={activeScene?.title ?? ''}
        leftPanel={leftPanel}
        explorerOpen={explorerOpen}
        onToggleChat={() => toggleLeftPanel('chat')}
        onToggleExplorer={() => setExplorerOpen((v) => !v)}
      />

      <div className="flex flex-1 overflow-hidden">
        <ActivityBar
          activePanel={leftPanel}
          onToggleChat={() => toggleLeftPanel('chat')}
          onToggleGit={() => toggleLeftPanel('git')}
          onToggleWiki={() => toggleLeftPanel('wiki')}
        />
        {leftPanel === 'chat' && <ChatBar />}
        {leftPanel === 'git' && projectId && accessToken && (
          <GitPanel token={accessToken} projectId={projectId} />
        )}
        {leftPanel === 'wiki' && projectId && accessToken && (
          <WikiPanel token={accessToken} projectId={projectId} />
        )}
        <ScribeEditor
          sceneTitle={activeScene?.title ?? 'Select a scene'}
          content={content}
          sceneSelected={!!activeScene}
          onChange={(val) => activeScene && handleContentChange(activeScene.id, val)}
        />
        {explorerOpen && (
          <ProjectExplorer
            projectTitle={projectTitle}
            chapters={chapters}
            selectedChapterId={selectedChapterId ?? ''}
            selectedSceneId={selectedSceneId ?? ''}
            onSelectScene={handleSelectScene}
            onCreateChapter={handleCreateChapter}
            onCreateScene={handleCreateScene}
          />
        )}
      </div>

      <StatusBar
        wordCount={wordCount}
        chapterTitle={activeChapter?.title ?? ''}
        sceneTitle={activeScene?.title ?? ''}
      />
    </div>
  )
}
