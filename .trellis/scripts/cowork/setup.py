#!/usr/bin/env python3
"""
Trellis Task Context Setup for ClawTeam Worktrees.

Sets up Trellis task context in a ClawTeam worktree so workers can follow
the Trellis workflow (read .current-task, follow spec, Hook injection, etc.).

Usage:
    python3 setup.py <worktree-path> <trellis-task-dir>

Example:
    python3 setup.py \\
        /home/user/.clawteam/workspaces/parallel-0415/worker-1 \\
        .trellis/tasks/04-15-user-auth

This script:
1. Validates worktree path exists
2. Locates Trellis task directory (worktree or main repo)
3. Copies task directory into worktree (if not already present)
4. Writes .trellis/.current-task in worktree
5. Verifies task.json exists
"""

from __future__ import annotations

import json
import os
import shutil
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


def _find_main_repo_from_worktree(wt: Path) -> Path | None:
    """Find the main repo root from a git worktree.

    In a worktree, .git is a file containing 'gitdir: <path>'.
    We extract the main repo root from that path.
    """
    git_file = wt / ".git"
    if not git_file.is_file():
        return None
    try:
        content = git_file.read_text(encoding="utf-8").strip()
        for line in content.splitlines():
            line = line.strip()
            if line.startswith("gitdir:"):
                gitdir = Path(line.split(":", 1)[1].strip())
                # gitdir points to: <main_repo>/.git/worktrees/<name>
                # main_git = <main_repo>/.git, main_repo = main_git.parent
                if "worktrees" in gitdir.parts:
                    main_git = gitdir.parent.parent
                    main_repo = main_git.parent
                    # Verify main repo exists (check for .git directory or file)
                    if (main_repo / ".git").exists():
                        return main_repo
                return None
    except (OSError, ValueError):
        return None
    return None


# =============================================================================
# Main
# =============================================================================


def setup_trellis_task_context(worktree_path: str, trellis_task_dir: str) -> bool:
    """Set up Trellis task context in a ClawTeam worktree.

    If the task directory is not already in the worktree (because
    _safe_copy_trellis_content excludes tasks/), it is copied from the
    main repo so that the Trellis Hook system can find prd.md,
    implement.jsonl, and other task files.

    Args:
        worktree_path: Absolute path to the worktree directory.
        trellis_task_dir: Relative path to the Trellis task directory
                          (relative to repo root, e.g., ".trellis/tasks/04-15-user-auth").

    Returns:
        True if setup succeeded, False otherwise.
    """
    wt = Path(worktree_path).resolve()
    if not wt.is_dir():
        log_error(f"Worktree path does not exist: {worktree_path}")
        return False

    trellis_dir = wt / ".trellis"
    if not trellis_dir.is_dir():
        log_warn(f".trellis/ directory not found in worktree: {wt}")
        log_warn("Trellis context setup skipped.")
        log_info("This is expected if ClawTeam didn't copy .trellis/ content.")
        log_info("The worker will need to rely on task description instead.")
        return False

    repo_root = _find_main_repo_from_worktree(wt)

    # Resolve the source task directory — try worktree first, then main repo
    source = wt / trellis_task_dir
    if not source.is_dir() and repo_root:
        source = repo_root / trellis_task_dir

    if not source.is_dir():
        log_error(f"Trellis task directory not found: {trellis_task_dir}")
        return False

    # Copy task directory into worktree if not already present
    # This is needed because _safe_copy_trellis_content excludes tasks/
    wt_target = wt / trellis_task_dir
    if not wt_target.is_dir():
        wt_target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copytree(str(source), str(wt_target))
        log_success(f"Task directory copied: {trellis_task_dir}")

    # Write .current-task (relative path from worktree root)
    current_task_file = trellis_dir / ".current-task"
    relative_path = os.path.relpath(wt_target, wt)
    current_task_file.write_text(relative_path, encoding="utf-8")
    log_success(f".current-task set → {relative_path}")

    # Verify task.json exists
    task_json = wt_target / "task.json"
    if not task_json.is_file():
        log_warn(f"task.json not found in {wt_target}")
        return False

    log_success(f"task.json found: {task_json}")

    # Print task info for verification
    try:
        task_data = json.loads(task_json.read_text(encoding="utf-8"))
        task_title = task_data.get("title", "Unknown")
        task_branch = task_data.get("branch", "N/A")
        log_info(f"Task: {task_title}")
        log_info(f"Branch: {task_branch}")
    except (json.JSONDecodeError, OSError):
        pass

    return True


def main() -> int:
    """CLI entry point."""
    if len(sys.argv) < 3:
        print("Usage: python3 setup.py <worktree-path> <trellis-task-dir>")
        print()
        print("Sets up Trellis task context in a ClawTeam worktree.")
        print()
        print("Arguments:")
        print("  worktree-path     Absolute path to the worktree directory")
        print("  trellis-task-dir  Relative path to Trellis task directory")
        print()
        print("Example:")
        print("  python3 setup.py /path/to/worktree .trellis/tasks/04-15-user-auth")
        return 1

    worktree_path = sys.argv[1]
    trellis_task_dir = sys.argv[2]

    print(f"{Colors.BLUE}=== Trellis Task Context Setup ==={Colors.NC}")
    log_info(f"Worktree: {worktree_path}")
    log_info(f"Task dir: {trellis_task_dir}")
    print()

    success = setup_trellis_task_context(worktree_path, trellis_task_dir)

    print()
    if success:
        log_success("Setup complete. Worker can now follow Trellis workflow.")
    else:
        log_warn("Setup incomplete. Worker will need alternative guidance.")

    return 0 if success else 1


if __name__ == "__main__":
    sys.exit(main())
