-- novel_structures: seeded template library — never modified by user actions.
-- phases JSONB is an ordered array of {name, description, hints} objects.
CREATE TABLE novel_structures (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    phases      JSONB       NOT NULL DEFAULT '[]',
    strengths   TEXT        NOT NULL DEFAULT '',
    risks       TEXT        NOT NULL DEFAULT '',
    sort_order  INT         NOT NULL DEFAULT 0
);

-- projects gains a nullable structure selection.
-- structure_id = NULL and structure_custom = NULL means freeform (no structure).
ALTER TABLE projects
    ADD COLUMN structure_id     UUID REFERENCES novel_structures(id) ON DELETE SET NULL,
    ADD COLUMN structure_custom JSONB;

CREATE INDEX novel_structures_sort_order_idx ON novel_structures(sort_order);
CREATE INDEX projects_structure_id_idx       ON projects(structure_id) WHERE structure_id IS NOT NULL;

-- ── Seed the 12 template structures ──────────────────────────────────────────
-- Dollar-quoting ($phases$) used for phases values to avoid escaping single
-- quotes inside JSON hint strings.

INSERT INTO novel_structures (name, description, phases, strengths, risks, sort_order)
VALUES (
  'Three-Act Structure',
  'The universal scaffold: Setup → Conflict → Resolution. Works across virtually every genre.',
  $phases$[
    {"name": "Act I – Setup",          "description": "Introduce protagonist, setting, and central problem. Inciting incident disrupts the status quo.", "hints": "Establish what your character wants and what is at stake."},
    {"name": "Act II – Confrontation", "description": "Escalating obstacles and complications. Midpoint revelation or reversal.", "hints": "Each scene should raise the stakes or reveal new information."},
    {"name": "Act III – Resolution",   "description": "Climax resolves the main conflict. Denouement establishes a new normal.", "hints": "Pay off every promise made in Act I."}
  ]$phases$,
  'Flexible, intuitive, and widely understood',
  'Can feel predictable if beats are too rigid',
  1
);

INSERT INTO novel_structures (name, description, phases, strengths, risks, sort_order)
VALUES (
  'Hero''s Journey',
  'Personal transformation through departure, initiation, and return. Ideal for epic and mythic stories.',
  $phases$[
    {"name": "Ordinary World",             "description": "Establish the hero in their familiar environment.", "hints": "Show what the hero stands to lose or leave behind."},
    {"name": "Call to Adventure",          "description": "An event disrupts the ordinary world and presents a challenge.", "hints": "Make the call feel inevitable in hindsight."},
    {"name": "Refusal & Mentor",           "description": "Hero hesitates; a mentor figure provides guidance or a gift.", "hints": "Refusal reveals the hero's flaw or fear."},
    {"name": "Crossing the Threshold",     "description": "Hero commits to the journey and enters the special world.", "hints": "The point of no return — raise the stakes here."},
    {"name": "Trials, Allies, Enemies",    "description": "Hero faces tests, meets helpers, and confronts enemies.", "hints": "Each trial should teach the hero something about themselves."},
    {"name": "Ordeal",                     "description": "Crisis — symbolic death and resurrection.", "hints": "The darkest moment; the hero must transform or fail."},
    {"name": "Reward & The Road Back",     "description": "Hero seizes the reward and begins the return journey.", "hints": "The reward may be external (object) or internal (wisdom)."},
    {"name": "Return with the Elixir",     "description": "Hero returns transformed and brings something back for their world.", "hints": "Show how the ordinary world has changed — or how the hero sees it differently."}
  ]$phases$,
  'Powerful emotional and thematic arc with deep mythic resonance',
  'Overuse and cliché if followed slavishly; poorly suited to non-transformational stories',
  2
);

INSERT INTO novel_structures (name, description, phases, strengths, risks, sort_order)
VALUES (
  'Save the Cat Beat Sheet',
  'Emotional beats at precise story percentages. Excellent for commercial fiction and fast-paced narratives.',
  $phases$[
    {"name": "Opening Image",          "description": "A single scene that sets tone, mood, and the hero's starting state.", "hints": "This image should mirror — and contrast — the Final Image."},
    {"name": "Theme Stated",           "description": "Someone (not the hero) states what the story is really about.", "hints": "The hero usually disagrees or ignores this at first."},
    {"name": "Set-Up & Catalyst",      "description": "Introduce the world; the catalyst (around 10%) disrupts everything.", "hints": "The catalyst must force the hero to act."},
    {"name": "Debate",                 "description": "Hero resists the change; internal or external deliberation.", "hints": "End the debate with a clear decision to commit."},
    {"name": "Break into Act II",      "description": "Hero enters a new world — physically, emotionally, or both.", "hints": "The hero is proactive now, not reactive."},
    {"name": "Fun & Games / Midpoint", "description": "The promise of the premise plays out. Midpoint is a false victory or defeat.", "hints": "Midpoint raises the stakes so that fun and games can no longer sustain the story."},
    {"name": "Bad Guys Close In",      "description": "Pressure mounts; internal conflicts and external antagonists intensify.", "hints": "Peel away the hero's support systems one by one."},
    {"name": "All Is Lost",            "description": "The lowest point — everything the hero wanted seems gone.", "hints": "Include a whiff of death — literal or metaphorical."},
    {"name": "Dark Night of the Soul", "description": "Hero sits with failure before finding the final solution.", "hints": "The answer must come from within, not from outside."},
    {"name": "Break into Act III",     "description": "Hero applies the theme to solve the problem in a new way.", "hints": "The fix must be a synthesis of what the hero has learned."},
    {"name": "Finale",                 "description": "Hero storms the castle; new world order is established.", "hints": "All subplots resolve; show the hero's transformation clearly."},
    {"name": "Final Image",            "description": "Mirror of the Opening Image — proof the hero has changed.", "hints": "Should feel earned, not just symmetrical."}
  ]$phases$,
  'Excellent pacing and strong reader engagement through precise emotional beats',
  'Can feel mechanical or formulaic if beats are treated as hard requirements',
  3
);

INSERT INTO novel_structures (name, description, phases, strengths, risks, sort_order)
VALUES (
  'Five-Act Structure',
  'Extended dramatic arc with space for consequences and reflection. Rooted in classical tragedy.',
  $phases$[
    {"name": "Act I – Exposition",      "description": "Introduce characters, setting, and the central tension.", "hints": "Establish the world before disrupting it."},
    {"name": "Act II – Rising Action",  "description": "Complications multiply; protagonist is drawn deeper into conflict.", "hints": "Each scene should make the problem harder to solve."},
    {"name": "Act III – Climax",        "description": "The turning point; maximum tension and decisive confrontation.", "hints": "The climax should feel both surprising and inevitable."},
    {"name": "Act IV – Falling Action", "description": "Immediate consequences of the climax play out.", "hints": "Falling action slows the pace — use it for character reflection."},
    {"name": "Act V – Resolution",      "description": "Final state of the world and characters is established.", "hints": "Leave room for ambiguity if the story calls for it."}
  ]$phases$,
  'Allows space for consequences, reflection, and moral complexity',
  'Slower pacing; less suited to thrillers or fast commercial fiction',
  4
);

INSERT INTO novel_structures (name, description, phases, strengths, risks, sort_order)
VALUES (
  'Fichtean Curve',
  'Constant escalation via crises. Opens in the middle of action with minimal setup.',
  $phases$[
    {"name": "Initial Crisis",           "description": "Begin in media res — immediate tension before any exposition.", "hints": "Trust the reader; backstory can trickle in later."},
    {"name": "Rising Complications",     "description": "A series of escalating crises, each harder than the last.", "hints": "No crisis should be resolved without creating a new problem."},
    {"name": "Climax",                   "description": "The final and most intense crisis; all threads converge.", "hints": "The climax should feel earned by the relentless build."},
    {"name": "Brief Resolution",         "description": "Short denouement — the curve ends, not lingers.", "hints": "Cut early; the reader does not need everything explained."}
  ]$phases$,
  'Relentless momentum; keeps readers turning pages',
  'Limited space for character introspection or slow world-building',
  5
);

INSERT INTO novel_structures (name, description, phases, strengths, risks, sort_order)
VALUES (
  'Mystery / Investigation Structure',
  'Question → Clues → Truth. Built for detective fiction, noir, and cozy mysteries.',
  $phases$[
    {"name": "The Crime / Mystery",  "description": "Introduce the central question or crime that must be solved.", "hints": "Plant all clues fairly — the reader should be able to solve it."},
    {"name": "Investigation Begins", "description": "Protagonist starts gathering evidence; false leads appear.", "hints": "Red herrings should feel plausible, not cheap."},
    {"name": "Rising Danger",        "description": "The investigation attracts opposition; stakes increase.", "hints": "Someone else may be threatened — raise the cost of failure."},
    {"name": "Dark Moment",          "description": "Protagonist hits a wall or faces a serious setback.", "hints": "The solution must be earned, not handed over."},
    {"name": "Revelation",           "description": "Truth is exposed in a satisfying, logical reveal.", "hints": "The reveal must be surprising yet inevitable in hindsight."},
    {"name": "Resolution",           "description": "Justice, escape, or moral ambiguity — the world rebalances.", "hints": "Decide in advance: will justice be served?"}
  ]$phases$,
  'Strong reader engagement through curiosity and fair-play logic',
  'Ending must feel earned; retroactive plot holes destroy trust',
  6
);

INSERT INTO novel_structures (name, description, phases, strengths, risks, sort_order)
VALUES (
  'Heist Structure',
  'Plan → Execute → Complicate → Reveal. High tension, ensemble casts, and clever misdirection.',
  $phases$[
    {"name": "The Score",              "description": "Introduce the target and the stakes — why this heist, why now.", "hints": "Make the target feel impossible. That is the point."},
    {"name": "Team Assembly",          "description": "Recruit the crew; establish each member's role and quirk.", "hints": "Each crew member should have a skill and a flaw."},
    {"name": "The Plan",               "description": "Walk through what is supposed to happen.", "hints": "The audience should believe the plan is airtight."},
    {"name": "Execution & Escalation", "description": "The plan begins — and immediately starts going wrong.", "hints": "Every complication should feel like it could end everything."},
    {"name": "Reversal / True Plan",   "description": "A twist reveals the real plan beneath the visible one.", "hints": "The twist must be planted early to feel fair."},
    {"name": "Aftermath",              "description": "The crew deals with consequences — success or failure.", "hints": "Heists rarely end cleanly; decide who pays the price."}
  ]$phases$,
  'High tension with satisfying misdirection and ensemble dynamics',
  'Requires airtight logic; plot holes are catastrophic in this structure',
  7
);

INSERT INTO novel_structures (name, description, phases, strengths, risks, sort_order)
VALUES (
  'Quest Structure',
  'Reach a goal through episodic trials. Clear objective, series of obstacles, final confrontation.',
  $phases$[
    {"name": "The Call & Objective",     "description": "Protagonist is given or chooses a clear goal.", "hints": "The goal should feel both achievable and far away."},
    {"name": "Departure",                "description": "Protagonist leaves the known world to pursue the quest.", "hints": "Show what is being left behind and why it matters."},
    {"name": "Trials & Companions",      "description": "A series of obstacles; allies and enemies appear.", "hints": "Vary the nature of each trial — not every challenge is physical."},
    {"name": "Lowest Point",             "description": "The quest seems lost; the protagonist must recommit.", "hints": "What the protagonist chooses here defines their character."},
    {"name": "Final Confrontation",      "description": "The goal is within reach — one last obstacle stands in the way.", "hints": "Use everything the protagonist has learned across the trials."},
    {"name": "Return or Transformation", "description": "The protagonist completes (or abandons) the quest; the world has changed.", "hints": "What did the journey cost? Was it worth it?"}
  ]$phases$,
  'Easily expandable; intuitive structure for adventure and fantasy',
  'Episodic weakness without a strong character arc threading the trials together',
  8
);

INSERT INTO novel_structures (name, description, phases, strengths, risks, sort_order)
VALUES (
  'Spiral / Descent Structure',
  'Gradual loss of control. Ideal for horror, tragedy, and psychological fiction.',
  $phases$[
    {"name": "Apparent Stability",       "description": "Establish the protagonist's normal life — often deceptively comfortable.", "hints": "Plant the seeds of the eventual collapse here."},
    {"name": "First Compromise",         "description": "A small moral or practical concession — easy to justify.", "hints": "The first step down must feel completely reasonable."},
    {"name": "Escalating Consequences",  "description": "Each compromise requires a larger one; the spiral tightens.", "hints": "Show the protagonist rationalizing at each step."},
    {"name": "Point of No Return",       "description": "The protagonist crosses a line they cannot uncross.", "hints": "This is often the emotional climax, before the literal one."},
    {"name": "Collapse or Transformation","description": "Irreversible breakdown, death, or fundamental change.", "hints": "Earned tragedy requires the audience to understand why it was inevitable."}
  ]$phases$,
  'Creates dread and inevitability; powerful emotional impact',
  'Emotionally heavy; bleak endings can alienate readers if not handled with care',
  9
);

INSERT INTO novel_structures (name, description, phases, strengths, risks, sort_order)
VALUES (
  'Kishōtenketsu',
  'Contrast rather than conflict. A four-phase structure rooted in East Asian narrative tradition.',
  $phases$[
    {"name": "Ki – Introduction", "description": "Establish the world, characters, and situation without conflict.", "hints": "Create comfort and familiarity — the reader should feel settled."},
    {"name": "Shō – Development", "description": "Deepen the world; explore characters and themes without forcing conflict.", "hints": "Resist the urge to introduce an antagonist here."},
    {"name": "Ten – Twist",       "description": "An unexpected element reframes everything established so far.", "hints": "The twist is contrast, not conflict — it should feel like a revelation, not a fight."},
    {"name": "Ketsu – Harmony",   "description": "The story synthesizes the old world with the new perspective.", "hints": "Resolution comes from insight, not victory."}
  ]$phases$,
  'Subtle and reflective; uniquely suited to slice-of-life and contemplative literary fiction',
  'Low external tension; conflict-driven readers may find it unsatisfying',
  10
);

INSERT INTO novel_structures (name, description, phases, strengths, risks, sort_order)
VALUES (
  'Multiple-POV / Braided Structure',
  'Separate arcs converge. Ideal for epics, political sagas, and stories with broad scope.',
  $phases$[
    {"name": "Thread Introduction",  "description": "Introduce each POV character and their separate world.", "hints": "Each thread needs its own stakes — not just context for the others."},
    {"name": "Parallel Development", "description": "Threads develop independently; thematic echoes emerge.", "hints": "Use thematic mirroring to tie threads together before they meet."},
    {"name": "Convergence Points",   "description": "Threads begin to intersect — causally or thematically.", "hints": "Every intersection should change at least one thread permanently."},
    {"name": "Collision / Climax",   "description": "All threads arrive at the same crisis simultaneously.", "hints": "Each POV character should face a unique version of the central conflict."},
    {"name": "Resolution",           "description": "Each thread resolves; the braided whole resolves.", "hints": "Some threads may end tragically while others succeed — that is the strength of this structure."}
  ]$phases$,
  'Extraordinary depth and scope; allows the story to inhabit multiple worlds simultaneously',
  'Requires strict discipline; reader confusion is a constant risk without clear thread differentiation',
  11
);

INSERT INTO novel_structures (name, description, phases, strengths, risks, sort_order)
VALUES (
  'Nonlinear / Mosaic Structure',
  'Meaning emerges from arrangement. Ideal for memory-driven, experimental, or trauma narratives.',
  $phases$[
    {"name": "Anchoring Fragment",    "description": "A scene or image that orients the reader despite the non-chronological order.", "hints": "Give the reader a hook to hold onto before pulling them through time."},
    {"name": "Fragmented Development","description": "Out-of-order scenes accumulate meaning through juxtaposition.", "hints": "Every juxtaposition is an argument — ask what each pairing says."},
    {"name": "Emergent Pattern",      "description": "The reader begins to understand the shape of the whole.", "hints": "Trust the reader to make connections; do not over-explain."},
    {"name": "Revelation or Synthesis","description": "The final arrangement makes meaning that linear order could not.", "hints": "The ending should make the reader want to re-read the beginning."}
  ]$phases$,
  'Mirrors memory, trauma, and subjective experience in ways linear structure cannot',
  'High cognitive load; risks losing readers who prefer conventional narrative flow',
  12
);
