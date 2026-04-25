#!/usr/bin/env python3
"""
Trellis Workflow - Automated Task Pipeline.

Orchestrates existing trellis commands (start, brainstorm, cowork) into
a fully automated end-to-end pipeline.

Usage:
    python3 ./.trellis/scripts/workflow.py run "<requirement>"
    python3 ./.trellis/scripts/workflow.py run --task-dir <path> "<requirement>"

The command auto-detects existing workflow state and resumes from
the last incomplete phase. No manual phase selection needed.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path


SCRIPTS_DIR = Path(__file__).resolve().parent


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Trellis Workflow - Automated Task Pipeline",
    )
    subparsers = parser.add_subparsers(dest="command")

    run_parser = subparsers.add_parser(
        "run", help="Run workflow (auto-detects and resumes)"
    )
    run_parser.add_argument(
        "requirement",
        nargs="?",
        help="Task requirement description",
    )
    run_parser.add_argument(
        "--task-dir",
        "-t",
        help="Task directory path (auto-detected from .current-task if not specified)",
    )

    subparsers.add_parser("status", help="Show current workflow status")

    args = parser.parse_args()

    if args.command == "run":
        return cmd_run(args)
    elif args.command == "status":
        return cmd_status(args)
    else:
        parser.print_help()
        return 0


def cmd_run(args: argparse.Namespace) -> int:
    sys.path.insert(0, str(SCRIPTS_DIR))

    from workflow.orchestrator import orchestrate

    requirement = args.requirement or ""
    if not requirement:
        print("Error: requirement description is required", file=sys.stderr)
        print('Usage: workflow.py run "<requirement>"', file=sys.stderr)
        return 1

    task_dir = None
    if args.task_dir:
        task_dir = Path(args.task_dir)

    result = orchestrate(requirement=requirement, task_dir=task_dir)

    if result.get("success"):
        status = result.get("status", "unknown")
        if status == "already_completed":
            print("\nWorkflow was already completed.")
            return 0
        print(f"\nWorkflow {status}.")
        return 0
    else:
        error = result.get("error", "Unknown error")
        failed_phase = result.get("failed_phase", "")
        print(f"\nWorkflow failed at phase '{failed_phase}': {error}", file=sys.stderr)
        return 1


def cmd_status(args: argparse.Namespace) -> int:
    sys.path.insert(0, str(SCRIPTS_DIR))

    from workflow.orchestrator import detect_existing_workflow, format_status_report
    from workflow.complexity import get_current_task_abs

    task_dir = get_current_task_abs()
    if not task_dir:
        print("No current task set.", file=sys.stderr)
        print(
            "Run: python3 ./.trellis/scripts/task.py start <task-dir>", file=sys.stderr
        )
        return 1

    state = detect_existing_workflow(task_dir)
    if not state:
        print(f"No workflow state found in {task_dir}")
        print('Run: workflow.py run "<requirement>" to start a workflow')
        return 1

    print(format_status_report(state, task_dir))
    return 0


if __name__ == "__main__":
    sys.exit(main())
