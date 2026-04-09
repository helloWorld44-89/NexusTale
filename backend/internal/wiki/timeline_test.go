package wiki

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

func ptr32(v int32) *int32 { return &v }

func makeEvent(id, anchorID *uuid.UUID, year, month, day, offY, offM, offD *int32, era string) sqlcgen.WikiTimelineEvent {
	e := sqlcgen.WikiTimelineEvent{ID: *id, Era: era}
	if anchorID != nil {
		e.AnchorEventID = pgtype.UUID{Bytes: [16]byte(*anchorID), Valid: true}
	}
	if year != nil {
		e.Year = pgtype.Int4{Int32: *year, Valid: true}
	}
	if month != nil {
		e.Month = pgtype.Int4{Int32: *month, Valid: true}
	}
	if day != nil {
		e.Day = pgtype.Int4{Int32: *day, Valid: true}
	}
	if offY != nil {
		e.AnchorOffsetYear = pgtype.Int4{Int32: *offY, Valid: true}
	}
	if offM != nil {
		e.AnchorOffsetMonth = pgtype.Int4{Int32: *offM, Valid: true}
	}
	if offD != nil {
		e.AnchorOffsetDay = pgtype.Int4{Int32: *offD, Valid: true}
	}
	return e
}

func TestResolveEvents_Absolute(t *testing.T) {
	id := uuid.New()
	rows := []sqlcgen.WikiTimelineEvent{
		makeEvent(&id, nil, ptr32(500), ptr32(3), ptr32(10), nil, nil, nil, "Age of Ash"),
	}
	result, err := ResolveEvents(rows)
	if err != nil {
		t.Fatal(err)
	}
	if *result[0].Year != 500 || *result[0].Month != 3 || *result[0].Day != 10 {
		t.Errorf("unexpected date: %v %v %v", result[0].Year, result[0].Month, result[0].Day)
	}
	if result[0].Era != "Age of Ash" {
		t.Errorf("unexpected era: %s", result[0].Era)
	}
}

func TestResolveEvents_AnchoredOffset(t *testing.T) {
	base := uuid.New()
	child := uuid.New()
	rows := []sqlcgen.WikiTimelineEvent{
		makeEvent(&base, nil, ptr32(100), ptr32(1), ptr32(1), nil, nil, nil, "Age of Stone"),
		makeEvent(&child, &base, nil, nil, nil, ptr32(50), ptr32(2), ptr32(5), ""),
	}
	result, err := ResolveEvents(rows)
	if err != nil {
		t.Fatal(err)
	}
	// Find child in result (order preserved).
	var cr *TimelineEventResponse
	for i := range result {
		if result[i].ID == child {
			cr = &result[i]
		}
	}
	if cr == nil {
		t.Fatal("child not found in result")
	}
	if *cr.Year != 150 || *cr.Month != 3 || *cr.Day != 6 {
		t.Errorf("resolved date wrong: year=%v month=%v day=%v", cr.Year, cr.Month, cr.Day)
	}
	// Era inherited from anchor.
	if cr.Era != "Age of Stone" {
		t.Errorf("era not inherited: %s", cr.Era)
	}
}

func TestResolveEvents_EraOverride(t *testing.T) {
	base := uuid.New()
	child := uuid.New()
	rows := []sqlcgen.WikiTimelineEvent{
		makeEvent(&base, nil, ptr32(200), nil, nil, nil, nil, nil, "Age of Stone"),
		makeEvent(&child, &base, nil, nil, nil, ptr32(10), nil, nil, "Age of Fire"),
	}
	result, err := ResolveEvents(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range result {
		if r.ID == child && r.Era != "Age of Fire" {
			t.Errorf("own era should take precedence, got: %s", r.Era)
		}
	}
}

func TestResolveEvents_ChainOfThree(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	c := uuid.New()
	rows := []sqlcgen.WikiTimelineEvent{
		makeEvent(&a, nil, ptr32(0), nil, nil, nil, nil, nil, ""),
		makeEvent(&b, &a, nil, nil, nil, ptr32(10), nil, nil, ""),
		makeEvent(&c, &b, nil, nil, nil, ptr32(10), nil, nil, ""),
	}
	result, err := ResolveEvents(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range result {
		if r.ID == c && *r.Year != 20 {
			t.Errorf("chain resolution wrong: got year %v, want 20", *r.Year)
		}
	}
}

func TestResolveEvents_CycleDetected(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	rows := []sqlcgen.WikiTimelineEvent{
		makeEvent(&a, &b, nil, nil, nil, ptr32(1), nil, nil, ""),
		makeEvent(&b, &a, nil, nil, nil, ptr32(1), nil, nil, ""),
	}
	_, err := ResolveEvents(rows)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}
