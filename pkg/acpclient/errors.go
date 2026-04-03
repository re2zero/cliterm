// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package acpclient

import "fmt"

// ACP 错误码
const (
	ErrCodeProcessStartFailed = -32001
	ErrCodeProcessDied        = -32002
	ErrCodeTimeout            = -32003
	ErrCodeProtocolError      = -32004
	ErrCodeAuthFailed         = -32005
	ErrCodePermissionDenied   = -32006
	ErrCodeInvalidResponse    = -32007
	ErrCodeConnectionLost     = -32008
)

// ACPError 表示 ACP 协议错误
type ACPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *ACPError) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("ACP error %d: %s (data: %+v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("ACP error %d: %s", e.Code, e.Message)
}

// NewProcessStartError 创建进程启动错误
func NewProcessStartError(original error) *ACPError {
	return &ACPError{
		Code:    ErrCodeProcessStartFailed,
		Message: "Failed to start acpx process",
		Data:    map[string]interface{}{"original": original.Error()},
	}
}

// NewProtocolError 创建协议错误
func NewProtocolError(format string, args ...interface{}) *ACPError {
	return &ACPError{
		Code:    ErrCodeProtocolError,
		Message: fmt.Sprintf(format, args...),
	}
}

// NewTimeoutError 创建超时错误
func NewTimeoutError(operation string) *ACPError {
	return &ACPError{
		Code:    ErrCodeTimeout,
		Message: fmt.Sprintf("Operation %s timed out", operation),
	}
}
