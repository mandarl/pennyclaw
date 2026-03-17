# PennyClaw Web UI Upgrade Plan

**Author:** Manus AI
**Date:** March 16, 2026
**Scope:** Surface all Tier 1-7 backend features in the embedded web UI

---

## Executive Summary

The PennyClaw web UI (`internal/channels/web/html.go`, ~1000 lines) currently surfaces only a fraction of the backend's capabilities. The UI provides chat, sessions, logs, settings, version/upgrade, file upload, and export. However, the backend exposes **28 API endpoints** across 7 subsystems — workspace, cron scheduler, skill packs, health/metrics, tasks, notes, and webhooks — that have no corresponding UI. This plan describes a comprehensive upgrade to bring the web UI to feature parity with the backend.

The upgrade is organized into **8 tiers**, ordered by user impact and implementation complexity. Each tier is self-contained and can be shipped independently.

---

## Current State Audit

### What the UI Already Has

| Feature | UI Element | Backend Endpoint |
|---|---|---|
| Chat | Main chat area with Markdown rendering | `POST /api/chat` |
| Sessions | Sidebar session list, create/switch/delete | `GET/POST /api/sessions`, `GET/DELETE /api/sessions/:id` |
| Logs | Slide-out panel with auto-refresh | `GET /api/logs` |
| Settings | Slide-out panel (LLM config, system prompt) | `GET/PUT /api/settings` |
| Version & Upgrade | Settings panel section | `GET /api/version`, `POST /api/upgrade` |
| File Upload | Drag-and-drop + button | `POST /api/upload` |
| Export | Header button, downloads Markdown | `GET /api/export` |
| Auth | Login overlay, token in localStorage | `GET /api/auth/check` |
| Token Usage | Sidebar footer counter | `GET /api/tokens` |
| Theme | Dark/light toggle | Client-side only |

### What the Backend Has But the UI Does Not Surface

| Subsystem | Endpoints | UI Status |
|---|---|---|
| **Health & Metrics** | `GET /api/health`, `GET /api/metrics` | No UI (only raw JSON/Prometheus text) |
| **Workspace** | `GET /api/workspace`, `GET/PUT/DELETE /api/workspace/:file`, `POST /api/workspace/bootstrap` | No UI |
| **Cron Scheduler** | `GET/POST /api/cron`, `GET/PUT/DELETE /api/cron/:id`, `GET /api/cron/:id/runs`, `POST /api/cron/:id/run` | No UI |
| **Skill Packs** | `GET /api/skills`, `GET/PUT/DELETE /api/skills/:name`, `GET /api/skills/search`, `POST /api/skills/install` | No UI |
| **Tasks** | Via agent skills (`task_add`, `task_list`, `task_update`, `task_delete`) | No UI (chat-only) |
| **Notes** | Via agent skills (`note_save`, `note_read`, `note_list`, `note_delete`, `note_search`) | No UI (chat-only) |
| **Webhooks** | `POST /api/webhooks` | No UI (config-only) |
| **Config Validation** | Startup-only | No UI feedback |

---

## Architecture Decision: Panel-Based Navigation

The current UI uses a **sidebar + slide-out panel** pattern. The sidebar has session navigation and footer buttons for Logs and Settings. The upgrade should extend this pattern rather than replace it.

**Proposed navigation model:**

The sidebar footer currently has: Settings, Logs, Sound toggle, Sign Out. The upgrade adds a **navigation section** between the session list and the footer, with icon+label buttons for each new panel:

```
┌──────────────────┐
│ 🪙 PennyClaw     │
├──────────────────┤
│ + New Chat       │
│                  │
│ [Session list]   │
│                  │
├──────────────────┤
│ TOOLS            │  ← New section header
│ ✓ Tasks          │  ← New
│ 📝 Notes         │  ← New
│ 📁 Workspace     │  ← New
│ ⏰ Scheduler     │  ← New
│ 🧩 Skills        │  ← New
│ 📊 Health        │  ← New
├──────────────────┤
│ Tokens: 12,345   │
│ ⚙ Settings       │
│ 📋 Logs          │
│ 🔊 Sound notifs  │
│ 🚪 Sign Out      │
└──────────────────┘
```

Each button opens a **slide-out panel** (same pattern as Logs and Settings). Panels are 500px wide on desktop, full-width on mobile.

---

## Tier A: Health Dashboard Panel

**Priority:** High — Most requested observability feature
**Complexity:** Medium
**New backend endpoints needed:** None (uses existing `/api/health`)

### Panel Layout

The Health panel displays a real-time dashboard of system and agent metrics, pulled from the `/api/health` endpoint.

**Section 1: Status Banner**

A colored banner at the top showing overall health status:
- Green banner: "Healthy" — all checks passing
- Yellow banner: "Degraded" — some warnings
- Red banner: "Unhealthy" — critical issues

**Section 2: System Metrics (2-column grid)**

| Metric Card | Source Field | Display |
|---|---|---|
| Memory | `system.memory_alloc_mb` | Gauge bar + "42 MB / 1024 MB" |
| CPU / Goroutines | `system.goroutines` | Number + trend arrow |
| Disk | `system.disk_used_pct` | Gauge bar + "12.4 GB / 30 GB" |
| GC Pauses | `system.gc_pause_ms` | Number + "ms" |
| Uptime | `system.uptime_seconds` | Human-readable "3d 14h 22m" |
| Go Version | `system.go_version` | Static text |

**Section 3: Agent Metrics (2-column grid)**

| Metric Card | Source Field | Display |
|---|---|---|
| Total Requests | `agent.total_requests` | Counter |
| Active Requests | `agent.active_requests` | Live counter |
| Avg Latency | `agent.avg_latency_ms` | Number + "ms" |
| P99 Latency | `agent.p99_latency_ms` | Number + "ms" |
| Error Rate | `agent.error_count / total_requests` | Percentage + color |
| Tool Calls | `agent.tool_calls` | Counter |

**Section 4: Health Checks**

A list of individual checks with status icons:
- `memory_usage`: ✅ OK / ⚠️ Warning / ❌ Critical
- `goroutine_count`: ✅ / ⚠️ / ❌
- `disk_usage`: ✅ / ⚠️ / ❌
- `error_rate`: ✅ / ⚠️ / ❌

Each check shows its current value and threshold.

**Footer:** Auto-refresh toggle (default: every 10 seconds) + "Last updated: 2s ago"

### Implementation Notes

- Add `openPanel('health')` to sidebar
- Fetch `/api/health` on panel open and on interval
- Use CSS gauge bars (no charting library needed — keep it lightweight)
- Color thresholds: green < 60%, yellow 60-85%, red > 85%
- Animate number transitions with CSS `transition` on opacity

---

## Tier B: Task Manager Panel

**Priority:** High — Core productivity feature
**Complexity:** Medium-High
**New backend endpoints needed:** Yes — need REST API for tasks (currently only agent skills)

### New Backend Endpoints Required

The task system currently only works through agent skills (`task_add`, `task_list`, etc.), which operate on `data/tasks.json`. The web UI needs direct REST endpoints:

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/tasks` | List all tasks (with query params: `status`, `priority`, `tag`) |
| `POST` | `/api/tasks` | Create a new task |
| `PUT` | `/api/tasks/:id` | Update a task |
| `DELETE` | `/api/tasks/:id` | Delete a task |

These endpoints should reuse the `TaskStore` from `internal/skills/productivity.go` to avoid duplicating logic.

### Panel Layout

**Header Bar:** "Tasks" title + filter dropdown (All / Active / Completed) + "Add Task" button

**Quick Add Bar:** A single-line input at the top with:
- Text input for task title
- Priority selector (low/medium/high/critical) — color-coded dots
- Optional due date picker (native `<input type="date">`)
- "Add" button

**Task List:** Each task is a card/row with:

```
┌─────────────────────────────────────────────────────┐
│ ○ [Priority dot] Task title                    [···]│
│   Due: Mar 20  │  Tags: #work #deploy          [✓] │
│   Notes: Deploy the new version to prod...          │
└─────────────────────────────────────────────────────┘
```

- Clicking the circle toggles completion (PATCH status)
- `[···]` opens a dropdown: Edit, Delete
- Priority dot colors: 🔴 critical, 🟠 high, 🟡 medium, ⚪ low
- Completed tasks show strikethrough text and muted colors
- Tags are clickable for filtering

**Empty State:** "No tasks yet. Add one above or ask PennyClaw to create tasks for you."

**Footer:** Task count summary: "3 active, 2 completed, 5 total"

### Implementation Notes

- Add 4 new handler methods to `server.go` that delegate to `TaskStore`
- The `TaskStore` needs to be passed to `NewServer` (or accessed via the agent)
- Sort: overdue first, then by priority (critical > high > medium > low), then by creation date
- Keyboard shortcut: `Ctrl+T` to focus the quick add input

---

## Tier C: Notes / Knowledge Base Panel

**Priority:** High — Core productivity feature
**Complexity:** Medium-High
**New backend endpoints needed:** Yes — need REST API for notes

### New Backend Endpoints Required

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/notes` | List all notes (returns name, size, modified date) |
| `GET` | `/api/notes/:name` | Read a specific note |
| `POST` | `/api/notes` | Create a new note |
| `PUT` | `/api/notes/:name` | Update a note |
| `DELETE` | `/api/notes/:name` | Delete a note |
| `GET` | `/api/notes/search?q=` | Search notes |

These endpoints should reuse the `NoteStore` from `internal/skills/productivity.go`.

### Panel Layout

**Header Bar:** "Notes" title + search input + "New Note" button

**Two-Pane Layout (within the 500px panel):**

Left pane (40% width, ~200px): Note list
- Each item shows: note name, first line preview, modified date
- Active note is highlighted
- Sorted by last modified (newest first)

Right pane (60% width, ~300px): Note editor/viewer
- Toggle between "View" (rendered Markdown) and "Edit" (textarea)
- View mode uses the same `marked.js` + `DOMPurify` already loaded
- Edit mode is a `<textarea>` with monospace font
- Save button (or Ctrl+S) saves changes via PUT

**Search Mode:** When the search input has text, the left pane shows search results with highlighted snippets instead of the full note list. Results come from `GET /api/notes/search?q=`.

**Empty State:** "Your knowledge base is empty. Save notes by asking PennyClaw, or create one here."

### Implementation Notes

- Reuse `NoteStore` from productivity.go
- Note names are sanitized (no path traversal) — same rules as the skill
- Markdown preview reuses the existing `renderMarkdown()` function
- Consider a debounced auto-save (save 2 seconds after last keystroke in edit mode)
- Keyboard shortcut: `Ctrl+N` to create a new note (avoid conflict with browser new window — use only when panel is open)

---

## Tier D: Workspace File Manager Panel

**Priority:** Medium — Power user feature
**Complexity:** Medium
**New backend endpoints needed:** None (uses existing `/api/workspace` endpoints)

### Panel Layout

**Header Bar:** "Workspace" title + "New File" button + "Reset Bootstrap" button

**File List:** A table/list of workspace files:

| Name | Size | Modified | Actions |
|---|---|---|---|
| `bootstrap.sh` | 1.2 KB | 2h ago | View / Edit / Delete |
| `notes.md` | 456 B | 1d ago | View / Edit / Delete |

**File Editor:** Clicking "View" or "Edit" opens a sub-view within the panel:
- Back button to return to file list
- File name as header
- Monospace textarea for editing
- Save / Cancel buttons
- For viewing: render Markdown files, show plain text for others

**Bootstrap Section:** A small info box at the bottom:
- Shows whether bootstrap has run ("Bootstrap: completed" or "Bootstrap: pending")
- "Reset Bootstrap" button triggers `POST /api/workspace/bootstrap`
- Explains: "Resetting bootstrap will re-run your onboarding script on the next conversation."

### Implementation Notes

- All endpoints already exist — this is pure frontend work
- File list from `GET /api/workspace` (returns `files` array and `needs_bootstrap` flag)
- File content from `GET /api/workspace/:filename`
- Save via `PUT /api/workspace/:filename`
- Delete via `DELETE /api/workspace/:filename`
- Add confirmation dialog before delete

---

## Tier E: Cron Scheduler Panel

**Priority:** Medium — Automation feature
**Complexity:** Medium-High
**New backend endpoints needed:** None (uses existing `/api/cron` endpoints)

### Panel Layout

**Header Bar:** "Scheduler" title + "New Job" button

**Job List:** Each job is a card:

```
┌─────────────────────────────────────────────────────┐
│ 📅 Daily Summary                          [enabled] │
│ Schedule: 0 9 * * *  (Every day at 9:00 AM)        │
│ Prompt: "Give me a summary of today's tasks..."     │
│ Last run: 2h ago (success)  │  Next: in 22h         │
│                              [Run Now] [Edit] [Del] │
└─────────────────────────────────────────────────────┘
```

- Enable/disable toggle per job
- "Run Now" button triggers `POST /api/cron/:id/run`
- Human-readable cron description (parse the expression client-side)
- Last run status with color (green = success, red = error)

**Create/Edit Form:** A modal or inline form:

| Field | Type | Description |
|---|---|---|
| Name | Text input | Job name |
| Schedule | Text input | Cron expression with helper text |
| Prompt | Textarea | The message to send to the agent |
| Enabled | Toggle | Whether the job is active |

Include a cron expression cheat sheet below the schedule input:
```
┌───────── minute (0-59)
│ ┌─────── hour (0-23)
│ │ ┌───── day of month (1-31)
│ │ │ ┌─── month (1-12)
│ │ │ │ ┌─ day of week (0-6, Sun=0)
│ │ │ │ │
* * * * *

Examples:
0 9 * * *     Every day at 9:00 AM
*/30 * * * *  Every 30 minutes
0 9 * * 1-5   Weekdays at 9:00 AM
```

**Run History:** Clicking a job expands to show recent runs from `GET /api/cron/:id/runs`:

| Run Time | Duration | Status | Output Preview |
|---|---|---|---|
| 2h ago | 3.2s | ✅ Success | "Here's your daily summary..." |
| 26h ago | 2.8s | ✅ Success | "Today's tasks: 3 active..." |

### Implementation Notes

- All endpoints already exist
- Client-side cron parsing: write a simple `describeCron(expr)` function (no library needed for common patterns)
- "Next run" calculation: parse cron expression to find next occurrence (or add a `next_run` field to the backend response)
- Job creation via `POST /api/cron` with JSON body
- Consider adding `GET /api/cron/:id/next` to the backend for accurate next-run time

---

## Tier F: Skills Manager Panel

**Priority:** Medium — Extensibility feature
**Complexity:** Medium
**New backend endpoints needed:** None (uses existing `/api/skills` endpoints)

### Panel Layout

**Header Bar:** "Skills" title + search input (searches ClawHub)

**Tab Bar:** "Installed" | "Browse ClawHub"

**Installed Tab:** List of installed skills:

```
┌─────────────────────────────────────────────────────┐
│ 🧩 web_search                              [on/off] │
│ Search the web using DuckDuckGo                     │
│ Author: built-in  │  Version: 1.0.0  │  Bundled     │
│                                            [Remove] │
└─────────────────────────────────────────────────────┘
```

- Toggle to enable/disable each skill (`PUT /api/skills/:name`)
- "Remove" button for non-bundled skills (`DELETE /api/skills/:name`)
- Bundled skills show "Bundled" badge and cannot be removed
- Clicking a skill expands to show its full description and instructions

**Browse ClawHub Tab:** Search results from ClawHub:

```
┌─────────────────────────────────────────────────────┐
│ 🧩 weather-forecast                       [Install] │
│ Get weather forecasts for any location              │
│ Author: community  │  Version: 1.2.0               │
└─────────────────────────────────────────────────────┘
```

- Search input triggers `GET /api/skills/search?q=`
- "Install" button triggers `POST /api/skills/install`
- Show install progress/status
- After install, skill appears in the "Installed" tab

### Implementation Notes

- All endpoints already exist
- Skill enable/disable via `PUT /api/skills/:name` with `{"enabled": true/false}`
- ClawHub search may fail (network) — show graceful error
- Consider caching search results client-side for 60 seconds

---

## Tier G: Settings Panel Expansion

**Priority:** Medium — Configuration completeness
**Complexity:** Low-Medium
**New backend endpoints needed:** Partial (need channel config endpoints)

### New Sections in Existing Settings Panel

The current Settings panel has: Version, LLM Configuration, System Prompt. Add these sections:

**Section: Channels**

```
CHANNELS
┌─────────────────────────────────────────┐
│ Web UI          [enabled] (always on)   │
│ Telegram Bot    [toggle]                │
│   Token: ****...abcd                    │
│ Discord Bot     [toggle]                │
│   Token: ****...efgh                    │
│ Webhooks        [toggle]                │
│   Secret: ****...ijkl                   │
│   Endpoint: /api/webhooks               │
└─────────────────────────────────────────┘
```

**Section: Email Notifications**

```
EMAIL
┌─────────────────────────────────────────┐
│ Enabled         [toggle]                │
│ SMTP Host:      [smtp.gmail.com      ]  │
│ SMTP Port:      [587                 ]  │
│ Username:       [you@gmail.com       ]  │
│ Password:       [••••••••            ]  │
│ From Name:      [PennyClaw           ]  │
│                        [Send Test Email]│
└─────────────────────────────────────────┘
```

**Section: Sandbox**

```
SANDBOX
┌─────────────────────────────────────────┐
│ Enabled         [toggle]                │
│ Work Directory: [/tmp/pennyclaw-sandbox]│
│ Max Timeout:    [30] seconds            │
│ Max Memory:     [128] MB                │
└─────────────────────────────────────────┘
```

**Section: Memory**

```
MEMORY
┌─────────────────────────────────────────┐
│ DB Path:        [data/pennyclaw.db   ]  │
│ Max History:    [50] messages            │
│ Persist:        [toggle]                │
│                                         │
│ Database size: 2.4 MB                   │
│                     [Clear All Sessions]│
└─────────────────────────────────────────┘
```

### New Backend Endpoints Required

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/config` | Return full config (sensitive fields masked) — already exists but needs expansion |
| `PUT` | `/api/config` | Update full config — already exists but needs expansion |
| `POST` | `/api/email/test` | Send a test email to verify SMTP config |

### Implementation Notes

- Expand `handleSettings` to include channel, email, sandbox, and memory config
- Add `handleTestEmail` endpoint that sends a test message using the current email config
- Mask sensitive fields (tokens, passwords) in GET responses
- "Clear All Sessions" calls `DELETE /api/sessions` (need to add bulk delete)
- Some changes require restart — show a banner: "Restart required for changes to take effect"

---

## Tier H: UI Polish & Cross-Cutting Concerns

**Priority:** Medium — Quality of life
**Complexity:** Low-Medium
**New backend endpoints needed:** Minimal

### H.1: Header Status Indicator Enhancement

Replace the static "Online" status with a live health indicator:

```
[●] Healthy  │  42 MB  │  v0.3.0
```

- Polls `/api/health` every 30 seconds
- Shows memory usage and version inline
- Color changes based on health status (green/yellow/red)
- Clicking opens the Health panel

### H.2: Keyboard Shortcuts Expansion

| Shortcut | Action |
|---|---|
| `Ctrl+K` | New chat (existing) |
| `Ctrl+L` | Clear chat (existing) |
| `Ctrl+E` | Export (existing) |
| `Ctrl+T` | Open Tasks panel |
| `Ctrl+J` | Open Scheduler panel |
| `Ctrl+H` | Open Health panel |
| `Ctrl+W` | Open Workspace panel |
| `Esc` | Close panels (existing) |

### H.3: Toast Notifications

Add a lightweight toast system (no library — pure CSS/JS) for:
- "Task created successfully"
- "Note saved"
- "Skill installed"
- "Settings saved" (replace current inline text)
- "Job triggered"
- Error messages

### H.4: Mobile Responsiveness for New Panels

- All new panels use `width: 100%` on mobile (same as existing panels)
- Two-pane layouts (Notes) collapse to single-pane with back navigation
- Task quick-add bar stacks vertically on mobile
- Health dashboard grid goes from 2-column to 1-column

### H.5: Config Validation Feedback

On the Settings panel, after saving, run client-side validation and show inline warnings:
- "Port must be between 1 and 65535"
- "API key appears to be an unresolved environment variable ($OPENAI_API_KEY)"
- "SMTP host is required when email is enabled"

---

## Implementation Order & Effort Estimates

| Tier | Feature | New Backend | New CSS | New JS | Estimated Lines | Priority |
|---|---|---|---|---|---|---|
| **A** | Health Dashboard | 0 endpoints | ~80 | ~60 | ~140 | P0 |
| **B** | Task Manager | 4 endpoints | ~100 | ~150 | ~350 (backend) + ~250 (frontend) | P0 |
| **C** | Notes Panel | 6 endpoints | ~80 | ~180 | ~300 (backend) + ~260 (frontend) | P0 |
| **D** | Workspace Manager | 0 endpoints | ~50 | ~100 | ~150 | P1 |
| **E** | Cron Scheduler | 0 endpoints | ~80 | ~160 | ~240 | P1 |
| **F** | Skills Manager | 0 endpoints | ~60 | ~120 | ~180 | P1 |
| **G** | Settings Expansion | 2 endpoints | ~40 | ~80 | ~150 (backend) + ~120 (frontend) | P2 |
| **H** | UI Polish | 0 endpoints | ~60 | ~80 | ~140 | P2 |

**Total estimated new lines:** ~2,280 (bringing html.go from ~1,000 to ~2,500 lines, plus ~800 lines of new backend handlers in server.go)

### Recommended Implementation Sequence

```
Phase 1 (Core Productivity):  A → B → C     [Health, Tasks, Notes]
Phase 2 (Power Features):     D → E → F     [Workspace, Scheduler, Skills]
Phase 3 (Polish):             G → H         [Settings, UI Polish]
```

Each phase ends with a critical review cycle and a version tag bump.

---

## Technical Constraints

**No new dependencies.** The web UI is embedded in the Go binary via `html.go`. All JavaScript must be vanilla JS (no React, no build step). The only external CDN resources are `marked.js`, `DOMPurify`, and `highlight.js` — all already loaded. No new CDN resources should be added.

**Single-file UI.** The entire UI lives in `html.go` as a Go string constant. This is intentional — it means the binary is self-contained. The upgrade must maintain this pattern. Consider splitting into multiple Go string constants (e.g., `indexCSS`, `indexJS`, `indexHTML`) within the same file for readability, but the final output must still be a single HTML page.

**Panel width.** All panels use the existing 500px slide-out pattern. No full-page views. This keeps the chat always visible and maintains the "chat-first" UX.

**Mobile.** All new panels must work at 100% width on screens < 768px. The existing `@media (max-width: 768px)` breakpoint applies.

**Auth.** All new endpoints must go through `requireAuth()`. The existing `apiFetch()` helper in the frontend automatically adds the Bearer token.

**Rate limiting.** New endpoints should not be rate-limited (only `/api/chat` is rate-limited, and that's intentional — the other endpoints are lightweight reads).

---

## New Backend Endpoints Summary

| Method | Path | Handler | Tier |
|---|---|---|---|
| `GET` | `/api/tasks` | `handleTasks` | B |
| `POST` | `/api/tasks` | `handleTasks` | B |
| `PUT` | `/api/tasks/:id` | `handleTaskByID` | B |
| `DELETE` | `/api/tasks/:id` | `handleTaskByID` | B |
| `GET` | `/api/notes` | `handleNotes` | C |
| `POST` | `/api/notes` | `handleNotes` | C |
| `GET` | `/api/notes/search` | `handleNotesSearch` | C |
| `GET` | `/api/notes/:name` | `handleNoteByName` | C |
| `PUT` | `/api/notes/:name` | `handleNoteByName` | C |
| `DELETE` | `/api/notes/:name` | `handleNoteByName` | C |
| `POST` | `/api/email/test` | `handleTestEmail` | G |
| `DELETE` | `/api/sessions` | `handleSessions` (extend) | G |

**Total: 12 new endpoint handlers** (some share a handler function with method switching).

---

## File Changes Summary

| File | Changes |
|---|---|
| `internal/channels/web/html.go` | Add ~1,500 lines: CSS for new panels, HTML for new panels, JS for new panel logic |
| `internal/channels/web/server.go` | Add ~400 lines: 12 new endpoint handlers, wire TaskStore and NoteStore |
| `internal/channels/web/server.go` | Modify `NewServer` to accept TaskStore and NoteStore |
| `cmd/pennyclaw/main.go` | Pass TaskStore and NoteStore to NewServer |
| `internal/agent/agent.go` | Expose TaskStore and NoteStore accessors |

---

## Risk Assessment

| Risk | Mitigation |
|---|---|
| `html.go` becomes unwieldy at 2,500+ lines | Split into `html_css.go`, `html_body.go`, `html_js.go` string constants, concatenated at serve time |
| Panel interactions conflict with chat UX | Panels slide over chat (existing pattern), backdrop prevents accidental chat interaction |
| Task/Note REST endpoints duplicate skill logic | Endpoints delegate to the same `TaskStore`/`NoteStore` — no logic duplication |
| Mobile UX for complex panels (Notes two-pane) | Collapse to single-pane with navigation, test on 375px viewport |
| Performance with many tasks/notes | Paginate lists (50 items per page), lazy-load note content |
