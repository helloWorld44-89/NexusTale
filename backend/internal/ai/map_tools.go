package ai

// map_tools.go — Map layout tool definitions and executor for D-Map M3.
//
// Mirrors tools.go's ManuscriptTools pattern exactly: MapTools is the tool
// set exposed to the AI in a map-scoped agent chat (StreamMapChatWithTools),
// executeMapToolCall dispatches a single ToolCall against the active map's
// layout, and every mutating op snapshots the layout beforehand so the
// frontend can offer a one-shot Undo (full-layout restore via PUT, same as
// a manual Map Studio save — there is no per-field undo for maps).
//
// Unlike ManuscriptTools, every map tool operates implicitly on the single
// map bound to the chat session (projectID/userID/mapID come from the
// request, not from tool arguments) — the model never has to carry or
// guess a map_id across calls.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jconder44/nexustale/internal/ai/adapters"
	"github.com/jconder44/nexustale/internal/maps"
)

// MapLayoutStore is the subset of maps.Service the AI map-tools executor
// needs. Implemented by *maps.Service; injected via WithMaps.
type MapLayoutStore interface {
	GetLayout(ctx context.Context, projectID, userID, mapID uuid.UUID) (maps.Layout, error)
	SetLayout(ctx context.Context, projectID, userID, mapID uuid.UUID, layout maps.Layout) error
}

// MapTools defines the tools Nexus may call in a map-scoped agent chat.
// Mirrors exactly what D-Map M2's canvas editor lets a writer do by hand.
var MapTools = []adapters.ToolDefinition{
	{
		Name: "list_map_layout",
		Description: "Read the current regions and symbols on this map, including their IDs. " +
			"Call this FIRST before any update/remove operation so you have the correct IDs to target.",
		InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
	},
	{
		Name:        "add_region",
		Description: "Add a new shaped region to the map (e.g. a landmass, lake, forest, or territory outline).",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"region_type": {"type": "string", "description": "Region category, e.g. 'mountain', 'lake', 'forest', 'territory'"},
				"color":       {"type": "string", "description": "Fill color as a CSS color string, e.g. '#4ade80'"},
				"points":      {"type": "array", "items": {"type": "object", "properties": {"x": {"type": "number"}, "y": {"type": "number"}}, "required": ["x","y"]}, "description": "Polygon vertices, in order, on a 900x600 canvas. At least 3 points."}
			},
			"required": ["region_type", "color", "points"]
		}`),
	},
	{
		Name:        "update_region",
		Description: "Update an existing region's type, color, and/or shape. Call list_map_layout first to find the region_id.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"region_id":   {"type": "string", "description": "ID of the region to update"},
				"region_type": {"type": "string", "description": "New region type (omit to keep existing)"},
				"color":       {"type": "string", "description": "New fill color (omit to keep existing)"},
				"points":      {"type": "array", "items": {"type": "object", "properties": {"x": {"type": "number"}, "y": {"type": "number"}}, "required": ["x","y"]}, "description": "New polygon vertices (omit to keep existing)"}
			},
			"required": ["region_id"]
		}`),
	},
	{
		Name:        "remove_region",
		Description: "Remove a region from the map. Call list_map_layout first to find the region_id.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {"region_id": {"type": "string", "description": "ID of the region to remove"}},
			"required": ["region_id"]
		}`),
	},
	{
		Name: "add_symbol",
		Description: "Place a new icon symbol on the map (e.g. a city, mountain peak, star, or point of interest). " +
			"Optionally link it to an existing region (region_id) or a wiki entity/other map (entity_id) — " +
			"call list_map_layout or list_wiki_entities first if you need those IDs.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"icon_type": {"type": "string", "description": "Icon type, e.g. 'city', 'mountain', 'star', 'forest', 'ruins'"},
				"x":         {"type": "number", "description": "X position on a 900x600 canvas"},
				"y":         {"type": "number", "description": "Y position on a 900x600 canvas"},
				"scale":     {"type": "number", "description": "Icon scale, default 1"},
				"rotation":  {"type": "number", "description": "Icon rotation in degrees, default 0"},
				"region_id": {"type": "string", "description": "Optional region this symbol sits inside"},
				"entity_id": {"type": "string", "description": "Optional wiki entity or map this symbol links to"}
			},
			"required": ["icon_type", "x", "y"]
		}`),
	},
	{
		Name:        "update_symbol",
		Description: "Update an existing symbol's icon, position, scale, rotation, or links. Call list_map_layout first to find the symbol_id.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"symbol_id": {"type": "string", "description": "ID of the symbol to update"},
				"icon_type": {"type": "string", "description": "New icon type (omit to keep existing)"},
				"x":         {"type": "number", "description": "New X position (omit to keep existing)"},
				"y":         {"type": "number", "description": "New Y position (omit to keep existing)"},
				"scale":     {"type": "number", "description": "New scale (omit to keep existing)"},
				"rotation":  {"type": "number", "description": "New rotation in degrees (omit to keep existing)"},
				"region_id": {"type": "string", "description": "New region link (omit to keep existing)"},
				"entity_id": {"type": "string", "description": "New entity/map link (omit to keep existing)"}
			},
			"required": ["symbol_id"]
		}`),
	},
	{
		Name:        "remove_symbol",
		Description: "Remove a symbol from the map. Call list_map_layout first to find the symbol_id.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {"symbol_id": {"type": "string", "description": "ID of the symbol to remove"}},
			"required": ["symbol_id"]
		}`),
	},
}

// executeMapToolCall dispatches a single tool invocation against the map
// bound to this chat (projectID/userID/mapID), mirroring executeToolCall.
func (s *Service) executeMapToolCall(ctx context.Context, projectID, userID, mapID uuid.UUID, tc adapters.ToolCall) (adapters.ToolResult, ToolEvent) {
	evt, err := s.runMapTool(ctx, projectID, userID, mapID, tc)
	evt.Tool = tc.Name
	if err != nil {
		evt.Result = "Error: " + err.Error()
		evt.IsError = true
	}
	return adapters.ToolResult{ID: tc.ID, Content: evt.Result, IsError: evt.IsError}, evt
}

func (s *Service) runMapTool(ctx context.Context, projectID, userID, mapID uuid.UUID, tc adapters.ToolCall) (ToolEvent, error) {
	if s.mapStore == nil {
		return ToolEvent{}, fmt.Errorf("map tools are not configured on this server")
	}
	switch tc.Name {
	case "list_map_layout":
		return s.toolListMapLayout(ctx, projectID, userID, mapID)
	case "add_region":
		return s.toolAddRegion(ctx, projectID, userID, mapID, tc.Input)
	case "update_region":
		return s.toolUpdateRegion(ctx, projectID, userID, mapID, tc.Input)
	case "remove_region":
		return s.toolRemoveRegion(ctx, projectID, userID, mapID, tc.Input)
	case "add_symbol":
		return s.toolAddSymbol(ctx, projectID, userID, mapID, tc.Input)
	case "update_symbol":
		return s.toolUpdateSymbol(ctx, projectID, userID, mapID, tc.Input)
	case "remove_symbol":
		return s.toolRemoveSymbol(ctx, projectID, userID, mapID, tc.Input)
	default:
		return ToolEvent{}, fmt.Errorf("unknown map tool: %q", tc.Name)
	}
}

func (s *Service) toolListMapLayout(ctx context.Context, projectID, userID, mapID uuid.UUID) (ToolEvent, error) {
	layout, err := s.mapStore.GetLayout(ctx, projectID, userID, mapID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("read layout: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("Current map layout (use these IDs when targeting existing regions/symbols):\n\n")

	if len(layout.Regions) == 0 {
		sb.WriteString("No regions yet.\n")
	} else {
		fmt.Fprintf(&sb, "Regions (%d):\n", len(layout.Regions))
		for _, r := range layout.Regions {
			fmt.Fprintf(&sb, "  [%s] %q color=%s, %d points  (region_id: %s)\n", r.Type, r.Type, r.Color, len(r.Points), r.ID)
		}
	}

	if len(layout.Symbols) == 0 {
		sb.WriteString("No symbols yet.\n")
	} else {
		fmt.Fprintf(&sb, "Symbols (%d):\n", len(layout.Symbols))
		for _, sym := range layout.Symbols {
			link := ""
			if sym.RegionID != "" {
				link += " region_id=" + sym.RegionID
			}
			if sym.EntityID != "" {
				link += " entity_id=" + sym.EntityID
			}
			fmt.Fprintf(&sb, "  [%s] at (%.0f, %.0f)%s  (symbol_id: %s)\n", sym.IconType, sym.X, sym.Y, link, sym.ID)
		}
	}

	return ToolEvent{Result: sb.String()}, nil
}

func (s *Service) toolAddRegion(ctx context.Context, projectID, userID, mapID uuid.UUID, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		RegionType string       `json:"region_type"`
		Color      string       `json:"color"`
		Points     []maps.Point `json:"points"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}
	if len(args.Points) < 3 {
		return ToolEvent{}, fmt.Errorf("a region needs at least 3 points")
	}

	layout, err := s.mapStore.GetLayout(ctx, projectID, userID, mapID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("read layout: %w", err)
	}
	before, _ := json.Marshal(layout)

	region := maps.MapRegion{ID: uuid.NewString(), Type: args.RegionType, Color: args.Color, Points: args.Points}
	layout.Regions = append(layout.Regions, region)

	if err := s.mapStore.SetLayout(ctx, projectID, userID, mapID, layout); err != nil {
		return ToolEvent{}, fmt.Errorf("write layout: %w", err)
	}

	return ToolEvent{
		Result:       fmt.Sprintf("Added region %q (ID: %s) with %d points.", region.Type, region.ID, len(region.Points)),
		MapID:        mapID.String(),
		CreatedID:    region.ID,
		CreatedType:  "map_region",
		BeforeLayout: string(before),
	}, nil
}

func (s *Service) toolUpdateRegion(ctx context.Context, projectID, userID, mapID uuid.UUID, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		RegionID   string       `json:"region_id"`
		RegionType *string      `json:"region_type"`
		Color      *string      `json:"color"`
		Points     []maps.Point `json:"points"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}

	layout, err := s.mapStore.GetLayout(ctx, projectID, userID, mapID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("read layout: %w", err)
	}
	before, _ := json.Marshal(layout)

	idx := -1
	for i, r := range layout.Regions {
		if r.ID == args.RegionID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ToolEvent{}, fmt.Errorf("region not found: %s", args.RegionID)
	}

	if args.RegionType != nil {
		layout.Regions[idx].Type = *args.RegionType
	}
	if args.Color != nil {
		layout.Regions[idx].Color = *args.Color
	}
	if args.Points != nil {
		layout.Regions[idx].Points = args.Points
	}

	if err := s.mapStore.SetLayout(ctx, projectID, userID, mapID, layout); err != nil {
		return ToolEvent{}, fmt.Errorf("write layout: %w", err)
	}

	return ToolEvent{
		Result:       fmt.Sprintf("Updated region %s.", args.RegionID),
		MapID:        mapID.String(),
		BeforeLayout: string(before),
	}, nil
}

func (s *Service) toolRemoveRegion(ctx context.Context, projectID, userID, mapID uuid.UUID, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		RegionID string `json:"region_id"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}

	layout, err := s.mapStore.GetLayout(ctx, projectID, userID, mapID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("read layout: %w", err)
	}
	before, _ := json.Marshal(layout)

	kept := layout.Regions[:0:0]
	found := false
	for _, r := range layout.Regions {
		if r.ID == args.RegionID {
			found = true
			continue
		}
		kept = append(kept, r)
	}
	if !found {
		return ToolEvent{}, fmt.Errorf("region not found: %s", args.RegionID)
	}
	layout.Regions = kept

	if err := s.mapStore.SetLayout(ctx, projectID, userID, mapID, layout); err != nil {
		return ToolEvent{}, fmt.Errorf("write layout: %w", err)
	}

	return ToolEvent{
		Result:       fmt.Sprintf("Removed region %s.", args.RegionID),
		MapID:        mapID.String(),
		BeforeLayout: string(before),
	}, nil
}

func (s *Service) toolAddSymbol(ctx context.Context, projectID, userID, mapID uuid.UUID, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		IconType string  `json:"icon_type"`
		X        float64 `json:"x"`
		Y        float64 `json:"y"`
		Scale    float64 `json:"scale"`
		Rotation float64 `json:"rotation"`
		RegionID string  `json:"region_id"`
		EntityID string  `json:"entity_id"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}
	if args.Scale == 0 {
		args.Scale = 1
	}

	layout, err := s.mapStore.GetLayout(ctx, projectID, userID, mapID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("read layout: %w", err)
	}
	before, _ := json.Marshal(layout)

	symbol := maps.MapSymbol{
		ID: uuid.NewString(), IconType: args.IconType, X: args.X, Y: args.Y,
		Scale: args.Scale, Rotation: args.Rotation, RegionID: args.RegionID, EntityID: args.EntityID,
	}
	layout.Symbols = append(layout.Symbols, symbol)

	if err := s.mapStore.SetLayout(ctx, projectID, userID, mapID, layout); err != nil {
		return ToolEvent{}, fmt.Errorf("write layout: %w", err)
	}

	return ToolEvent{
		Result:       fmt.Sprintf("Added symbol %q (ID: %s) at (%.0f, %.0f).", symbol.IconType, symbol.ID, symbol.X, symbol.Y),
		MapID:        mapID.String(),
		CreatedID:    symbol.ID,
		CreatedType:  "map_symbol",
		BeforeLayout: string(before),
	}, nil
}

func (s *Service) toolUpdateSymbol(ctx context.Context, projectID, userID, mapID uuid.UUID, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		SymbolID string   `json:"symbol_id"`
		IconType *string  `json:"icon_type"`
		X        *float64 `json:"x"`
		Y        *float64 `json:"y"`
		Scale    *float64 `json:"scale"`
		Rotation *float64 `json:"rotation"`
		RegionID *string  `json:"region_id"`
		EntityID *string  `json:"entity_id"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}

	layout, err := s.mapStore.GetLayout(ctx, projectID, userID, mapID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("read layout: %w", err)
	}
	before, _ := json.Marshal(layout)

	idx := -1
	for i, sym := range layout.Symbols {
		if sym.ID == args.SymbolID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ToolEvent{}, fmt.Errorf("symbol not found: %s", args.SymbolID)
	}

	if args.IconType != nil {
		layout.Symbols[idx].IconType = *args.IconType
	}
	if args.X != nil {
		layout.Symbols[idx].X = *args.X
	}
	if args.Y != nil {
		layout.Symbols[idx].Y = *args.Y
	}
	if args.Scale != nil {
		layout.Symbols[idx].Scale = *args.Scale
	}
	if args.Rotation != nil {
		layout.Symbols[idx].Rotation = *args.Rotation
	}
	if args.RegionID != nil {
		layout.Symbols[idx].RegionID = *args.RegionID
	}
	if args.EntityID != nil {
		layout.Symbols[idx].EntityID = *args.EntityID
	}

	if err := s.mapStore.SetLayout(ctx, projectID, userID, mapID, layout); err != nil {
		return ToolEvent{}, fmt.Errorf("write layout: %w", err)
	}

	return ToolEvent{
		Result:       fmt.Sprintf("Updated symbol %s.", args.SymbolID),
		MapID:        mapID.String(),
		BeforeLayout: string(before),
	}, nil
}

func (s *Service) toolRemoveSymbol(ctx context.Context, projectID, userID, mapID uuid.UUID, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		SymbolID string `json:"symbol_id"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}

	layout, err := s.mapStore.GetLayout(ctx, projectID, userID, mapID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("read layout: %w", err)
	}
	before, _ := json.Marshal(layout)

	kept := layout.Symbols[:0:0]
	found := false
	for _, sym := range layout.Symbols {
		if sym.ID == args.SymbolID {
			found = true
			continue
		}
		kept = append(kept, sym)
	}
	if !found {
		return ToolEvent{}, fmt.Errorf("symbol not found: %s", args.SymbolID)
	}
	layout.Symbols = kept

	if err := s.mapStore.SetLayout(ctx, projectID, userID, mapID, layout); err != nil {
		return ToolEvent{}, fmt.Errorf("write layout: %w", err)
	}

	return ToolEvent{
		Result:       fmt.Sprintf("Removed symbol %s.", args.SymbolID),
		MapID:        mapID.String(),
		BeforeLayout: string(before),
	}, nil
}
