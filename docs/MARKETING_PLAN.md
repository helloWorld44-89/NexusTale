# NexusTale — Alpha Marketing Plan

A living document for recruiting the alpha cohort and building the audience that will carry NexusTale to beta. Updated as strategy evolves.

---

## Goal

Recruit **20–50 engaged writers** for the invite-only alpha. Quality over quantity — one writer who finishes the guide wizard and sends feedback is worth twenty who sign up and never log in.

**Not the goal:** viral reach, press coverage, or revenue. Those belong to beta. Alpha is about learning.

---

## Target audience

**Primary — the ideal alpha writer:**
- Writes sci-fi or fantasy at novel length (not flash, not short stories)
- Has outgrown Google Docs or Notion for manuscript organization
- Already uses a dedicated writing tool (Scrivener, Obsidian, World Anvil) — understands the category
- Curious about AI assistance but not dependent on it (has their own API keys or uses Ollama)
- Tolerant of rough edges; willing to give detailed feedback

**Secondary:**
- Co-writing duos or author + editor pairs who want async review tooling
- Writers who've struggled to keep their world wiki and manuscript in sync across separate apps

**Not the target for alpha:**
- Romance / thriller / literary fiction — the tools are genre-neutral but the marketing will resonate more with worldbuilding-heavy writers first
- Writers who expect a polished consumer product with zero friction

---

## Positioning

**One-liner:** "The writing tool built like a writer thinks — branching timelines, living worlds, and an AI that knows your story."

**Core differentiators to lead with:**

| Feature | How to frame it for writers |
|---|---|
| Chronicle / TravelTo | "Save points for your novel. Branch into a 'what if' without losing your draft." |
| World wiki wired to manuscript | "One app, not five. Your entities, magic rules, and timeline live next to the prose." |
| AI with story context | "Nexus has read your whole manuscript. It won't suggest a character who died in chapter 3." |
| Async collaboration | "Co-authors on different schedules. Reviewers who annotate without editing." |
| Always exportable | "Your manuscript is always yours — download as Markdown, Word, or EPUB any time." |

**Avoid:** "AI writing tool" — too generic and triggers skepticism. Lead with the manuscript and world management; AI is a supporting feature.

---

## Phase 1 — Pre-launch (now → alpha ready)

Alpha is currently blocked on the git-first architecture migration (see PROJECT_PLAN.md Phase C+ gate). Use this time to build audience so invites go out to warm leads, not cold email.

### 1.1 Build in public

Short posts documenting the build — what was shipped, why a decision was made, what broke. Framed as engineering + craft intersection, not just a changelog.

**Where:**
- GitHub Releases (zero-friction, already part of the repo workflow)
- A simple static page at the alpha domain (reuse `/about` until a proper landing page exists)
- Twitter/X and Bluesky — short-form clips of features in action (Chronicle timeline, wiki graph, Workshop agent run)

**Cadence:** once a week when possible; skip weeks rather than post filler.

**Topics that resonate with the target audience:**
- "Why we built version control for novels"
- "How the world wiki stays in sync with the prose"
- "Building an AI that reads your manuscript before helping write it"
- "Async collaboration for co-authors who aren't in the same timezone"

### 1.2 Community presence

Participate organically — answer questions, share perspective, mention NexusTale only when directly relevant. Do not spam.

**Primary communities:**

| Community | Angle |
|---|---|
| r/worldbuilding | Worldbuilding tools, wiki design, keeping lore consistent |
| r/fantasywriters | Writing process, outlining tools, revision workflows |
| r/scifiwriting | Same as above; science worldbuilding angle |
| r/writing | Broader audience; useful for positioning |
| r/selfpublish | Workflow efficiency, Scrivener-expat angle |
| NaNoWriMo forums (nanowrimo.org) | Outline tools, beat structure, word count tracking |
| World Anvil Discord | Direct overlap: writers who want wiki + manuscript together |

**What not to do:** create posts that are purely promotional; mention NexusTale in unrelated threads; post the same content to multiple subreddits simultaneously.

### 1.3 Waitlist

A signup form — email + "what do you write?" — live at `/` (the public landing page).

**Goal before opening alpha:** 100–200 interested writers on the list.

**Implementation (done):**
- `POST /api/v1/waitlist` endpoint — no auth required; upserts by email so duplicate submissions are harmless
- `waitlist_signups` table (migration 030): `id`, `email UNIQUE`, `what_they_write`, `created_at`
- Public landing page at `/` with hero, feature highlights, known limitations, and the waitlist form
- Unauthenticated visitors land at `/`; authenticated users are redirected to `/dashboard`
- Query `SELECT * FROM waitlist_signups ORDER BY created_at` to review and segment signups before sending invites

Use the "what do you write?" field to filter and segment cohorts — fantasy/sci-fi writers get first invites.

---

## Phase 2 — Alpha launch

### 2.1 Cohort strategy

Roll out in waves of **10–15 writers** with ~2 weeks between waves. This keeps feedback volume manageable and lets P0 bugs surface before the next cohort lands.

**Wave 1 (10 writers):**
- Prioritize: genre writers with worldbuilding focus, Scrivener expats, writers you can reach directly for detailed feedback
- Source: personal network + top of waitlist

**Wave 2 (15 writers):**
- Prioritize: co-writing pairs or author + editor pairs (exercises C3 collaboration path)
- Source: waitlist + targeted outreach in writing communities

**Wave 3 (25 writers):**
- Open to general waitlist; looser selection criteria
- Source: full waitlist

### 2.2 Invite email

Personal tone. No marketing language. What to include:
- Why they were chosen (brief, genuine)
- What alpha means: rough edges, no SLA, their feedback shapes the roadmap
- One sentence on what to try first (the guide wizard → write a scene → Chronicle)
- Direct `/invites/:token` link
- Where to send feedback: GitHub Issues link + (optionally) Discord invite

### 2.3 Feedback channels

| Channel | Purpose |
|---|---|
| GitHub Issues (already linked in app) | Bug reports, feature requests — structured and searchable |
| Discord server (alpha-only, optional) | Direct conversation, faster feedback loop, community between writers |
| Bi-weekly check-in email | "What's the most frustrating thing this week?" — 2 questions max |

**Discord is optional for alpha** — only worth creating if at least one writer is actively using it within the first week. Otherwise GitHub Issues is sufficient.

### 2.4 What to measure

These are the signals that tell you whether alpha is working, not vanity metrics:

| Signal | Target | Why it matters |
|---|---|---|
| Guide wizard completion rate | >50% of alpha users finish step 1 (Premise) | First meaningful engagement gate |
| Chronicle frequency | ≥1 Chronicle per writer per week | Writers finding value in version history |
| Weekly active writers | ≥60% of cohort active in week 2 | Retention, not just activation |
| Feedback volume | ≥1 issue/message per writer in first 2 weeks | Are they engaged enough to report? |
| Export usage | At least one export per writer | Trust signal — they have content worth exporting |
| Collaboration MR | ≥1 MR opened across the cohort | Exercises the full async collab path |

---

## Phase 3 — Beta transition

Triggered by alpha graduation criteria (see PROJECT_PLAN.md). Not a date — a milestone.

### 3.1 Pre-beta checklist

- [ ] Landing page with feature highlights and pricing preview
- [ ] Product Hunt launch prepared (tagline, gallery, first comment)
- [ ] Press list: writing tool blogs (The Write Practice, Jane Friedman, Writer's Digest), indie author newsletters
- [ ] OpenAPI spec complete (required before external beta clients)
- [ ] Billing wired (Stripe/Paddle + plan-check middleware per monetization section in PROJECT_PLAN.md)
- [ ] Public sign-up enabled (remove invite gate)

### 3.2 Beta positioning shift

For beta, the story changes from "built in public" to "trusted by writers":
- Lead with alpha testimonials and workflows (with permission)
- Shift content toward "how to use NexusTale for X" guides
- Product Hunt + Hacker News "Show HN" on launch day

---

## Content calendar skeleton

Use this as a repeating template once in the pre-launch phase:

| Week | Content |
|---|---|
| Week 1 | "Why version control for novels" — the Chronicle origin story |
| Week 2 | Feature clip: wiki entity graph (d3 visualization) |
| Week 3 | "How async collaboration works for co-authors" |
| Week 4 | Feature clip: Workshop agent mode writing a scene |
| Week 5 | "Building the AI that reads your manuscript" |
| Week 6 | Feature clip: Chronicle timeline + TravelTo |
| Repeat | Rotate: origin story → feature clip → how-it-works article |

---

## Budget

Alpha marketing budget: **$0**. Organic community participation, build-in-public posts, and personal outreach only.

Pre-beta budget (when ready): small spend on a proper landing page design + optional paid Product Hunt featured slot (~$100–200). No ad spend until proven retention from organic.

---

*See also: PROJECT_PLAN.md Phase C+ (alpha gate) and Alpha Release Plan for the technical readiness checklist.*
