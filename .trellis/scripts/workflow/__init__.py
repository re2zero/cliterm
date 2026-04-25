"""
Workflow orchestration package for Trellis.

Provides automated end-to-end task execution by orchestrating
existing trellis commands (start, brainstorm, cowork).

Modules:
    complexity - Task complexity detection
    orchestrator - Workflow orchestration engine
    reporter - Summary report generation
"""

from __future__ import annotations

from pathlib import Path

SCRIPTS_DIR = Path(__file__).parent.parent
