package project

import (
	"time"

	"github.com/google/uuid"
)

// ── Git / Chronicle DTOs ──────────────────────────────────────────────────────

type ChronicleRequest struct {
	Note string `json:"note" binding:"required,min=1,max=500"`
}

type DivergeRequest struct {
	TimelineName string `json:"timeline_name" binding:"required,min=1,max=100"`
	// FromSHA pins the Diverge to a specific Chronicle; defaults to HEAD.
	FromSHA string `json:"from_sha"`
}

// ChronicleEntry is one commit in the Lore (git log).
type ChronicleEntry struct {
	SHA       string    `json:"sha"`
	ShortSHA  string    `json:"short_sha"`
	Note      string    `json:"note"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

// TimelineInfo describes a branch (Timeline).
type TimelineInfo struct {
	Name      string    `json:"name"`
	IsCanon   bool      `json:"is_canon"`
	IsActive  bool      `json:"is_active"`
	HeadSHA   string    `json:"head_sha"`
	CreatedAt time.Time `json:"created_at"`
}

// CanonizeResult is returned by the Canonize operation.
type CanonizeResult struct {
	MergedSHA   string `json:"merged_sha,omitempty"`
	HasParadox  bool   `json:"has_paradox"`
	ParadoxNote string `json:"paradox_note,omitempty"`
}

// GitStatusResponse is returned by GET /git/status.
type GitStatusResponse struct {
	CurrentTimeline string          `json:"current_timeline"`
	LastChronicle   *ChronicleEntry `json:"last_chronicle,omitempty"`
}

// EchoResponse is returned by GET /git/echo.
type EchoResponse struct {
	FromSHA string `json:"from_sha"`
	ToSHA   string `json:"to_sha"`
	Diff    string `json:"diff"`
}

// Request DTOs

type CreateActRequest struct {
	Title     string `json:"title" binding:"required,min=1,max=200"`
	Summary   string `json:"summary"`
	SortOrder int32  `json:"sort_order"`
}

type UpdateActRequest struct {
	Title     *string `json:"title"`
	Summary   *string `json:"summary"`
	SortOrder *int32  `json:"sort_order"`
}

type CreateProjectRequest struct {
	Title       string   `json:"title" binding:"required,min=1,max=200"`
	Description string   `json:"description"`
	Genres      []string `json:"genres"`
}

type UpdateProjectRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
}

type CreateChapterRequest struct {
	Title     string `json:"title" binding:"required,min=1,max=200"`
	Summary   string `json:"summary"`
	SortOrder int32  `json:"sort_order"`
	ActID     string `json:"-"` // populated from URL param, not request body
}

type UpdateChapterRequest struct {
	Title     *string `json:"title"`
	Summary   *string `json:"summary"`
	SortOrder *int32  `json:"sort_order"`
}

type CreateSceneRequest struct {
	Title     string   `json:"title" binding:"required,min=1,max=200"`
	Content   string   `json:"content"`
	POV       string   `json:"pov"`
	Tense     string   `json:"tense"`
	Tags      []string `json:"tags"`
	Summary   string   `json:"summary"`
	SortOrder int32    `json:"sort_order"`

	// ProjectID and UserID are set by the handler (not from JSON) for git dual-write.
	ProjectID uuid.UUID `json:"-"`
	UserID    uuid.UUID `json:"-"`
}

type UpdateSceneRequest struct {
	Title        *string   `json:"title"`
	Content      *string   `json:"content"`
	POV          *string   `json:"pov"`
	Tense        *string   `json:"tense"`
	Tags         *[]string `json:"tags"`
	Summary      *string   `json:"summary"`
	SummaryStale *bool     `json:"summary_stale"`
	SortOrder    *int32    `json:"sort_order"`

	// Handler-only fields (not from JSON): git dual-write + summary notifier.
	ProjectID    uuid.UUID `json:"-"`
	NotifyUserID uuid.UUID `json:"-"`
	NotifyBranch string    `json:"-"`
}

// Response DTOs

type ProjectStatsResponse struct {
	SceneCount     int32     `json:"scene_count"`
	ChapterCount   int32     `json:"chapter_count"`
	TotalWordCount int32     `json:"total_word_count"`
	LastUpdatedAt  time.Time `json:"last_updated_at"`
}

type ProjectResponse struct {
	ID          uuid.UUID `json:"id"`
	OwnerID     uuid.UUID `json:"owner_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Genres      []string  `json:"genres"`
	Archived    bool      `json:"archived"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ActResponse struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	SortOrder int32     `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ChapterResponse struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	ActID     uuid.UUID `json:"act_id"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	SortOrder int32     `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SceneResponse struct {
	ID           uuid.UUID `json:"id"`
	ChapterID    uuid.UUID `json:"chapter_id"`
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	POV          string    `json:"pov"`
	Tense        string    `json:"tense"`
	Tags         []string  `json:"tags"`
	Summary      string    `json:"summary"`
	SummaryStale bool      `json:"summary_stale"`
	SortOrder    int32     `json:"sort_order"`
	WordCount    int32     `json:"word_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
