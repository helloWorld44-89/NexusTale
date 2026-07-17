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
