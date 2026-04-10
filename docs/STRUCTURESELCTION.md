
# Novel Structure Recommendation & Scoring System
*For AI-assisted novel writing applications*

This document defines:
- A standardized question set for authors
- A weighted scoring system that maps answers to novel structures
- A method for selecting primary and secondary (borrowed) structures
- A framework for freestyle and hybrid story designs

The intent is **guidance, not enforcement**.  
All recommendations are suggestions and may be overridden by the author at any time.

---

## Part I: Author Question Set

Authors may answer quickly, skip questions, or revise later.
Unanswered questions simply contribute no score.

---

### A. Desired Reader Experience

**Q1. What reading experience do you want to create?**  
(select all that apply)

- ☐ Fast-paced / page-turner
- ☐ Suspenseful / tense
- ☐ Emotional / introspective
- ☐ Epic / mythic
- ☐ Cozy / low-stakes
- ☐ Experimental / unconventional

---

### B. Story Driver

**Q2. What primarily drives your story?**

- ☐ External plot and events
- ☐ Character transformation
- ☐ A mystery or question
- ☐ Theme, mood, or contrast
- ☐ A plan, mission, or objective

---

### C. Conflict & Stakes

**Q3. How central is conflict?**

- ☐ One clear central conflict
- ☐ Many escalating conflicts
- ☐ Ongoing crises with little downtime
- ☐ Minimal or indirect conflict

**Q4. How do stakes change over time?**

- ☐ Gradual escalation
- ☐ Constant high pressure
- ☐ Episodic challenges
- ☐ Mostly internal or emotional

---

### D. Character Arc

**Q5. How much does the protagonist change?**

- ☐ Fundamental personal transformation
- ☐ Moderate growth
- ☐ Little to no change
- ☐ Moral or psychological decline

**Q6. What matters more: success or transformation?**

- ☐ Transformation
- ☐ Success
- ☐ Both equally

---

### E. Pacing & Shape

**Q7. How should the story begin?**

- ☐ Calm setup
- ☐ Immediate action
- ☐ Story already in motion
- ☐ Ordinary life before disruption

**Q8. How should the story end?**

- ☐ Clean resolution
- ☐ Bittersweet resolution
- ☐ Ambiguous ending
- ☐ Tragic or inevitable outcome

---

### F. Structural Preference

**Q9. How much structure do you want?**

- ☐ Very clear acts and beats
- ☐ Loose guidance
- ☐ Minimal structure (freestyle)

**Q10. Are you intentionally borrowing from any structure?**  
(optional)

- ☐ Three-Act
- ☐ Hero’s Journey
- ☐ Mystery
- ☐ Heist
- ☐ Save the Cat
- ☐ Other / Custom

---

## Part II: Scoring Matrix

Each answer contributes points to one or more structures.
Multiple structures may accumulate points simultaneously.

---

### Core Structures Tracked

- **Three-Act Structure**
- **Hero’s Journey**
- **Save the Cat**
- **Fichtean Curve**
- **Mystery / Investigation**
- **Heist**
- **Quest**
- **Spiral / Descent**
- **Kishōtenketsu**
- **Multiple-POV / Braided**
- **Nonlinear / Mosaic**

---

### Example Scoring Rules (Representative)

> Scores are additive.  
> Skipped questions add 0 points.

---

#### A. Reading Experience

- Fast-paced → +2 Fichtean, +1 Save the Cat
- Suspenseful → +2 Mystery, +1 Fichtean
- Emotional / introspective → +2 Hero’s Journey, +2 Spiral
- Epic / mythic → +3 Hero’s Journey, +2 Quest
- Cozy / low-stakes → +2 Kishōtenketsu
- Experimental → +3 Nonlinear, +2 Kishōtenketsu

---

#### B. Story Driver

- External plot → +2 Three-Act, +2 Save the Cat
- Character transformation → +3 Hero’s Journey, +2 Spiral
- Mystery/question → +4 Mystery
- Theme/contrast → +3 Kishōtenketsu, +2 Literary
- Plan/mission → +3 Heist, +2 Quest

---

#### C. Conflict & Stakes

- One central conflict → +3 Three-Act
- Many escalating conflicts → +2 Five-Act, +2 Braided
- Constant crises → +4 Fichtean
- Minimal conflict → +4 Kishōtenketsu

- Gradual escalation → +2 Three-Act
- Constant pressure → +3 Fichtean
- Episodic challenges → +3 Quest
- Mostly internal → +3 Spiral

---

#### D. Character Arc

- Fundamental transformation → +4 Hero’s Journey
- Moderate growth → +2 Three-Act
- No change → +3 Mystery (classic detective)
- Decline → +4 Spiral / Descent

- Transformation > Success → +2 Hero’s Journey
- Success > Transformation → +2 Heist
- Equal → +2 Three-Act

---

#### E. Pacing & Shape

- Calm setup → +2 Three-Act
- Immediate action → +3 Fichtean
- Already in motion → +3 In Medias Res
- Ordinary life → +2 Hero’s Journey

- Clean resolution → +2 Three-Act
- Bittersweet → +2 Five-Act
- Ambiguous → +3 Nonlinear
- Tragic → +3 Spiral

---

#### F. Structural Preference

- Very clear beats → +3 Save the Cat
- Loose guidance → +2 Three-Act
- Freestyle → +3 Custom / Hybrid (no default winner)

---

## Part III: Recommendation Logic

### Primary Structure
- The structure with the **highest total score**
- Requires a minimum threshold (e.g. 6 points)
- Otherwise defaults to **Freestyle**

### Secondary / Borrowed Structures
- Any structure within **70–80% of the top score**
- The AI may suggest:
  - Borrowed beats
  - Optional pacing checks
  - Hybrid outlines

---

## Part IV: Hybrid & Freestyle Mode

If the author selects **Freestyle** or defines a custom structure:

The AI should ask (optionally):

- How many acts or phases?
- What marks the midpoint?
- What changes by the end?
- Any rules the AI should follow or avoid?

The AI should then:
- Treat the author’s answers as canonical
- Use existing structures only as *inspiration*
- Avoid forcing external beats

---

## Part V: AI Usage Guidelines

The structure system should be used to:

- Suggest outlines and turning points
- Generate context-aware prompts
- Flag pacing issues (soft warnings only)
- Adapt suggestions as the story evolves

### The system must NOT:
- Enforce beats rigidly
- Block author choices
- Override explicit instructions

---

## Summary

This scoring system enables:
- Transparent recommendations
- Multi-structure blending
- Author-led control
- AI guidance without rigidity

**Structure is a tool, not a rule.**
