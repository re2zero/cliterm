#!/usr/bin/env python3
"""
Workflow Report Generator.

Generates a summary report after workflow completion,
covering all phases, changes, and recommendations.
"""

from __future__ import annotations

import json
import subprocess
from datetime import datetime
from pathlib import Path


FILE_WORKFLOW_JSON = "workflow.json"
FILE_TASK_JSON = "task.json"
FILE_WORKFLOW_SUMMARY = "workflow-summary.md"


def _read_json(path: Path) -> dict | None:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (FileNotFoundError, json.JSONDecodeError, OSError):
        return None


def _run_git(args: list[str], cwd: Path | None = None) -> tuple[int, str, str]:
    try:
        result = subprocess.run(
            ["git", "-c", "i18n.logOutputEncoding=UTF-8"] + args,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            cwd=cwd,
            check=False,
        )
        return result.returncode, result.stdout, result.stderr
    except Exception as e:
        return 1, "", str(e)


def _get_changed_files(task_dir: Path) -> list[dict]:
    """Get list of changed files since workflow started."""
    wf_state = _read_json(task_dir / FILE_WORKFLOW_JSON)
    if not wf_state:
        return []

    started = wf_state.get("started_at", "")
    if not started:
        return []

    ret, stdout, _ = _run_git(["log", "--oneline", "-20"])
    if ret != 0:
        return []

    changed: list[dict] = []
    for line in stdout.strip().splitlines():
        if not line.strip():
            continue
        parts = line.split(" ", 1)
        if len(parts) < 2:
            continue
        commit_hash = parts[0]
        commit_msg = parts[1]

        ret, date_out, _ = _run_git(["log", "-1", "--format=%aI", commit_hash])
        if ret != 0:
            continue

        commit_date = date_out.strip()
        if commit_date >= started:
            ret, diff_out, _ = _run_git(
                ["diff-tree", "--no-commit-id", "--name-status", "-r", commit_hash]
            )
            if ret == 0:
                for diff_line in diff_out.strip().splitlines():
                    if not diff_line.strip():
                        continue
                    diff_parts = diff_line.split("\t", 1)
                    status = diff_parts[0][:1] if diff_parts else "?"
                    filepath = diff_parts[1] if len(diff_parts) > 1 else diff_parts[0]
                    changed.append(
                        {
                            "file": filepath,
                            "status": status,
                            "commit": commit_hash[:8],
                            "message": commit_msg,
                        }
                    )

    return changed


def generate_report(task_dir: Path) -> Path | None:
    """Generate workflow summary report.

    Args:
        task_dir: Path to task directory.

    Returns:
        Path to generated report, or None on failure.
    """
    wf_state = _read_json(task_dir / FILE_WORKFLOW_JSON)
    task_data = _read_json(task_dir / FILE_TASK_JSON)

    if not wf_state and not task_data:
        return None

    lines: list[str] = []
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

    title = "Workflow Summary"
    if task_data:
        task_title = task_data.get("title", "Untitled")
        title = f"Workflow Summary: {task_title}"

    lines.append(f"# {title}")
    lines.append("")
    lines.append(f"Generated: {now}")
    lines.append("")

    if wf_state:
        lines.append("## Workflow Info")
        lines.append("")
        lines.append("| Field | Value |")
        lines.append("|-------|-------|")
        lines.append(f"| Status | {wf_state.get('status', 'unknown')} |")
        lines.append(f"| Started | {wf_state.get('started_at', 'unknown')} |")
        lines.append(f"| Updated | {wf_state.get('updated_at', 'unknown')} |")

        completed = wf_state.get("phases_completed", [])
        failed = wf_state.get("phases_failed", [])
        lines.append(f"| Completed Phases | {', '.join(completed) or 'none'} |")
        if failed:
            lines.append(f"| Failed Phases | {', '.join(failed)} |")

        lines.append("")

    if task_data:
        lines.append("## Task Info")
        lines.append("")
        lines.append("| Field | Value |")
        lines.append("|-------|-------|")
        lines.append(f"| Name | {task_data.get('name', 'N/A')} |")
        lines.append(f"| Priority | {task_data.get('priority', 'N/A')} |")
        lines.append(f"| Assignee | {task_data.get('assignee', 'N/A')} |")
        lines.append(f"| Branch | {task_data.get('branch', 'N/A')} |")
        lines.append(f"| Scope | {task_data.get('scope', 'N/A')} |")
        lines.append("")

    changed_files = _get_changed_files(task_dir)
    if changed_files:
        lines.append("## Changes")
        lines.append("")
        lines.append("| Status | File | Commit |")
        lines.append("|--------|------|--------|")
        for cf in changed_files:
            status_icon = {"A": "+", "M": "~", "D": "-"}.get(cf["status"], "?")
            lines.append(
                f"| {status_icon} {cf['status']} | `{cf['file']}` | {cf['commit']} |"
            )
        lines.append("")

        added = sum(1 for cf in changed_files if cf["status"] == "A")
        modified = sum(1 for cf in changed_files if cf["status"] == "M")
        deleted = sum(1 for cf in changed_files if cf["status"] == "D")
        lines.append(
            f"**Total: {len(changed_files)} file(s)** "
            f"(+{added} added, ~{modified} modified, -{deleted} deleted)"
        )
        lines.append("")

    errors = []
    if wf_state:
        errors = wf_state.get("errors", [])

    if errors:
        lines.append("## Errors Encountered")
        lines.append("")
        for err in errors:
            lines.append(f"- {err}")
        lines.append("")

    lines.append("## Next Steps")
    lines.append("")
    lines.append("1. Review the changes above")
    lines.append("2. Run tests and verify functionality")
    lines.append("3. Run code quality checks")
    lines.append("4. Commit and create PR if satisfied")
    lines.append("5. Archive task when verified: `/trellis:archive`")
    lines.append("")

    report_path = task_dir / FILE_WORKFLOW_SUMMARY
    try:
        report_path.write_text("\n".join(lines), encoding="utf-8")
        return report_path
    except (OSError, IOError):
        return None
