import subprocess
import time
import threading
from typing import Dict, Any, Optional


class Monitor:
    """Monitor a process and collect resource usage metrics."""

    def __init__(self, command: str):
        self.command = command
        self.process: Optional[subprocess.Popen] = None
        self.ps_process = None
        self._captured_stdout: str = ""
        self._captured_stderr: str = ""
        self._reader_threads: list = []

    def start(self):
        self.process = subprocess.Popen(
            self.command, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE
        )

        self._reader_threads.append(
            threading.Thread(target=self._drain, args=("stdout",), daemon=True)
        )
        self._reader_threads.append(
            threading.Thread(target=self._drain, args=("stderr",), daemon=True)
        )
        for t in self._reader_threads:
            t.start()

        try:
            import psutil

            self.ps_process = psutil.Process(self.process.pid)
        except (ImportError, Exception):
            self.ps_process = None

    def _drain(self, stream_name: str, chunk_size: int = 4096):
        stream = getattr(self.process, stream_name, None)
        if stream is None:
            return
        chunks: list = []
        while True:
            chunk = stream.read(chunk_size)
            if not chunk:
                break
            chunks.append(chunk if isinstance(chunk, str) else chunk.decode(errors="replace"))
        text = "".join(chunks)
        lines = text.splitlines()
        if stream_name == "stdout":
            self._captured_stdout = "\n".join(lines[-50:])
        else:
            self._captured_stderr = "\n".join(lines[-50:])

    def collect_metrics(self, duration_seconds: float = 5.0) -> Dict[str, Any]:
        defaults = {
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

        if self.ps_process is None or self.process is None:
            return defaults.copy()

        start_time = time.time()
        collected: Dict[str, Any] = {}
        cpu_samples: list = []

        try:
            while time.time() - start_time < duration_seconds:
                if self.process.poll() is not None:
                    break

                try:
                    mem = self.ps_process.memory_info().rss / 1024 / 1024
                    cpu = self.ps_process.cpu_percent(interval=0.1)
                    collected["memory_mb"] = mem
                    collected["peak_memory_mb"] = max(collected.get("peak_memory_mb", 0), mem)
                    collected["cpu_percent"] = cpu
                    cpu_samples.append(cpu)
                except Exception:
                    pass

                time.sleep(0.1)

            if self.process.poll() is None:
                self.process.terminate()
                try:
                    self.process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    self.process.kill()

            exit_code = self.process.wait()
            collected["exit_code"] = exit_code
            collected["runtime_seconds"] = time.time() - start_time
            collected["avg_cpu_percent"] = sum(cpu_samples) / len(cpu_samples) if cpu_samples else 0

            try:
                collected["ctx_switches"] = self.ps_process.num_ctx_switches()
            except Exception:
                collected["ctx_switches"] = {"voluntary": 0, "involuntary": 0}

            for t in self._reader_threads:
                t.join(timeout=3)
            collected["stdout_tail"] = self._captured_stdout
            collected["stderr_tail"] = self._captured_stderr
        finally:
            if self.process and self.process.poll() is None:
                self.process.terminate()
                try:
                    self.process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    self.process.kill()

        result = defaults.copy()
        result.update(collected)
        return result


def compile_check(build_command: str) -> Dict[str, Any]:
    start_time = time.time()

    try:
        result = subprocess.run(
            build_command, shell=True, capture_output=True, text=True, timeout=300
        )

        elapsed = time.time() - start_time

        if result.returncode == 0:
            return {
                "success": True,
                "time_seconds": elapsed,
                "command": build_command,
            }
        else:
            return {
                "success": False,
                "time_seconds": elapsed,
                "error": result.stderr,
                "command": build_command,
            }

    except subprocess.TimeoutExpired:
        return {
            "success": False,
            "time_seconds": 300,
            "error": "Compilation timeout after 300 seconds",
            "command": build_command,
        }
    except Exception as e:
        return {
            "success": False,
            "time_seconds": time.time() - start_time,
            "error": str(e),
            "command": build_command,
        }
