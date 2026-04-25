# /trellis:workflow - Automated Task Pipeline

Orchestrate the full development pipeline: brainstorm -> implement -> check -> verify.

This is a **pure orchestrator** — it calls existing trellis commands (start, brainstorm, cowork) and does NOT modify them.

---

## Operation Types

| Marker | Meaning | Executor |
|--------|---------|----------|
| `[AI]` | Script calls or decisions executed by AI | You (AI) |
| `[USER]` | Verification, PR creation, archive | User |

---

## Step 0: Initialize `[AI]`

```bash
python3 ./.trellis/scripts/get_context.py
```

Verify developer identity is set. If not, run `/trellis:start` first.

**Check for unread notifications** (from cron babysit or previous sessions):

```bash
NOTIFS=$(ls ~/.trellis/notifications/*.json 2>/dev/null)
if [ -n "$NOTIFS" ]; then
  for f in ~/.trellis/notifications/*.json; do echo "---"; cat "$f"; done
  rm ~/.trellis/notifications/*.json
fi
```

If notifications were found, report them to the user (type and detail). Notifications are auto-cleared after reporting.

---

## Step 1: Create Task `[AI]`

```bash
python3 ./.trellis/scripts/task.py create "<user requirement>"
```

Take the user's requirement and create a task. Record the output `<task-dir>`.

```bash
python3 ./.trellis/scripts/task.py init-context <task-dir> backend
```

Use the appropriate dev type based on the requirement (backend/frontend/etc).

```bash
python3 ./.trellis/scripts/task.py start <task-dir>
```

---

## Step 2: Detect Existing State `[AI]`

```bash
python3 ./.trellis/scripts/workflow.py status
```

Or check manually:

```bash
cat <task-dir>/workflow.json
```

**Decision tree:**

| Condition | Action |
|-----------|--------|
| No `workflow.json` | New workflow — start from Phase 1 |
| `status: completed` | Already done — show report, stop |
| `status: failed` with `phases_completed` | Ask: "Continue from `<phase>` or discard?" |
| `status: running` | Auto-resume from `current_phase` (no prompt) |

**If resuming:**
- `status: running` → Auto-resume from `current_phase`. Show one-line status, then continue.
- `status: failed` → Ask: "Detected workflow failed at `<phase>`. Continue or discard?"
- If **discard**: run `python3 ./.trellis/scripts/task.py finish` to clean up, then restart from Step 1
- If **continue**: skip to the appropriate phase below

---

## Step 3: Brainstorm `[AI]`

### Simple Task Detection

Evaluate the requirement:

| Signal | Simple | Complex |
|--------|--------|---------|
| Description length | < 50 chars | >= 50 chars |
| Keywords | fix, patch, typo, bug, minor, cleanup, rename | implement, create, design, architect, migrate, integrate |
| Expected files | <= 3 | > 3 |
| Estimated time | < 15 min | >= 15 min |

### If Simple Task

Generate PRD directly (skip Q&A):

```bash
# PRD is auto-generated in brainstorm phase
```

Create a concise PRD in `<task-dir>/prd.md`:
- Title and overview
- Key requirements (bullet list)
- Technical approach
- Acceptance criteria

### If Complex Task

Run the full brainstorm process:

```
/trellis:brainstorm
```

Follow brainstorm steps until PRD is finalized and confirmed.

### After PRD: Write Complexity to task.json

Evaluate the task complexity and write it to task.json's meta field:

```bash
python3 -c "
import json
from pathlib import Path
p = Path('<TASK_DIR>/task.json')
d = json.loads(p.read_text())
d.setdefault('meta', {})['complexity'] = '<trivial|simple|complex>'
p.write_text(json.dumps(d, indent=2, ensure_ascii=False))
"
```

Complexity evaluation:
- **trivial**: Single file, typo fix, config change, < 5 lines of code
- **simple**: 1-3 files, straightforward logic, no architecture decisions
- **complex**: 4+ files, new features, architecture decisions, multi-module

This `meta.complexity` value is used by:
- Step 3.5: Whether to skip Decompose
- Step 5 (Check): Whether to skip or simplify checks
- dispatch.py: Which worker prompt tier to use

---

## Step 3.5: Decompose `[AI]`

> **Skip condition**: If `task.json`'s `meta.complexity` is `"trivial"` or `"simple"`, skip this step entirely. Only `"complex"` tasks need decomposition.

After brainstorm produces `<task-dir>/prd.md`, analyze whether the task should be split into independent subtasks for parallel execution.

### Evaluation

Read `<task-dir>/prd.md` and check:

| Condition | Description |
|-----------|-------------|
| Actionable H2 sections >= 2 | PRD has at least 2 `##` sections with implementation content (skip "Overview", "Constraints", "Goals", "Notes", "Scope" etc.) |
| No cross-section dependencies | Sections do not reference each other's outputs or state (e.g. "module A's return value feeds into module B") |
| Non-overlapping file groups | File paths mentioned in each section have no intersection |

### If ALL conditions met — Split

For each actionable H2 section:

```bash
CHILD_DIR=$(python3 ./.trellis/scripts/task.py create "<section-title>" --parent <task-dir>)
```

Write `<CHILD_DIR>/prd.md` with focused content:

```markdown
# <Section Title>

> Parent: <parent-task-title>

## Shared Context

<Copy overview/constraints/goals from parent PRD — everything before the first actionable section>

---

<Original section content>
```

Initialize context for each child task (inherit parent's dev_type):

```bash
python3 ./.trellis/scripts/task.py init-context <CHILD_DIR> <dev_type>
```

Update parent `<task-dir>/prd.md` — replace functional sections with subtask index:

```markdown
## Subtasks

- [ ] Subtask 1: <child1-title> → <child1-dir>
- [ ] Subtask 2: <child2-title> → <child2-dir>
```

### If NOT all conditions met — Skip

Do not split. Keep the single-task structure. Proceed to Step 4.

---

## Step 4: Implement (via Cowork) `[AI]`

> **CRITICAL: You MUST invoke `/trellis:cowork` for ALL implementation work. You are the coordinator, NOT the implementer. DO NOT write code directly in this session.**

```bash
# Check cowork availability
clawteam --version 2>/dev/null || echo "NOT_AVAILABLE"
```

**Mandatory action** — invoke cowork:

```
/trellis:cowork
```

Cowork will:
1. Analyze task scope and parallelism
2. Choose Direct mode (current session) or Clone mode (parallel workers)
3. Execute implementation
4. Run tests (coverage >= 85%)
5. Perform code review and quality gate (Clone mode)

**Strict rules:**
- **DO NOT** start coding yourself — cowork is the implementer, you are the coordinator
- **DO NOT** override cowork's mode choice — cowork decides Direct vs Clone automatically
- **Wait** for cowork to complete all its steps before proceeding to Step 5
- If clawteam is NOT available, cowork will fall back to Direct mode automatically — still let cowork handle it via `/trellis:cowork`
- The ONLY case where you skip `/trellis:cowork` is if the cowork command itself errors out or does not exist

---

## Step 5: Check `[AI]`

> **Skip condition**:
> - `meta.complexity` = `"trivial"` → Skip entirely (worker already ran tests)
> - `meta.complexity` = `"simple"` → Optional (worker tests should be sufficient)
> - `meta.complexity` = `"complex"` → Run full check

```bash
python3 ./.trellis/scripts/task.py validate <task-dir>
```

Run any project-specific checks:

```bash
# Example: run project lint/typecheck if configured
# Check AGENTS.md or project docs for the correct command
```

If checks fail, attempt to fix:
- Read error messages
- Apply fixes
- Re-run checks
- If still failing after 3 attempts, mark phase as failed and pause

---

## Step 6: Verify and Report `[AI]`

Generate the workflow summary:

```bash
python3 ./.trellis/scripts/workflow.py run --task-dir <task-dir> "<requirement>"
```

Or generate report directly:

```bash
python3 -c "from workflow.reporter import generate_report; from pathlib import Path; print(generate_report(Path('<task-dir>')))"
```

Update workflow state to completed.

---

## Step 7: Handoff to User `[USER]`

Present the summary report and next steps:

```
=== Workflow Complete ===

Task: <task title>
Completed phases: brainstorm, implement, check, verify
Report: <task-dir>/workflow-summary.md

Next steps:
  1. Review the summary report
  2. Review code changes: git diff
  3. Test manually if needed
  4. Commit: /trellis:commit
  5. Archive when verified: /trellis:archive
```

---

## Resume Protocol

When a user runs `/trellis:workflow` in a new session:

1. Run Step 0 and Step 2 (detect state)
2. If existing workflow found:
   - `status: running` → Auto-resume from `current_phase` (no user prompt needed)
   - `status: failed` → Show failure info, ask "Continue from `<phase>` or discard?"
   - `status: completed` → Show report
   - If continue → jump to the appropriate step
   - If discard → clean up and restart

---

## Principles

1. **Orchestrate only** — never modify start, brainstorm, or cowork commands
2. **Auto-detect state** — always check for existing workflow before starting
3. **Full automation** — user provides requirement, system handles everything
4. **Clean recovery** — support discard and restart with proper cleanup
5. **Cowork decides** — never override cowork's Direct/Clone mode choice
6. **Report everything** — generate clear summary for user verification
