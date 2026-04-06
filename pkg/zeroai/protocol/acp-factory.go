// Package protocol implements ACP (Agent Control Protocol) connection factories
//
// This file provides factory functions for creating ACP connections for different backends.
package protocol

func NewConnection(backend AcpBackend) Connection {
	return &AcpConnection{
		config:     AcpSessionConfig{Backend: backend},
		pendingReq: make(map[int]*PendingRequest),
		shutdownCh: make(chan struct{}),
	}
}
