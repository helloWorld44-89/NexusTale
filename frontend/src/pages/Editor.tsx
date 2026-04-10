// Editor.tsx — main writing shell, wired to real API.
import { useState, useCallback, useEffect, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'
import { api } from '@/services/api'
import type { Act, Chapter, Scene } from '@/services/api'
import TopBar from '@/components/layout/TopBar'
import StatusBar from '@/components/layout/StatusBar'
import ChatBar from '@/components/ai/ChatBar'
import GitPanel from '@/components/editor/GitPanel'
import WikiPanel from '@/components/wiki/WikiPanel'
import ScribeEditor from '@/components/editor/ScribeEditor'
import SceneMetadataPanel from '@/components/editor/SceneMetadataPanel'
import ProjectExplorer from '@/components/project/ProjectExplorer'
import type { ActItem } from '@/components/project/ProjectExplorer'
import ActivityBar from '@/components/layout/ActivityBar'

export type LeftPanel = 'chat' | 'git' | 'wiki' | 'none'

// Chapter augmented with its scenes for the explorer tree.
interface ChapterWithScenes extends Chapter {
  scenes: Scene[]
}

// Act augmented with its chapters (each holding scenes).
interface ActWithChapters extends Act {
  chapters: ChapterWithScenes[]
}

const AUTOSAVE_MS = 1500

export default function Editor() {
  const { id: projectId } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const accessToken = useAuthStore((s) => s.accessToken)

  const [projectTitle,      setProjectTitle]      = useState('')
  const [acts,              setActs]              = useState<ActWithChapters[]>([])
  const [selectedChapterId, setSelectedChapterId] = useState<string | null>(null)
  const [selectedSceneId,   setSelectedSceneId]   = useState<string | null>(null)
  const [sceneContents,     setSceneContents]     = useState<Record<string, string>>({})
  const [sceneData,         setSceneData]         = useState<Record<string, Scene>>({})
  const [sceneToChapter,    setSceneToChapter]    = useState<Record<string, string>>({})
  const [loading,           setLoading]           = useState(true)
  const [leftPanel,         setLeftPanel]         = useState<LeftPanel>('chat')
  const [explorerOpen,      setExplorerOpen]      = useState(true)
  const [focusMode,         setFocusMode]         = useState(false)

  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // ── Focus mode keyboard shortcuts ──────────────────────────────────────────
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'F11') {
        e.preventDefault()
        setFocusMode((v) => !v)
      }
      if (e.key === 'Escape') {
        setFocusMode(false)
      }
    }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [])

  // ── Load project data ───────────────────────────────────────────────────────

  useEffect(() => {
    if (!projectId || !accessToken) return

    let cancelled = false

    const load = async () => {
      try {
        const [project, rawActs] = await Promise.all([
          api.projects.get(accessToken, projectId),
          api.acts.list(accessToken, projectId),
        ])

        if (cancelled) return

        const sortedActs = [...rawActs].sort((a, b) => a.sort_order - b.sort_order)

        // For each act, fetch its chapters in parallel.
        const chapterLists = await Promise.all(
          sortedActs.map((act) => api.chapters.list(accessToken, projectId, act.id))
        )

        if (cancelled) return

        // For each chapter, fetch its scenes in parallel (all chapters at once).
        const allChapters = chapterLists.flat()
        const sceneLists  = await Promise.all(
          allChapters.map((ch) => api.scenes.list(accessToken, ch.id))
        )

        if (cancelled) return

        const contents:   Record<string, string>  = {}
        const data:       Record<string, Scene>   = {}
        const toChapter:  Record<string, string>  = {}

        // Map scene lists back to chapters.
        let sceneIdx = 0
        const actsWithChapters: ActWithChapters[] = sortedActs.map((act, ai) => {
          const rawChapters = chapterLists[ai] ?? []
          const sorted = [...rawChapters].sort((a, b) => a.sort_order - b.sort_order)
          const chapters: ChapterWithScenes[] = sorted.map((ch) => {
            const scenes = [...(sceneLists[sceneIdx++] ?? [])].sort((a, b) => a.sort_order - b.sort_order)
            for (const sc of scenes) {
              contents[sc.id]  = sc.content
              data[sc.id]      = sc
              toChapter[sc.id] = ch.id
            }
            return { ...ch, scenes }
          })
          return { ...act, chapters }
        })

        setProjectTitle(project.title)
        setActs(actsWithChapters)
        setSceneContents(contents)
        setSceneData(data)
        setSceneToChapter(toChapter)

        // Default selection: first act → first chapter → first scene.
        const firstCh = actsWithChapters[0]?.chapters[0]
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
      if (!chapterId || !accessToken) return
      api.scenes.update(accessToken, chapterId, sceneId, { content: value })
        .catch(() => {})
    }, AUTOSAVE_MS)
  }, [accessToken, sceneToChapter])

  const handleSelectScene = useCallback((chapterId: string, sceneId: string) => {
    setSelectedChapterId(chapterId)
    setSelectedSceneId(sceneId)
  }, [])

  const handleCreateAct = useCallback(async (title: string) => {
    if (!projectId || !accessToken) return
    const sortOrder = acts.length
    const act = await api.acts.create(accessToken, projectId, title, '', sortOrder)
    const newAct: ActWithChapters = { ...act, chapters: [] }
    setActs((prev) => [...prev, newAct])
  }, [projectId, accessToken, acts.length])

  const handleCreateChapter = useCallback(async (actId: string, title: string) => {
    if (!projectId || !accessToken) return
    const act = acts.find((a) => a.id === actId)
    const sortOrder = (act?.chapters.length ?? 0) + 1
    const chapter = await api.chapters.create(accessToken, projectId, actId, title, sortOrder)
    const newChapter: ChapterWithScenes = { ...chapter, scenes: [] }
    setActs((prev) =>
      prev.map((a) => a.id === actId ? { ...a, chapters: [...a.chapters, newChapter] } : a)
    )
    setSelectedChapterId(chapter.id)
    setSelectedSceneId(null)
  }, [projectId, accessToken, acts])

  const handleCreateScene = useCallback(async (chapterId: string, title: string) => {
    if (!accessToken) return
    const chapter = acts.flatMap((a) => a.chapters).find((c) => c.id === chapterId)
    const sortOrder = (chapter?.scenes.length ?? 0) + 1
    const scene = await api.scenes.create(accessToken, chapterId, title, sortOrder)
    setSceneContents((prev) => ({ ...prev, [scene.id]: '' }))
    setSceneData((prev) => ({ ...prev, [scene.id]: scene }))
    setSceneToChapter((prev) => ({ ...prev, [scene.id]: chapterId }))
    setActs((prev) =>
      prev.map((a) => ({
        ...a,
        chapters: a.chapters.map((c) =>
          c.id === chapterId ? { ...c, scenes: [...c.scenes, scene] } : c
        ),
      }))
    )
    setSelectedChapterId(chapterId)
    setSelectedSceneId(scene.id)
  }, [accessToken, acts])

  const toggleLeftPanel = useCallback((panel: LeftPanel) => {
    setLeftPanel((prev) => (prev === panel ? 'none' : panel))
  }, [])

  // ── Derived display values ──────────────────────────────────────────────────

  const activeAct     = acts.find((a) => a.chapters.some((c) => c.id === selectedChapterId))
  const activeChapter = activeAct?.chapters.find((c) => c.id === selectedChapterId)
  const activeScene   = activeChapter?.scenes.find((s) => s.id === selectedSceneId)
  const content       = activeScene ? (sceneContents[activeScene.id] ?? '') : ''
  const wordCount     = content.trim() === '' ? 0 : content.trim().split(/\s+/).length

  // Only show the act in the breadcrumb when the act layer is visible
  // (i.e. more than one act, or a custom-named act).
  const actsAreHidden = acts.length === 1 && acts[0]?.title === 'Act 1'
  const actTitle      = actsAreHidden ? '' : (activeAct?.title ?? '')

  // Map acts to the shape ProjectExplorer expects.
  const explorerActs: ActItem[] = acts.map((a) => ({
    id:       a.id,
    title:    a.title,
    chapters: a.chapters.map((c) => ({
      id:     c.id,
      title:  c.title,
      scenes: c.scenes.map((s) => ({ id: s.id, title: s.title })),
    })),
  }))

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
      {!focusMode && (
        <TopBar
          projectTitle={projectTitle}
          actTitle={actTitle}
          chapterTitle={activeChapter?.title ?? ''}
          sceneTitle={activeScene?.title ?? ''}
          leftPanel={leftPanel}
          explorerOpen={explorerOpen}
          focusMode={focusMode}
          onToggleChat={() => toggleLeftPanel('chat')}
          onToggleExplorer={() => setExplorerOpen((v) => !v)}
          onToggleFocus={() => setFocusMode((v) => !v)}
        />
      )}

      <div className="flex flex-1 overflow-hidden">
        {!focusMode && (
          <ActivityBar
            activePanel={leftPanel}
            onToggleChat={() => toggleLeftPanel('chat')}
            onToggleGit={() => toggleLeftPanel('git')}
            onToggleWiki={() => toggleLeftPanel('wiki')}
          />
        )}
        {!focusMode && leftPanel === 'chat' && <ChatBar />}
        {!focusMode && leftPanel === 'git' && projectId && accessToken && (
          <GitPanel token={accessToken} projectId={projectId} />
        )}
        {!focusMode && leftPanel === 'wiki' && projectId && accessToken && (
          <WikiPanel token={accessToken} projectId={projectId} currentContent={content} />
        )}
        <div className="flex flex-col flex-1 overflow-hidden relative">
          {/* Floating exit button — only in focus mode */}
          {focusMode && (
            <button
              onClick={() => setFocusMode(false)}
              title="Exit focus mode (Esc)"
              className="absolute top-3 right-4 z-10 flex items-center gap-1.5 px-2.5 py-1 rounded text-xs text-brand-text-muted hover:text-brand-text bg-brand-bg-card/60 hover:bg-brand-bg-card border border-brand-border/40 transition-all opacity-20 hover:opacity-100"
            >
              <ExitFocusIcon />
              Esc
            </button>
          )}
          <ScribeEditor
            sceneTitle={activeScene?.title ?? 'Select a scene'}
            content={content}
            sceneSelected={!!activeScene}
            onChange={(val) => activeScene && handleContentChange(activeScene.id, val)}
          />
          {activeScene && accessToken && selectedChapterId && sceneData[activeScene.id] && (
            <SceneMetadataPanel
              token={accessToken}
              chapterId={selectedChapterId}
              scene={sceneData[activeScene.id]}
              onUpdate={(updated) => setSceneData((prev) => ({ ...prev, [updated.id]: updated }))}
            />
          )}
        </div>
        {!focusMode && explorerOpen && (
          <ProjectExplorer
            projectTitle={projectTitle}
            acts={explorerActs}
            selectedChapterId={selectedChapterId ?? ''}
            selectedSceneId={selectedSceneId ?? ''}
            onSelectScene={handleSelectScene}
            onCreateAct={handleCreateAct}
            onCreateChapter={handleCreateChapter}
            onCreateScene={handleCreateScene}
          />
        )}
      </div>

      {!focusMode && (
        <StatusBar
          wordCount={wordCount}
          chapterTitle={activeChapter?.title ?? ''}
          sceneTitle={activeScene?.title ?? ''}
        />
      )}
    </div>
  )
}

function ExitFocusIcon() {
  return (
    <svg className="w-3 h-3" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M6 2H2v4M10 2h4v4M6 14H2v-4M10 14h4v-4" />
    </svg>
  )
}
