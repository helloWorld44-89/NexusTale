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

**Tertiary — AI-skeptic writers (valid segment, different message):**
- Writers who are opposed to or uncomfortable with AI on ethical, craft, or data-ownership grounds
- Scrivener / World Anvil expats who are frustrated that tools they trusted are forcing AI on them
- Privacy-conscious writers who won't put an unpublished manuscript on a cloud server they don't control
- Self-hosters and open-source advocates who overlap with the writing community
- These writers do not need AI to get full value from NexusTale — Chronicle, wiki, collaboration, and export stand entirely on their own. See the AI-Skeptic Messaging section for how to reach them.

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

## Segment: AI-Skeptic Writers

This segment requires a different message, different communities, and different content — not a different product. NexusTale's architecture genuinely supports what they care about. No spin required.

### Who they are

There are four distinct sub-groups with different concerns. Don't treat them as one monolith.

| Sub-group | What they actually fear | What resolves it |
|---|---|---|
| **Craft purists** | AI will hollow out their voice; the work won't feel earned | AI is never automatic — it only activates on an explicit request |
| **Ethics objectors** | Their writing feeds training data they didn't consent to | BYOK = their key, their agreement with the provider; Ollama = never leaves their machine |
| **Data / ownership anxious** | Their unpublished novel lives on a server they don't control | Git-first architecture, export-always, AGPL license, self-host option |
| **Quality skeptics** | AI produces flat generic prose | They don't have to use it; the tool is full-featured without it |

The mistake is trying to convert any of them. Don't argue about AI. Argue about the manuscript management problem — which is real for all of them regardless of their AI position.

### Core message for this segment

> "A home for your novel that isn't trying to write it for you."

This works because it's true. Chronicle, the wiki, collaboration, and export have nothing to do with AI. AI features are available for writers who want them; they are off by default and require an explicit action to use.

**Message hierarchy — in this order for this audience:**

1. **Chronicle first** — "Save points for your novel. Branch into a what-if without losing your draft. Travel back to any version you've ever saved." This resonates with any writer who has lost work or been afraid to revise. Zero AI involved.

2. **World wiki** — "Your lore lives next to your prose, not in a separate tab you'll forget to update." World Anvil users who are anti-AI but frustrated with the disconnect from their manuscript are a direct target.

3. **Data ownership** — "Your manuscript is always exportable. Self-host it entirely. The source code is on GitHub under AGPL." Addresses the surveillance and lock-in concern directly.

4. **AI is opt-in and explicit** — Only mention this near the bottom, and only to defuse the concern, not to sell: *"If you want AI features, they're there. They require your own API key. We don't run AI in the background. We don't train on your work."*

### The Ollama angle

Ollama support is a complete answer to the data sovereignty concern — a writer using a local model sends zero manuscript content to any external server. This deserves its own explicit statement in copy targeting this segment:

> "Run NexusTale with a local AI model. Your manuscript never leaves your machine."

This is a genuine differentiator. Sudowrite, NovelAI, and every AI-first writing tool cannot say this.

### The AGPL / self-host angle

Anti-AI writers and self-hosters overlap heavily — both tend to be technically-oriented, privacy-conscious, and skeptical of VC-backed tools that will eventually remove features or sell data. The AGPL license and self-host option are trust signals that no major writing tool currently offers:

> "We open-sourced NexusTale. If you don't trust our hosted service, run it yourself."

This isn't a feature to build before using this message — it's a statement to make once the AGPL license is in place (see MONETIZATION_PLAN.md self-hosting section).

### Where to find them and what to say

| Community | Lead with | Avoid |
|---|---|---|
| r/writing | Chronicle + wiki. Mention AI only if asked. | Any "AI-powered" framing |
| r/fantasywriters, r/scifiwriting | Chronicle origin story. World wiki for lore-heavy writers. | AI features |
| Mastodon / fediverse | Self-host + AGPL. Fediverse writers are disproportionately privacy-conscious and open-source-aware. | Hosted cloud service as the default pitch |
| World Anvil Discord | Wiki + manuscript disconnect. Many WA users are frustrated non-AI users already paying for two apps. | — |
| r/selfhosted | AGPL + Docker Compose + Ollama. Rare crossover with writers but a passionate one. | Anything that sounds like SaaS lock-in |
| Authors Guild / SFWA adjacent | Data ownership + export-always + no training on manuscripts. These organizations have taken formal anti-AI positions. | "AI-powered" or anything that implies NexusTale generates content automatically |

### What not to do

- Do not argue that AI is good, actually — it doesn't matter if you're right; you will lose the room
- Do not sneak them into AI features ("just try it once") — this is exactly the behavior they're fleeing from in other tools
- Do not use "AI-powered" in any copy targeting this segment — Chronicle, wiki, and git are the headline
- Do not associate with public figures who are visibly AI-enthusiast — let the product speak for itself
- Do not imply their concerns are irrational — the ethical concerns around training data are legitimate and widely shared

### Build-in-public content that resonates

These posts work for the general audience too — they're not niche. The anti-AI crowd will find them via community participation; the pro-AI crowd will also appreciate the Chronicle and ownership framing.

- *"We built version control for novels — no AI required"* — the Chronicle origin story reframed
- *"Why NexusTale works completely offline"* — Ollama + self-host walk-through
- *"Your novel belongs to you: git-first architecture and what it means for writers"*
- *"We open-sourced NexusTale"* — the AGPL announcement post (publish when license is added to the repo)
- *"Why we didn't build AI autocomplete"* — on the deliberate choice to make AI explicit and opt-in rather than ambient

### Waitlist segmentation note

The waitlist "what do you write?" field doesn't capture AI attitude. Consider adding a second optional question at some point: *"Do you currently use AI tools in your writing? (yes / no / curious but haven't tried)"* — this lets you segment the cohort and send different invite messaging to each group. Do not add this to the current waitlist form (keep it short); revisit for the beta public sign-up.

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

| Community | Angle | AI mention? |
|---|---|---|
| r/worldbuilding | Worldbuilding tools, wiki design, keeping lore consistent | Optional |
| r/fantasywriters | Writing process, outlining tools, revision workflows | Optional |
| r/scifiwriting | Same as above; science worldbuilding angle | Optional |
| r/writing | Broader audience; useful for positioning | Never lead with it |
| r/selfpublish | Workflow efficiency, Scrivener-expat angle | Optional |
| NaNoWriMo forums (nanowrimo.org) | Outline tools, beat structure, word count tracking | Optional |
| World Anvil Discord | Direct overlap: writers who want wiki + manuscript together | Never lead with it |
| Mastodon / fediverse writing community | Self-host, AGPL, data ownership, Ollama — this crowd is waiting for a tool like this | Never |
| r/selfhosted | AGPL + Docker Compose + writer crossover | Never |

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

- [ ] Landing page with feature highlights and pricing preview (use tier names from `MONETIZATION_PLAN.md`: Inkwell / Scribe / Chronicler / Studio)
- [ ] Product Hunt launch prepared (tagline, gallery, first comment)
- [ ] Press list: writing tool blogs (The Write Practice, Jane Friedman, Writer's Digest), indie author newsletters
- [ ] OpenAPI spec complete (required before external beta clients)
- [ ] Billing wired (Stripe + plan-check middleware — see `docs/MONETIZATION_PLAN.md` for full implementation checklist)
- [ ] Lifetime deal page/section live and clearly time-capped (200 buyers hard cap; communicate this publicly)
- [ ] Public sign-up enabled (remove invite gate)

### 3.2 Beta positioning shift

For beta, the story changes from "built in public" to "trusted by writers":
- Lead with alpha testimonials and workflows (with permission)
- Shift content toward "how to use NexusTale for X" guides
- Product Hunt + Hacker News "Show HN" on launch day

### 3.3 Pricing messaging

**Model to communicate:** platform subscription — writers bring their own API keys. NexusTale never marks up AI costs.

**Lead with the free tier honestly:** Inkwell is genuinely useful for short-form work; the upgrade reason is natural (2-project / 3-chapter limit surfaces when a writer actually needs more, not before).

**Lifetime deal framing:** "We're opening beta and 200 early supporters can lock in a lifetime price. We won't do this again." No artificial urgency language — the hard cap is the urgency.

**Comparison angles to use in copy:**
- vs. Scrivener: "Chronicle + wiki + AI — Scrivener can't do any of these."
- vs. Sudowrite: "NexusTale knows your whole story. And you can export your manuscript any time."
- vs. World Anvil + Scrivener together: "One app. One subscription."

**What not to say:** do not compare token prices or claim NexusTale is "cheaper than ChatGPT" — the value prop is the platform, not the AI cost.

---

## Content calendar skeleton

Use this as a repeating template once in the pre-launch phase:

| Week | Content | Primary audience |
|---|---|---|
| Week 1 | "Why version control for novels" — the Chronicle origin story | All writers, anti-AI friendly |
| Week 2 | Feature clip: wiki entity graph (d3 visualization) | Worldbuilders, World Anvil expats |
| Week 3 | "How async collaboration works for co-authors" | Co-authors, editors |
| Week 4 | Feature clip: Workshop agent mode writing a scene | AI-curious writers |
| Week 5 | "Why NexusTale works completely offline" — Ollama + self-host walk-through | Anti-AI / privacy-conscious crowd |
| Week 6 | Feature clip: Chronicle timeline + TravelTo | All writers, anti-AI friendly |
| Week 7 | "We open-sourced NexusTale" — AGPL announcement (publish when license lands) | Self-hosters, fediverse, anti-AI crowd |
| Week 8 | "Why we didn't build AI autocomplete" — on making AI explicit and opt-in | Anti-AI crowd + earns trust from skeptics |
| Repeat | Rotate: origin story → feature clip → how-it-works article | — |

**Note on audience balance:** roughly half the content calendar should work for the AI-skeptic segment without any mention of AI — Chronicle, wiki, collaboration, ownership, and self-hosting posts all qualify. This isn't a concession; it's accurate positioning. AI is one feature, not the product.

---

## Budget

Alpha marketing budget: **$0**. Organic community participation, build-in-public posts, and personal outreach only.

Pre-beta budget (when ready): small spend on a proper landing page design + optional paid Product Hunt featured slot (~$100–200). No ad spend until proven retention from organic.

---

*See also: PROJECT_PLAN.md Phase C+ (alpha gate) and Alpha Release Plan for the technical readiness checklist.*
