// ScribeEditor — centered writing area with Beat expansion toolbar.
import BeatInput from './BeatInput'

interface ScribeEditorProps {
  sceneTitle:  string
  content:     string
  sceneSelected: boolean
  onChange:    (value: string) => void
  // Beat/AI props — only provided when a scene is active
  token?:      string
  projectId?:  string
  sceneId?:    string
  promptId?:   string | null
  branch?:     string
}

export default function ScribeEditor({
  sceneTitle,
  content,
  sceneSelected,
  onChange,
  token,
  projectId,
  sceneId,
  promptId,
  branch,
}: ScribeEditorProps) {
  const handleBeatAccept = (text: string) => {
    onChange(content + text)
  }

  return (
    <div className="flex-1 flex flex-col overflow-hidden bg-brand-bg">

      {/* Scene title strip */}
      <div className="px-8 pt-8 pb-3 max-w-3xl w-full mx-auto">
        <h1 className="text-xl font-semibold text-brand-text/80 tracking-tight">
          {sceneTitle}
        </h1>
      </div>

      {/* Writing surface */}
      <div className="flex-1 overflow-y-auto px-8 pb-4">
        <div className="max-w-3xl mx-auto h-full">
          {sceneSelected ? (
            <textarea
              value={content}
              onChange={(e) => onChange(e.target.value)}
              placeholder="Begin your scene…"
              spellCheck
              className="w-full h-full min-h-[60vh] resize-none bg-transparent text-brand-text text-base leading-8 placeholder:text-brand-muted/40 focus:outline-none font-serif"
            />
          ) : (
            <div className="flex items-center justify-center h-full">
              <p className="text-brand-muted text-sm">Select a scene to start writing</p>
            </div>
          )}
        </div>
      </div>

      {/* Beat expansion toolbar — only when scene is active and AI props provided */}
      {sceneSelected && token && projectId && sceneId && (
        <BeatInput
          token={token}
          projectId={projectId}
          sceneId={sceneId}
          promptId={promptId ?? null}
          branch={branch}
          onAccept={handleBeatAccept}
        />
      )}
    </div>
  )
}
