---
name: clawteam-verifier
description: >
  Use when a ClawTeam worker agent completes a task and the leader needs to
  validate build success, runtime stability, and resource usage before merging
  worktrees or creating PRs. Trigger on "verify worker output", "quality gate",
  "build check", "resource monitoring", or when a ClawTeam template reaches its
  verification stage.
version: 0.3.0
---

# ClawTeam Verifier

Post-task quality gate for ClawTeam worker outputs. Validates compilation,
monitors runtime resources, and enforces thresholds — all in one shot.

## Overview

The verifier is a **standby agent** in a ClawTeam team. It polls inbox for
JSON verification tasks, executes build + run checks, and reports pass/warn/fail
to the leader. No manual intervention required.

## When to Use

- After a ClawTeam worker marks a task `completed`
- Before merging a worktree (`clawteam workspace merge`)
- Before creating a PR
- When a ClawTeam template reaches its quality-gate stage

**NOT for:** code review (use `code-review-assistant`), unit tests (use
`generating-qt-unit-tests`), or static analysis.

## Verification Task Schema

The leader sends a JSON payload via inbox:

```json
{
  "target_agent": "worker1",
  "work_dir": "/home/user/.clawteam/workspaces/my-team/worker1",
  "build_command": "cmake --build build --parallel",
  "run_command": "./build/bin/myapp",
  "thresholds": {
    "max_memory_mb": 512,
    "max_cpu_percent": 80,
    "max_time_seconds": 10
  }
}
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `target_agent` | yes | — | Name of the worker being verified |
| `work_dir` | yes | cwd | Absolute path to worker's workspace |
| `build_command` | yes | — | Shell command to compile the project |
| `run_command` | yes | — | Shell command to run the binary |
| `thresholds` | yes | — | Resource limits (see below) |

## Verification Chain

```
Compile
  ↓ fail → ❌ FAIL (report immediately, skip runtime)
  ↓ pass
Phase 1: Fast Run (3s timeout, snapshot metrics)
  ↓ crash → ❌ FAIL (report immediately)
  ↓ pass
Phase 2: Deep Run (max_time_seconds, peak/avg metrics, captured output)
  ↓ resource exceed → ⚠️ WARNING
  ↓ pass → ✅ PASS
```

Phase 1 catches instant crashes with minimal overhead. Phase 2 monitors real
runtime behavior — peak memory, average CPU, and captured stderr for diagnosis.

## Threshold Defaults by App Type

| App Type | `max_memory_mb` | `max_cpu_percent` | `max_time_seconds` |
|----------|-----------------|-------------------|--------------------|
| GUI (DDE/Qt) | 1024 | 80 | 15 |
| CLI tool | 256 | 50 | 5 |
| Background service | 512 | 60 | 10 |
| Library/component | 128 | 40 | 3 |

## Status Determination

```
compile_failed          → fail (skip runtime)
fast crash (non-SIGTERM) → fail (skip deep)
deep crash (non-SIGTERM) → fail
memory > 120%           → fail (uses peak_memory_mb)
cpu > limit             → warning (uses avg_cpu_percent)
memory > 100%           → warning
otherwise               → pass
```

## Verifier Agent Constraints

| Rule | Why |
|------|-----|
| Process one task at a time | Sequential avoids resource contention |
| Never exit, always return to standby | Team lifecycle may need multiple verifications |
| Validate JSON before execution | Malformed input causes silent failures |
| Use `psutil` for monitoring | Required dependency: `pip install psutil>=5.9.0` |
| Use fast smoke-test commands | `--version`, `--help` — avoid long-running suites |

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| No report generated | `psutil` not installed | `pip install psutil>=5.9.0` |
| Verification timeout | `run_command` hangs | Use `--version` instead of full test suite |
| Compile fails | Worker code has syntax errors | Check report, notify worker to fix |
| Memory exceeds limit | Memory leak in build output | Raise threshold or request worker fix |
| Verifier unresponsive | Inbox not being polled | Check verifier tmux session is alive |

## Implementation

The verification logic is in `scripts/`:

```python
import sys, json
sys.path.insert(0, '/path/to/skills')
from verifier.scripts.verifier import execute_verification

task = json.loads(inbox_message)
result = execute_verification(task)
# result.status: 'pass' | 'warning' | 'fail'
# result.report_file: path to markdown report
# result.summary: brief text for inbox
```

Key modules:
- `scripts/verifier.py` — orchestration and status determination
- `scripts/monitor.py` — `compile_check()` and `Monitor` (psutil-based)
- `scripts/reporter.py` — markdown report generation
