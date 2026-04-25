#!/usr/bin/env python3
"""
ClawTeam Merge - Merge Worktrees and Cleanup.

Usage:
    python3 merge.py --team <team-name>
    python3 merge.py --team <team-name> --cleanup

This script:
1. Lists all worktrees for the team
2. Merges each worker's worktree
3. Optionally cleans up the team
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


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


def _run_git(args: list[str], cwd: Path | None = None) -> tuple[int, str, str]:
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
    except Exception as e:
        return 1, "", str(e)


# =============================================================================
# Operations
# =============================================================================


def list_worktrees(team_name: str) -> dict:
    """List all worktrees for a team."""
    ret, stdout, stderr = _run_clawteam(
        ["workspace", "list", team_name], json_output=True
    )

    if ret != 0:
        return {"success": False, "error": stderr or stdout}

    workers = []
    try:
        result = json.loads(stdout)
        if isinstance(result, list):
            workers = [
                str(item.get("name", item.get("worker", "")))
                for item in result
                if item.get("name") or item.get("worker")
            ]
        elif isinstance(result, dict):
            items = result.get(
                "workspaces", result.get("workers", result.get("items", []))
            )
            workers = [
                str(item.get("name", item.get("worker", "")))
                for item in items
                if item.get("name") or item.get("worker")
            ]
    except (json.JSONDecodeError, AttributeError):
        pass

    return {
        "success": True,
        "output": stdout,
        "workers": workers,
    }


def merge_worktree(team_name: str, worker_name: str) -> dict:
    """Merge a worker's worktree."""
    ret, stdout, stderr = _run_clawteam(["workspace", "merge", team_name, worker_name])

    if ret != 0:
        return {"success": False, "error": stderr or stdout, "output": stdout}

    return {"success": True, "output": stdout}


def cleanup_team(team_name: str, force: bool = True) -> dict:
    """Cleanup a team."""
    args = ["team", "cleanup", team_name]
    if force:
        args.append("--force")

    ret, stdout, stderr = _run_clawteam(args)

    if ret != 0:
        return {"success": False, "error": stderr or stdout}

    return {"success": True, "output": stdout}


def show_git_diff() -> dict:
    """Show git diff stats."""
    ret, stdout, stderr = _run_git(["diff", "--stat", "HEAD"])

    return {
        "success": ret == 0,
        "output": stdout or stderr,
    }


# =============================================================================
# Main
# =============================================================================


def merge_team(team_name: str, cleanup: bool = False) -> dict:
    """Merge all worktrees for a team.

    Args:
        team_name: Team name.
        cleanup: Whether to cleanup team after merge.

    Returns:
        Dict with merge result.
    """
    print(f"{Colors.BLUE}=== Merging Team Worktrees ==={Colors.NC}")
    log_info(f"Team: {team_name}")
    print()

    # List worktrees
    list_result = list_worktrees(team_name)
    if not list_result["success"]:
        return {"success": False, "error": list_result.get("error")}

    print(list_result["output"])
    workers = list_result.get("workers", [])

    if not workers:
        log_warn("No workers found")
        return {"success": True, "merged": []}

    # Merge each worker
    merged = []
    failed = []

    for worker in workers:
        print()
        log_info(f"Merging: {worker}")
        merge_result = merge_worktree(team_name, worker)

        if merge_result["success"]:
            log_success(f"Merged: {worker}")
            merged.append(worker)
        else:
            log_error(f"Failed to merge {worker}: {merge_result.get('error')}")
            failed.append(worker)

    # Show diff stats
    print()
    print(f"{Colors.BLUE}=== Changes Summary ==={Colors.NC}")
    diff_result = show_git_diff()
    if diff_result["success"]:
        print(diff_result["output"])
    else:
        log_warn("Could not get diff stats")

    # Cleanup if requested
    if cleanup:
        print()
        print(f"{Colors.BLUE}=== Cleaning Up ==={Colors.NC}")
        cleanup_result = cleanup_team(team_name)
        if cleanup_result["success"]:
            log_success(f"Team cleaned up: {team_name}")
        else:
            log_warn(f"Cleanup failed: {cleanup_result.get('error')}")

    return {
        "success": len(failed) == 0,
        "merged": merged,
        "failed": failed,
    }


def main() -> int:
    """CLI entry point."""
    import argparse

    parser = argparse.ArgumentParser(
        description="ClawTeam Merge - Merge Worktrees and Cleanup"
    )
    parser.add_argument("--team", "-t", required=True, help="Team name to merge")
    parser.add_argument(
        "--cleanup", "-c", action="store_true", help="Cleanup team after merge"
    )

    args = parser.parse_args()

    result = merge_team(args.team, args.cleanup)

    print()
    if result["success"]:
        log_success("Merge complete!")
        if result.get("merged"):
            print(f"  Merged workers: {', '.join(result['merged'])}")
    else:
        log_error("Merge completed with errors")
        if result.get("failed"):
            print(f"  Failed workers: {', '.join(result['failed'])}")
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
