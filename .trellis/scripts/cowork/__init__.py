"""
ClawTeam × Trellis Integration Scripts.

This module provides scripts for the cowork command, enabling:
- Parallel dispatch (clone mode): Parallelize existing Trellis tasks with ClawTeam workers
- Workers write unit tests with >= 85% coverage after code implementation
- clawteam-verifier performs final quality gate after merge

Key Principles:
- Trellis is the spec source (`.trellis/spec/`, code-spec hooks)
- ClawTeam is the execution engine (worktree, agent lifecycle)
- User is the only decision maker (verify, finish, archive)
- Workers are user extensions (all commits belong to user)
"""

from pathlib import Path

# Script directory
SCRIPTS_DIR = Path(__file__).parent
