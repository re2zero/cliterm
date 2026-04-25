# /trellis:cowork - Parallel Task Dispatch with ClawTeam

Dispatch existing Trellis tasks to ClawTeam workers for parallel execution, or proceed in the current session.

This command runs in the **user's main session** - not in worker sessions.

---

## Role

- You are the user's assistant, NOT a ClawTeam leader or worker
- You analyze tasks, propose modes, and orchestrate dispatch
- You do NOT write code - code is done by workers in worktrees
- You do NOT archive tasks - user must verify and archive manually

---

## Step 1: Get Context `[AI]`

```bash
python3 ./.trellis/scripts/get_context.py
clawteam --version 2>/dev/null || echo "NOT_AVAILABLE"
```

Check developer identity, active tasks, current task status, and ClawTeam availability.

**Check for unread notifications:**

```bash
NOTIFS=$(ls ~/.trellis/notifications/*.json 2>/dev/null)
if [ -n "$NOTIFS" ]; then
  for f in ~/.trellis/notifications/*.json; do echo "---"; cat "$f"; done
  rm ~/.trellis/notifications/*.json
fi
```

If notifications were found, report them to the user. Notifications are auto-cleared after reporting.

---

### 1.5 Load Cowork Config `[AI]`

Read `.trellis/config.yaml` (if it exists) to determine cowork feature settings:

```bash
cat .trellis/config.yaml
```

Extract the `cowork` section. These settings control how dispatch and babysit run:

| Setting | Config Key | Default | Used In |
|---------|-----------|---------|---------|
| Worker profile | `cowork.worker_profiles.default` | null (ClawTeam default) | Step 3.1 dispatch |
| Profile routing | `cowork.worker_profiles.map` | {} (no routing) | Step 3.1 dispatch |
| Auto PR | `cowork.auto_pr` | false | Step 3.2 babysit |
| Polling interval | `cowork.babysit.interval` | 10s | Step 3.2 babysit |
| Timeout | `cowork.babysit.timeout` | 300s | Step 3.2 babysit |
| Auto merge | `cowork.babysit.auto_merge` | true | Step 3.2 babysit |
| Max workers | `cowork.max_workers` | 4 | Step 3.1 dispatch |

If config is missing or has no `cowork` section, use defaults.
CLI flags always override config values.

---

## Step 2: Analyze and Propose `[AI]`

Read current task (`prd.md`, `task.json`), list all active tasks, evaluate:

| Factor | Threshold |
|--------|-----------|
| Scale | >5 files or >3 modules = **large** |
| Parallelism | No shared state between subtasks = **parallelizable** |
| Decomposed | task.json has non-empty `children[]` = **already decomposed, must use Clone mode** |

Determine recommendation: **Clone mode** if large/parallelizable/decomposed AND ClawTeam available, **Direct mode** otherwise.

> **IMPORTANT**: If `task.json` has `children[]`, the task was pre-decomposed by workflow Step 3.5. You MUST choose Clone mode — dispatch.py will auto-detect children and dispatch each to a separate worker.

Present options to user. Wait for response. If user does not respond within 10 seconds, proceed with the recommended mode automatically.

### Large/Parallelizable Task Template

```
Analysis: large parallelizable task (X modules, Y independent subtasks).

Recommended: **A. Clone Mode** (parallel workers with independent worktrees)

**A. Clone Mode** (recommended)
   - Each parallel task gets an isolated worker with its own worktree
   - Workers write unit tests after implementation (coverage >= 85%)
   - Post-merge: code review + clawteam-verifier quality gate

**B. Direct Mode**
   - Complete in current session step by step

Choose A or B (timeout 10s, default: A).
```

### Small Task Template

```
Analysis: small task (X files), current session is more efficient.

Recommended: **A. Direct Mode**

**A. Direct Mode** (recommended)
**B. Clone Mode**

Choose A or B (timeout 10s, default: A).
```

---

## Step 3: Execute Clone Mode `[AI]`

> **IMPORTANT:** This workflow uses babysit.py — a deterministic background monitor that runs outside the AI session and writes results to a JSON file (zero streaming token cost). It polls task status via `clawteam --json task list <team>` and auto-merges worker branches on completion.

### 3.1 Dispatch (first batch)

```bash
# Record workflow start point for code-review later
WORKFLOW_START_COMMIT=$(git rev-parse HEAD)

# Dispatch workers
python3 ./.trellis/scripts/cowork/dispatch.py --task-dir <TRELLIS_TASK_DIR> --team <TEAM_NAME> [--max-workers 4]
```

**Worker routing (from config):**

If `cowork.worker_profiles` is configured in config.yaml, pass the profile settings to dispatch:

```bash
# If cowork.worker_profiles.default is set:
python3 ./.trellis/scripts/cowork/dispatch.py --task-dir <TRELLIS_TASK_DIR> --profile <DEFAULT_PROFILE>

# If cowork.worker_profiles.map is set (non-empty):
python3 ./.trellis/scripts/cowork/dispatch.py --task-dir <TRELLIS_TASK_DIR> --profile-map '<PROFILE_MAP_JSON>'

# If both are set, pass both:
python3 ./.trellis/scripts/cowork/dispatch.py --task-dir <TRELLIS_TASK_DIR> --profile <DEFAULT_PROFILE> --profile-map '<PROFILE_MAP_JSON>'
```

Note: dispatch.py reads config.yaml automatically for defaults.
CLI `--profile` and `--profile-map` override config values.
Per-task override: set `meta.profile` in child task.json.

`dispatch.py` reads `task.json`'s `children[]`, filters out completed subtasks, and dispatches up to `--max-workers` pending subtasks (one worker each).

**Key points:**
- Each worker gets an isolated worktree on branch `clawteam/<TEAM_NAME>/worker-N`
- All worker branches are based on the **current branch** (not hardcoded to "main")
- Workers are instructed to: implement → test → commit → report DONE
- Workers do **NOT** merge, cleanup, or stop their own sessions

Output includes `remaining` count — the number of subtasks still waiting in queue.

### 3.2 Babysit (Background Monitor)

```bash
RESULT_FILE="/tmp/babysit-${TEAM_NAME}.json"
python3 ./.trellis/scripts/cowork/babysit.py --team <TEAM_NAME> --result-file "$RESULT_FILE" &
BABYSIT_PID=$!
```

Note: babysit.py reads config.yaml for defaults (timeout, interval, auto_pr, auto_merge).
CLI flags override config. Key overrides:
- `--timeout 600` — override config.babysit.timeout
- `--auto-pr` — override config.auto_pr (enables PR creation)
- `--no-merge` — override config.babysit.auto_merge (disables auto-merge)

The babysit script runs in the background, polls every 10s (by default), and writes a JSON result file when done. It auto-merges completed worker branches.

**Wait for result** (deterministic, no streaming token cost):

```bash
while [ ! -s "$RESULT_FILE" ]; do
  echo "Workers still running... checking in 30s"
  sleep 30
done
cat "$RESULT_FILE"
kill $BABYSIT_PID 2>/dev/null
```

**Result file format:**
- `{"success": true, "reason": "all_completed", ...}` → all workers done
- `{"success": false, "reason": "worker_failed", "detail": "..."}` → a worker failed
- `{"success": false, "reason": "timeout", ...}` → timeout reached

### 3.2.5 Mid-Task Intervention

Activate this step ONLY when you detect:
- Worker stuck (no progress update for >10 minutes)
- Same error repeated 3+ times in worker logs
- Implementation clearly diverging from PRD spec

If none of these conditions apply, skip this step entirely.

If a worker is going off-track or stalled:

```bash
# Option A: Send message via inbox (recommended, works with all backends)
clawteam inbox send <TEAM> <WORKER> "STOP. Focus on API layer first."

# Option B: Direct terminal input (tmux backend only)
tmux send-keys -t clawteam-<TEAM>:<WORKER> "Read the PRD again, section 2 is wrong." Enter

# Option C: If worker is completely stuck (2+ nudges failed)
clawteam lifecycle request-shutdown <TEAM> leader <WORKER> --force
clawteam spawn --team <TEAM> --agent-name <WORKER> --task "Resume task: read prd.md and continue" --replace
```

### 3.3 Handle Merge Failures (if any)

If babysit exits with merge_failed:

```bash
# Check current status
git status
git log --oneline --graph -10

# Conflicts were auto-aborted by babysit
# Manually resolve:
# 1. Manually merge: git merge clawteam/<TEAM_NAME>/<worker-name>
# 2. Resolve conflicts
# 3. Commit: git commit
# 4. Restart: python3 .trellis/scripts/cowork/babysit.py --team <TEAM_NAME> --timeout 300 --result-file /tmp/babysit-<TEAM_NAME>.json
```

### 3.4 Code Review (using Subagent)

Now that all workers are done and all branches are merged to the current branch, run code review:

```bash
# Define review range
REVIEW_RANGE="${WORKFLOW_START_COMMIT}..HEAD"
```

Invoke the Agent tool with these parameters:

```
subagent_type: "check"
description: "Code review for workflow"
prompt: """
You are reviewing all code changes merged in this workflow.

Review Range: {REVIEW_RANGE}
Current Branch: {CURRENT_BRANCH}

Workers completed:
  - worker-1: implemented feature X
  - worker-2: implemented feature Y
  - ... (from monitor output)

Review focus areas:
1. **Code Quality**: Maintainability, readability, structure
2. **Security**: Potential vulnerabilities, unsafe operations
3. **Performance**: Inefficient algorithms, resource leaks
4. **Conventions**: Coding standards, naming conventions
5. **Testing**: Test coverage, test quality

Report format:
- CRITICAL: must-fix issues (security, crashes, data loss)
- HIGH: should-fix issues (major bugs, performance problems)
- MEDIUM: nice-to-fix issues (minor bugs, code smell)
- LOW: nitpicks (formatting, minor style issues)

If you find CRITICAL or HIGH severity issues:
1. List each issue with: file:line, severity, description, suggested fix
2. Return summary: "REVIEW_FAILED: N CRITICAL, M HIGH issues found"

If no CRITICAL or HIGH issues:
1. Summarize MEDIUM/LOW issues found (if any)
2. Return summary: "REVIEW_PASSED: N MEDIUM, M LOW issues found"
"""
```

**Check the result:**
- If result contains "REVIEW_FAILED" → report issues to user, DO NOT cleanup, exit
- If result contains "REVIEW_PASSED" → proceed to verification

### 3.5 Quality Gate (using Subagent with clawteam-verifier skill)

Run final verification:

```
subagent_type: "check"
description: "Build and test verification"
prompt: """
You are verifying the build and tests for this workflow.

## Step 1: Load clawteam-verifier skill
Use the skill tool to load "clawteam-verifier". This skill contains verification logic and thresholds.

## Step 2: Build the project
Build command: {BUILD_CMD}  # e.g., "cmake --build build --parallel"

## Step 3: Run tests
Test command: {TEST_CMD}  # e.g., "./build/tests/unit_tests"

## Step 4: Monitor resource usage
During test run, monitor:
- Memory usage (max: 512 MB)
- CPU usage (max: 80%)
- Time (max: 15 seconds)

## Step 5: Report results
Report format:
- If build fails: "VERIFY_FAILED: Build error: <error details>"
- If tests fail: "VERIFY_FAILED: Test failures: <details>"
- If thresholds exceeded: "VERIFY_WARNING: Resource usage: memory=XXX MB, cpu=XX%, time=XX s"
- If all pass: "VERIFY_PASSED: Build successful, tests pass, resources within limits"

Provide detailed output for each step.
"""
```

**Check the result:**
- If result contains "VERIFY_FAILED" → report failure to user, DO NOT cleanup, exit
- If result contains "VERIFY_WARNING" → report warning, proceed to cleanup
- If result contains "VERIFY_PASSED" → proceed to cleanup

### 3.6 Cleanup

After verification passes, cleanup the clawteam team:

```bash
clawteam team cleanup <TEAM_NAME> --force
```

Worker sessions and worktrees have already been cleaned by babysit (Step 3.2).

### 3.7 Completion

```
Clone mode complete. Quality: review passed, verifier passed.

Workers completed and merged:
  - worker-1: +X/-Y files (coverage: 92%)
  - worker-2: +X/-Y files (coverage: 87%)

All changes on current branch: {CURRENT_BRANCH}
Commit range: {WORKFLOW_START_COMMIT}..HEAD

Next steps:
  /trellis:finish-work   # for each task
  /trellis:archive       # when verified
```

---

## Direct Mode

If user chose Direct Mode, follow standard Trellis workflow:
1. Read PRD and specs -> 2. Implement -> 3. Unit tests (coverage >= 85%) -> 4. Checks -> 5. `/trellis:finish-work` -> 6. User archives

---

## Principles

1. **User decides** - LLM recommends, user chooses (10s timeout auto-selects recommendation)
2. **User archives** - no agent runs finish-work or archive
3. **Trellis is truth** - workers execute real `.trellis/tasks/` tasks
4. **ClawTeam is engine** - dispatch, lifecycle, worktree management
5. **Unified commits** - all commits use user's git identity
6. **Coverage >= 85%** - workers must write unit tests meeting this threshold
7. **Quality gate** - code review + clawteam-verifier after merge before cleanup

---

## Cron Monitoring

For continuous monitoring without an AI session (separate from the main workflow):

```bash
# Add to crontab (every 10 minutes)
crontab -e
# Add this line:
*/10 * * * * python3 /absolute/path/to/.trellis/scripts/cowork/babysit.py --all --timeout 600 2>/dev/null
```

Notifications are written to `~/.trellis/notifications/` and shown at the start of each workflow/cowork session.

This is a standalone maintenance task, not part of the automated pipeline.
Configure in config.yaml under `cowork.babysit` for timeout/interval defaults.
