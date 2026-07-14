package wiki

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// ResolveEvents computes resolved absolute dates for a flat list of timeline
// events, handling relative anchoring ("Event B happens 3 years after Event A").
//
// Resolution rules:
//   - Events without an anchor_event_id are already absolute; year/month/day
//     are used as-is.
//   - Anchored events compute their absolute date as:
//       resolved_year  = anchor.resolved_year  + anchor_offset_year
//       resolved_month = anchor.resolved_month + anchor_offset_month
//       resolved_day   = anchor.resolved_day   + anchor_offset_day
//     Offsets default to 0 when NULL. No calendar normalization is applied
//     (month=15 is valid for fictional calendars).
//   - Era is inherited from the anchor chain when the event's own era is empty.
//   - Cycles (A anchors B anchors A) are detected via DFS and returned as an
//     error naming the offending event.
//
// The returned slice is in the same order as the input.
func ResolveEvents(rows []sqlcgen.WikiTimelineEvent) ([]TimelineEventResponse, error) {
	// Index by ID for O(1) anchor lookups.
	byID := make(map[uuid.UUID]*sqlcgen.WikiTimelineEvent, len(rows))
	for i := range rows {
		byID[rows[i].ID] = &rows[i]
	}

	// resolved caches already-computed results to avoid redundant DFS traversals.
	type resolved struct {
		year  *int32
		month *int32
		day   *int32
		era   string
	}
	cache := make(map[uuid.UUID]*resolved, len(rows))

	// maxAnchorDepth caps the DFS chain length to prevent stack overflow from
	// pathologically deep (but non-cyclic) anchor chains. 50 levels covers any
	// realistic timeline ("The war ended 3 years after the treaty, which was 2
	// years after…") while bounding recursion to a safe depth.
	const maxAnchorDepth = 50

	// visiting tracks the current DFS path for cycle detection.
	visiting := make(map[uuid.UUID]bool, len(rows))

	var resolve func(id uuid.UUID, depth int) (*resolved, error)
	resolve = func(id uuid.UUID, depth int) (*resolved, error) {
		if depth > maxAnchorDepth {
			return nil, fmt.Errorf("anchor chain exceeds maximum depth (%d) at event %s — check for an extremely long chain", maxAnchorDepth, id)
		}
		if r, ok := cache[id]; ok {
			return r, nil
		}
		if visiting[id] {
			return nil, fmt.Errorf("cycle detected at timeline event %s", id)
		}
		e, ok := byID[id]
		if !ok {
			// Anchor points outside this project's event list — treat as unresolvable.
			return nil, fmt.Errorf("anchor event %s not found in project", id)
		}

		visiting[id] = true
		defer func() { visiting[id] = false }()

		var r resolved

		if !e.AnchorEventID.Valid {
			// Absolute event — use stored values directly.
			if e.Year.Valid {
				v := e.Year.Int32
				r.year = &v
			}
			if e.Month.Valid {
				v := e.Month.Int32
				r.month = &v
			}
			if e.Day.Valid {
				v := e.Day.Int32
				r.day = &v
			}
			r.era = e.Era
		} else {
			anchorID := uuid.UUID(e.AnchorEventID.Bytes)
			anchor, err := resolve(anchorID, depth+1)
			if err != nil {
				return nil, err
			}

			// Apply integer offsets. A missing anchor date component stays nil
			// unless an offset is specified (offset on nil = treat anchor as 0).
			offsetYear := int32(0)
			if e.AnchorOffsetYear.Valid {
				offsetYear = e.AnchorOffsetYear.Int32
			}
			offsetMonth := int32(0)
			if e.AnchorOffsetMonth.Valid {
				offsetMonth = e.AnchorOffsetMonth.Int32
			}
			offsetDay := int32(0)
			if e.AnchorOffsetDay.Valid {
				offsetDay = e.AnchorOffsetDay.Int32
			}

			if anchor.year != nil || e.AnchorOffsetYear.Valid {
				base := int32(0)
				if anchor.year != nil {
					base = *anchor.year
				}
				v := base + offsetYear
				r.year = &v
			}
			if anchor.month != nil || e.AnchorOffsetMonth.Valid {
				base := int32(0)
				if anchor.month != nil {
					base = *anchor.month
				}
				v := base + offsetMonth
				r.month = &v
			}
			if anchor.day != nil || e.AnchorOffsetDay.Valid {
				base := int32(0)
				if anchor.day != nil {
					base = *anchor.day
				}
				v := base + offsetDay
				r.day = &v
			}

			// Era: own era takes precedence; fall back to anchor's era.
			if e.Era != "" {
				r.era = e.Era
			} else {
				r.era = anchor.era
			}
		}

		cache[id] = &r
		return &r, nil
	}

	result := make([]TimelineEventResponse, len(rows))
	for i, e := range rows {
		r, err := resolve(e.ID, 0)
		if err != nil {
			return nil, err
		}

		resp := TimelineEventResponse{
			ID:          e.ID,
			ProjectID:   e.ProjectID,
			Name:        e.Name,
			Description: e.Description,
			Era:         r.era,
			Year:        r.year,
			Month:       r.month,
			Day:         r.day,
			CreatedAt:   e.CreatedAt.Time,
			UpdatedAt:   e.UpdatedAt.Time,
		}
		if e.EntityID.Valid {
			id := uuid.UUID(e.EntityID.Bytes)
			resp.EntityID = &id
		}
		if e.AnchorEventID.Valid {
			id := uuid.UUID(e.AnchorEventID.Bytes)
			resp.AnchorEventID = &id
			if e.AnchorOffsetYear.Valid {
				v := e.AnchorOffsetYear.Int32
				resp.AnchorOffsetYear = &v
			}
			if e.AnchorOffsetMonth.Valid {
				v := e.AnchorOffsetMonth.Int32
				resp.AnchorOffsetMonth = &v
			}
			if e.AnchorOffsetDay.Valid {
				v := e.AnchorOffsetDay.Int32
				resp.AnchorOffsetDay = &v
			}
		}

		result[i] = resp
	}
	return result, nil
}
