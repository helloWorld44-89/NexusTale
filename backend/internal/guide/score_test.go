package guide

import (
	"testing"
)

// allStructures is a minimal fake catalog matching the 12 seeded names.
var allStructures = []Structure{
	{ID: "id-three-act", Name: sThreeAct},
	{ID: "id-heros-journey", Name: sHerosJourney},
	{ID: "id-save-the-cat", Name: sSaveTheCat},
	{ID: "id-five-act", Name: sFiveAct},
	{ID: "id-fichtean", Name: sFichtean},
	{ID: "id-mystery", Name: sMystery},
	{ID: "id-heist", Name: sHeist},
	{ID: "id-quest", Name: sQuest},
	{ID: "id-spiral", Name: sSpiral},
	{ID: "id-kishotenketsu", Name: sKishotenketsu},
	{ID: "id-braided", Name: sBraided},
	{ID: "id-nonlinear", Name: sNonlinear},
}

func TestScore_EmptyAnswers_ReturnsEmpty(t *testing.T) {
	got := Score(map[string][]string{}, allStructures)
	if len(got) != 0 {
		t.Errorf("expected empty result, got %d entries", len(got))
	}
}

func TestScore_HerosJourneyProfile(t *testing.T) {
	// Answers heavily weighted toward Hero's Journey.
	answers := map[string][]string{
		"q1": {"epic", "emotional"},             // +3 HJ, +2 Quest, +2 HJ, +2 Spiral
		"q2": {"character_transformation"},      // +3 HJ, +2 Spiral
		"q5": {"fundamental_transformation"},    // +4 HJ
		"q6": {"transformation"},               // +2 HJ
		"q7": {"ordinary_life"},                // +2 HJ
	}
	// Expected HJ score: 3+2+3+4+2+2 = 16
	got := Score(answers, allStructures)
	if len(got) == 0 {
		t.Fatal("expected results, got empty")
	}
	if got[0].Name != sHerosJourney {
		t.Errorf("expected primary = Hero's Journey, got %q", got[0].Name)
	}
	if got[0].IsSecondary {
		t.Error("primary result should not be marked secondary")
	}
}

func TestScore_MysteryProfile(t *testing.T) {
	answers := map[string][]string{
		"q1": {"suspenseful"},    // +2 Mystery, +1 Fichtean
		"q2": {"mystery"},        // +4 Mystery
		"q5": {"little_change"},  // +3 Mystery
		"q10": {"mystery"},       // +4 Mystery
	}
	// Mystery: 2+4+3+4 = 13; Fichtean: 1
	got := Score(answers, allStructures)
	if len(got) == 0 {
		t.Fatal("expected results, got empty")
	}
	if got[0].Name != sMystery {
		t.Errorf("expected primary = Mystery, got %q", got[0].Name)
	}
}

func TestScore_FreeformProfile_BelowThreshold(t *testing.T) {
	// Only freestyle answer — contributes nothing.
	answers := map[string][]string{
		"q9": {"freestyle"},
	}
	got := Score(answers, allStructures)
	if len(got) != 0 {
		t.Errorf("expected empty (freeform), got %d entries", len(got))
	}
}

func TestScore_SecondaryStructure(t *testing.T) {
	// Three-Act profile with some Save the Cat overlap.
	answers := map[string][]string{
		"q1": {"fast_paced"},          // +2 Fichtean, +1 STC
		"q2": {"external_plot"},       // +2 Three-Act, +2 STC
		"q3": {"one_central"},         // +3 Three-Act
		"q4": {"gradual"},             // +2 Three-Act
		"q9": {"very_clear"},          // +3 STC
	}
	// Three-Act: 2+3+2 = 7; Save the Cat: 1+2+3 = 6; Fichtean: 2
	got := Score(answers, allStructures)
	if len(got) == 0 {
		t.Fatal("expected results, got empty")
	}
	primary := got[0]
	if primary.IsSecondary {
		t.Error("top result should not be secondary")
	}

	// At least one secondary should exist (STC is ≥70% of top score).
	hasSecondary := false
	for _, s := range got[1:] {
		if s.IsSecondary {
			hasSecondary = true
			break
		}
	}
	if !hasSecondary {
		t.Error("expected at least one secondary structure")
	}
}

func TestScore_PrimaryNotMarkedSecondary(t *testing.T) {
	answers := map[string][]string{
		"q2": {"mystery"}, // +4 Mystery — enough to qualify
		"q5": {"little_change"}, // +3 Mystery
	}
	got := Score(answers, allStructures)
	if len(got) == 0 {
		t.Fatal("expected results, got empty")
	}
	if got[0].IsSecondary {
		t.Error("primary (highest score) must never be marked secondary")
	}
}

func TestScore_ResultsAreSorted(t *testing.T) {
	answers := map[string][]string{
		"q1": {"epic", "emotional"},
		"q2": {"character_transformation"},
		"q5": {"fundamental_transformation"},
	}
	got := Score(answers, allStructures)
	for i := 1; i < len(got); i++ {
		if got[i].Score > got[i-1].Score {
			t.Errorf("results not sorted: index %d (%d pts) > index %d (%d pts)",
				i, got[i].Score, i-1, got[i-1].Score)
		}
	}
}

func TestScore_UnknownStructureNameIgnored(t *testing.T) {
	// Pass a structures list that is missing some names — should not panic.
	partial := []Structure{
		{ID: "id-mystery", Name: sMystery},
	}
	answers := map[string][]string{
		"q2": {"mystery"}, // +4 Mystery
		"q5": {"little_change"}, // +3 Mystery
	}
	got := Score(answers, partial)
	if len(got) != 1 || got[0].Name != sMystery {
		t.Errorf("unexpected result: %+v", got)
	}
}
