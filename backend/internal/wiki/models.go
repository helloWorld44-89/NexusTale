package wiki

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ========================
// Entity DTOs
// ========================

type CreateEntityRequest struct {
	// ParentEntityID is only set when creating a child entity (e.g. lore under a location).
	// The handler populates it from the URL rather than the request body for child routes.
	ParentEntityID *uuid.UUID      `json:"parent_entity_id"`
	Type           string          `json:"type" binding:"required,oneof=character location faction item concept lore"`
	Name           string          `json:"name" binding:"required,min=1,max=200"`
	Summary        string          `json:"summary"`
	Attributes     json.RawMessage `json:"attributes"`
}

type UpdateEntityRequest struct {
	Name       *string         `json:"name"`
	Summary    *string         `json:"summary"`
	Attributes json.RawMessage `json:"attributes"`
}

type EntityResponse struct {
	ID             uuid.UUID       `json:"id"`
	ProjectID      uuid.UUID       `json:"project_id"`
	ParentEntityID *uuid.UUID      `json:"parent_entity_id,omitempty"`
	Type           string          `json:"type"`
	Name           string          `json:"name"`
	Summary        string          `json:"summary"`
	Attributes     json.RawMessage `json:"attributes"`
	// ImageURL is a short-lived presigned MinIO URL; nil when no image is stored.
	ImageURL  *string   `json:"image_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ========================
// Relationship DTOs
// ========================

type CreateRelationshipRequest struct {
	FromEntityID uuid.UUID `json:"from_entity_id" binding:"required"`
	ToEntityID   uuid.UUID `json:"to_entity_id" binding:"required"`
	Type         string    `json:"type" binding:"required"`
	Description  string    `json:"description"`
}

type RelationshipResponse struct {
	ID           uuid.UUID `json:"id"`
	ProjectID    uuid.UUID `json:"project_id"`
	FromEntityID uuid.UUID `json:"from_entity_id"`
	ToEntityID   uuid.UUID `json:"to_entity_id"`
	Type         string    `json:"type"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

// ========================
// Magic Rule DTOs
// ========================

// MagicRuleAttributes holds structured optional fields for a magic system.
// All fields are optional — writers who prefer freeform prose leave them empty.
type MagicRuleAttributes struct {
	Powers        string `json:"powers,omitempty"`
	Limitations   string `json:"limitations,omitempty"`
	Cost          string `json:"cost,omitempty"`
	Source        string `json:"source,omitempty"`
	Accessibility string `json:"accessibility,omitempty"`
	// RulesClarity is "defined" | "mysterious" | "mixed".
	RulesClarity string `json:"rules_clarity,omitempty"`
}

type CreateMagicRuleRequest struct {
	Name        string              `json:"name" binding:"required,min=1,max=200"`
	Description string              `json:"description"`
	Attrs       MagicRuleAttributes `json:"attributes"`
}

type UpdateMagicRuleRequest struct {
	Name        *string              `json:"name"`
	Description *string              `json:"description"`
	Attrs       *MagicRuleAttributes `json:"attributes"`
}

type MagicRuleResponse struct {
	ID          uuid.UUID           `json:"id"`
	ProjectID   uuid.UUID           `json:"project_id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Attributes  MagicRuleAttributes `json:"attributes"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// ========================
// Timeline Event DTOs
// ========================

type CreateTimelineEventRequest struct {
	EntityID    *uuid.UUID `json:"entity_id"`
	Name        string     `json:"name" binding:"required,min=1,max=200"`
	Description string     `json:"description"`
	Era         string     `json:"era"`
	// Absolute date — mutually exclusive with AnchorEventID.
	Year  *int32 `json:"year"`
	Month *int32 `json:"month"`
	Day   *int32 `json:"day"`
	// Relative anchoring — set this OR absolute year/month/day, not both.
	AnchorEventID    *uuid.UUID `json:"anchor_event_id"`
	AnchorOffsetYear *int32     `json:"anchor_offset_year"`
	AnchorOffsetMonth *int32    `json:"anchor_offset_month"`
	AnchorOffsetDay  *int32     `json:"anchor_offset_day"`
}

type UpdateTimelineEventRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Era         *string `json:"era"`
	// Absolute date fields — mutually exclusive with AnchorEventID.
	Year  *int32 `json:"year"`
	Month *int32 `json:"month"`
	Day   *int32 `json:"day"`
	// Relative anchoring — set this OR absolute year/month/day, not both.
	AnchorEventID    *uuid.UUID `json:"anchor_event_id"`
	AnchorOffsetYear *int32     `json:"anchor_offset_year"`
	AnchorOffsetMonth *int32    `json:"anchor_offset_month"`
	AnchorOffsetDay  *int32     `json:"anchor_offset_day"`
}

type TimelineEventResponse struct {
	ID          uuid.UUID  `json:"id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	EntityID    *uuid.UUID `json:"entity_id,omitempty"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Era         string     `json:"era"`
	// Resolved absolute date (computed from anchor chain if anchored).
	Year  *int32 `json:"year,omitempty"`
	Month *int32 `json:"month,omitempty"`
	Day   *int32 `json:"day,omitempty"`
	// Anchor metadata — present when this event uses relative anchoring.
	AnchorEventID    *uuid.UUID `json:"anchor_event_id,omitempty"`
	AnchorOffsetYear *int32     `json:"anchor_offset_year,omitempty"`
	AnchorOffsetMonth *int32    `json:"anchor_offset_month,omitempty"`
	AnchorOffsetDay  *int32     `json:"anchor_offset_day,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ========================
// Composite responses
// ========================

// GraphResponse is returned by GET /wiki/graph.
// The frontend can use entities + relationships to render a node-edge diagram.
type GraphResponse struct {
	Entities      []EntityResponse       `json:"entities"`
	Relationships []RelationshipResponse `json:"relationships"`
}
