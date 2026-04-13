package export

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jconder44/nexustale/pkg/apperror"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
	"github.com/jconder44/nexustale/pkg/storage"
)

const epubExpiry = 24 * time.Hour

// Service handles project export operations — synchronous Markdown zip streaming
// and asynchronous EPUB generation via a background worker pool.
type Service struct {
	queries *sqlcgen.Queries
	store   *storage.Client
	workCh  chan epubJob
}

type epubJob struct {
	jobID     uuid.UUID
	projectID uuid.UUID
	title     string
}

// JobResponse is the API-facing representation of an export job.
type JobResponse struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Format      string `json:"format"`
	Status      string `json:"status"`
	DownloadURL string `json:"download_url,omitempty"` // populated when status=done
	ErrorMsg    string `json:"error_msg,omitempty"`    // populated when status=failed
	ExpiresAt   string `json:"expires_at,omitempty"`   // RFC3339; populated when status=done
	CreatedAt   string `json:"created_at"`
}

func NewService(queries *sqlcgen.Queries, store *storage.Client) *Service {
	return &Service{
		queries: queries,
		store:   store,
		workCh:  make(chan epubJob, 64),
	}
}

// ExportMarkdown streams the project as a zip of Markdown files directly into w.
// The response can be piped straight to http.ResponseWriter; no temp file needed.
func (s *Service) ExportMarkdown(ctx context.Context, projectID uuid.UUID, w io.Writer) error {
	return WriteMarkdownZip(ctx, s.queries, projectID, w)
}

// EnqueueEPUB inserts a pending export job and sends it to the background worker
// pool. Returns the job ID for the caller to surface as a 202 response.
func (s *Service) EnqueueEPUB(ctx context.Context, projectID, userID uuid.UUID, title string) (uuid.UUID, error) {
	job, err := s.queries.InsertExportJob(ctx, sqlcgen.InsertExportJobParams{
		ProjectID: projectID,
		UserID:    userID,
		Format:    "epub",
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert export job: %w", err)
	}

	select {
	case s.workCh <- epubJob{jobID: job.ID, projectID: projectID, title: title}:
	default:
		// Buffer full — the job row exists in the DB with status=pending.
		// At this scale (single-writer dev tool) this should never happen.
		slog.Warn("export worker channel full; job remains pending", "job_id", job.ID)
	}

	return job.ID, nil
}

// GetJob fetches a single export job by ID, verifies it belongs to userID,
// and attaches a presigned download URL when the job is done.
func (s *Service) GetJob(ctx context.Context, jobID, userID uuid.UUID) (*JobResponse, error) {
	job, err := s.queries.GetExportJob(ctx, jobID)
	if err != nil {
		return nil, apperror.NotFound("export job", jobID.String())
	}
	if job.UserID != userID {
		return nil, apperror.Forbidden("export job belongs to another user")
	}
	return s.toResponse(ctx, job), nil
}

// ListJobs returns the 20 most recent export jobs for the given project.
func (s *Service) ListJobs(ctx context.Context, projectID uuid.UUID) ([]*JobResponse, error) {
	rows, err := s.queries.ListExportJobsByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list export jobs: %w", err)
	}
	out := make([]*JobResponse, len(rows))
	for i, j := range rows {
		out[i] = s.toResponse(ctx, j)
	}
	return out, nil
}

// StartWorkers launches n background goroutines that process EPUB jobs.
// Call once during startup; goroutines run for the lifetime of the process.
func (s *Service) StartWorkers(n int) {
	for i := 0; i < n; i++ {
		go s.runWorker()
	}
}

func (s *Service) runWorker() {
	for job := range s.workCh {
		s.processEPUB(job)
	}
}

func (s *Service) processEPUB(job epubJob) {
	ctx := context.Background()

	if err := s.queries.UpdateExportJobProcessing(ctx, job.jobID); err != nil {
		slog.Error("failed to mark export job processing", "job_id", job.jobID, "error", err)
		return
	}

	tmpPath, err := BuildEPUB(ctx, s.queries, job.projectID, job.title)
	if err != nil {
		slog.Error("epub build failed", "job_id", job.jobID, "error", err)
		s.markFailed(ctx, job.jobID, err.Error())
		return
	}
	defer os.Remove(tmpPath)

	f, err := os.Open(tmpPath)
	if err != nil {
		s.markFailed(ctx, job.jobID, "open temp file: "+err.Error())
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		s.markFailed(ctx, job.jobID, "stat temp file: "+err.Error())
		return
	}

	key := fmt.Sprintf("exports/%s/%s.epub", job.projectID, job.jobID)
	if err := s.store.PutObject(ctx, key, "application/epub+zip", f, info.Size()); err != nil {
		s.markFailed(ctx, job.jobID, "upload to storage: "+err.Error())
		return
	}

	exp := time.Now().Add(epubExpiry)
	if err := s.queries.UpdateExportJobDone(ctx, sqlcgen.UpdateExportJobDoneParams{
		ID:        job.jobID,
		MinioKey:  pgtype.Text{String: key, Valid: true},
		ExpiresAt: pgtype.Timestamptz{Time: exp, Valid: true},
	}); err != nil {
		slog.Error("failed to mark export job done", "job_id", job.jobID, "error", err)
	}

	slog.Info("epub export complete", "job_id", job.jobID, "key", key)
}

func (s *Service) markFailed(ctx context.Context, jobID uuid.UUID, msg string) {
	if err := s.queries.UpdateExportJobFailed(ctx, sqlcgen.UpdateExportJobFailedParams{
		ID:       jobID,
		ErrorMsg: pgtype.Text{String: msg, Valid: true},
	}); err != nil {
		slog.Error("failed to mark export job failed", "job_id", jobID, "error", err)
	}
}

func (s *Service) toResponse(ctx context.Context, job sqlcgen.ExportJob) *JobResponse {
	r := &JobResponse{
		ID:        job.ID.String(),
		ProjectID: job.ProjectID.String(),
		Format:    job.Format,
		Status:    job.Status,
		CreatedAt: job.CreatedAt.Time.Format(time.RFC3339),
	}

	if job.ErrorMsg.Valid {
		r.ErrorMsg = job.ErrorMsg.String
	}

	if job.Status == "done" && job.MinioKey.Valid && job.ExpiresAt.Valid {
		r.ExpiresAt = job.ExpiresAt.Time.Format(time.RFC3339)
		url, err := s.store.PresignedGetURL(ctx, job.MinioKey.String, epubExpiry)
		if err != nil {
			slog.Warn("failed to presign export URL", "key", job.MinioKey.String, "error", err)
		} else {
			r.DownloadURL = url
		}
	}

	return r
}
