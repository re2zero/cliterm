import os
from datetime import datetime
from typing import Dict, Any, Optional

_SIGTERM_CODES = {-15, -9, -6}


def _check_exit(runtime: Dict[str, Any]) -> Optional[str]:
    if runtime is None:
        return None
    code = runtime.get("exit_code", 0)
    if code != 0 and code not in _SIGTERM_CODES:
        return f"crash, exit code {code}"
    return None


def _resource_status(runtime: Dict[str, Any], thresholds: Dict[str, Any]) -> str:
    if runtime is None:
        return "pass"
    max_mem = max(thresholds.get("max_memory_mb", 512), 1)
    max_cpu = max(thresholds.get("max_cpu_percent", 80), 1)

    mem_val = runtime.get("peak_memory_mb", 0)
    cpu_val = runtime.get("avg_cpu_percent", 0)

    if mem_val > max_mem * 1.2:
        return "fail"
    if mem_val > max_mem or cpu_val > max_cpu:
        return "warning"
    return "pass"


def generate_summary(
    agent_name: str,
    metrics: Dict[str, Any],
    thresholds: Dict[str, Any],
) -> str:
    if not metrics["compile"]["success"]:
        return f"❌ FAILED: {agent_name} (compilation error)"

    active = metrics.get("deep") or metrics.get("fast") or {}

    crash_msg = _check_exit(active)
    if crash_msg:
        return f"❌ FAILED: {agent_name} ({crash_msg})"

    max_mem = max(thresholds.get("max_memory_mb", 512), 1)
    max_cpu = max(thresholds.get("max_cpu_percent", 80), 1)

    mem_mb = active.get("peak_memory_mb", 0)
    cpu_val = active.get("avg_cpu_percent", 0)
    memory_percent = int(mem_mb / max_mem * 100)

    if memory_percent > 100:
        return f"⚠️ VERIFIED: {agent_name} (mem {mem_mb:.0f}MB/{thresholds['max_memory_mb']}MB)"
    if cpu_val > max_cpu:
        return f"⚠️ VERIFIED: {agent_name} (cpu {cpu_val:.0f}%/{max_cpu}%)"
    return f"✅ VERIFIED: {agent_name} (mem {memory_percent}%, cpu {cpu_val:.0f}%)"


def _runtime_table(runtime: Dict[str, Any], max_mem: int, max_cpu: int) -> str:
    if runtime is None:
        return "  (skipped)"

    mem_val = runtime.get("peak_memory_mb", 0)
    cpu_val = runtime.get("avg_cpu_percent", 0)
    mem_end = runtime.get("memory_mb", 0)
    cpu_end = runtime.get("cpu_percent", 0)
    exit_code = runtime.get("exit_code", -1)
    duration = runtime.get("runtime_seconds", 0)
    exit_ok = "✅" if (exit_code == 0 or exit_code in _SIGTERM_CODES) else "❌"
    mem_ok = "✅" if mem_val <= max_mem else "⚠️"
    cpu_ok = "✅" if cpu_val <= max_cpu else "⚠️"

    lines = [
        f"| Memory | {mem_val:.1f} MB | {max_mem} MB | {int(mem_val / max_mem * 100)}% | {mem_ok} |",
        f"| CPU    | {cpu_val:.1f}%   | {max_cpu}%  | {int(cpu_val / max_cpu * 100)}%   | {cpu_ok} |",
        f"| Exit   | {exit_code}  | 0 | - | {exit_ok} |",
        f"| Time   | {duration:.1f}s  | - | - | - |",
        f"| Mem(end) | {mem_end:.1f} MB | - | - | - |",
        f"| CPU(end) | {cpu_end:.1f}%   | - | - | - |",
    ]
    return "\n".join(lines)


def generate_markdown_report(
    agent_name: str,
    metrics: Dict[str, Any],
    thresholds: Dict[str, Any],
) -> str:
    compile_result = metrics["compile"]
    fast = metrics.get("fast")
    deep = metrics.get("deep")

    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    filename = f"verifier-{agent_name}-{timestamp}-fast+deep.md"
    report_dir = os.path.expanduser("~/.clawteam/reports")
    os.makedirs(report_dir, exist_ok=True)
    filepath = os.path.join(report_dir, filename)

    max_mem = max(thresholds.get("max_memory_mb", 512), 1)
    max_cpu = max(thresholds.get("max_cpu_percent", 80), 1)

    crash_msg = _check_exit(deep or {})
    res = _resource_status(deep or {}, thresholds)

    if not compile_result["success"]:
        overall = "❌ 失败"
        overall_summary = f"编译失败：{compile_result.get('error', 'Unknown error')}"
    elif crash_msg:
        overall = "❌ 失败"
        overall_summary = f"程序崩溃，{crash_msg}"
    elif res == "fail":
        overall = "❌ 失败"
        overall_summary = "资源严重超限"
    elif res == "warning":
        overall = "⚠️ 警告"
        overall_summary = "资源轻微超限"
    else:
        overall = "✅ 通过"
        overall_summary = "所有检查项均通过验证"

    compile_section = (
        f"### 编译\n"
        f"- **状态**: {'✅ 通过' if compile_result['success'] else '❌ 失败'}\n"
        f"- **命令**: `{compile_result.get('command', 'N/A')}`\n"
        f"- **耗时**: {compile_result.get('time_seconds', 0):.1f} 秒\n"
    )
    if not compile_result["success"]:
        compile_section += f"\n### 错误输出\n```\n{compile_result.get('error', '')}\n```\n"

    fast_duration = f"{fast['runtime_seconds']:.1f}s" if fast else "-"
    fast_table = _runtime_table(fast, max_mem, max_cpu) if fast else "  (skipped)"

    deep_section = ""
    if deep:
        stderr_tail = deep.get("stderr_tail", "").strip()
        deep_duration = f"{deep['runtime_seconds']:.1f}s"
        deep_table = _runtime_table(deep, max_mem, max_cpu)
        deep_section = f"### 运行（{deep_duration} 监控）\n\n{deep_table}\n"
        if stderr_tail:
            deep_section += f"\n### stderr（末尾 50 行）\n```\n{stderr_tail}\n```\n"

    import json as _json

    metrics_json = _json.dumps(metrics, ensure_ascii=False, default=str)

    content = (
        f"# 验证报告：{agent_name}\n\n"
        f"**时间**: {datetime.now().isoformat()}\n"
        f"**目标**: {agent_name} 的工作输出\n"
        f"**验证者**: clawteam-verifier\n"
        f"**模式**: 快速验证 → 深度验证\n"
        f"---\n\n"
        f"## 验证结论：{overall}\n\n{overall_summary}\n"
        f"---\n\n"
        f"## 阶段 1：快速验证\n\n{compile_section}\n"
        f"### 运行（{fast_duration}）\n\n{fast_table}\n"
        f"---\n\n"
        f"## 阶段 2：深度验证\n\n{deep_section}\n"
        f"---\n\n"
        f"## 元数据\n\n"
        f"```json\n{metrics_json}\n```\n"
    )

    with open(filepath, "w") as f:
        f.write(content)

    return filepath
