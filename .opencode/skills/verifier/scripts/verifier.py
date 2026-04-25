import os
from typing import Dict, Any, Optional
from .monitor import compile_check, Monitor
from .reporter import generate_markdown_report, generate_summary

_FAST_TIMEOUT = 3

_SIGTERM_CODES = {-15, -9, -6}

_EMPTY_RUNTIME = {
    "memory_mb": 0,
    "peak_memory_mb": 0,
    "cpu_percent": 0,
    "avg_cpu_percent": 0,
    "exit_code": -1,
    "runtime_seconds": 0,
    "ctx_switches": {"voluntary": 0, "involuntary": 0},
    "stdout_tail": "",
    "stderr_tail": "",
}


def _maybe_wrap_offscreen(run_command: str, work_dir: str) -> str:
    cmake_path = os.path.join(work_dir, "CMakeLists.txt")
    if os.path.exists(cmake_path):
        return f"QT_QPA_PLATFORM=offscreen {run_command}"
    return run_command


def _is_sigterm(exit_code: int) -> bool:
    return exit_code in _SIGTERM_CODES


def _run_once(command: str, duration: float) -> Dict[str, Any]:
    monitor = Monitor(command)
    monitor.start()
    return monitor.collect_metrics(duration_seconds=duration)


def execute_verification(task: Dict[str, Any]) -> Dict[str, Any]:
    target_agent = task["target_agent"]
    work_dir = task.get("work_dir", os.getcwd())
    build_command = task["build_command"]
    run_command = task["run_command"]
    thresholds = task["thresholds"]

    original_dir = os.getcwd()
    try:
        os.chdir(work_dir)

        compile_result = compile_check(build_command)

        if not compile_result["success"]:
            metrics = {"compile": compile_result, "fast": None, "deep": None}
            report_file = generate_markdown_report(target_agent, metrics, thresholds)
            summary = "❌ FAILED: compilation error"
            return {
                "status": "fail",
                "target_agent": target_agent,
                "metrics": metrics,
                "report_file": report_file,
                "summary": summary,
            }

        # Phase 1: Fast (3s smoke test)
        fast_metrics = _run_once(run_command, _FAST_TIMEOUT)

        fast_ok = fast_metrics["exit_code"] == 0 or _is_sigterm(fast_metrics["exit_code"])

        if not fast_ok:
            metrics = {"compile": compile_result, "fast": fast_metrics, "deep": None}
            report_file = generate_markdown_report(target_agent, metrics, thresholds)
            summary = generate_summary(target_agent, metrics, thresholds)
            status = determine_status(metrics, thresholds)
            return {
                "status": status,
                "target_agent": target_agent,
                "metrics": metrics,
                "report_file": report_file,
                "summary": summary,
            }

        # Phase 2: Deep (real runtime monitoring)
        deep_command = _maybe_wrap_offscreen(run_command, work_dir)
        deep_duration = thresholds.get("max_time_seconds", 10)
        deep_metrics = _run_once(deep_command, deep_duration)
        metrics = {"compile": compile_result, "fast": fast_metrics, "deep": deep_metrics}

        report_file = generate_markdown_report(target_agent, metrics, thresholds)
        status = determine_status(metrics, thresholds)
        summary = generate_summary(target_agent, metrics, thresholds)

        return {
            "status": status,
            "target_agent": target_agent,
            "metrics": metrics,
            "report_file": report_file,
            "summary": summary,
        }
    finally:
        os.chdir(original_dir)


def determine_status(metrics: Dict[str, Any], thresholds: Dict[str, Any]) -> str:
    if not metrics["compile"]["success"]:
        return "fail"

    deep = metrics.get("deep")
    fast = metrics.get("fast")
    active = deep if deep else fast

    if active is None:
        return "fail"

    if active["exit_code"] != 0 and not _is_sigterm(active["exit_code"]):
        return "fail"

    mem_key = "peak_memory_mb" if deep else "memory_mb"
    memory_mb = active.get(mem_key, 0)
    max_memory = max(thresholds.get("max_memory_mb", 512), 1)

    if memory_mb > max_memory:
        return "fail" if memory_mb > max_memory * 1.2 else "warning"

    cpu_key = "avg_cpu_percent" if deep else "cpu_percent"
    cpu_percent = active.get(cpu_key, 0)
    max_cpu = max(thresholds.get("max_cpu_percent", 80), 1)

    if cpu_percent > max_cpu:
        return "warning"

    return "pass"
