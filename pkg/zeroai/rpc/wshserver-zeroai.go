// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// ZeroAI WSH Server RPC handlers
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/wavetermdev/waveterm/pkg/panichandler"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/zeroai/agent"
	"github.com/wavetermdev/waveterm/pkg/zeroai/protocol"
	"github.com/wavetermdev/waveterm/pkg/zeroai/service"
	"github.com/wavetermdev/waveterm/pkg/zeroai/store"
	"github.com/wavetermdev/waveterm/pkg/zeroai/team"
)

type WshRpcZeroaiServer struct {
	sessionService  SessionServiceInterface
	messageService  MessageServiceInterface
	agentService    *service.AgentService
	providerService *service.ProviderService
	teamCoordinator *team.Coordinator
	messageRouter   *team.MessageRouter
	blockManager    *team.BlockManager
	defaultBackend  string
}

type SessionServiceInterface interface {
	CreateSession(ctx context.Context, opts agent.AgentSessionOptions) (*agent.AgentSession, error)
	GetSession(sessionID string) (*agent.AgentSession, error)
	ListSessions() ([]*agent.AgentSession, error)
	DeleteSession(sessionID string) error
	SetWorkDir(sessionID string, workDir string) error
	StoreSession(session *store.Session) error
}

type MessageServiceInterface interface {
	AddMessage(msg *store.Message) error
	GetSessionMessages(sessionID string) ([]*store.Message, error)
	DeleteSessionMessages(sessionID string) error
}

func (*WshRpcZeroaiServer) WshServerImpl() {}

func NewWshRpcZeroaiServer(
	sessionService SessionServiceInterface,
	messageService MessageServiceInterface,
	agentService *service.AgentService,
	providerService *service.ProviderService,
	teamCoordinator *team.Coordinator,
	messageRouter *team.MessageRouter,
	blockManager *team.BlockManager,
) *WshRpcZeroaiServer {
	return &WshRpcZeroaiServer{
		sessionService:  sessionService,
		messageService:  messageService,
		agentService:    agentService,
		providerService: providerService,
		teamCoordinator: teamCoordinator,
		messageRouter:   messageRouter,
		blockManager:    blockManager,
		defaultBackend:  "claude",
	}
}

func (zs *WshRpcZeroaiServer) ZeroAiCreateSessionCommand(ctx context.Context, req wshrpc.CommandZeroAiCreateSessionData) (wshrpc.CommandZeroAiCreateSessionRtnData, error) {
	defer func() {
		panichandler.PanicHandler("ZeroAiCreateSessionCommand", recover())
	}()

	if zs.sessionService == nil {
		return wshrpc.CommandZeroAiCreateSessionRtnData{}, fmt.Errorf("session service not initialized")
	}

	backend := req.Backend
	if backend == "" {
		backend = zs.defaultBackend
	}

	var customProvider *wshrpc.ZeroAiProviderInfo
	if zs.providerService != nil {
		providers, _ := zs.providerService.ListProviders()
		for _, p := range providers {
			if p.ID == backend && p.IsCustom {
				customProvider = p
				break
			}
		}
	}

	if customProvider == nil {
		agentConfig := agent.AgentConfig{Backend: backend}
		startCtx, startCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer startCancel()
		ag, err := zs.agentService.GetAgent(startCtx, agentConfig)
		if err != nil {
			return wshrpc.CommandZeroAiCreateSessionRtnData{}, fmt.Errorf("failed to get agent: %w", err)
		}
		if !ag.IsRunning() {
			if err := ag.Start(startCtx); err != nil {
				return wshrpc.CommandZeroAiCreateSessionRtnData{}, fmt.Errorf("failed to start agent: %w", err)
			}
		}

		opts := agent.AgentSessionOptions{
			Backend:       backend,
			WorkDir:       req.WorkDir,
			Model:         req.Model,
			ResumeSession: false,
			YoloMode:      req.YoloMode,
		}
		log.Printf("[DEBUG] ZeroAiCreateSessionCommand: YoloMode=%v", req.YoloMode)

		session, err := ag.CreateSession(ctx, opts)
		if err != nil {
			return wshrpc.CommandZeroAiCreateSessionRtnData{}, err
		}

		storeSession := &store.Session{
			ID:        session.ID,
			Backend:   session.Backend,
			WorkDir:   session.WorkDir,
			Model:     session.Model,
			Provider:  session.Provider,
			CreatedAt: session.CreatedAt,
			UpdatedAt: session.UpdatedAt,
		}
		if err := zs.sessionService.StoreSession(storeSession); err != nil {
			log.Printf("failed to store session: %v", err)
		}

		return wshrpc.CommandZeroAiCreateSessionRtnData{
			SessionID: session.ID,
		}, nil
	}

	sessionID := fmt.Sprintf("custom-%s-%d", backend, time.Now().UnixMilli())
	storeSession := &store.Session{
		ID:        sessionID,
		Backend:   backend,
		WorkDir:   req.WorkDir,
		Model:     req.Model,
		Provider:  backend,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}
	if err := zs.sessionService.StoreSession(storeSession); err != nil {
		return wshrpc.CommandZeroAiCreateSessionRtnData{}, fmt.Errorf("failed to store session: %w", err)
	}

	return wshrpc.CommandZeroAiCreateSessionRtnData{
		SessionID: sessionID,
	}, nil
}

func (zs *WshRpcZeroaiServer) ZeroAiGetSessionCommand(ctx context.Context, req wshrpc.CommandZeroAiGetSessionData) (wshrpc.ZeroAiSessionWrapper, error) {
	defer func() {
		panichandler.PanicHandler("ZeroAiGetSessionCommand", recover())
	}()

	session, err := zs.sessionService.GetSession(req.SessionID)
	if err != nil {
		return wshrpc.ZeroAiSessionWrapper{}, err
	}
	return toSessionWrapper(session), nil
}

func (zs *WshRpcZeroaiServer) ZeroAiListSessionsCommand(ctx context.Context, req wshrpc.CommandZeroAiListSessionsData) (wshrpc.CommandZeroAiListSessionsRtnData, error) {
	defer func() {
		panichandler.PanicHandler("ZeroAiListSessionsCommand", recover())
	}()

	sessions, err := zs.sessionService.ListSessions()
	if err != nil {
		return wshrpc.CommandZeroAiListSessionsRtnData{}, err
	}
	result := make([]*wshrpc.ZeroAiSessionInfo, len(sessions))
	for i, s := range sessions {
		result[i] = &wshrpc.ZeroAiSessionInfo{
			SessionID:     s.ID,
			Provider:      s.Backend,
			Model:         s.Model,
			WorkDir:       s.WorkDir,
			CreatedAt:     s.CreatedAt,
			LastMessageAt: 0,
		}
	}
	return wshrpc.CommandZeroAiListSessionsRtnData{
		Sessions: result,
	}, nil
}

func (zs *WshRpcZeroaiServer) ZeroAiDeleteSessionCommand(ctx context.Context, req wshrpc.CommandZeroAiDeleteSessionData) error {
	defer func() {
		panichandler.PanicHandler("ZeroAiDeleteSessionCommand", recover())
	}()

	err := zs.sessionService.DeleteSession(req.SessionID)
	if err != nil {
		return err
	}
	if zs.messageService != nil {
		if delErr := zs.messageService.DeleteSessionMessages(req.SessionID); delErr != nil {
			log.Printf("failed to delete session messages: %v", delErr)
		}
	}
	return nil
}

func (zs *WshRpcZeroaiServer) ZeroAiSetWorkDirCommand(ctx context.Context, req wshrpc.CommandZeroAiSetWorkDirData) error {
	defer func() {
		panichandler.PanicHandler("ZeroAiSetWorkDirCommand", recover())
	}()
	return zs.sessionService.SetWorkDir(req.SessionID, req.WorkDir)
}

func (zs *WshRpcZeroaiServer) getBackendForSession(ctx context.Context, sessionID string) string {
	session, err := zs.sessionService.GetSession(sessionID)
	if err != nil || session.Backend == "" {
		return zs.defaultBackend
	}
	return session.Backend
}

func (zs *WshRpcZeroaiServer) ZeroAiSendMessageCommand(ctx context.Context, req wshrpc.CommandZeroAiSendMessageData) (wshrpc.CommandZeroAiSendMessageRtnData, error) {
	defer func() {
		panichandler.PanicHandler("ZeroAiSendMessageCommand", recover())
	}()

	backend := zs.getBackendForSession(ctx, req.SessionID)
	ag, err := zs.agentService.GetAgent(ctx, agent.AgentConfig{Backend: backend})
	if err != nil {
		return wshrpc.CommandZeroAiSendMessageRtnData{}, err
	}

	input := agent.SendMessageInput{Content: req.Content}
	if zs.messageService != nil {
		zs.messageService.AddMessage(&store.Message{
			SessionID: req.SessionID,
			Role:      req.Role,
			Content:   req.Content,
		})
	}

	eventCh, err := ag.SendMessage(ctx, req.SessionID, input)
	if err != nil {
		return wshrpc.CommandZeroAiSendMessageRtnData{}, err
	}

	for event := range eventCh {
		if zs.messageService != nil {
			zs.messageService.AddMessage(&store.Message{
				SessionID: req.SessionID,
				Role:      "assistant",
				Content:   extractEventText(event.Data),
				EventType: string(event.Type),
			})
		}
		if event.Type == agent.EventTypeEndTurn {
			break
		}
	}

	return wshrpc.CommandZeroAiSendMessageRtnData{
		MessageID: 0,
		Streaming: false,
	}, nil
}

func (zs *WshRpcZeroaiServer) ZeroAiSendStreamMessageCommand(ctx context.Context, req wshrpc.CommandZeroAiSendMessageData) chan wshrpc.RespOrErrorUnion[wshrpc.ZeroAiStreamMessageEvent] {
	defer func() {
		panichandler.PanicHandler("ZeroAiSendStreamMessageCommand", recover())
	}()

	rtn := make(chan wshrpc.RespOrErrorUnion[wshrpc.ZeroAiStreamMessageEvent], 128)

	go func() {
		defer close(rtn)

		backend := zs.getBackendForSession(ctx, req.SessionID)
		ag, err := zs.agentService.GetAgent(ctx, agent.AgentConfig{Backend: backend})
		if err != nil {
			sendError(rtn, err)
			return
		}

		input := agent.SendMessageInput{Content: req.Content}
		if zs.messageService != nil {
			zs.messageService.AddMessage(&store.Message{
				SessionID: req.SessionID,
				Role:      req.Role,
				Content:   req.Content,
			})
		}

		eventCh, err := ag.SendMessage(ctx, req.SessionID, input)
		if err != nil {
			sendError(rtn, err)
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-eventCh:
				if !ok {
					return
				}

				msg := &wshrpc.ZeroAiMessageWrapper{
					SessionID: event.Session,
					Role:      "assistant",
					EventType: string(event.Type),
				}

				if event.Data != nil {
					switch d := event.Data.(type) {
					case string:
						msg.Content = d
					case *protocol.AcpSessionUpdate:
						msg.Content = extractEventText(d.Content)
						msg.Metadata = d.AsMetadata()
					default:
						msg.Content = extractEventContentRaw(event.Data)
					}
				}

				resp := wshrpc.RespOrErrorUnion[wshrpc.ZeroAiStreamMessageEvent]{}
				resp.Response.Message = msg

				select {
				case rtn <- resp:
				case <-ctx.Done():
					return
				}

				if event.Type == agent.EventTypeEndTurn {
					return
				}

				if len(eventCh) > 0 {
					time.Sleep(16 * time.Millisecond)
				}
			}
		}
	}()

	return rtn
}

func (zs *WshRpcZeroaiServer) ZeroAiGetMessagesCommand(ctx context.Context, req wshrpc.CommandZeroAiGetMessagesData) (wshrpc.CommandZeroAiGetMessagesRtnData, error) {
	defer func() {
		panichandler.PanicHandler("ZeroAiGetMessagesCommand", recover())
	}()

	var messages []*store.Message
	if zs.messageService != nil {
		msgs, err := zs.messageService.GetSessionMessages(req.SessionID)
		if err != nil {
			return wshrpc.CommandZeroAiGetMessagesRtnData{}, err
		}
		messages = msgs
	}

	result := make([]*wshrpc.ZeroAiMessageInfo, len(messages))
	for i, msg := range messages {
		result[i] = &wshrpc.ZeroAiMessageInfo{
			ID:        msg.ID,
			SessionID: msg.SessionID,
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
		}
	}
	return wshrpc.CommandZeroAiGetMessagesRtnData{
		Messages: result,
	}, nil
}

func (zs *WshRpcZeroaiServer) ZeroAiGetAgentsCommand(ctx context.Context, req wshrpc.CommandZeroAiGetAgentsData) ([]wshrpc.ZeroAiAgentInfo, error) {
	defer func() {
		panichandler.PanicHandler("ZeroAiGetAgentsCommand", recover())
	}()

	agents := zs.agentService.ListAgents()
	result := make([]wshrpc.ZeroAiAgentInfo, len(agents))
	for i, a := range agents {
		result[i] = wshrpc.ZeroAiAgentInfo{
			Backend:     a.Backend,
			Model:       a.Backend,
			Provider:    "cli",
			DisplayName: a.Backend,
			Description: a.CliPath,
			Enabled:     a.IsRunning,
		}
	}

	customProviders, _ := zs.providerService.ListProviders()
	for _, prov := range customProviders {
		result = append(result, wshrpc.ZeroAiAgentInfo{
			Backend:     prov.ID,
			Model:       prov.DefaultModel,
			Provider:    "custom",
			DisplayName: prov.DisplayName,
			Description: prov.CliCommand,
			Enabled:     prov.IsAvailable,
		})
	}
	return result, nil
}

func (zs *WshRpcZeroaiServer) ZeroAiConfirmPermissionCommand(ctx context.Context, req wshrpc.CommandZeroAiConfirmPermissionData) error {
	defer func() {
		panichandler.PanicHandler("ZeroAiConfirmPermissionCommand", recover())
	}()

	backend := zs.getBackendForSession(ctx, req.SessionID)
	ag, err := zs.agentService.GetAgent(ctx, agent.AgentConfig{Backend: backend})
	if err != nil {
		return err
	}
	return ag.ConfirmPermission(ctx, req.SessionID, req.CallID, req.OptionID)
}

func (zs *WshRpcZeroaiServer) ZeroAiCancelStreamCommand(ctx context.Context, req wshrpc.ZeroAiCancelStreamData) error {
	defer func() {
		panichandler.PanicHandler("ZeroAiCancelStreamCommand", recover())
	}()

	backend := zs.getBackendForSession(ctx, req.SessionID)
	ag, err := zs.agentService.GetAgent(ctx, agent.AgentConfig{Backend: backend})
	if err != nil {
		return err
	}
	ag.CancelPrompt()
	return nil
}

func toSessionWrapper(session *agent.AgentSession) wshrpc.ZeroAiSessionWrapper {
	return wshrpc.ZeroAiSessionWrapper{
		ID:            session.ID,
		Backend:       session.Backend,
		WorkDir:       session.WorkDir,
		Model:         session.Model,
		Provider:      session.Backend,
		ThinkingLevel: "",
		YoloMode:      false,
		SessionID:     session.ID,
		CreatedAt:     session.CreatedAt,
		UpdatedAt:     session.UpdatedAt,
	}
}

func extractEventText(data interface{}) string {
	if data == nil {
		return ""
	}
	if str, ok := data.(string); ok {
		return str
	}
	if m, ok := data.(map[string]interface{}); ok {
		if text, ok := m["text"].(string); ok {
			return text
		}
	}
	if blocks, ok := data.([]interface{}); ok {
		var parts []string
		for _, b := range blocks {
			if block, ok := b.(map[string]interface{}); ok {
				if ct, ok := block["content"].(map[string]interface{}); ok {
					if text, ok := ct["text"].(string); ok {
						parts = append(parts, text)
						continue
					}
				}
				if text, ok := block["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "")
		}
	}
	if bytes, err := json.Marshal(data); err == nil {
		return string(bytes)
	}
	return ""
}

func extractEventContentRaw(data interface{}) string {
	if data == nil {
		return ""
	}
	if str, ok := data.(string); ok {
		return str
	}
	if blocks, ok := data.([]interface{}); ok {
		var parts []string
		for _, b := range blocks {
			if block, ok := b.(map[string]interface{}); ok {
				if ct, ok := block["content"].(map[string]interface{}); ok {
					if text, ok := ct["text"].(string); ok {
						parts = append(parts, text)
						continue
					}
				}
				if text, ok := block["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "")
		}
	}
	if bytes, err := json.Marshal(data); err == nil {
		return string(bytes)
	}
	return ""
}

func sendError(ch chan wshrpc.RespOrErrorUnion[wshrpc.ZeroAiStreamMessageEvent], err error) {
	resp := wshrpc.RespOrErrorUnion[wshrpc.ZeroAiStreamMessageEvent]{}
	resp.Error = err
	ch <- resp
}

func (zs *WshRpcZeroaiServer) ZeroAiListProvidersCommand(ctx context.Context, req wshrpc.CommandZeroAiListProvidersData) (wshrpc.CommandZeroAiListProvidersRtnData, error) {
	defer func() {
		panichandler.PanicHandler("ZeroAiListProvidersCommand", recover())
	}()
	providers, err := zs.providerService.ListProviders()
	if err != nil {
		return wshrpc.CommandZeroAiListProvidersRtnData{}, err
	}
	return wshrpc.CommandZeroAiListProvidersRtnData{
		Providers: providers,
	}, nil
}

func (zs *WshRpcZeroaiServer) ZeroAiSaveProviderCommand(ctx context.Context, req wshrpc.CommandZeroAiSaveProviderData) error {
	defer func() {
		panichandler.PanicHandler("ZeroAiSaveProviderCommand", recover())
	}()
	return zs.providerService.SaveProvider(req)
}

func (zs *WshRpcZeroaiServer) ZeroAiDeleteProviderCommand(ctx context.Context, req wshrpc.CommandZeroAiDeleteProviderData) error {
	defer func() {
		panichandler.PanicHandler("ZeroAiDeleteProviderCommand", recover())
	}()
	return zs.providerService.DeleteProvider(req.ProviderID)
}

func (zs *WshRpcZeroaiServer) ZeroAiTestProviderCommand(ctx context.Context, req wshrpc.CommandZeroAiTestProviderData) (wshrpc.CommandZeroAiTestProviderRtnData, error) {
	defer func() {
		panichandler.PanicHandler("ZeroAiTestProviderCommand", recover())
	}()
	result, err := zs.providerService.TestProvider(ctx, req.ProviderID)
	if err != nil {
		return wshrpc.CommandZeroAiTestProviderRtnData{}, err
	}
	return wshrpc.CommandZeroAiTestProviderRtnData{
		Result: result,
	}, nil
}

func (zs *WshRpcZeroaiServer) ZeroAiSpawnAgentBlockCommand(ctx context.Context, req ZeroAiSpawnAgentBlockData) (*ZeroAiSpawnAgentBlockRtnData, error) {
	defer func() {
		panichandler.PanicHandler("ZeroAiSpawnAgentBlockCommand", recover())
	}()

	if req.AgentID == "" {
		return nil, fmt.Errorf("agent ID is required")
	}
	if req.TabID == "" {
		return nil, fmt.Errorf("tab ID is required")
	}

	if zs.blockManager == nil {
		return nil, fmt.Errorf("block manager not initialized")
	}

	opts := team.SpawnBlockOpts{
		TabID:     req.TabID,
		AgentID:   req.AgentID,
		AgentName: req.AgentName,
		TeamID:    req.TeamID,
		Role:      team.MemberRole(req.Role),
		Prompt:    req.Prompt,
		WorkDir:   req.WorkDir,
	}

	block, err := zs.blockManager.SpawnAgentBlock(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &ZeroAiSpawnAgentBlockRtnData{
		BlockID: block.BlockID,
		AgentID: block.AgentID,
	}, nil
}

func (zs *WshRpcZeroaiServer) ZeroAiCreateAgentRoleCommand(ctx context.Context, req ZeroAiCreateAgentRoleData) (*ZeroAiCreateAgentRoleRtnData, error) {
	defer func() {
		panichandler.PanicHandler("ZeroAiCreateAgentRoleCommand", recover())
	}()

	if req.Name == "" {
		return nil, fmt.Errorf("agent name is required")
	}
	if req.Backend == "" {
		return nil, fmt.Errorf("backend is required")
	}

	agentID := fmt.Sprintf("agent-%s-%d", req.Name, time.Now().UnixMilli())

	// For now, we validate and return success
	// The actual persistence will be handled by the frontend model
	// TODO: Store the role definition when backend persistence is ready

	return &ZeroAiCreateAgentRoleRtnData{
		AgentID: agentID,
	}, nil
}
