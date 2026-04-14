package guide

import "sort"

// Structure is the minimal DB record the scoring function needs.
// The handler fetches all novel_structures rows and converts them before calling Score.
type Structure struct {
	ID   string
	Name string
}

// ScoreRequest holds author answers keyed by question ID.
// Each value is a slice because multi-select questions (Q1, Q10) are allowed.
// Unanswered questions simply contribute 0 points.
type ScoreRequest struct {
	Answers map[string][]string `json:"answers"`
}

// StructureScore is one entry in the ranked recommendation list.
type StructureScore struct {
	StructureID string `json:"structure_id"`
	Name        string `json:"name"`
	Score       int    `json:"score"`
	IsSecondary bool   `json:"is_secondary"`
}

// ── Internal keys ─────────────────────────────────────────────────────────────
// These match the names seeded in migration 015 so the name→ID lookup is stable.

const (
	sThreeAct      = "Three-Act Structure"
	sHerosJourney  = "Hero's Journey"
	sSaveTheCat    = "Save the Cat Beat Sheet"
	sFiveAct       = "Five-Act Structure"
	sFichtean      = "Fichtean Curve"
	sMystery       = "Mystery / Investigation Structure"
	sHeist         = "Heist Structure"
	sQuest         = "Quest Structure"
	sSpiral        = "Spiral / Descent Structure"
	sKishotenketsu = "Kishōtenketsu"
	sBraided       = "Multiple-POV / Braided Structure"
	sNonlinear     = "Nonlinear / Mosaic Structure"
)

// scoreMatrix maps question_key → answer_value → structure_name → points.
// Skipped questions contribute 0 — just absent from the map.
var scoreMatrix = map[string]map[string]map[string]int{
	// Q1 — Desired reading experience (multi-select)
	"q1": {
		"fast_paced":   {sFichtean: 2, sSaveTheCat: 1},
		"suspenseful":  {sMystery: 2, sFichtean: 1},
		"emotional":    {sHerosJourney: 2, sSpiral: 2},
		"epic":         {sHerosJourney: 3, sQuest: 2},
		"cozy":         {sKishotenketsu: 2},
		"experimental": {sNonlinear: 3, sKishotenketsu: 2},
	},
	// Q2 — Primary story driver
	"q2": {
		"external_plot":            {sThreeAct: 2, sSaveTheCat: 2},
		"character_transformation": {sHerosJourney: 3, sSpiral: 2},
		"mystery":                  {sMystery: 4},
		"theme_contrast":           {sKishotenketsu: 3, sFiveAct: 2},
		"plan_mission":             {sHeist: 3, sQuest: 2},
	},
	// Q3 — How central is conflict?
	"q3": {
		"one_central":     {sThreeAct: 3},
		"many_escalating": {sFiveAct: 2, sBraided: 2},
		"constant_crises": {sFichtean: 4},
		"minimal":         {sKishotenketsu: 4},
	},
	// Q4 — How do stakes change?
	"q4": {
		"gradual":           {sThreeAct: 2},
		"constant_pressure": {sFichtean: 3},
		"episodic":          {sQuest: 3},
		"mostly_internal":   {sSpiral: 3},
	},
	// Q5 — How much does the protagonist change?
	"q5": {
		"fundamental_transformation": {sHerosJourney: 4},
		"moderate_growth":            {sThreeAct: 2},
		"little_change":              {sMystery: 3},
		"moral_decline":              {sSpiral: 4},
	},
	// Q6 — What matters more: success or transformation?
	"q6": {
		"transformation": {sHerosJourney: 2},
		"success":        {sHeist: 2},
		"both":           {sThreeAct: 2},
	},
	// Q7 — How should the story begin?
	"q7": {
		"calm_setup":        {sThreeAct: 2},
		"immediate_action":  {sFichtean: 3},
		"already_in_motion": {sFichtean: 2},
		"ordinary_life":     {sHerosJourney: 2},
	},
	// Q8 — How should the story end?
	"q8": {
		"clean":       {sThreeAct: 2},
		"bittersweet": {sFiveAct: 2},
		"ambiguous":   {sNonlinear: 3},
		"tragic":      {sSpiral: 3},
	},
	// Q9 — How much structure do you want?
	"q9": {
		"very_clear": {sSaveTheCat: 3},
		"loose":      {sThreeAct: 2},
		// "freestyle" intentionally contributes nothing — signals freeform intent
	},
	// Q10 — Are you borrowing from any structure? (multi-select; direct +4 boost)
	"q10": {
		"three_act":     {sThreeAct: 4},
		"heros_journey": {sHerosJourney: 4},
		"mystery":       {sMystery: 4},
		"heist":         {sHeist: 4},
		"save_the_cat":  {sSaveTheCat: 4},
		// "other" / "custom" intentionally contributes nothing
	},
}

const minThreshold = 6 // primary structure must reach this to qualify

// Score runs the deterministic scoring matrix against the author's answers and
// returns structures ranked by score. Structures below minThreshold are excluded.
// If no structure qualifies, the returned slice is empty — the caller should
// treat that as "freeform recommended."
//
// Secondary structures are those within 70% of the top score (and ≥ minThreshold).
func Score(answers map[string][]string, structures []Structure) []StructureScore {
	// Build name → ID lookup from the provided DB rows.
	idByName := make(map[string]string, len(structures))
	for _, s := range structures {
		idByName[s.Name] = s.ID
	}

	// Accumulate scores.
	totals := make(map[string]int)
	for qKey, answerMap := range scoreMatrix {
		for _, answer := range answers[qKey] {
			for structName, pts := range answerMap[answer] {
				totals[structName] += pts
			}
		}
	}

	// Find the highest score.
	top := 0
	for _, score := range totals {
		if score > top {
			top = score
		}
	}

	// Nothing qualifies.
	if top < minThreshold {
		return []StructureScore{}
	}

	// Collect qualifying structures.
	secondaryThreshold := int(float64(top) * 0.70)
	var results []StructureScore
	for name, score := range totals {
		if score < minThreshold {
			continue
		}
		id, ok := idByName[name]
		if !ok {
			continue // structure not in DB (shouldn't happen after seed)
		}
		results = append(results, StructureScore{
			StructureID: id,
			Name:        name,
			Score:       score,
			IsSecondary: score < top && score >= secondaryThreshold,
		})
	}

	// Sort: primary first (score desc), then secondary (score desc), then by name for stability.
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Name < results[j].Name
	})

	return results
}
