#!/usr/bin/env python3
"""
ClawTeam Parallel Dispatch - Clone Mode.

Dispatch existing Trellis tasks to ClawTeam workers for parallel execution.

Usage:
    python3 dispatch.py --task-dir <trellis-task-dir> [--team <team-name>] [--max-workers 4]

This script:
1. Reads task.json, detects child subtasks via children[]
2. Filters out already-completed subtasks (status=completed)
3. Dispatches up to --max-workers pending subtasks (one worker each)
4. Marks dispatched subtasks as status=in_progress
5. Reports remaining count for the caller to dispatch next batch later

If no children exist, dispatches single task as before.
"""

from __future__ import annotations

import json
import subprocess
import sys
from datetime import datetime
from pathlib import Path

try:
    from . import cowork_config
except ImportError:
    import cowork_config


FILE_TASK_JSON = "task.json"


def get_repo_root() -> Path:
    """Resolve the repository root from current working directory."""
    result = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode == 0:
        return Path(result.stdout.strip())
    return Path.cwd()


# =============================================================================
# Colors
# =============================================================================


class Colors:
    RED = "\033[0;31m"
    GREEN = "\033[0;32m"
    YELLOW = "\033[1;33m"
    BLUE = "\033[0;34m"
    NC = "\033[0m"


def log_info(msg: str) -> None:
    print(f"{Colors.BLUE}[INFO]{Colors.NC} {msg}")


def log_success(msg: str) -> None:
    print(f"{Colors.GREEN}[SUCCESS]{Colors.NC} {msg}")


def log_warn(msg: str) -> None:
    print(f"{Colors.YELLOW}[WARN]{Colors.NC} {msg}")


def log_error(msg: str) -> None:
    print(f"{Colors.RED}[ERROR]{Colors.NC} {msg}")


# =============================================================================
# Worker Prompt Template
# =============================================================================

WORKER_PROMPT_TEMPLATE = """You are executing a Trellis task as the user extensions.

Task directory: {task_dir}

## Setup

1. Run: python3 .trellis/scripts/cowork/setup.py "$(pwd)" "{task_dir}"
   This copies the task directory into the worktree and sets .current-task,
   enabling the Trellis Hook system to inject code-spec and context.

## Task Requirements

- Follow the Trellis workflow: read prd.md, follow code-spec files from Hook system
- Implement the feature/fix as specified in PRD and code-spec
- Write comprehensive unit tests for ALL new code
- Unit test coverage must reach 85% or above
- If coverage is below 85%, add more tests until the threshold is met
- Run tests and verify all pass before committing

## Progress Reporting (during worker loop step "Do the work")

The default worker protocol only reports to leader at completion. For long tasks,
you MUST send progress updates so the coordinator can track your status:

- After reading PRD and understanding requirements:
  clawteam inbox send {team_name} leader "PROGRESS: requirements understood, starting implementation"
- After code implementation is complete (before writing tests):
  clawteam inbox send {team_name} leader "PROGRESS: code implementation done, writing tests"
- After tests pass (before commit):
  clawteam inbox send {team_name} leader "PROGRESS: tests passing, coverage XX%, about to commit"
- On any error that blocks progress:
  clawteam inbox send {team_name} leader "FAILED: <error description>"

## Inbox Monitoring

- After every major step (PRD read, implementation, tests), check your inbox:
  clawteam inbox receive {team_name}
- If you receive a message from leader, follow the instruction immediately.
- Leader may correct your direction or ask you to stop — obey without question.

## Constraints

- You are a clone of the user. All code belongs to the user.
- Do NOT execute /trellis:finish-work or /trellis:archive
- Do NOT modify .trellis/.developer
- Unit test coverage >= 85% is MANDATORY - task is not complete without meeting this threshold"""


WORKER_PROMPT_TRIVIAL = """You are executing a Trellis task as the user extensions.

Task directory: {task_dir}

## Setup
1. Run: python3 .trellis/scripts/cowork/setup.py "$(pwd)" "{task_dir}"

## Task
Implement the fix/change as specified in prd.md. Commit when done.

## Inbox
Before committing, check for leader instructions:
  clawteam inbox receive {team_name}
If leader sent corrections, follow them before committing.

## Constraints
Do NOT run /trellis:finish-work or /trellis:archive."""

WORKER_PROMPT_SIMPLE = """You are executing a Trellis task as the user extensions.

Task directory: {task_dir}

## Setup
1. Run: python3 .trellis/scripts/cowork/setup.py "$(pwd)" "{task_dir}"

## Task
- Follow the Trellis workflow: read prd.md, follow code-spec files from Hook system
- Implement the feature/fix as specified in PRD and code-spec
- Write unit tests for new code, verify all pass before committing
- On blocking error: clawteam inbox send {team_name} leader "FAILED: <error>"

## Inbox
- Check inbox after implementation: clawteam inbox receive {team_name}
- If leader sends corrections, follow immediately.

## Constraints
- You are a clone of the user. All code belongs to the user.
- Do NOT execute /trellis:finish-work or /trellis:archive"""


WORKER_PROMPTS = {
    "trivial": WORKER_PROMPT_TRIVIAL,
    "simple": WORKER_PROMPT_SIMPLE,
    "complex": WORKER_PROMPT_TEMPLATE,
}


def _select_prompt(task_data: dict) -> str:
    complexity = task_data.get("meta", {}).get("complexity", "complex")
    return WORKER_PROMPTS.get(complexity, WORKER_PROMPT_TEMPLATE)


# =============================================================================
# Helpers
# =============================================================================


def _run_clawteam(args: list[str], json_output: bool = False) -> tuple[int, str, str]:
    """Run clawteam command."""
    cmd = ["clawteam"]
    if json_output:
        cmd.append("--json")
    cmd.extend(args)

    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            check=False,
        )
        return result.returncode, result.stdout, result.stderr
    except FileNotFoundError:
        return 1, "", "clawteam command not found"


def _read_task_json(task_dir: Path) -> dict | None:
    """Read task.json from Trellis task directory."""
    task_json = task_dir / FILE_TASK_JSON
    if not task_json.is_file():
        return None
    try:
        return json.loads(task_json.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError):
        return None


def _write_task_json(task_dir: Path, data: dict) -> bool:
    """Write dict to task.json in the given directory."""
    try:
        (task_dir / FILE_TASK_JSON).write_text(
            json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8",
        )
        return True
    except (OSError, IOError):
        return False


def _generate_team_name() -> str:
    """Generate a team name for parallel execution."""
    return f"parallel-{datetime.now().strftime('%m%d-%H%M')}"


# =============================================================================
# Main Operations
# =============================================================================


def create_team(team_name: str, description: str) -> dict:
    """Create a ClawTeam team."""
    ret, stdout, stderr = _run_clawteam(
        [
            "team",
            "spawn-team",
            team_name,
            "-d",
            description,
        ]
    )

    if ret != 0:
        return {"success": False, "error": stderr or stdout}

    return {"success": True, "team_name": team_name}


def create_task(
    team_name: str,
    title: str,
    owner: str,
    trellis_task_dir: str,
    branch: str,
    blocked_by: list[str] | None = None,
) -> dict:
    """Create a ClawTeam task mapped to Trellis task.

    The Trellis task directory is encoded in the description since
    ClawTeam tasks don't support custom metadata fields.
    """
    description = (
        f"TRELLIS_TASK_DIR={trellis_task_dir} | "
        f"Branch: {branch} | "
        f"Execute the Trellis task in this directory. "
        f"Read .trellis/.current-task after setup. "
        f"REQUIREMENT: unit test coverage >= 85%."
    )

    args = [
        "task",
        "create",
        team_name,
        title,
        "-o",
        owner,
        "-d",
        description,
    ]

    if blocked_by:
        args.extend(["--blocked-by", ",".join(blocked_by)])

    ret, stdout, stderr = _run_clawteam(args, json_output=True)

    if ret != 0:
        return {"success": False, "error": stderr or stdout}

    try:
        result = json.loads(stdout)
        return {"success": True, "task_id": result.get("id")}
    except json.JSONDecodeError:
        # If not JSON, try to extract from text output
        return {"success": True, "raw_output": stdout}


def spawn_worker(
    team_name: str,
    worker_name: str,
    task_prompt: str,
    profile: str | None = None,
) -> dict:
    """Spawn a ClawTeam worker agent."""
    args = [
        "spawn",
        "-t",
        team_name,
        "-n",
        worker_name,
        "--task",
        task_prompt,
        "--workspace",
    ]
    if profile:
        args.extend(["--profile", profile])

    ret, stdout, stderr = _run_clawteam(args)

    if ret != 0:
        return {"success": False, "error": stderr or stdout}

    return {"success": True, "worker_name": worker_name}


def _select_profile(task_data: dict, default_profile: str | None = None, profile_map: dict | None = None) -> str | None:
    """Select profile for a task based on meta.profile, profile_map, or default."""
    meta = task_data.get("meta", {})
    # Priority: meta.profile > profile_map[dev_type] > default_profile
    if "profile" in meta:
        return meta["profile"]
    if profile_map:
        dev_type = task_data.get("dev_type", "")
        if dev_type in profile_map:
            return profile_map[dev_type]
    return default_profile


def dispatch_task(
    task_dir: str,
    team_name: str | None = None,
    worker_name: str | None = None,
    max_workers: int = 4,
    profile: str | None = None,
    profile_map: dict | None = None,
) -> dict:
    """Dispatch a Trellis task to ClawTeam worker(s).

    If the task has children (subtasks), dispatches up to max_workers
    pending subtasks (skipping completed ones). Caller should call
    again with the same --team to dispatch the next batch after workers
    finish. If no children exist, dispatches as a single task.
    """
    project_root = get_repo_root()
    task_path = Path(task_dir)
    if not task_path.is_absolute():
        task_path = project_root / task_dir

    if not task_path.is_dir():
        return {"success": False, "error": f"Task directory not found: {task_dir}"}

    task_data = _read_task_json(task_path)
    if not task_data:
        return {
            "success": False,
            "error": f"task.json not found or invalid in {task_dir}",
        }

    children = task_data.get("children", [])

    # Normalize children: list[dict] -> list[str] (directory names)
    if children and isinstance(children[0], dict):
        children = [c["id"] for c in children if "id" in c]

    if not children:
        return _dispatch_single(task_path, task_data, team_name, worker_name, profile, profile_map)

    return _dispatch_children(task_path, task_data, children, team_name, max_workers, profile, profile_map)


def _dispatch_single(
    task_path: Path,
    task_data: dict,
    team_name: str | None,
    worker_name: str | None,
    default_profile: str | None = None,
    profile_map: dict | None = None,
) -> dict:
    """Dispatch a single task (no subtasks) to one worker."""
    task_title = task_data.get("title", "Untitled Task")
    branch = task_data.get("branch", "")

    # Select profile for this task
    profile = _select_profile(task_data, default_profile, profile_map)

    if not team_name:
        team_name = _generate_team_name()

    if not worker_name:
        task_slug = task_path.name
        worker_name = f"worker-{task_slug[:10]}"

    # Create team if needed
    log_info(f"Setting up team: {team_name}")
    create_result = create_team(team_name, f"Parallel execution for Trellis tasks")
    if create_result.get("success"):
        log_success(f"Team created: {team_name}")

    # Create ClawTeam task
    log_info(f"Creating task: {task_title}")
    task_result = create_task(
        team_name=team_name,
        title=task_title,
        owner=worker_name,
        trellis_task_dir=str(task_path),
        branch=branch,
    )

    if not task_result.get("success"):
        return {
            "success": False,
            "error": f"Failed to create ClawTeam task: {task_result.get('error')}",
        }

    log_success(f"ClawTeam task created: {task_result.get('task_id', 'N/A')}")

    # Generate worker prompt
    task_prompt = _select_prompt(task_data).format(team_name=team_name, task_dir=str(task_path))

    # Spawn worker
    log_info(f"Spawning worker: {worker_name}")
    spawn_result = spawn_worker(
        team_name=team_name,
        worker_name=worker_name,
        task_prompt=task_prompt,
        profile=profile,
    )

    if not spawn_result.get("success"):
        return {
            "success": False,
            "error": f"Failed to spawn worker: {spawn_result.get('error')}",
        }

    return {
        "success": True,
        "team_name": team_name,
        "worker_count": 1,
        "remaining": 0,
        "total": 1,
        "completed": 0,
        "workers": [{
            "worker_name": worker_name,
            "task_title": task_title,
            "task_dir": str(task_path),
        }],
    }


def _dispatch_children(
    parent_path: Path,
    parent_data: dict,
    children: list[str],
    team_name: str | None,
    max_workers: int,
    default_profile: str | None = None,
    profile_map: dict | None = None,
) -> dict:
    """Dispatch a batch of pending child subtasks to workers."""
    parent_title = parent_data.get("title", "Parent Task")
    tasks_dir = parent_path.parent

    # Collect pending (not yet dispatched or completed)
    pending = []
    for child_name in children:
        child_path = tasks_dir / child_name
        child_data = _read_task_json(child_path)
        if not child_data:
            log_warn(f"Skipping {child_name}: task.json not found")
            continue
        status = child_data.get("status", "planning")
        if status in ("completed", "in_progress"):
            continue
        pending.append((child_name, child_path, child_data))

    total = len(children)
    completed = total - len(pending)
    to_dispatch = pending[:max_workers]
    remaining = len(pending) - len(to_dispatch)

    if not to_dispatch:
        log_info(f"All {total} subtasks completed")
        return {
            "success": True,
            "team_name": team_name,
            "worker_count": 0,
            "remaining": 0,
            "total": total,
            "completed": completed,
            "workers": [],
            "all_done": True,
        }

    log_info(
        f"Subtasks: {total} total, {completed} completed, "
        f"{len(to_dispatch)} dispatching, {remaining} queued (max_workers={max_workers})"
    )

    if not team_name:
        team_name = _generate_team_name()

    # Create team if needed (ignore error if already exists for replenish)
    log_info(f"Setting up team: {team_name}")
    create_team(team_name, f"Parallel execution: {parent_title}")

    workers = []
    for idx, (child_name, child_path, child_data) in enumerate(to_dispatch):
        child_title = child_data.get("title", child_name)
        branch = child_data.get("branch", parent_data.get("branch", ""))
        wname = f"worker-{idx + 1}-{child_name[:12]}"

        log_info(f"Creating task: {child_title}")
        task_result = create_task(
            team_name=team_name,
            title=child_title,
            owner=wname,
            trellis_task_dir=str(child_path),
            branch=branch,
        )

        if not task_result.get("success"):
            log_error(f"Failed to create ClawTeam task for {child_title}")
            continue

        log_success(f"ClawTeam task created: {task_result.get('task_id', 'N/A')}")

        task_prompt = _select_prompt(child_data).format(
            team_name=team_name, task_dir=str(child_path),
        )

        # Select profile for this child task
        child_profile = _select_profile(child_data, default_profile, profile_map)

        log_info(f"Spawning worker: {wname}")
        spawn_result = spawn_worker(
            team_name=team_name,
            worker_name=wname,
            task_prompt=task_prompt,
            profile=child_profile,
        )

        if not spawn_result.get("success"):
            log_error(f"Failed to spawn worker {wname}")
            continue

        # Mark subtask as in_progress
        child_data["status"] = "in_progress"
        _write_task_json(child_path, child_data)

        workers.append({
            "worker_name": wname,
            "task_title": child_title,
            "task_dir": str(child_path),
        })
        log_success(f"Dispatched: {child_name} -> {wname}")

    return {
        "success": len(workers) > 0,
        "team_name": team_name,
        "worker_count": len(workers),
        "remaining": remaining,
        "total": total,
        "completed": completed,
        "workers": workers,
        "all_done": remaining == 0 and completed + len(workers) == total,
    }


# =============================================================================
# CLI
# =============================================================================


def main() -> int:
    """CLI entry point."""
    import argparse

    config = cowork_config.load_cowork_config()
    worker_profiles = config.get("worker_profiles", {})
    config_profile_map = worker_profiles.get("map", {})

    parser = argparse.ArgumentParser(
        description="ClawTeam Parallel Dispatch - Clone Mode"
    )
    parser.add_argument(
        "--task-dir", "-t", required=True, help="Trellis task directory path"
    )
    parser.add_argument("--team", help="Team name (auto-created if not provided)")
    parser.add_argument(
        "--worker", "-w", help="Worker name (single-task mode only)"
    )
    parser.add_argument(
        "--max-workers", "-m", type=int,
        default=config.get("max_workers", 4),
        help=f"Max parallel workers per batch (default: {config.get('max_workers', 4)})",
    )
    parser.add_argument(
        "--profile",
        default=worker_profiles.get("default"),
        help="Default ClawTeam profile for all workers",
    )
    parser.add_argument(
        "--profile-map", help="JSON mapping dev_type -> profile_name, e.g. '{\"backend\":\"claude-sonnet\"}'"
    )
    parser.add_argument("--json", action="store_true", help="Output as JSON")

    args = parser.parse_args()

    # Config provides dict, CLI provides JSON string
    profile_map = config_profile_map
    if args.profile_map:
        try:
            profile_map = json.loads(args.profile_map)
        except json.JSONDecodeError:
            log_error(f"Invalid JSON in --profile-map: {args.profile_map}")
            return 1

    result = dispatch_task(
        task_dir=args.task_dir,
        team_name=args.team,
        worker_name=args.worker,
        max_workers=args.max_workers,
        profile=args.profile,
        profile_map=profile_map,
    )

    if args.json:
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        if result["success"]:
            print()
            log_success(
                f"Dispatched {result['worker_count']} worker(s) "
                f"({result.get('completed', 0)}/{result['total']} completed, "
                f"{result['remaining']} remaining)"
            )
            print()
            print(f"  Team:    {result['team_name']}")
            print(f"  Workers: {result['worker_count']}")
            for w in result.get("workers", []):
                print(f"    - {w['worker_name']}: {w['task_title']}")
            if result.get("remaining", 0) > 0:
                print()
                log_warn(
                    f"{result['remaining']} subtask(s) queued "
                    f"— call dispatch again after a worker finishes"
                )
            if result.get("all_done"):
                print()
                log_success("All subtasks dispatched and done!")
            print()
            print("Monitor commands:")
            print(f"  python3 .trellis/scripts/cowork/babysit.py --team {result['team_name']} --timeout 300 --result-file /tmp/babysit-{result['team_name']}.json")
            print(f"  # Add --auto-pr to auto-create PR when done")
            print(f"  clawteam board show {result['team_name']}")
        else:
            log_error(result.get("error", "Dispatch failed"))
            return 1

    return 0 if result["success"] else 1


if __name__ == "__main__":
    sys.exit(main())
