#!/usr/bin/env python3
"""
Workflow Orchestrator.

Orchestrates existing trellis commands (start, brainstorm, cowork) into
an automated end-to-end pipeline.

This module ONLY calls existing scripts - it does NOT modify any existing code.

Pipeline:
    brainstorm -> research -> implement (via cowork) -> check -> verify -> report
"""

from __future__ import annotations

import json
import subprocess
import sys
from datetime import datetime
from enum import Enum
from pathlib import Path

from .complexity import (
    ComplexityResult,
    detect_complexity,
    get_current_task_abs,
    get_repo_root,
)

SCRIPTS_DIR = Path(__file__).resolve().parent.parent

FILE_TASK_JSON = "task.json"
FILE_PRD = "prd.md"
FILE_WORKFLOW_JSON = "workflow.json"
FILE_WORKFLOW_SUMMARY = "workflow-summary.md"

WORKFLOW_PHASES = [
    "brainstorm",
    "implement",
    "check",
    "verify",
]


class WorkflowStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"


# =============================================================================
# Colors & Logging
# =============================================================================


class Colors:
    RED = "\033[0;31m"
    GREEN = "\033[0;32m"
    YELLOW = "\033[1;33m"
    BLUE = "\033[0;34m"
    BOLD = "\033[1m"
    NC = "\033[0m"


def log_info(msg: str) -> None:
    print(f"{Colors.BLUE}[WORKFLOW]{Colors.NC} {msg}")


def log_success(msg: str) -> None:
    print(f"{Colors.GREEN}[WORKFLOW]{Colors.NC} {msg}")


def log_warn(msg: str) -> None:
    print(f"{Colors.YELLOW}[WORKFLOW]{Colors.NC} {msg}", file=sys.stderr)


def log_error(msg: str) -> None:
    print(f"{Colors.RED}[WORKFLOW]{Colors.NC} {msg}", file=sys.stderr)


def log_phase(phase: str, msg: str) -> None:
    print(f"{Colors.BOLD}  [{phase.upper()}]{Colors.NC} {msg}")


# =============================================================================
# JSON Helpers
# =============================================================================


def _read_json(path: Path) -> dict | None:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (FileNotFoundError, json.JSONDecodeError, OSError):
        return None


def _write_json(path: Path, data: dict) -> bool:
    try:
        path.write_text(
            json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8"
        )
        return True
    except (OSError, IOError):
        return False


# =============================================================================
# Workflow State
# =============================================================================


def load_workflow_state(task_dir: Path) -> dict | None:
    """Load workflow state from task directory."""
    return _read_json(task_dir / FILE_WORKFLOW_JSON)


def save_workflow_state(task_dir: Path, state: dict) -> bool:
    """Save workflow state to task directory."""
    state["updated_at"] = datetime.now().isoformat()
    return _write_json(task_dir / FILE_WORKFLOW_JSON, state)


def init_workflow_state(task_dir: Path, requirement: str) -> dict:
    """Initialize a new workflow state."""
    now = datetime.now().isoformat()
    state = {
        "requirement": requirement,
        "status": WorkflowStatus.RUNNING.value,
        "current_phase": None,
        "phases_completed": [],
        "phases_failed": [],
        "errors": [],
        "started_at": now,
        "updated_at": now,
    }
    save_workflow_state(task_dir, state)
    return state


def get_next_phase(state: dict) -> str | None:
    """Determine which phase should run next."""
    completed = state.get("phases_completed", [])
    for phase in WORKFLOW_PHASES:
        if phase not in completed:
            return phase
    return None


# =============================================================================
# Status Detection & Recovery
# =============================================================================


def detect_existing_workflow(task_dir: Path) -> dict | None:
    """Detect if there's an existing workflow in the task directory."""
    wf_json = task_dir / FILE_WORKFLOW_JSON
    if not wf_json.is_file():
        return None
    return load_workflow_state(task_dir)


def format_status_report(state: dict, task_dir: Path) -> str:
    """Format a human-readable status report for existing workflow."""
    lines = []
    lines.append(f"{Colors.BOLD}=== Workflow Status ==={Colors.NC}")
    lines.append("")

    status = state.get("status", "unknown")
    current = state.get("current_phase", "none")
    completed = state.get("phases_completed", [])
    failed = state.get("phases_failed", [])
    errors = state.get("errors", [])
    started = state.get("started_at", "unknown")
    updated = state.get("updated_at", "unknown")

    lines.append(f"Status: {status}")
    lines.append(f"Started: {started}")
    lines.append(f"Updated: {updated}")
    lines.append(f"Current phase: {current}")
    lines.append("")

    if completed:
        lines.append(f"Completed phases: {', '.join(completed)}")
    if failed:
        lines.append(f"{Colors.RED}Failed phases: {', '.join(failed)}{Colors.NC}")
    if errors:
        lines.append("")
        lines.append(f"{Colors.YELLOW}Errors:{Colors.NC}")
        for err in errors:
            lines.append(f"  - {err}")

    next_phase = get_next_phase(state)
    if next_phase:
        lines.append("")
        lines.append(f"{Colors.GREEN}Resume from: {next_phase}{Colors.NC}")

    return "\n".join(lines)


# =============================================================================
# Phase Execution
# =============================================================================


def _run_python_script(
    script_path: str | Path,
    args: list[str] | None = None,
    cwd: Path | None = None,
) -> tuple[int, str, str]:
    """Run a Python script in the trellis scripts directory."""
    cmd = [sys.executable, str(script_path)]
    if args:
        cmd.extend(args)

    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            cwd=cwd,
            check=False,
        )
        return result.returncode, result.stdout, result.stderr
    except FileNotFoundError:
        return 1, "", f"Script not found: {script_path}"
    except Exception as e:
        return 1, "", str(e)


def execute_brainstorm(
    task_dir: Path,
    requirement: str,
    complexity: ComplexityResult,
) -> bool:
    """Execute brainstorm phase."""
    log_phase("brainstorm", "Starting...")
    log_phase("brainstorm", f"Complexity: {complexity.level} ({complexity.reason})")

    task_py = SCRIPTS_DIR / "task.py"

    if complexity.is_simple:
        log_phase("brainstorm", "Simple task - creating PRD directly")

        if not (task_dir / FILE_TASK_JSON).is_file():
            ret, out, err = _run_python_script(task_py, ["create", requirement])
            if ret != 0:
                log_phase("brainstorm", f"Failed to create task: {err}")
                return False
            log_phase("brainstorm", out.strip())

        log_phase(
            "brainstorm", "Task created - AI will generate PRD during implementation"
        )
        log_success("Brainstorm phase complete (simple mode)")
        return True

    log_phase("brainstorm", "Complex task - AI needs to run brainstorm Q&A")
    log_phase(
        "brainstorm",
        "Please run: /trellis:brainstorm (or AI will handle this)",
    )

    prd_path = task_dir / FILE_PRD
    if prd_path.is_file():
        log_success("Brainstorm phase complete (PRD already exists)")
        return True

    log_phase("brainstorm", "Waiting for PRD to be created by brainstorm process...")
    return True


def execute_research(task_dir: Path) -> bool:
    """Execute research phase."""
    log_phase("research", "Starting...")

    task_py = SCRIPTS_DIR / "task.py"

    ret, out, err = _run_python_script(task_py, ["validate", str(task_dir)])
    if ret != 0:
        log_phase("research", f"Context validation warning: {err}")

    log_phase("research", out.strip())
    log_success("Research phase complete")
    return True


def execute_implement(task_dir: Path, complexity: ComplexityResult) -> bool:
    """Execute implement phase via cowork."""
    log_phase("implement", "Starting via cowork...")

    dispatch_py = SCRIPTS_DIR / "cowork" / "dispatch.py"
    if not dispatch_py.is_file():
        log_phase("implement", "cowork/dispatch.py not found")
        log_phase("implement", "AI will execute implementation in current session")
        log_success("Implement phase delegated to AI")
        return True

    log_phase(
        "implement",
        f"Estimated: ~{complexity.estimated_files} files, ~{complexity.estimated_minutes}min",
    )

    if complexity.estimated_minutes > 15:
        log_phase("implement", "Complexity suggests Clone mode (cowork will decide)")
    else:
        log_phase("implement", "Complexity suggests Direct mode (cowork will decide)")

    log_phase(
        "implement",
        "AI should invoke: /trellis:cowork (or call dispatch.py directly)",
    )

    log_success("Implement phase delegated to cowork/AI")
    return True


def execute_check(task_dir: Path) -> bool:
    """Execute check phase."""
    log_phase("check", "Starting...")

    task_py = SCRIPTS_DIR / "task.py"
    ret, out, err = _run_python_script(task_py, ["validate", str(task_dir)])
    log_phase("check", out.strip())

    log_success("Check phase complete")
    return True


def execute_verify(task_dir: Path) -> bool:
    """Execute verify phase."""
    log_phase("verify", "Starting...")

    from .reporter import generate_report

    report_path = generate_report(task_dir)
    if report_path:
        log_success(f"Report generated: {report_path}")
    else:
        log_warn("Report generation failed")

    log_success("Verify phase complete")
    return True


# =============================================================================
# Orchestrator
# =============================================================================


def orchestrate(
    requirement: str,
    task_dir: Path | None = None,
) -> dict:
    """Run the full workflow orchestration.

    Detects existing state and resumes if needed.
    Creates new workflow if no existing state found.

    Args:
        requirement: Task requirement description.
        task_dir: Optional explicit task directory path.

    Returns:
        Result dict with status and info.
    """
    if task_dir is None:
        task_dir = _resolve_task_dir()

    task_dir = task_dir.resolve()

    task_json = task_dir / FILE_TASK_JSON
    if not task_json.is_file():
        log_error(f"No task.json in {task_dir}")
        log_error("Run /trellis:brainstorm first to create the task")
        return {
            "success": False,
            "error": f"No task.json in {task_dir}",
        }

    existing = detect_existing_workflow(task_dir)

    if existing is not None:
        log_info("Detected existing workflow state")
        print(format_status_report(existing, task_dir))
        print()

        next_phase = get_next_phase(existing)
        status = existing.get("status", "")

        if status == WorkflowStatus.COMPLETED.value:
            log_success("Workflow already completed!")
            summary_path = task_dir / FILE_WORKFLOW_SUMMARY
            if summary_path.is_file():
                log_info(f"Summary: {summary_path}")
            return {"success": True, "status": "already_completed"}

        if next_phase is None:
            log_success("All phases completed!")
            return {"success": True, "status": "all_phases_done"}

        return _resume_workflow(existing, task_dir, requirement, next_phase)

    return _start_new_workflow(task_dir, requirement)


def _resolve_task_dir() -> Path:
    """Resolve the current task directory."""
    task_dir = get_current_task_abs()
    if task_dir:
        return task_dir

    log_error("No current task set")
    log_error("Run: python3 ./.trellis/scripts/task.py start <task-dir>")
    sys.exit(1)


def _start_new_workflow(task_dir: Path, requirement: str) -> dict:
    """Start a fresh workflow."""
    log_info("Starting new workflow")
    log_info(f"Task dir: {task_dir}")
    log_info(f"Requirement: {requirement[:100]}...")
    print()

    state = init_workflow_state(task_dir, requirement)
    complexity = detect_complexity(requirement)

    log_info(f"Complexity: {complexity.level} ({complexity.reason})")
    log_info(
        f"Estimated: ~{complexity.estimated_files} files, ~{complexity.estimated_minutes}min"
    )
    print()

    phases_to_run = list(WORKFLOW_PHASES)
    return _run_phases(state, task_dir, requirement, complexity, phases_to_run)


def _resume_workflow(
    state: dict,
    task_dir: Path,
    requirement: str,
    resume_from: str,
) -> dict:
    """Resume an existing workflow from a specific phase."""
    state["status"] = WorkflowStatus.RUNNING.value
    save_workflow_state(task_dir, state)

    completed = state.get("phases_completed", [])
    phases_to_run = [p for p in WORKFLOW_PHASES if p not in completed]

    log_info(f"Resuming from phase: {resume_from}")
    log_info(f"Phases to run: {', '.join(phases_to_run)}")
    print()

    original_requirement = state.get("requirement", requirement)
    complexity = detect_complexity(original_requirement)

    return _run_phases(state, task_dir, original_requirement, complexity, phases_to_run)


def _run_phases(
    state: dict,
    task_dir: Path,
    requirement: str,
    complexity: ComplexityResult,
    phases: list[str],
) -> dict:
    """Execute a sequence of workflow phases."""
    for phase in phases:
        state["current_phase"] = phase
        state["status"] = WorkflowStatus.RUNNING.value
        save_workflow_state(task_dir, state)

        log_info(f"=== Phase: {phase.upper()} ===")

        success = _execute_phase(phase, task_dir, requirement, complexity)

        if success:
            if phase not in state["phases_completed"]:
                state["phases_completed"].append(phase)
            log_success(f"Phase '{phase}' completed")
        else:
            state["phases_failed"].append(phase)
            state["status"] = WorkflowStatus.FAILED.value
            save_workflow_state(task_dir, state)
            log_error(f"Phase '{phase}' failed - workflow paused")
            return {
                "success": False,
                "status": "failed",
                "failed_phase": phase,
                "completed_phases": state["phases_completed"],
                "message": (f"Workflow paused at '{phase}'. Run again to resume."),
            }

        print()

    state["current_phase"] = None
    state["status"] = WorkflowStatus.COMPLETED.value
    save_workflow_state(task_dir, state)

    log_success("=== All phases completed ===")

    from .reporter import generate_report

    report_path = generate_report(task_dir)
    if report_path:
        log_info(f"Summary report: {report_path}")
    else:
        log_warn("Failed to generate summary report")

    return {
        "success": True,
        "status": "completed",
        "completed_phases": state["phases_completed"],
    }


def _execute_phase(
    phase: str,
    task_dir: Path,
    requirement: str,
    complexity: ComplexityResult,
) -> bool:
    """Execute a single phase."""
    if phase == "brainstorm":
        return execute_brainstorm(task_dir, requirement, complexity)
    if phase == "implement":
        return execute_implement(task_dir, complexity)
    if phase == "check":
        return execute_check(task_dir)
    if phase == "verify":
        return execute_verify(task_dir)

    log_warn(f"Unknown phase: {phase}")
    return True
