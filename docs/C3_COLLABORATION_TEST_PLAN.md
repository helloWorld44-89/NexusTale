# C3 Collaboration — Manual Test Plan

All tests are manual (browser + API). Run them in order within each phase; later phases depend on state set up by earlier ones. Each phase maps directly to the C3 implementation step of the same name.

---

## Test environment setup

Before any test phase, stand up the full stack:

```bash
make dev       # Postgres + Redis + MinIO
make run       # API on :8080
# frontend dev server or deployed build
```

### Accounts to create

Register all four accounts before starting. Keep these credentials handy throughout.

| Handle | Email | Password | Role in tests |
|---|---|---|---|
| **Owner** | owner@test.com | Test1234! | Project owner throughout |
| **CoAuthor** | coauthor@test.com | Test1234! | Invited as co-author |
| **Editor** | editor@test.com | Test1234! | Invited as editor |
| **Reviewer** | reviewer@test.com | Test1234! | Invited as reviewer |
| **Outsider** | outsider@test.com | Test1234! | Never invited — used for access-control tests |

### Project to create

Log in as **Owner** and create:
- **Title**: "The Shattered Meridian"
- Add at least one chapter and two scenes with real content (needed for later phases)
- Note the project ID from the URL — you will need it repeatedly

---

## Phase C3.0 — Collaborator roles + invite system

> **Status**: Implemented. Run these tests after deploying C3.0.

### 3.0-A — Happy-path invite and accept (per role)

Run this sequence three times: once for CoAuthor, once for Editor, once for Reviewer.

**As Owner:**

1. Navigate to `/projects/{id}` (ProjectHome).
2. Scroll to the **Collaborators** panel.
3. Click **+ Invite**.
4. Enter the collaborator's email and select the appropriate role.
5. Click **Send invite**.

**Expected:**
- A green invite row appears in the pending section (greyed out, labeled "pending").
- A blue invite-link box appears with a URL like `https://…/invites/{token}`.
- Clicking **Copy link** copies the URL to clipboard.

**As Collaborator (new browser tab / incognito):**

6. Open the invite URL.
7. **If not logged in**: you should see the accept page with project title, inviter name, and role description. A **Sign in to accept** button is shown.
8. Click **Sign in to accept** → you are taken to `/login?redirect=/invites/{token}`.
9. Log in as the collaborator.
10. You are redirected back to the invite accept page (still showing project info).
11. Click **Accept and join project**.

**Expected:**
- A brief "Joined! Redirecting…" message appears.
- After ~1.8 s you land on `/projects/{id}` (ProjectHome for that project).

**Back as Owner:**

12. Refresh ProjectHome.

**Expected:**
- The collaborator appears in the **Collaborators** panel with correct name, email, and role badge.
- The pending invite row for that email is gone.

**As Collaborator:**

13. Navigate to `/dashboard`.

**Expected:**
- "The Shattered Meridian" appears in the project list alongside any projects they own.

---

### 3.0-B — Invite accept while already logged in

1. Log in as **Reviewer** in a normal tab.
2. In the same tab, visit a fresh invite URL (create a new one as Owner if needed).
3. The accept page should show project info and an **Accept and join project** button immediately — no login step needed.
4. Accept.

**Expected:** Redirects to ProjectHome. Reviewer appears in collaborator list.

---

### 3.0-C — Invite validation — error cases

Run as **Owner** for each case. Each should show a clear error message in the invite form.

| # | Action | Expected error |
|---|---|---|
| C1 | Enter an email address not registered in NexusTale | "no NexusTale account found for …" |
| C2 | Invite an email that is already an active collaborator | "user is already a collaborator on this project" |
| C3 | Leave the email field blank and submit | Button should remain disabled / form won't submit |
| C4 | Enter an invalid role (via browser dev tools) | 400 from API |

---

### 3.0-D — Accept validation — error cases

| # | Setup | Action | Expected |
|---|---|---|---|
| D1 | Generate an invite, wait 7 days (or manually set `expires_at` to the past in DB) | Visit invite URL | "this invite has expired" |
| D2 | Accept a valid invite as the correct user | Visit the same invite URL again | "this invite has already been accepted" |
| D3 | Generate invite for editor@test.com | Log in as outsider@test.com and visit the invite URL | "this invite was sent to a different email address" |
| D4 | Generate invite for coauthor@test.com | Log in as coauthor@test.com (already a collaborator from 3.0-A) and visit the URL | "you are already a collaborator on this project" |

---

### 3.0-E — Role UI enforcement

**As Owner:**

1. Open ProjectHome → Collaborators panel.
2. Verify the **+ Invite** button is visible.
3. Verify each collaborator row has a **Remove** button.
4. Verify pending invite rows have a **Copy link** button.

**As CoAuthor (log in as them, navigate to the project):**

5. Open ProjectHome → Collaborators panel.
6. Verify **no** invite form or **+ Invite** button is shown.
7. Verify **no** Remove buttons are shown on any collaborator row.
8. Verify you can see the collaborator list (read-only).

Repeat step 5–8 for Editor and Reviewer.

---

### 3.0-F — Remove collaborator

1. Log in as **Owner**. Navigate to ProjectHome.
2. Click **Remove** next to the Reviewer's row.
3. Confirm the dialog.

**Expected:**
- Reviewer disappears from the collaborator list immediately.
- Log in as Reviewer → navigate to `/dashboard`.
- "The Shattered Meridian" is no longer in their project list.
- If Reviewer tries to navigate directly to `/projects/{id}`, the API returns 403 (the frontend redirects to dashboard or shows an error).

4. Re-invite Reviewer from Owner's ProjectHome.

**Expected:** Invite flow works normally — there is no "already a collaborator" error since the row was removed.

---

### 3.0-G — Access control (Outsider)

Log in as **Outsider** and attempt the following. Each should be blocked.

| # | Attempt | Expected |
|---|---|---|
| G1 | Navigate to `/projects/{id}` in browser | Redirected to dashboard (or 403 from API + frontend handles it) |
| G2 | `GET /api/v1/projects/{id}` with Outsider's JWT | 403 Forbidden |
| G3 | `GET /api/v1/projects/{id}/collaborators` with Outsider's JWT | 403 Forbidden |
| G4 | `POST /api/v1/projects/{id}/invites` with Outsider's JWT | 403 Forbidden |

---

### 3.0-H — Project list union

Precondition: CoAuthor has accepted an invite (from 3.0-A).

1. Log in as **CoAuthor**. Go to `/dashboard`.
2. "The Shattered Meridian" should appear in the list even though CoAuthor is not the owner.
3. CoAuthor creates their own project ("My Project").
4. Dashboard should show both "The Shattered Meridian" and "My Project".
5. Navigating to "The Shattered Meridian" should open ProjectHome normally.

---

## Phase C3.1 — Collaborator-scoped git operations

> **Status**: Not yet implemented. Fill in tests when C3.1 ships.

### Setup

Precondition: CoAuthor has an accepted invite on "The Shattered Meridian" with a clone at `repos/{projectId}-collab-{userId}/`.

### 3.1-A — Chronicle on collaborator branch

1. Log in as **CoAuthor**. Open the project in the Editor.
2. Create a new chapter: "Chapter 6 — The Fractured Reach".
3. Add a scene with ~200 words of content.
4. Chronicle (commit) from the git panel: note "Added Chapter 6".

**Expected:**
- Chronicle succeeds.
- The commit appears in CoAuthor's Lore (history) view.
- Owner's Lore (on canon branch) does NOT show this commit — the histories are independent.

### 3.1-B — Branch scope enforcement

1. As **CoAuthor**, attempt to Diverge to a branch named `canon` or a branch outside their prefix.

**Expected:** 403 — collaborator may only work within their assigned branch prefix.

### 3.1-C — Reviewer cannot Chronicle

1. Log in as **Reviewer**. Open the project.
2. Attempt to Chronicle.

**Expected:** 403 — reviewer role does not have Chronicle access.

---

## Phase C3.2 — Merge request system

> **Status**: Not yet implemented. Fill in tests when C3.2 ships.

### Setup

Precondition: CoAuthor has Chronicled new content on their branch (from C3.1-A).

### 3.2-A — Open a merge request

1. Log in as **CoAuthor**.
2. Navigate to the Merge Requests section of the project.
3. Click **New merge request**.
4. Title: "Add Chapter 6 — The Fractured Reach". Add a short description.
5. Submit.

**Expected:**
- Merge request appears in the list with status "open".
- Owner receives a notification (see C3.5).

### 3.2-B — View diff

1. Log in as **Owner**.
2. Open the merge request.
3. Click **View diff**.

**Expected:**
- Per-scene diff is shown for any scenes changed/added by CoAuthor.
- New scenes show as pure additions (green highlight).
- No "paradox" conflict indicator for a simple additive MR.

### 3.2-C — Approve and merge (no conflict)

1. As **Owner**, review the diff.
2. Click **Merge**.

**Expected:**
- MR status changes to "merged".
- CoAuthor's new chapter and scenes appear in the Owner's project explorer.
- The canon git branch advances to include CoAuthor's commits.

### 3.2-D — Reject a merge request

1. CoAuthor opens a new MR.
2. Owner opens the MR, clicks **Reject**, adds a reviewer note.

**Expected:**
- MR status changes to "rejected".
- Reviewer note is visible to CoAuthor.
- CoAuthor receives a notification.

### 3.2-E — Conflict detection (Paradox)

1. Owner edits a scene on canon and Chronicles.
2. CoAuthor edits the same scene on their branch and Chronicles.
3. CoAuthor opens a MR.
4. Owner opens the diff.

**Expected:**
- The conflicting scene is flagged with a conflict indicator.
- The "Merge" button is disabled until the conflict is resolved via the prose diff UI.

---

## Phase C3.3 — Prose diff + conflict resolution UI

> **Status**: Not yet implemented. Fill in tests when C3.3 ships.

### Setup

Precondition: A merge request exists with at least one conflicting scene (from C3.2-E).

### 3.3-A — Conflict resolution UI renders

1. Log in as **Owner**. Open the conflicting MR.
2. Open the diff view.

**Expected:**
- Each conflicting scene shows two columns: "Canon" (left) and "Co-author" (right).
- Changed words are highlighted: deletions in red-strikethrough, additions in green.
- Three resolution buttons per scene: **Keep Canon**, **Use Co-author**, **Edit manually**.
- **Merge** button is disabled until all scenes are resolved.

### 3.3-B — Resolve via "Keep Canon"

1. Click **Keep Canon** on a conflicting scene.

**Expected:**
- Scene is marked resolved (visual indicator).
- Canon text will be preserved after merge.

### 3.3-C — Resolve via "Use Co-author"

1. Click **Use Co-author** on a conflicting scene.

**Expected:**
- Scene is marked resolved.
- Co-author's text will be written to canon after merge.

### 3.3-D — Resolve via manual edit

1. Click **Edit** on a conflicting scene.
2. An inline editor opens with both versions blended or side-by-side.
3. Edit the text to a custom blend of both.
4. Click **Save resolution**.

**Expected:**
- Scene is marked resolved with a "custom" indicator.
- The custom text will be written to canon after merge.

### 3.3-E — Bulk accept

1. On a MR with 5+ conflicting scenes, click **Accept All Co-author**.

**Expected:**
- All scenes are marked resolved in one action.
- **Merge** button becomes enabled.

### 3.3-F — Merge after resolution

1. With all scenes resolved, click **Merge**.

**Expected:**
- MR status → "merged".
- Canon scenes reflect the resolved text.
- Owner's editor shows the merged content.

---

## Phase C3.4 — Reviewer annotations

> **Status**: Not yet implemented. Fill in tests when C3.4 ships.

### Setup

Precondition: Reviewer has accepted an invite. A scene with at least 3 paragraphs of content exists.

### 3.4-A — Create an annotation (note)

1. Log in as **Reviewer**. Open the project editor. Open a scene.
2. Select a sentence (drag to highlight text).
3. A "Add note" popover should appear.
4. Type "This paragraph feels rushed — consider adding a sensory beat here." Select type **Note**.
5. Save.

**Expected:**
- A yellow underline appears on the selected text.
- The annotation appears in the **Annotation sidebar** on the right.

### 3.4-B — Create a suggestion

1. Select a different sentence.
2. Type "Suggest replacing 'he walked' with 'he trudged'." Select type **Suggestion**.
3. Save.

**Expected:**
- Blue underline on the selected text.
- Suggestion appears in sidebar with a blue badge.

### 3.4-C — Create a question

1. Select another passage.
2. Type "Is this the first time we've seen this character use magic?" Select type **Question**.
3. Save.

**Expected:**
- Purple underline. Question badge in sidebar.

### 3.4-D — View annotations as Owner

1. Log in as **Owner**. Open the same scene.

**Expected:**
- All three annotation underlines are visible.
- Clicking any underline opens a popover showing: note text, author name, type badge, and a **Resolve** button.
- Sidebar shows all annotations with author info.

### 3.4-E — Resolve an annotation

1. As **Owner**, click the Note annotation → click **Resolve**.

**Expected:**
- The yellow underline disappears from the text.
- The annotation is removed from the sidebar (or shown as resolved/greyed depending on implementation).

### 3.4-F — Role restrictions

| # | Action | Who | Expected |
|---|---|---|---|
| F1 | Attempt to create an annotation | **Owner** (not reviewer) | Owner can create annotations (editors/owners have annotation rights) |
| F2 | Attempt to resolve an annotation | **Reviewer** | Resolve button not shown — only owner can resolve |
| F3 | Attempt to create an annotation via API | **Outsider** | 403 |

### 3.4-G — Jump to annotation from sidebar

1. Open a scene with multiple annotations.
2. Click an annotation in the sidebar.

**Expected:**
- The editor scrolls to and highlights the annotated text.

---

## Phase C3.5 — Notifications

> **Status**: Not yet implemented. Fill in tests when C3.5 ships.

### 3.5-A — Notification bell appears on invite

1. Log in as **CoAuthor**.
2. As **Owner** (in a separate tab), send a new invite to coauthor@test.com for a second project.
3. Wait up to 60 s (poll interval) or refresh the page.

**Expected:**
- A badge appears on the notification bell in the TopBar showing "1".
- Opening the bell dropdown shows: "Owner invited you to [project name] as Co-author."

### 3.5-B — Merge request notification

1. Log in as **CoAuthor**. Open a MR.
2. As **Owner** (separate tab), approve the MR.
3. Wait / refresh as CoAuthor.

**Expected:**
- Notification: "[project name] — your merge request was approved."

### 3.5-C — Rejection notification

1. Owner rejects a MR.
2. CoAuthor receives: "[project name] — your merge request was rejected."

### 3.5-D — Annotation notification

1. Reviewer adds an annotation.
2. Owner receives: "[project name] — Reviewer left a note on [scene name]."

### 3.5-E — Mark as read

1. Owner has 3 unread notifications.
2. Click one notification → it navigates to the relevant MR/annotation.

**Expected:**
- That notification is marked read (badge count drops by 1, item no longer bold/highlighted).

3. Click **Mark all read**.

**Expected:**
- Badge disappears entirely. All notification items show as read.

### 3.5-F — Old notifications persist

1. Mark all notifications as read.
2. Refresh.

**Expected:**
- The last 20 read notifications are still visible in the dropdown (not deleted), just without the unread badge.

---

## Regression checks (run after each phase)

After completing any C3 phase, verify these pre-C3 features still work:

| Check | How |
|---|---|
| Owner can still Chronicle on canon | Git panel → Chronicle → success |
| AI features work for owner on the project | Open editor → Nexus chat → send message → response streams |
| AI features work for a collaborator with their own API key | Log in as CoAuthor → configure API key in Settings → open project → AI chat |
| Export still works | ProjectHome → Export → download Markdown |
| Wiki is readable by all project members | Log in as Reviewer → navigate to Wiki → entity list loads |
| Project stats load | ProjectHome → stats cards render without error |
| Chapter summaries still regenerate | Edit a scene → wait 30 s → check chapter summary badge |

---

## Known gaps (C3 MVP scope)

These are intentional limitations in C3. Do not raise them as bugs:

- Collaborators **cannot edit existing canon scenes** inline — they can only create new content on their branch. Suggested edits to existing text go through the annotation system.
- Branch-scoped DB content isolation is not implemented. Two users editing the same scene simultaneously will overwrite each other's changes (this is a C4 concern).
- No email delivery for invites — the owner must share the invite link manually.
- No real-time notification push — the bell polls every 60 seconds.
- Reviewer role cannot use AI features (no Chronicle access, so no branch context for AI). This is expected.
