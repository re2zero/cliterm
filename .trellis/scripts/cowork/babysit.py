#!/usr/bin/env python3
"""
Babysit - Deterministic Background Monitor for ClawTeam Workers.

Runs as a background process and writes a single JSON result file when done.
Zero streaming token cost — the AI session only reads the result file.

Usage:
    python3 babysit.py --team <name> --timeout 300 --result-file /tmp/babysit-<TEAM>.json
    python3 babysit.py --all --timeout 600

Polling loop (every 10 seconds):
    1. clawteam --json task list <team> -> track status transitions
    2. clawteam inbox receive leader -> check for FAILED messages
    3. On completion: write JSON result to result-file
    4. On --all mode: write notifications to ~/.trellis/notifications/
"""

from __future__ import annotations

import json
import subprocess
import sys
import time
from datetime import datetime
from pathlib import Path

try:
    from . import cowork_config
except ImportError:
    import cowork_config


# =============================================================================
# Colors (stderr only — stdout must be silent)
# =============================================================================


class Colors:
    RED = "\033[0;31m"
    GREEN = "\033[0;32m"
    YELLOW = "\033[1;33m"
    NC = "\033[0m"


def log_error(msg: str) -> None:
    print(f"{Colors.RED}[BABYSIT]{Colors.NC} {msg}", file=sys.stderr)


def log_info(msg: str) -> None:
    print(f"{Colors.GREEN}[BABYSIT]{Colors.NC} {msg}", file=sys.stderr)


# =============================================================================
# Notification directory
# =============================================================================

NOTIFICATIONS_DIR = Path.home() / ".trellis" / "notifications"


def ensure_notifications_dir() -> Path:
    """Create notifications directory if it doesn't exist."""
    NOTIFICATIONS_DIR.mkdir(parents=True, exist_ok=True)
    return NOTIFICATIONS_DIR


def write_notification(team: str, ntype: str, detail: str = "", elapsed: int = 0) -> Path:
    """Write a notification file to ~/.trellis/notifications/."""
    ensure_notifications_dir()
    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    filename = f"{timestamp}-{team}-{ntype}.json"
    path = NOTIFICATIONS_DIR / filename

    notification = {
        "team": team,
        "type": ntype,
        "detail": detail,
        "elapsed": elapsed,
        "timestamp": datetime.now().isoformat(),
    }

    path.write_text(json.dumps(notification, indent=2, ensure_ascii=False), encoding="utf-8")
    return path


# =============================================================================
# ClawTeam & Git helpers
# =============================================================================


def _run_clawteam(args: list[str]) -> tuple[int, str, str]:
    """Run clawteam command."""
    cmd = ["clawteam"] + args
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


def _run_git(args: list[str], cwd: str = None) -> tuple[int, str, str]:
    """Run git command."""
    cmd = ["git"] + args
    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            check=False,
            cwd=cwd,
        )
        return result.returncode, result.stdout, result.stderr
    except FileNotFoundError:
        return 1, "", "git command not found"


def _run_bash(args: list[str]) -> tuple[int, str, str]:
    """Run bash command."""
    try:
        result = subprocess.run(
            args,
            capture_output=True,
            text=True,
            check=False,
        )
        return result.returncode, result.stdout, result.stderr
    except FileNotFoundError:
        return 1, "", f"{args[0]} command not found"


# =============================================================================
# Task status helpers
# =============================================================================


def get_task_summary(team_name: str) -> dict:
    """Parse task list JSON into summary counts.

    Uses ``clawteam --json task list <team>`` to get structured JSON.
    """
    ret, stdout, _ = _run_clawteam(["--json", "task", "list", team_name])
    if ret != 0 or not stdout.strip():
        return {
            "total": 0,
            "completed": 0,
            "in_progress": 0,
            "pending": 0,
            "blocked": 0,
            "tasks": [],
        }
    try:
        tasks = json.loads(stdout)
        if not isinstance(tasks, list):
            tasks = []
    except (json.JSONDecodeError, TypeError):
        tasks = []

    statuses = [str(t.get("status", "")).lower() for t in tasks]
    return {
        "total": len(tasks),
        "completed": sum(1 for s in statuses if s in ("completed", "done")),
        "in_progress": sum(1 for s in statuses if s == "in_progress"),
        "pending": sum(1 for s in statuses if s == "pending"),
        "blocked": sum(1 for s in statuses if s == "blocked"),
        "tasks": tasks,
    }


def get_current_branch() -> str:
    """Get current git branch."""
    ret, stdout, _ = _run_git(["rev-parse", "--abbrev-ref", "HEAD"])
    if ret == 0:
        return stdout.strip()
    return "main"


def merge_worker_branch(team_name: str, worker_name: str) -> dict:
    """Merge worker branch to current branch."""
    branch_name = f"clawteam/{team_name}/{worker_name}"
    log_info(f"Merging {branch_name} into current branch...")

    ret, stdout, stderr = _run_git(["merge", "--no-ff", "--no-edit", branch_name])
    if ret == 0:
        log_info(f"Merged {branch_name}")
        return {"success": True}
    else:
        _run_git(["merge", "--abort"])
        log_error(f"Merge conflict in {branch_name}, aborted")
        return {"success": False, "error": stderr}


def cleanup_worker_worktree(team_name: str, worker_name: str) -> dict:
    """Clean up worker's worktree."""
    ret, stdout, stderr = _run_clawteam(
        ["workspace", "cleanup", team_name, worker_name, "--force"]
    )
    if ret == 0:
        log_info(f"Cleaned worktree: {worker_name}")
    return {"success": ret == 0}


def _auto_create_pr(team_name: str, summary: dict) -> dict:
    """Auto-create PR after all tasks complete using gh CLI.

    Steps:
    1. Check if we're on a feature branch (not main/master)
    2. git add -A && git commit (if uncommitted changes)
    3. git push origin HEAD (with --force-with-lease for safety)
    4. gh pr create --fill
    5. Return result dict
    """
    branch = get_current_branch()

    # Don't PR to main from main
    if branch in ("main", "master"):
        return {"success": False, "error": f"On default branch '{branch}', skipping PR"}

    # Commit uncommitted changes
    ret, stdout, stderr = _run_git(["status", "--porcelain"])
    if ret == 0 and stdout.strip():
        _run_git(["add", "-A"])
        ret, _, stderr = _run_git(["commit", "-m", f"feat({team_name}): complete parallel tasks"])
        if ret != 0:
            return {"success": False, "error": f"Commit failed: {stderr}"}

    # Push
    ret, stdout, stderr = _run_git(["push", "origin", "HEAD", "--force-with-lease"])
    if ret != 0:
        # Try without --force-with-lease for new branches
        ret, stdout, stderr = _run_git(["push", "-u", "origin", "HEAD"])
        if ret != 0:
            return {"success": False, "error": f"Push failed: {stderr}"}

    # Create PR
    ret, stdout, stderr = _run_bash(["gh", "pr", "create", "--fill"])
    if ret != 0:
        return {"success": False, "error": f"PR creation failed: {stderr}"}

    # Extract PR URL from output
    pr_url = stdout.strip().splitlines()[-1] if stdout.strip() else ""
    return {"success": True, "url": pr_url, "branch": branch}


def stop_worker_session(team_name: str, worker_name: str) -> dict:
    """Stop worker's session."""
    ret, stdout, stderr = _run_clawteam(
        ["lifecycle", "request-shutdown", team_name, "leader", worker_name, "--force"]
    )
    if ret == 0:
        log_info(f"Stopped session: {worker_name}")
    return {"success": ret == 0}


# =============================================================================
# Babysit loop
# =============================================================================


def babysit_team(
    team_name: str,
    timeout: int = 300,
    result_file: str | None = None,
    interval: int = 10,
    auto_merge: bool = True,
    auto_pr: bool = False,
) -> dict:
    """Background monitor for a single team.

    Polls task status every `interval` seconds. Writes result to `result_file`
    when done. Returns the result dict.

    Exit conditions:
    - All tasks completed  -> {"success": true, "reason": "all_completed", ...}
    - Timeout             -> {"success": false, "reason": "timeout", ...}
    - Worker FAILURE msg   -> {"success": false, "reason": "worker_failed", ...}
    """
    start = time.time()
    prev_summary: dict | None = None
    seen_failures: set[str] = set()

    log_info(f"Monitoring team '{team_name}' | interval={interval}s | timeout={timeout}s")

    while True:
        elapsed = int(time.time() - start)
        remaining = timeout - elapsed

        if remaining <= 0:
            result = {
                "success": False,
                "reason": "timeout",
                "elapsed": elapsed,
                "timestamp": datetime.now().isoformat(),
                "team": team_name,
            }
            _write_result(result_file, result)
            write_notification(team_name, "timeout", f"No completion within {timeout}s", elapsed)
            return result

        # --- 1. Check leader inbox for FAILED messages (peek, non-destructive) ---
        # Use peek (not receive) to avoid consuming PROGRESS messages
        # that the AI session should be able to read later.
        ret, inbox_out, _ = _run_clawteam(["inbox", "peek", team_name, "--agent", "leader"])
        if ret == 0 and inbox_out.strip():
            for line in inbox_out.strip().splitlines():
                line = line.strip()
                if not line or "FAILED:" not in line:
                    continue
                dedup_key = line[:80]
                if dedup_key in seen_failures:
                    continue
                seen_failures.add(dedup_key)

                log_error(f"Worker failure: {line}")
                result = {
                    "success": False,
                    "reason": "worker_failed",
                    "detail": line,
                    "elapsed": elapsed,
                    "timestamp": datetime.now().isoformat(),
                    "team": team_name,
                }
                _write_result(result_file, result)
                write_notification(team_name, "worker_failed", line, elapsed)
                return result

        # --- 2. Check task status ---
        summary = get_task_summary(team_name)
        log_info(
            f"[{elapsed}s] {summary['completed']}/{summary['total']} "
            f"(done={summary['completed']} active={summary['in_progress']} "
            f"pending={summary['pending']}) [{remaining}s left]"
        )

        # Auto-merge on task completion transition
        if auto_merge and summary["tasks"] and prev_summary and prev_summary["tasks"]:
            for t in summary["tasks"]:
                if t.get("status") == "completed":
                    owner = t.get("owner", "")
                    prev_task = next(
                        (pt for pt in prev_summary.get("tasks", [])
                         if pt.get("id") == t.get("id")),
                        None,
                    )
                    was_in_progress = prev_task and prev_task.get("status") == "in_progress"
                    if was_in_progress:
                        log_info(f"{owner} task completed, auto-merging...")
                        merge_worker_branch(team_name, owner)
                        cleanup_worker_worktree(team_name, owner)
                        stop_worker_session(team_name, owner)

        # All completed?
        if summary["total"] > 0 and summary["completed"] == summary["total"]:
            result = {
                "success": True,
                "reason": "all_completed",
                "elapsed": elapsed,
                "tasks": summary["tasks"],
                "timestamp": datetime.now().isoformat(),
                "team": team_name,
            }
            log_info(f"All {summary['total']} tasks completed ({elapsed}s)")

            # Auto PR (before writing result, so PR info is included)
            if auto_pr:
                pr_result = _auto_create_pr(team_name, summary)
                result["pr"] = pr_result
                if pr_result["success"]:
                    write_notification(team_name, "auto_pr", pr_result["url"], elapsed)
                else:
                    write_notification(team_name, "auto_pr_failed", pr_result["error"], elapsed)

            write_notification(team_name, "all_completed", f"All {summary['total']} tasks done", elapsed)
            _write_result(result_file, result)

            return result

        prev_summary = summary
        time.sleep(interval)


def babysit_all(timeout: int = 600, interval: int = 10, auto_pr: bool = False) -> list[dict]:
    """Scan all active ClawTeam teams and babysit them.

    Checks ~/.clawteam/teams/ for active teams and monitors each.
    Writes notifications to ~/.trellis/notifications/.
    """
    teams_dir = Path.home() / ".clawteam" / "teams"
    if not teams_dir.is_dir():
        log_error(f"No teams directory found: {teams_dir}")
        return []

    results = []
    for team_entry in teams_dir.iterdir():
        if not team_entry.is_dir():
            continue
        team_name = team_entry.name
        log_info(f"Checking team: {team_name}")

        # Quick status check
        summary = get_task_summary(team_name)
        if summary["total"] == 0:
            continue

        if summary["completed"] == summary["total"]:
            # Already complete — write notification
            write_notification(team_name, "all_completed",
                               f"All {summary['total']} tasks already done", 0)
            results.append({
                "team": team_name,
                "status": "already_completed",
                "summary": summary,
            })
        elif summary["in_progress"] > 0:
            # Active — babysit
            result = babysit_team(
                team_name=team_name,
                timeout=timeout,
                interval=interval,
                auto_merge=True,
                auto_pr=auto_pr,
            )
            results.append(result)
        else:
            # Pending but not active — skip
            results.append({
                "team": team_name,
                "status": "pending_not_active",
                "summary": summary,
            })

    return results


def _write_result(result_file: str | None, result: dict) -> None:
    """Write result to file if path provided."""
    if not result_file:
        return
    try:
        Path(result_file).write_text(
            json.dumps(result, indent=2, ensure_ascii=False), encoding="utf-8"
        )
        log_info(f"Result written to {result_file}")
    except OSError as e:
        log_error(f"Failed to write result file: {e}")


# =============================================================================
# CLI
# =============================================================================


def main() -> int:
    import argparse

    config = cowork_config.load_cowork_config()
    babysit_config = config.get("babysit", {})

    parser = argparse.ArgumentParser(
        description="Babysit - Deterministic background monitor for ClawTeam workers"
    )
    parser.add_argument("--team", "-t", help="Team name to monitor")
    parser.add_argument(
        "--all",
        action="store_true",
        help="Monitor all active teams (scans ~/.clawteam/teams/)",
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=babysit_config.get("timeout", 300),
        help=f"Max polling duration in seconds (default: {babysit_config.get('timeout', 300)})",
    )
    parser.add_argument(
        "--result-file",
        "-r",
        help="Path to write JSON result file (single-team mode)",
    )
    parser.add_argument(
        "--interval",
        type=int,
        default=babysit_config.get("interval", 10),
        help=f"Polling interval in seconds (default: {babysit_config.get('interval', 10)})",
    )
    parser.add_argument(
        "--no-merge",
        action="store_true",
        help="Disable auto-merge on task completion",
    )
    parser.add_argument(
        "--auto-pr",
        action="store_true",
        help="Auto-create PR via gh CLI after all tasks complete",
    )

    args = parser.parse_args()

    # Handle auto_pr default from config (store_true doesn't support default well)
    if not args.auto_pr:
        args.auto_pr = config.get("auto_pr", False)

    if args.all:
        if args.team:
            parser.error("Cannot specify both --team and --all")
        results = babysit_all(timeout=args.timeout, interval=args.interval, auto_pr=args.auto_pr)
        print(json.dumps(results, indent=2, ensure_ascii=False), file=sys.stderr)
        return 0

    if not args.team:
        parser.error("Either --team or --all is required")

    result = babysit_team(
        team_name=args.team,
        timeout=args.timeout,
        result_file=args.result_file,
        interval=args.interval,
        auto_merge=not args.no_merge,
        auto_pr=args.auto_pr,
    )

    return 0 if result["success"] else 1


if __name__ == "__main__":
    sys.exit(main())
