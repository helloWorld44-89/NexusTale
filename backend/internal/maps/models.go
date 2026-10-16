package maps

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// defaultLayout is written to git on map creation — an empty two-array
// layout matching the shape D-Map M2's canvas editor will populate.
const defaultLayout = `{"regions":[],"symbols":[]}`

type CreateMapRequest struct {
	Name           string  `json:"name" binding:"required"`
	MapType        string  `json:"map_type" binding:"required,oneof=world region city galaxy planet custom"`
	ParentEntityID *string `json:"parent_entity_id"`
}

type UpdateMapRequest struct {
	Name    *string          `json:"name"`
	MapType *string          `json:"map_type" binding:"omitempty,oneof=world region city galaxy planet custom"`
	Layout  *json.RawMessage `json:"layout"`
}

// MapResponse is the wire shape for a map. Layout is omitted (nil) in list
// responses — fetching it requires a git read per row, so ListMaps skips it,
// matching wiki.toEntityResponse's list-vs-detail split.
type MapResponse struct {
	ID             uuid.UUID       `json:"id"`
	ProjectID      uuid.UUID       `json:"project_id"`
	ParentEntityID *uuid.UUID      `json:"parent_entity_id,omitempty"`
	Name           string          `json:"name"`
	MapType        string          `json:"map_type"`
	Layout         json.RawMessage `json:"layout,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type mapAttrs struct {
	MapType string `json:"map_type"`
}

// Point, MapRegion, MapSymbol, and Layout mirror the frontend's
// MapRegion/MapSymbol/MapLayout shapes in frontend/src/services/api.ts —
// structured Go types over the otherwise-opaque layout JSON so the AI tool
// executor (D-Map M3) can decode, mutate, and re-encode a map's layout
// instead of hand-rolling JSON patches.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type MapRegion struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	Color  string  `json:"color"`
	Points []Point `json:"points"`
}

type MapSymbol struct {
	ID       string  `json:"id"`
	IconType string  `json:"icon_type"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Scale    float64 `json:"scale"`
	Rotation float64 `json:"rotation"`
	RegionID string  `json:"region_id,omitempty"`
	EntityID string  `json:"entity_id,omitempty"`
}

type Layout struct {
	Regions []MapRegion `json:"regions"`
	Symbols []MapSymbol `json:"symbols"`
}
