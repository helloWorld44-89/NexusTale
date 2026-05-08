// Editor.tsx — main writing shell, wired to real API.
import { useState, useCallback, useEffect, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/app/store/authStore'
import { api } from '@/services/api'
import type { Act, Chapter, Scene, Annotation } from '@/services/api'
import TopBar from '@/components/layout/TopBar'
import StatusBar from '@/components/layout/StatusBar'
import ChatBar from '@/components/ai/ChatBar'
import ContextPanel from '@/components/ai/ContextPanel'
import WorkshopPanel from '@/components/ai/WorkshopPanel'
import GitPanel from '@/components/editor/GitPanel'
import WikiPanel from '@/components/wiki/WikiPanel'
import ScribeEditor, { type ScribeEditorHandle } from '@/components/editor/ScribeEditor'
import AnnotationSidebar from '@/components/editor/AnnotationSidebar'
import SceneMetadataPanel from '@/components/editor/SceneMetadataPanel'
import ProjectExplorer from '@/components/project/ProjectExplorer'
import type { ActItem } from '@/components/project/ProjectExplorer'
import ActivityBar from '@/components/layout/ActivityBar'
import { ErrorBoundary } from '@/components/shared/ErrorBoundary'
import WalkthroughOverlay from '@/components/shared/WalkthroughOverlay'
import { useWalkthrough } from '@/hooks/useWalkthrough'

export type LeftPanel = 'chat' | 'context' | 'workshop' | 'git' | 'wiki' | 'annotations' | 'none'

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
  const { accessToken, user, logout } = useAuthStore((s) => ({
    accessToken: s.accessToken,
    user: s.user,
    logout: s.logout,
  }))

  const handleLogout = useCallback(async () => {
    await logout()
    navigate('/login', { replace: true })
  }, [logout, navigate])

  const [projectTitle,      setProjectTitle]      = useState('')
  const [projectPhase,      setProjectPhase]      = useState<string>('drafting')
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
  const [selectedPromptId,  setSelectedPromptId]  = useState<string | null>(null)
  const [currentBranch,      setCurrentBranch]      = useState<string>('canon')
  const [refreshKey,         setRefreshKey]         = useState(0)
  const [pendingAnnotation,  setPendingAnnotation]  = useState<Annotation | null>(null)
  const [annotationCount,    setAnnotationCount]    = useState(0)
  // Track which panels have ever been opened so we can keep them mounted
  // (CSS hidden) instead of unmounting — preserves conversation history in
  // ChatBar and WorkshopPanel across panel switches.
  const [panelsMounted,      setPanelsMounted]      = useState<Set<LeftPanel>>(() => new Set(['chat']))

  const saveTimerRef    = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pendingSaveRef  = useRef<(() => void) | null>(null)
  const scribeEditorRef = useRef<ScribeEditorHandle>(null)

  const walkthrough = useWalkthrough(!loading)

  const handlePhaseChange = useCallback(async (phase: string) => {
    if (!projectId || !accessToken) return
    setProjectPhase(phase)
    await api.phase.set(accessToken, projectId, phase)
  }, [projectId, accessToken])

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
        const [project, rawActs, gitStatus] = await Promise.all([
          api.projects.get(accessToken, projectId),
          api.acts.list(accessToken, projectId),
          api.git.status(accessToken, projectId).catch(() => null),
        ])

        if (!cancelled && gitStatus) {
          setCurrentBranch(gitStatus.current_timeline ?? 'canon')
        }

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
        setProjectPhase(project.phase ?? 'drafting')
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
  // refreshKey triggers a tree reload when agent creates are undone.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, accessToken, refreshKey])

  // ── Content editing + auto-save ─────────────────────────────────────────────

  const handleContentChange = useCallback((sceneId: string, value: string) => {
    setSceneContents((prev) => ({ ...prev, [sceneId]: value }))

    // Keep sceneData.word_count in sync so SceneMetadataPanel shows the same
    // number as the StatusBar without waiting for the server round-trip.
    const wc = value.trim() === '' ? 0 : value.trim().split(/\s+/).length
    setSceneData((prev) => {
      const sc = prev[sceneId]
      if (!sc) return prev
      return { ...prev, [sceneId]: { ...sc, word_count: wc } }
    })

    const doSave = () => {
      const chapterId = sceneToChapter[sceneId]
      if (!chapterId || !accessToken) return
      api.scenes.update(accessToken, chapterId, sceneId, { content: value }, currentBranch)
        .catch(() => {})
    }

    pendingSaveRef.current = doSave
    if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
    saveTimerRef.current = setTimeout(() => {
      pendingSaveRef.current = null
      doSave()
    }, AUTOSAVE_MS)
  }, [accessToken, sceneToChapter, currentBranch])

  // Flush any pending autosave when the selected scene changes so edits are
  // never lost when the writer clicks to a different scene before the debounce fires.
  useEffect(() => {
    return () => {
      if (pendingSaveRef.current) {
        if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
        pendingSaveRef.current()
        pendingSaveRef.current = null
      }
    }
  }, [selectedSceneId])

  // Best-effort flush on tab/window close.
  useEffect(() => {
    const handleBeforeUnload = () => {
      if (pendingSaveRef.current) {
        pendingSaveRef.current()
        pendingSaveRef.current = null
      }
    }
    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => window.removeEventListener('beforeunload', handleBeforeUnload)
  }, [])

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
    setPanelsMounted((prev) => {
      if (prev.has(panel)) return prev
      const next = new Set(prev)
      next.add(panel)
      return next
    })
  }, [])

  // ── Derived display values ──────────────────────────────────────────────────

  const activeAct     = acts.find((a) => a.chapters.some((c) => c.id === selectedChapterId))
  const activeChapter = activeAct?.chapters.find((c) => c.id === selectedChapterId)
  const activeScene   = activeChapter?.scenes.find((s) => s.id === selectedSceneId)

  // Append AI-generated text from any panel into the active scene.
  const handleInsertToScene = useCallback((text: string) => {
    if (!activeScene) return
    const current = sceneContents[activeScene.id] ?? ''
    const next = current + (current.endsWith('\n') ? '\n' : '\n\n') + text
    handleContentChange(activeScene.id, next)
  }, [activeScene, sceneContents, handleContentChange])

  // Refresh a scene's in-editor content after an agent tool writes to it.
  const handleToolWrite = useCallback(async (sceneId: string, chapterId: string) => {
    if (!accessToken) return
    try {
      const scene = await api.scenes.get(accessToken, chapterId, sceneId)
      setSceneContents((prev) => ({ ...prev, [sceneId]: scene.content }))
    } catch {
      // non-critical — editor will reflect next autosave
    }
  }, [accessToken])

  // Reload the full project tree after an agent create is undone.
  const handleTreeRefresh = useCallback(() => {
    setRefreshKey((k) => k + 1)
  }, [])
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
      scenes: c.scenes.map((s) => ({ id: s.id, title: s.title, scene_role: (s as { attributes?: { scene_role?: string } }).attributes?.scene_role })),
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
          projectId={projectId ?? ''}
          projectTitle={projectTitle}
          projectPhase={projectPhase}
          actTitle={actTitle}
          chapterTitle={activeChapter?.title ?? ''}
          sceneTitle={activeScene?.title ?? ''}
          displayName={user?.display_name ?? ''}
          token={accessToken ?? ''}
          leftPanel={leftPanel}
          explorerOpen={explorerOpen}
          focusMode={focusMode}
          onToggleChat={() => toggleLeftPanel('chat')}
          onToggleExplorer={() => setExplorerOpen((v) => !v)}
          onToggleFocus={() => setFocusMode((v) => !v)}
          onLogout={handleLogout}
          onPhaseChange={handlePhaseChange}
        />
      )}

      <div className="flex flex-1 overflow-hidden">
        {!focusMode && (
          <ActivityBar
            activePanel={leftPanel}
            onToggleChat={() => toggleLeftPanel('chat')}
            onToggleContext={() => toggleLeftPanel('context')}
            onToggleWorkshop={() => toggleLeftPanel('workshop')}
            onToggleGit={() => toggleLeftPanel('git')}
            onToggleWiki={() => toggleLeftPanel('wiki')}
            onToggleAnnotations={() => toggleLeftPanel('annotations')}
            annotationCount={annotationCount}
          />
        )}
        {!focusMode && panelsMounted.has('chat') && projectId && accessToken && (
          <div style={{ display: leftPanel === 'chat' ? 'flex' : 'none' }}>
            <ErrorBoundary label="Nexus chat">
              <ChatBar
                token={accessToken}
                projectId={projectId}
                sceneId={selectedSceneId ?? undefined}
                branch={currentBranch}
                promptId={selectedPromptId}
                onInsertToScene={activeScene ? handleInsertToScene : undefined}
              />
            </ErrorBoundary>
          </div>
        )}
        {!focusMode && leftPanel === 'context' && projectId && accessToken && (
          <div className="w-72 shrink-0 flex flex-col border-r border-brand-border bg-brand-bg overflow-hidden">
            <ErrorBoundary label="context panel">
              <ContextPanel token={accessToken} projectId={projectId} sceneId={selectedSceneId ?? undefined} branch={currentBranch} />
            </ErrorBoundary>
          </div>
        )}
        {!focusMode && panelsMounted.has('workshop') && projectId && accessToken && (
          <div style={{ display: leftPanel === 'workshop' ? 'flex' : 'none' }}>
            <ErrorBoundary label="Workshop">
              <WorkshopPanel
                token={accessToken}
                projectId={projectId}
                projectPhase={projectPhase}
                sceneId={selectedSceneId ?? undefined}
                branch={currentBranch}
                promptId={selectedPromptId}
                onInsertToScene={activeScene ? handleInsertToScene : undefined}
                onToolWrite={handleToolWrite}
                onStructureChange={handleTreeRefresh}
              />
            </ErrorBoundary>
          </div>
        )}
        {!focusMode && leftPanel === 'git' && projectId && accessToken && (
          <ErrorBoundary label="Chronicle panel">
            <GitPanel
              token={accessToken}
              projectId={projectId}
              onBranchChange={setCurrentBranch}
            />
          </ErrorBoundary>
        )}
        {!focusMode && leftPanel === 'wiki' && projectId && accessToken && (
          <ErrorBoundary label="wiki panel">
            <WikiPanel token={accessToken} projectId={projectId} currentContent={content} />
          </ErrorBoundary>
        )}
        {!focusMode && leftPanel === 'annotations' && projectId && accessToken && (
          <ErrorBoundary label="annotations panel">
            <AnnotationSidebar
              token={accessToken}
              projectId={projectId}
              sceneId={selectedSceneId}
              currentUserId={user?.id ?? ''}
              ownerId={user?.id ?? ''}
              onJump={(start, end) => scribeEditorRef.current?.jumpToAnnotation(start, end)}
              newAnnotation={pendingAnnotation}
              onAnnotationConsumed={() => setPendingAnnotation(null)}
            />
          </ErrorBoundary>
        )}
        <ErrorBoundary label="scene editor">
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
            ref={scribeEditorRef}
            sceneTitle={activeScene?.title ?? 'Select a scene'}
            content={content}
            sceneSelected={!!activeScene}
            onChange={(val) => activeScene && handleContentChange(activeScene.id, val)}
            token={accessToken ?? undefined}
            projectId={projectId}
            sceneId={activeScene?.id}
            promptId={selectedPromptId}
            branch={currentBranch}
            projectPhase={projectPhase}
            onAnnotationCreated={(ann) => {
              setPendingAnnotation(ann)
              setAnnotationCount(c => c + 1)
              if (leftPanel !== 'annotations') toggleLeftPanel('annotations')
            }}
          />
          {activeScene && accessToken && selectedChapterId && projectId && sceneData[activeScene.id] && (
            <SceneMetadataPanel
              token={accessToken}
              chapterId={selectedChapterId}
              projectId={projectId}
              scene={sceneData[activeScene.id]}
              selectedPromptId={selectedPromptId}
              onUpdate={(updated) => setSceneData((prev) => ({ ...prev, [updated.id]: updated }))}
              onPromptChange={setSelectedPromptId}
            />
          )}
        </div>
        </ErrorBoundary>
        {!focusMode && explorerOpen && (
          <ErrorBoundary label="project explorer">
            <ProjectExplorer
              projectTitle={projectTitle}
              acts={explorerActs}
              selectedChapterId={selectedChapterId ?? ''}
              selectedSceneId={selectedSceneId ?? ''}
              onSelectScene={handleSelectScene}
              onCreateAct={handleCreateAct}
              onCreateChapter={handleCreateChapter}
              onCreateScene={handleCreateScene}
              token={accessToken ?? undefined}
              projectId={projectId}
              branch={currentBranch}
            />
          </ErrorBoundary>
        )}
      </div>

      {!focusMode && (
        <StatusBar
          wordCount={wordCount}
          chapterTitle={activeChapter?.title ?? ''}
          sceneTitle={activeScene?.title ?? ''}
        />
      )}

      {walkthrough.active && (
        <WalkthroughOverlay
          step={walkthrough.step}
          onNext={walkthrough.next}
          onSkip={walkthrough.skip}
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
