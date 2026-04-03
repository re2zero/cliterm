// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package acpclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// StdioTransport 表示 stdio 传输层
type StdioTransport struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.Reader
	stderr    io.Reader
	command   string   // Full command string for cleanup tracking
	args      []string // Arguments for cleanup tracking
	mu        sync.Mutex
	connected bool
	pid       int // Process ID for cleanup
}

// NewStdioTransport 创建新的 stdio 传输
func NewStdioTransport(command string, args []string) *StdioTransport {
	cmd := exec.Command(command, args...)

	return &StdioTransport{
		cmd:     cmd,
		command: command,
		args:    args,
	}
}

// Connect 连接到 acpx 进程
func (t *StdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	// 创建管道
	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return NewProcessStartError(fmt.Errorf("failed to create stdin pipe: %w", err))
	}

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return NewProcessStartError(fmt.Errorf("failed to create stdout pipe: %w", err))
	}

	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		return NewProcessStartError(fmt.Errorf("failed to create stderr pipe: %w", err))
	}

	t.stdin = stdin
	t.stdout = stdout
	t.stderr = stderr

	// 启动进程
	if err := t.cmd.Start(); err != nil {
		return NewProcessStartError(fmt.Errorf("failed to start process: %w", err))
	}

	// Track process ID for cleanup
	t.pid = t.cmd.Process.Pid
	fmt.Printf("[ACP-TRANSPORT] Process started with PID=%d: %s %v\n", t.pid, t.command, t.args)

	// 等待进程就绪
	select {
	case <-ctx.Done():
		if t.cmd.Process != nil {
			t.cmd.Process.Kill()
		}
		return ctx.Err()
	case <-time.After(500 * time.Millisecond):
		// 简单等待，实际应该更复杂的就绪检测
		t.connected = true
		return nil
	}
}

// WriteMessage 写入 JSON-RPC 消息
func (t *StdioTransport) WriteMessage(msg *ACPMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return NewProtocolError("transport not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// 每条消息一行 + newline for ACP protocol
	_, err = t.stdin.Write([]byte(string(data)))
	if err != nil {
		return NewProtocolError("failed to write message: %w", err)
	}
	_, err = t.stdin.Write([]byte("\n"))
	if err != nil {
		return NewProtocolError("failed to write message: %w", err)
	}

	return nil
}

// ReadMessage 读取 JSON-RPC 消息
func (t *StdioTransport) ReadMessage() (*ACPMessage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	reader := bufio.NewReader(t.stdout)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, NewProtocolError("failed to read message: %w", err)
	}

	return ParseACPMessage([]byte(line))
}

// Close 关闭传输
func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	t.connected = false

	// 关闭管道
	if t.stdin != nil {
		t.stdin.Close()
	}

	// 终止进程 - 使用更激进的方法确保清理
	if t.cmd != nil && t.cmd.Process != nil {
		pid := t.pid
		fmt.Printf("[ACP-TRANSPORT] Closing transport for PID=%d\n", pid)

		// First try graceful SIGTERM
		t.cmd.Process.Signal(syscall.SIGTERM)

		// Wait a bit for graceful shutdown
		done := make(chan error, 1)
		go func() {
			done <- t.cmd.Wait()
		}()

		select {
		case <-done:
			fmt.Printf("[ACP-TRANSPORT] Process %d terminated gracefully\n", pid)
		case <-time.After(500 * time.Millisecond):
			fmt.Printf("[ACP-TRANSPORT] Process %d did not exit gracefully, forcing kill\n", pid)
			t.cmd.Process.Kill()
			t.cmd.Wait()
		}

		// Additional cleanup for NPX child processes
		if t.command == "npx" {
			killNPXChildProcesses(pid)
		}
	}

	return nil
}

// killNPXChildProcesses attempts to kill child processes spawned by NPX
// This is a best-effort cleanup since NPX may spawn processes that become detached
func killNPXChildProcesses(parentPid int) {
	// Try to find and kill child processes
	// This uses ps command to find children, then SIGKILL them
	fmt.Printf("[ACP-TRANSPORT] Attempting to cleanup NPX children of PID=%d\n", parentPid)

	// Use pgrep to find processes that might be related
	// This is Unix-specific and may not work on Windows
	bashCmd := exec.Command("bash", "-c",
		fmt.Sprintf("pkill -9 -P %d 2>/dev/null || true", parentPid))
	if err := bashCmd.Run(); err != nil {
		fmt.Printf("[ACP-TRANSPORT] Warning: failed to cleanup NPX children: %v\n", err)
	}
}

// IsConnected 检查是否已连接
func (t *StdioTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}
