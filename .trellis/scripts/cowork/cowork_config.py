#!/usr/bin/env python3
"""
Cowork configuration reader.

Reads cowork settings from .trellis/config.yaml.
Priority: CLI flags > task.json meta.cowork > config.yaml cowork > defaults.

Config schema in config.yaml:

  cowork:
    worker_profiles:
      default: null          # Default ClawTeam profile for all workers
      map:                   # Map dev_type to profile name
        backend: claude-sonnet
        frontend: claude-haiku
    auto_pr: false           # Auto-create PR via gh CLI when done
    babysit:
      interval: 10           # Polling interval in seconds
      timeout: 300           # Max monitoring time in seconds
      auto_merge: true       # Auto-merge worker branches on completion
    max_workers: 4           # Max parallel workers per dispatch batch
"""

from __future__ import annotations

import subprocess
from pathlib import Path


# =============================================================================
# Default Configuration
# =============================================================================

DEFAULTS = {
    "worker_profiles": {"default": None, "map": {}},
    "auto_pr": False,
    "babysit": {"interval": 10, "timeout": 300, "auto_merge": True},
    "max_workers": 4,
}


# =============================================================================
# Minimal YAML Parser (no dependencies)
# =============================================================================


def _unquote(s: str) -> str:
    """Remove exactly one layer of matching surrounding quotes.

    Unlike str.strip('"'), this only removes the outermost pair,
    preserving any nested quotes inside the value.

    Examples:
        _unquote('"hello"')        -> 'hello'
        _unquote("'hello'")        -> 'hello'
        _unquote('"echo \\'hi\\'"')  -> "echo 'hi'"
        _unquote('hello')          -> 'hello'
        _unquote('"hello\\'')       -> '"hello\\''  (mismatched, unchanged)
    """
    if len(s) >= 2 and s[0] == s[-1] and s[0] in ('"', "'"):
        return s[1:-1]
    return s


def _load_yaml(path: Path) -> dict:
    """Parse simple YAML with nested dict support (no dependencies).

    Supports:
        - key: value (string)
        - key: (followed by list items)
            - item1
            - item2
        - key: (followed by nested dict)
            nested_key: value
            nested_key2:
              - item

    Uses indentation to detect nesting (2+ spaces deeper = child).

    Args:
        path: Path to YAML file.

    Returns:
        Parsed dict (values can be str, list[str], or dict).
    """
    try:
        content = path.read_text(encoding="utf-8")
    except (OSError, IOError):
        return {}

    lines = content.splitlines()
    result: dict = {}
    _parse_yaml_block(lines, 0, 0, result)
    return result


def _parse_yaml_block(
    lines: list[str], start: int, min_indent: int, target: dict
) -> int:
    """Parse a YAML block into target dict, returning next line index."""
    i = start
    current_list: list | None = None

    while i < len(lines):
        line = lines[i]
        stripped = line.strip()

        # Skip empty lines and comments
        if not stripped or stripped.startswith("#"):
            i += 1
            continue

        # Calculate indentation
        indent = len(line) - len(line.lstrip())

        # If dedented past our block, we're done
        if indent < min_indent:
            break

        if stripped.startswith("- "):
            if current_list is not None:
                current_list.append(_unquote(stripped[2:].strip()))
            i += 1
        elif ":" in stripped:
            key, _, value = stripped.partition(":")
            key = key.strip()
            value = _unquote(value.strip())
            current_list = None

            if value:
                # key: value
                target[key] = value
                i += 1
            else:
                # key: (no value) — peek ahead to determine list vs nested dict
                next_i, next_line = _next_content_line(lines, i + 1)
                if next_i >= len(lines):
                    target[key] = {}
                    i = next_i
                elif next_line.strip().startswith("- "):
                    # It's a list
                    current_list = []
                    target[key] = current_list
                    i += 1
                else:
                    next_indent = len(next_line) - len(next_line.lstrip())
                    if next_indent > indent:
                        # It's a nested dict
                        nested: dict = {}
                        target[key] = nested
                        i = _parse_yaml_block(lines, i + 1, next_indent, nested)
                    else:
                        # Empty value, same or less indent follows
                        target[key] = {}
                        i += 1
        else:
            i += 1

    return i


def _next_content_line(lines: list[str], start: int) -> tuple[int, str]:
    """Find the next non-empty, non-comment line."""
    i = start
    while i < len(lines):
        stripped = lines[i].strip()
        if stripped and not stripped.startswith("#"):
            return i, lines[i]
        i += 1
    return i, ""


# =============================================================================
# Repo Root Detection
# =============================================================================


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
# Config Loading
# =============================================================================


def _convert_type(value: str) -> str | int | bool | None:
    """Convert string value to appropriate type.

    Converts:
        "true"/"false" -> bool
        "null"/"none"/"~" -> None
        Numeric strings -> int
        Other -> string (unchanged)
    """
    if isinstance(value, str):
        lower_val = value.lower()
        if lower_val == "true":
            return True
        if lower_val == "false":
            return False
        if lower_val in ("null", "none", "~"):
            return None
        if value.isdigit():
            return int(value)
    return value


def load_cowork_config(repo_root: Path | None = None) -> dict:
    """Load cowork configuration from .trellis/config.yaml.

    Args:
        repo_root: Repository root path. Defaults to auto-detected.

    Returns:
        Dict with cowork settings merged with defaults.
    """
    if repo_root is None:
        repo_root = get_repo_root()

    config_path = repo_root / ".trellis" / "config.yaml"

    # Load config file
    config_data = _load_yaml(config_path)

    # Extract cowork section
    cowork_config = config_data.get("cowork", {})

    # Merge with defaults (defaults are overridden by config file)
    result = DEFAULTS.copy()

    # Merge top-level cowork config
    for key in ["auto_pr", "max_workers"]:
        if key in cowork_config:
            result[key] = _convert_type(cowork_config[key])

    # Merge worker_profiles (nested dict)
    if "worker_profiles" in cowork_config:
        wp = cowork_config["worker_profiles"]
        result["worker_profiles"] = result["worker_profiles"].copy()
        if "default" in wp:
            result["worker_profiles"]["default"] = _convert_type(wp["default"])
        if "map" in wp and isinstance(wp["map"], dict):
            result["worker_profiles"]["map"] = wp["map"].copy()

    # Merge babysit config (nested dict)
    if "babysit" in cowork_config:
        bs = cowork_config["babysit"]
        result["babysit"] = result["babysit"].copy()
        for key in ["interval", "timeout", "auto_merge"]:
            if key in bs:
                result["babysit"][key] = _convert_type(bs[key])

    return result


def get_cowork_setting(
    key: str,
    default: None | None = None,
    repo_root: Path | None = None,
) -> None | str | int | bool | dict | list:
    """Get a single cowork configuration value.

    Supports nested keys with dot notation, e.g. "babysit.timeout".

    Args:
        key: Configuration key (supports dot notation for nested values).
        default: Default value if key not found.
        repo_root: Repository root path. Defaults to auto-detected.

    Returns:
        Configuration value or default.
    """
    config = load_cowork_config(repo_root)

    # Handle nested keys
    if "." in key:
        parts = key.split(".")
        value = config
        for part in parts:
            if isinstance(value, dict) and part in value:
                value = value[part]
            else:
                return default
        return value

    return config.get(key, default)


def merge_config(
    cli_val: None | None,
    meta_val: None | None = None,
    config_val: None | None = None,
    default: None | None = None,
) -> None | None:
    """Merge configuration values with priority: CLI > meta > config > default.

    Args:
        cli_val: Value from CLI argument.
        meta_val: Value from task.json meta.cowork.
        config_val: Value from config.yaml cowork section.
        default: Built-in default value.

    Returns:
        The first non-None value in priority order.
    """
    if cli_val is not None:
        return cli_val
    if meta_val is not None:
        return meta_val
    if config_val is not None:
        return config_val
    return default
