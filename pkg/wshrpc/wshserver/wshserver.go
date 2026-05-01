// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package wshserver

// this file contains the implementation of the wsh server methods

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/skratchdot/open-golang/open"
	"github.com/wavetermdev/waveterm/pkg/aiusechat"
	"github.com/wavetermdev/waveterm/pkg/aiusechat/chatstore"
	"github.com/wavetermdev/waveterm/pkg/aiusechat/uctypes"
	"github.com/wavetermdev/waveterm/pkg/baseds"
	"github.com/wavetermdev/waveterm/pkg/blockcontroller"
	"github.com/wavetermdev/waveterm/pkg/blocklogger"
	"github.com/wavetermdev/waveterm/pkg/buildercontroller"
	"github.com/wavetermdev/waveterm/pkg/team"
	"github.com/wavetermdev/waveterm/pkg/filebackup"
	"github.com/wavetermdev/waveterm/pkg/filestore"
	"github.com/wavetermdev/waveterm/pkg/genconn"
	"github.com/wavetermdev/waveterm/pkg/jobcontroller"
	"github.com/wavetermdev/waveterm/pkg/panichandler"
	"github.com/wavetermdev/waveterm/pkg/remote"
	"github.com/wavetermdev/waveterm/pkg/remote/conncontroller"
	"github.com/wavetermdev/waveterm/pkg/remote/fileshare/wshfs"
	"github.com/wavetermdev/waveterm/pkg/secretstore"
	"github.com/wavetermdev/waveterm/pkg/suggestion"
	"github.com/wavetermdev/waveterm/pkg/telemetry"
	"github.com/wavetermdev/waveterm/pkg/telemetry/telemetrydata"
	"github.com/wavetermdev/waveterm/pkg/util/envutil"
	"github.com/wavetermdev/waveterm/pkg/util/shellutil"
	"github.com/wavetermdev/waveterm/pkg/util/utilfn"
	"github.com/wavetermdev/waveterm/pkg/waveai"
	"github.com/wavetermdev/waveterm/pkg/waveappstore"
	"github.com/wavetermdev/waveterm/pkg/waveapputil"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/wavejwt"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wcloud"
	"github.com/wavetermdev/waveterm/pkg/wconfig"
	"github.com/wavetermdev/waveterm/pkg/wcore"
	"github.com/wavetermdev/waveterm/pkg/wps"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshutil"
	"github.com/wavetermdev/waveterm/pkg/wsl"
	"github.com/wavetermdev/waveterm/pkg/wslconn"
	"github.com/wavetermdev/waveterm/pkg/wstore"
	"github.com/wavetermdev/waveterm/tsunami/build"
)

var InvalidWslDistroNames = []string{"docker-desktop", "docker-desktop-data"}

type WshServer struct{}

func (*WshServer) WshServerImpl() {}

var WshServerImpl = WshServer{}

func (ws *WshServer) GetJwtPublicKeyCommand(ctx context.Context) (string, error) {
	return wavejwt.GetPublicKeyBase64(), nil
}

func (ws *WshServer) TestCommand(ctx context.Context, data string) error {
	defer func() {
		panichandler.PanicHandler("TestCommand", recover())
	}()
	rpcSource := wshutil.GetRpcSourceFromContext(ctx)
	log.Printf("TEST src:%s | %s\n", rpcSource, data)
	return nil
}

func (ws *WshServer) TestMultiArgCommand(ctx context.Context, arg1 string, arg2 int, arg3 bool) (string, error) {
	defer func() {
		panichandler.PanicHandler("TestMultiArgCommand", recover())
	}()
	rpcSource := wshutil.GetRpcSourceFromContext(ctx)
	rtn := fmt.Sprintf("src:%s arg1:%q arg2:%d arg3:%t", rpcSource, arg1, arg2, arg3)
	log.Printf("TESTMULTI %s\n", rtn)
	return rtn, nil
}

// for testing
func (ws *WshServer) MessageCommand(ctx context.Context, data wshrpc.CommandMessageData) error {
	log.Printf("MESSAGE: %s\n", data.Message)
	return nil
}

// for testing
func (ws *WshServer) StreamTestCommand(ctx context.Context) chan wshrpc.RespOrErrorUnion[int] {
	rtn := make(chan wshrpc.RespOrErrorUnion[int])
	go func() {
		defer func() {
			panichandler.PanicHandler("StreamTestCommand", recover())
		}()
		for i := 1; i <= 5; i++ {
			rtn <- wshrpc.RespOrErrorUnion[int]{Response: i}
			time.Sleep(1 * time.Second)
		}
		close(rtn)
	}()
	return rtn
}

func (ws *WshServer) StreamWaveAiCommand(ctx context.Context, request wshrpc.WaveAIStreamRequest) chan wshrpc.RespOrErrorUnion[wshrpc.WaveAIPacketType] {
	return waveai.RunAICommand(ctx, request)
}

func MakePlotData(ctx context.Context, blockId string) error {
	block, err := wstore.DBMustGet[*waveobj.Block](ctx, blockId)
	if err != nil {
		return err
	}
	viewName := block.Meta.GetString(waveobj.MetaKey_View, "")
	if viewName != "cpuplot" && viewName != "sysinfo" {
		return fmt.Errorf("invalid view type: %s", viewName)
	}
	return filestore.WFS.MakeFile(ctx, blockId, "cpuplotdata", nil, wshrpc.FileOpts{})
}

func SavePlotData(ctx context.Context, blockId string, history string) error {
	block, err := wstore.DBMustGet[*waveobj.Block](ctx, blockId)
	if err != nil {
		return err
	}
	viewName := block.Meta.GetString(waveobj.MetaKey_View, "")
	if viewName != "cpuplot" && viewName != "sysinfo" {
		return fmt.Errorf("invalid view type: %s", viewName)
	}
	// todo: interpret the data being passed
	// for now, this is just to throw an error if the block was closed
	historyBytes, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("unable to serialize plot data: %v", err)
	}
	// ignore MakeFile error (already exists is ok)
	return filestore.WFS.WriteFile(ctx, blockId, "cpuplotdata", historyBytes)
}

func (ws *WshServer) GetMetaCommand(ctx context.Context, data wshrpc.CommandGetMetaData) (waveobj.MetaMapType, error) {
	obj, err := wstore.DBGetORef(ctx, data.ORef)
	if err != nil {
		return nil, fmt.Errorf("error getting object: %w", err)
	}
	if obj == nil {
		return nil, fmt.Errorf("object not found: %s", data.ORef)
	}
	return waveobj.GetMeta(obj), nil
}

func (ws *WshServer) UpdateTabNameCommand(ctx context.Context, tabId string, newName string) error {
	oref := waveobj.ORef{OType: waveobj.OType_Tab, OID: tabId}
	err := wstore.UpdateTabName(ctx, tabId, newName)
	if err != nil {
		return fmt.Errorf("error updating tab name: %w", err)
	}
	wcore.SendWaveObjUpdate(oref)
	return nil
}

func (ws *WshServer) UpdateWorkspaceTabIdsCommand(ctx context.Context, workspaceId string, tabIds []string) error {
	oref := waveobj.ORef{OType: waveobj.OType_Workspace, OID: workspaceId}
	err := wcore.UpdateWorkspaceTabIds(ctx, workspaceId, tabIds)
	if err != nil {
		return fmt.Errorf("error updating workspace tab ids: %w", err)
	}
	wcore.SendWaveObjUpdate(oref)
	return nil
}

func (ws *WshServer) SetMetaCommand(ctx context.Context, data wshrpc.CommandSetMetaData) error {
	log.Printf("SetMetaCommand: %s | %v\n", data.ORef, data.Meta)
	oref := data.ORef
	err := wstore.UpdateObjectMeta(ctx, oref, data.Meta, false)
	if err != nil {
		return fmt.Errorf("error updating object meta: %w", err)
	}
	wcore.SendWaveObjUpdate(oref)
	return nil
}

func (ws *WshServer) GetRTInfoCommand(ctx context.Context, data wshrpc.CommandGetRTInfoData) (*waveobj.ObjRTInfo, error) {
	return wstore.GetRTInfo(data.ORef), nil
}

func (ws *WshServer) SetRTInfoCommand(ctx context.Context, data wshrpc.CommandSetRTInfoData) error {
	if data.Delete {
		wstore.DeleteRTInfo(data.ORef)
		return nil
	}
	wstore.SetRTInfo(data.ORef, data.Data)
	return nil
}

func (ws *WshServer) ResolveIdsCommand(ctx context.Context, data wshrpc.CommandResolveIdsData) (wshrpc.CommandResolveIdsRtnData, error) {
	rtn := wshrpc.CommandResolveIdsRtnData{}
	rtn.ResolvedIds = make(map[string]waveobj.ORef)
	var firstErr error
	for _, simpleId := range data.Ids {
		oref, err := resolveSimpleId(ctx, data, simpleId)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if oref == nil {
			continue
		}
		rtn.ResolvedIds[simpleId] = *oref
	}
	if firstErr != nil && len(data.Ids) == 1 {
		return rtn, firstErr
	}
	return rtn, nil
}

func (ws *WshServer) CreateBlockCommand(ctx context.Context, data wshrpc.CommandCreateBlockData) (*waveobj.ORef, error) {
	ctx = waveobj.ContextWithUpdates(ctx)
	tabId := data.TabId
	blockData, err := wcore.CreateBlock(ctx, tabId, data.BlockDef, data.RtOpts)
	if err != nil {
		return nil, fmt.Errorf("error creating block: %w", err)
	}
	var layoutAction *waveobj.LayoutActionData
	if data.TargetBlockId != "" {
		switch data.TargetAction {
		case "replace":
			layoutAction = &waveobj.LayoutActionData{
				ActionType:    wcore.LayoutActionDataType_Replace,
				TargetBlockId: data.TargetBlockId,
				BlockId:       blockData.OID,
				Focused:       data.Focused,
			}
			err = wcore.DeleteBlock(ctx, data.TargetBlockId, false)
			if err != nil {
				return nil, fmt.Errorf("error deleting block (trying to do block replace): %w", err)
			}
		case "splitright":
			layoutAction = &waveobj.LayoutActionData{
				ActionType:    wcore.LayoutActionDataType_SplitHorizontal,
				BlockId:       blockData.OID,
				TargetBlockId: data.TargetBlockId,
				Position:      "after",
				Focused:       data.Focused,
			}
		case "splitleft":
			layoutAction = &waveobj.LayoutActionData{
				ActionType:    wcore.LayoutActionDataType_SplitHorizontal,
				BlockId:       blockData.OID,
				TargetBlockId: data.TargetBlockId,
				Position:      "before",
				Focused:       data.Focused,
			}
		case "splitup":
			layoutAction = &waveobj.LayoutActionData{
				ActionType:    wcore.LayoutActionDataType_SplitVertical,
				BlockId:       blockData.OID,
				TargetBlockId: data.TargetBlockId,
				Position:      "before",
				Focused:       data.Focused,
			}
		case "splitdown":
			layoutAction = &waveobj.LayoutActionData{
				ActionType:    wcore.LayoutActionDataType_SplitVertical,
				BlockId:       blockData.OID,
				TargetBlockId: data.TargetBlockId,
				Position:      "after",
				Focused:       data.Focused,
			}
		default:
			return nil, fmt.Errorf("invalid target action: %s", data.TargetAction)
		}
	} else {
		layoutAction = &waveobj.LayoutActionData{
			ActionType: wcore.LayoutActionDataType_Insert,
			BlockId:    blockData.OID,
			Magnified:  data.Magnified,
			Ephemeral:  data.Ephemeral,
			Focused:    data.Focused,
		}
	}
	err = wcore.QueueLayoutActionForTab(ctx, tabId, *layoutAction)
	if err != nil {
		return nil, fmt.Errorf("error queuing layout action: %w", err)
	}
	updates := waveobj.ContextGetUpdatesRtn(ctx)
	wps.Broker.SendUpdateEvents(updates)
	return &waveobj.ORef{OType: waveobj.OType_Block, OID: blockData.OID}, nil
}

func (ws *WshServer) CreateSubBlockCommand(ctx context.Context, data wshrpc.CommandCreateSubBlockData) (*waveobj.ORef, error) {
	parentBlockId := data.ParentBlockId
	blockData, err := wcore.CreateSubBlock(ctx, parentBlockId, data.BlockDef)
	if err != nil {
		return nil, fmt.Errorf("error creating block: %w", err)
	}
	blockRef := &waveobj.ORef{OType: waveobj.OType_Block, OID: blockData.OID}
	return blockRef, nil
}

func (ws *WshServer) ControllerDestroyCommand(ctx context.Context, blockId string) error {
	blockcontroller.DestroyBlockController(blockId)
	return nil
}

func (ws *WshServer) ControllerResyncCommand(ctx context.Context, data wshrpc.CommandControllerResyncData) error {
	ctx = genconn.ContextWithConnData(ctx, data.BlockId)
	ctx = termCtxWithLogBlockId(ctx, data.BlockId)
	return blockcontroller.ResyncController(ctx, data.TabId, data.BlockId, data.RtOpts, data.ForceRestart)
}

func (ws *WshServer) ControllerInputCommand(ctx context.Context, data wshrpc.CommandBlockInputData) error {
	inputUnion := &blockcontroller.BlockInputUnion{
		SigName:  data.SigName,
		TermSize: data.TermSize,
	}
	if len(data.InputData64) > 0 {
		inputBuf := make([]byte, base64.StdEncoding.DecodedLen(len(data.InputData64)))
		nw, err := base64.StdEncoding.Decode(inputBuf, []byte(data.InputData64))
		if err != nil {
			return fmt.Errorf("error decoding input data: %w", err)
		}
		inputUnion.InputData = inputBuf[:nw]
	}
	return blockcontroller.SendInput(data.BlockId, inputUnion)
}

func (ws *WshServer) ControllerAppendOutputCommand(ctx context.Context, data wshrpc.CommandControllerAppendOutputData) error {
	outputBuf := make([]byte, base64.StdEncoding.DecodedLen(len(data.Data64)))
	nw, err := base64.StdEncoding.Decode(outputBuf, []byte(data.Data64))
	if err != nil {
		return fmt.Errorf("error decoding output data: %w", err)
	}
	err = blockcontroller.HandleAppendBlockFile(data.BlockId, wavebase.BlockFile_Term, outputBuf[:nw])
	if err != nil {
		return fmt.Errorf("error appending to block file: %w", err)
	}
	return nil
}

func (ws *WshServer) FileCreateCommand(ctx context.Context, data wshrpc.FileData) error {
	data.Data64 = ""
	err := wshfs.PutFile(ctx, data)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	return nil
}

func (ws *WshServer) FileMkdirCommand(ctx context.Context, data wshrpc.FileData) error {
	return wshfs.Mkdir(ctx, data.Info.Path)
}

func (ws *WshServer) FileDeleteCommand(ctx context.Context, data wshrpc.CommandDeleteFileData) error {
	return wshfs.Delete(ctx, data)
}

func (ws *WshServer) FileInfoCommand(ctx context.Context, data wshrpc.FileData) (*wshrpc.FileInfo, error) {
	return wshfs.Stat(ctx, data.Info.Path)
}

func (ws *WshServer) FileListCommand(ctx context.Context, data wshrpc.FileListData) ([]*wshrpc.FileInfo, error) {
	return wshfs.ListEntries(ctx, data.Path, data.Opts)
}

func (ws *WshServer) FileListStreamCommand(ctx context.Context, data wshrpc.FileListData) <-chan wshrpc.RespOrErrorUnion[wshrpc.CommandRemoteListEntriesRtnData] {
	return wshfs.ListEntriesStream(ctx, data.Path, data.Opts)
}

func (ws *WshServer) FileWriteCommand(ctx context.Context, data wshrpc.FileData) error {
	return wshfs.PutFile(ctx, data)
}

func (ws *WshServer) FileReadCommand(ctx context.Context, data wshrpc.FileData) (*wshrpc.FileData, error) {
	return wshfs.Read(ctx, data)
}

func (ws *WshServer) FileStreamCommand(ctx context.Context, data wshrpc.CommandFileStreamData) (*wshrpc.FileInfo, error) {
	return wshfs.FileStream(ctx, data)
}

func (ws *WshServer) FileCopyCommand(ctx context.Context, data wshrpc.CommandFileCopyData) error {
	return wshfs.Copy(ctx, data)
}

func (ws *WshServer) FileMoveCommand(ctx context.Context, data wshrpc.CommandFileCopyData) error {
	return wshfs.Move(ctx, data)
}

func (ws *WshServer) FileAppendCommand(ctx context.Context, data wshrpc.FileData) error {
	return wshfs.Append(ctx, data)
}

func (ws *WshServer) FileJoinCommand(ctx context.Context, paths []string) (*wshrpc.FileInfo, error) {
	if len(paths) < 2 {
		if len(paths) == 0 {
			return nil, fmt.Errorf("no paths provided")
		}
		return wshfs.Stat(ctx, paths[0])
	}
	return wshfs.Join(ctx, paths[0], paths[1:]...)
}

func (ws *WshServer) FileRestoreBackupCommand(ctx context.Context, data wshrpc.CommandFileRestoreBackupData) error {
	expandedBackupPath, err := wavebase.ExpandHomeDir(data.BackupFilePath)
	if err != nil {
		return fmt.Errorf("failed to expand backup file path: %w", err)
	}
	expandedRestorePath, err := wavebase.ExpandHomeDir(data.RestoreToFileName)
	if err != nil {
		return fmt.Errorf("failed to expand restore file path: %w", err)
	}
	return filebackup.RestoreBackup(expandedBackupPath, expandedRestorePath)
}

func (ws *WshServer) GetTempDirCommand(ctx context.Context, data wshrpc.CommandGetTempDirData) (string, error) {
	tempDir := os.TempDir()
	if data.FileName != "" {
		// Reduce to a simple file name to avoid absolute paths or traversal
		name := filepath.Base(data.FileName)
		// Normalize/trim any stray separators and whitespace
		name = strings.Trim(name, `/\`+" ")
		if name == "" || name == "." {
			return tempDir, nil
		}
		return filepath.Join(tempDir, name), nil
	}
	return tempDir, nil
}

func (ws *WshServer) WriteTempFileCommand(ctx context.Context, data wshrpc.CommandWriteTempFileData) (string, error) {
	if data.FileName == "" {
		return "", fmt.Errorf("filename is required")
	}
	name := filepath.Base(data.FileName)
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid filename")
	}
	tempDir, err := os.MkdirTemp("", "waveterm-")
	if err != nil {
		return "", fmt.Errorf("error creating temp directory: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(data.Data64)
	if err != nil {
		return "", fmt.Errorf("error decoding base64 data: %w", err)
	}
	tempPath := filepath.Join(tempDir, name)
	err = os.WriteFile(tempPath, decoded, 0600)
	if err != nil {
		return "", fmt.Errorf("error writing temp file: %w", err)
	}
	return tempPath, nil
}

func (ws *WshServer) DeleteSubBlockCommand(ctx context.Context, data wshrpc.CommandDeleteBlockData) error {
	if data.BlockId == "" {
		return fmt.Errorf("blockid is required")
	}
	err := wcore.DeleteBlock(ctx, data.BlockId, false)
	if err != nil {
		return fmt.Errorf("error deleting block: %w", err)
	}
	return nil
}

func (ws *WshServer) DeleteBlockCommand(ctx context.Context, data wshrpc.CommandDeleteBlockData) error {
	if data.BlockId == "" {
		return fmt.Errorf("blockid is required")
	}
	ctx = waveobj.ContextWithUpdates(ctx)
	tabId, err := wstore.DBFindTabForBlockId(ctx, data.BlockId)
	if err != nil {
		return fmt.Errorf("error finding tab for block: %w", err)
	}
	if tabId == "" {
		return fmt.Errorf("no tab found for block")
	}
	err = wcore.DeleteBlock(ctx, data.BlockId, true)
	if err != nil {
		return fmt.Errorf("error deleting block: %w", err)
	}
	wcore.QueueLayoutActionForTab(ctx, tabId, waveobj.LayoutActionData{
		ActionType: wcore.LayoutActionDataType_Remove,
		BlockId:    data.BlockId,
	})
	updates := waveobj.ContextGetUpdatesRtn(ctx)
	wps.Broker.SendUpdateEvents(updates)
	return nil
}

func (ws *WshServer) WaitForRouteCommand(ctx context.Context, data wshrpc.CommandWaitForRouteData) (bool, error) {
	waitCtx, cancelFn := context.WithTimeout(ctx, time.Duration(data.WaitMs)*time.Millisecond)
	defer cancelFn()
	err := wshutil.DefaultRouter.WaitForRegister(waitCtx, data.RouteId)
	return err == nil, nil
}

func (ws *WshServer) EventRecvCommand(ctx context.Context, data wps.WaveEvent) error {
	return nil
}

func (ws *WshServer) EventPublishCommand(ctx context.Context, data wps.WaveEvent) error {
	rpcSource := wshutil.GetRpcSourceFromContext(ctx)
	if rpcSource == "" {
		return fmt.Errorf("no rpc source set")
	}
	if data.Sender == "" {
		data.Sender = rpcSource
	}
	wps.Broker.Publish(data)
	return nil
}

func (ws *WshServer) EventSubCommand(ctx context.Context, data wps.SubscriptionRequest) error {
	rpcSource := wshutil.GetRpcSourceFromContext(ctx)
	if rpcSource == "" {
		return fmt.Errorf("no rpc source set")
	}
	wps.Broker.Subscribe(rpcSource, data)
	return nil
}

func (ws *WshServer) EventUnsubCommand(ctx context.Context, data string) error {
	rpcSource := wshutil.GetRpcSourceFromContext(ctx)
	if rpcSource == "" {
		return fmt.Errorf("no rpc source set")
	}
	wps.Broker.Unsubscribe(rpcSource, data)
	return nil
}

func (ws *WshServer) EventUnsubAllCommand(ctx context.Context) error {
	rpcSource := wshutil.GetRpcSourceFromContext(ctx)
	if rpcSource == "" {
		return fmt.Errorf("no rpc source set")
	}
	wps.Broker.UnsubscribeAll(rpcSource)
	return nil
}

func (ws *WshServer) EventReadHistoryCommand(ctx context.Context, data wshrpc.CommandEventReadHistoryData) ([]*wps.WaveEvent, error) {
	events := wps.Broker.ReadEventHistory(data.Event, data.Scope, data.MaxItems)
	return events, nil
}

func (ws *WshServer) SetConfigCommand(ctx context.Context, data wshrpc.MetaSettingsType) error {
	return wconfig.SetBaseConfigValue(data.MetaMapType)
}

func (ws *WshServer) SetConnectionsConfigCommand(ctx context.Context, data wshrpc.ConnConfigRequest) error {
	return wconfig.SetConnectionsConfigValue(data.Host, data.MetaMapType)
}

func (ws *WshServer) GetFullConfigCommand(ctx context.Context) (wconfig.FullConfigType, error) {
	watcher := wconfig.GetWatcher()
	return watcher.GetFullConfig(), nil
}

func (ws *WshServer) GetWaveAIModeConfigCommand(ctx context.Context) (wconfig.AIModeConfigUpdate, error) {
	fullConfig := wconfig.GetWatcher().GetFullConfig()
	resolvedConfigs := aiusechat.ComputeResolvedAIModeConfigs(fullConfig)
	return wconfig.AIModeConfigUpdate{Configs: resolvedConfigs}, nil
}

func (ws *WshServer) ConnStatusCommand(ctx context.Context) ([]wshrpc.ConnStatus, error) {
	rtn := conncontroller.GetAllConnStatus()
	return rtn, nil
}

func (ws *WshServer) WslStatusCommand(ctx context.Context) ([]wshrpc.ConnStatus, error) {
	rtn := wslconn.GetAllConnStatus()
	return rtn, nil
}

func termCtxWithLogBlockId(ctx context.Context, logBlockId string) context.Context {
	if logBlockId == "" {
		return ctx
	}
	block, err := wstore.DBMustGet[*waveobj.Block](ctx, logBlockId)
	if err != nil {
		return ctx
	}
	connDebug := block.Meta.GetString(waveobj.MetaKey_TermConnDebug, "")
	if connDebug == "" {
		return ctx
	}
	return blocklogger.ContextWithLogBlockId(ctx, logBlockId, connDebug == "debug")
}

func (ws *WshServer) ConnEnsureCommand(ctx context.Context, data wshrpc.ConnExtData) error {
	ctx = genconn.ContextWithConnData(ctx, data.LogBlockId)
	ctx = termCtxWithLogBlockId(ctx, data.LogBlockId)
	if strings.HasPrefix(data.ConnName, "wsl://") {
		distroName := strings.TrimPrefix(data.ConnName, "wsl://")
		return wslconn.EnsureConnection(ctx, distroName)
	}
	return conncontroller.EnsureConnection(ctx, data.ConnName)
}

func (ws *WshServer) ConnDisconnectCommand(ctx context.Context, connName string) error {
	if conncontroller.IsLocalConnName(connName) {
		return nil
	}
	if strings.HasPrefix(connName, "wsl://") {
		distroName := strings.TrimPrefix(connName, "wsl://")
		conn := wslconn.GetWslConn(distroName)
		if conn == nil {
			return fmt.Errorf("distro not found: %s", connName)
		}
		return conn.Close()
	}
	connOpts, err := remote.ParseOpts(connName)
	if err != nil {
		return fmt.Errorf("error parsing connection name: %w", err)
	}
	conn := conncontroller.MaybeGetConn(connOpts)
	if conn == nil {
		return fmt.Errorf("connection not found: %s", connName)
	}
	return conn.Close()
}

func (ws *WshServer) ConnConnectCommand(ctx context.Context, connRequest wshrpc.ConnRequest) error {
	if conncontroller.IsLocalConnName(connRequest.Host) {
		return nil
	}
	ctx = genconn.ContextWithConnData(ctx, connRequest.LogBlockId)
	ctx = termCtxWithLogBlockId(ctx, connRequest.LogBlockId)
	connName := connRequest.Host
	if strings.HasPrefix(connName, "wsl://") {
		distroName := strings.TrimPrefix(connName, "wsl://")
		conn := wslconn.GetWslConn(distroName)
		if conn == nil {
			return fmt.Errorf("connection not found: %s", connName)
		}
		return conn.Connect(ctx)
	}
	connOpts, err := remote.ParseOpts(connName)
	if err != nil {
		return fmt.Errorf("error parsing connection name: %w", err)
	}
	conn := conncontroller.GetConn(connOpts)
	if conn == nil {
		return fmt.Errorf("connection not found: %s", connName)
	}
	return conn.Connect(ctx, &connRequest.Keywords)
}

func (ws *WshServer) ConnReinstallWshCommand(ctx context.Context, data wshrpc.ConnExtData) error {
	if conncontroller.IsLocalConnName(data.ConnName) {
		return nil
	}
	ctx = genconn.ContextWithConnData(ctx, data.LogBlockId)
	ctx = termCtxWithLogBlockId(ctx, data.LogBlockId)
	connName := data.ConnName
	if strings.HasPrefix(connName, "wsl://") {
		distroName := strings.TrimPrefix(connName, "wsl://")
		conn := wslconn.GetWslConn(distroName)
		if conn == nil {
			return fmt.Errorf("connection not found: %s", connName)
		}
		return conn.InstallWsh(ctx, "")
	}
	connOpts, err := remote.ParseOpts(connName)
	if err != nil {
		return fmt.Errorf("error parsing connection name: %w", err)
	}
	conn := conncontroller.GetConn(connOpts)
	if conn == nil {
		return fmt.Errorf("connection not found: %s", connName)
	}
	return conn.InstallWsh(ctx, "")
}

func (ws *WshServer) ConnUpdateWshCommand(ctx context.Context, remoteInfo wshrpc.RemoteInfo) (bool, error) {
	handler := wshutil.GetRpcResponseHandlerFromContext(ctx)
	if handler == nil {
		return false, fmt.Errorf("could not determine handler from context")
	}
	connName := handler.GetRpcContext().Conn
	if connName == "" {
		return false, fmt.Errorf("invalid remote info: missing connection name")
	}

	log.Printf("checking wsh version for connection %s (current: %s)", connName, remoteInfo.ClientVersion)
	upToDate, _, _, err := conncontroller.IsWshVersionUpToDate(ctx, remoteInfo.ClientVersion)
	if err != nil {
		return false, fmt.Errorf("unable to compare wsh version: %w", err)
	}
	if upToDate {
		// no need to update
		log.Printf("wsh is already up to date for connection %s", connName)
		return false, nil
	}

	// todo: need to add user input code here for validation

	if strings.HasPrefix(connName, "wsl://") {
		return false, fmt.Errorf("connupdatewshcommand is not supported for wsl connections")
	}
	connOpts, err := remote.ParseOpts(connName)
	if err != nil {
		return false, fmt.Errorf("error parsing connection name: %w", err)
	}
	conn := conncontroller.GetConn(connOpts)
	if conn == nil {
		return false, fmt.Errorf("connection not found: %s", connName)
	}
	err = conn.UpdateWsh(ctx, connName, &remoteInfo)
	if err != nil {
		return false, fmt.Errorf("wsh update failed for connection %s: %w", connName, err)
	}

	// todo: need to add code for modifying configs?
	return true, nil
}

func (ws *WshServer) ConnListCommand(ctx context.Context) ([]string, error) {
	return conncontroller.GetConnectionsList()
}

func (ws *WshServer) WslListCommand(ctx context.Context) ([]string, error) {
	distros, err := wsl.RegisteredDistros(ctx)
	if err != nil {
		return nil, err
	}
	var distroNames []string
	for _, distro := range distros {
		distroName := distro.Name()
		if utilfn.ContainsStr(InvalidWslDistroNames, distroName) {
			continue
		}
		distroNames = append(distroNames, distroName)
	}
	return distroNames, nil
}

func (ws *WshServer) WslDefaultDistroCommand(ctx context.Context) (string, error) {
	distro, ok, err := wsl.DefaultDistro(ctx)
	if err != nil {
		return "", fmt.Errorf("unable to determine default distro: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("unable to determine default distro")
	}
	return distro.Name(), nil
}

/**
 * Dismisses the WshFail Command in runtime memory on the backend
 */
func (ws *WshServer) DismissWshFailCommand(ctx context.Context, connName string) error {
	if strings.HasPrefix(connName, "wsl://") {
		distroName := strings.TrimPrefix(connName, "wsl://")
		conn := wslconn.GetWslConn(distroName)
		if conn == nil {
			return fmt.Errorf("connection not found: %s", connName)
		}
		conn.ClearWshError()
		conn.FireConnChangeEvent()
		return nil
	}
	opts, err := remote.ParseOpts(connName)
	if err != nil {
		return err
	}
	conn := conncontroller.GetConn(opts)
	if conn == nil {
		return fmt.Errorf("connection %s not found", connName)
	}
	conn.ClearWshError()
	conn.FireConnChangeEvent()
	return nil
}

func (ws *WshServer) NotifySystemResumeCommand(ctx context.Context) error {
	log.Printf("NotifySystemResumeCommand called\n")
	return nil
}

func (ws *WshServer) FindGitBashCommand(ctx context.Context, rescan bool) (string, error) {
	fullConfig := wconfig.GetWatcher().GetFullConfig()
	return shellutil.FindGitBash(&fullConfig, rescan), nil
}

func waveFileToWaveFileInfo(wf *filestore.WaveFile) *wshrpc.WaveFileInfo {
	return &wshrpc.WaveFileInfo{
		ZoneId:    wf.ZoneId,
		Name:      wf.Name,
		Opts:      wf.Opts,
		CreatedTs: wf.CreatedTs,
		Size:      wf.Size,
		ModTs:     wf.ModTs,
		Meta:      wf.Meta,
	}
}

func (ws *WshServer) BlockInfoCommand(ctx context.Context, blockId string) (*wshrpc.BlockInfoData, error) {
	blockData, err := wstore.DBMustGet[*waveobj.Block](ctx, blockId)
	if err != nil {
		return nil, fmt.Errorf("error getting block: %w", err)
	}
	tabId, err := wstore.DBFindTabForBlockId(ctx, blockId)
	if err != nil {
		return nil, fmt.Errorf("error finding tab for block: %w", err)
	}
	workspaceId, err := wstore.DBFindWorkspaceForTabId(ctx, tabId)
	if err != nil {
		return nil, fmt.Errorf("error finding window for tab: %w", err)
	}
	fileList, err := filestore.WFS.ListFiles(ctx, blockId)
	if err != nil {
		return nil, fmt.Errorf("error listing blockfiles: %w", err)
	}
	var fileInfoList []*wshrpc.WaveFileInfo
	for _, wf := range fileList {
		fileInfoList = append(fileInfoList, waveFileToWaveFileInfo(wf))
	}
	return &wshrpc.BlockInfoData{
		BlockId:     blockId,
		TabId:       tabId,
		WorkspaceId: workspaceId,
		Block:       blockData,
		Files:       fileInfoList,
	}, nil
}

func (ws *WshServer) DebugTermCommand(ctx context.Context, data wshrpc.CommandDebugTermData) (*wshrpc.CommandDebugTermRtnData, error) {
	if data.BlockId == "" {
		return nil, fmt.Errorf("blockid is required")
	}
	if data.Size <= 0 {
		return nil, fmt.Errorf("size must be greater than 0")
	}
	waveFile, err := filestore.WFS.Stat(ctx, data.BlockId, wavebase.BlockFile_Term)
	if err == fs.ErrNotExist {
		return &wshrpc.CommandDebugTermRtnData{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error statting term file: %w", err)
	}
	readSize := data.Size
	dataLength := waveFile.DataLength()
	if readSize > dataLength {
		readSize = dataLength
	}
	readOffset := waveFile.Size - readSize
	readOffset, readData, err := filestore.WFS.ReadAt(ctx, data.BlockId, wavebase.BlockFile_Term, readOffset, readSize)
	if err != nil {
		return nil, fmt.Errorf("error reading term file: %w", err)
	}
	return &wshrpc.CommandDebugTermRtnData{
		Offset: readOffset,
		Data64: base64.StdEncoding.EncodeToString(readData),
	}, nil
}

func (ws *WshServer) WaveInfoCommand(ctx context.Context) (*wshrpc.WaveInfoData, error) {
	return &wshrpc.WaveInfoData{
		Version:   wavebase.WaveVersion,
		ClientId:  wstore.GetClientId(),
		BuildTime: wavebase.BuildTime,
		ConfigDir: wavebase.GetWaveConfigDir(),
		DataDir:   wavebase.GetWaveDataDir(),
	}, nil
}

func (ws *WshServer) MacOSVersionCommand(ctx context.Context) (string, error) {
	return wavebase.ClientMacOSVersion(), nil
}

// BlocksListCommand returns every block visible in the requested
// scope (current workspace by default).
func (ws *WshServer) BlocksListCommand(
	ctx context.Context,
	req wshrpc.BlocksListRequest) ([]wshrpc.BlocksListEntry, error) {
	var results []wshrpc.BlocksListEntry

	// Resolve the set of workspaces to inspect
	var workspaceIDs []string
	if req.WorkspaceId != "" {
		workspaceIDs = []string{req.WorkspaceId}
	} else if req.WindowId != "" {
		win, err := wcore.GetWindow(ctx, req.WindowId)
		if err != nil {
			return nil, err
		}
		workspaceIDs = []string{win.WorkspaceId}
	} else {
		// "current" == first workspace in client focus list
		client, err := wstore.DBGetSingleton[*waveobj.Client](ctx)
		if err != nil {
			return nil, err
		}
		if len(client.WindowIds) == 0 {
			return nil, fmt.Errorf("no active window")
		}
		win, err := wcore.GetWindow(ctx, client.WindowIds[0])
		if err != nil {
			return nil, err
		}
		workspaceIDs = []string{win.WorkspaceId}
	}

	for _, wsID := range workspaceIDs {
		wsData, err := wcore.GetWorkspace(ctx, wsID)
		if err != nil {
			return nil, err
		}

		windowId, err := wstore.DBFindWindowForWorkspaceId(ctx, wsID)
		if err != nil {
			log.Printf("error finding window for workspace %s: %v", wsID, err)
		}

		for _, tabID := range wsData.TabIds {
			tab, err := wstore.DBMustGet[*waveobj.Tab](ctx, tabID)
			if err != nil {
				return nil, err
			}
			for _, blkID := range tab.BlockIds {
				blk, err := wstore.DBMustGet[*waveobj.Block](ctx, blkID)
				if err != nil {
					return nil, err
				}
				results = append(results, wshrpc.BlocksListEntry{
					WindowId:    windowId,
					WorkspaceId: wsID,
					TabId:       tabID,
					BlockId:     blkID,
					Meta:        blk.Meta,
				})
			}
		}
	}
	return results, nil
}

func (ws *WshServer) WorkspaceListCommand(ctx context.Context) ([]wshrpc.WorkspaceInfoData, error) {
	workspaceList, err := wcore.ListWorkspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing workspaces: %w", err)
	}
	var rtn []wshrpc.WorkspaceInfoData
	for _, workspaceEntry := range workspaceList {
		workspaceData, err := wcore.GetWorkspace(ctx, workspaceEntry.WorkspaceId)
		if err != nil {
			return nil, fmt.Errorf("error getting workspace: %w", err)
		}
		rtn = append(rtn, wshrpc.WorkspaceInfoData{
			WindowId:      workspaceEntry.WindowId,
			WorkspaceData: workspaceData,
		})
	}
	return rtn, nil
}

func (ws *WshServer) ListAllAppsCommand(ctx context.Context) ([]wshrpc.AppInfo, error) {
	return waveappstore.ListAllApps()
}

func (ws *WshServer) ListAllEditableAppsCommand(ctx context.Context) ([]wshrpc.AppInfo, error) {
	return waveappstore.ListAllEditableApps()
}

func (ws *WshServer) ListAllAppFilesCommand(ctx context.Context, data wshrpc.CommandListAllAppFilesData) (*wshrpc.CommandListAllAppFilesRtnData, error) {
	if data.AppId == "" {
		return nil, fmt.Errorf("must provide an appId to ListAllAppFilesCommand")
	}
	result, err := waveappstore.ListAllAppFiles(data.AppId)
	if err != nil {
		return nil, err
	}
	entries := make([]wshrpc.DirEntryOut, len(result.Entries))
	for i, entry := range result.Entries {
		entries[i] = wshrpc.DirEntryOut{
			Name:         entry.Name,
			Dir:          entry.Dir,
			Symlink:      entry.Symlink,
			Size:         entry.Size,
			Mode:         entry.Mode,
			Modified:     entry.Modified,
			ModifiedTime: entry.ModifiedTime,
		}
	}
	return &wshrpc.CommandListAllAppFilesRtnData{
		Path:         result.Path,
		AbsolutePath: result.AbsolutePath,
		ParentDir:    result.ParentDir,
		Entries:      entries,
		EntryCount:   result.EntryCount,
		TotalEntries: result.TotalEntries,
		Truncated:    result.Truncated,
	}, nil
}

func (ws *WshServer) ReadAppFileCommand(ctx context.Context, data wshrpc.CommandReadAppFileData) (*wshrpc.CommandReadAppFileRtnData, error) {
	if data.AppId == "" {
		return nil, fmt.Errorf("must provide an appId to ReadAppFileCommand")
	}
	fileData, err := waveappstore.ReadAppFile(data.AppId, data.FileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &wshrpc.CommandReadAppFileRtnData{
				NotFound: true,
			}, nil
		}
		return nil, fmt.Errorf("failed to read app file: %w", err)
	}
	return &wshrpc.CommandReadAppFileRtnData{
		Data64: base64.StdEncoding.EncodeToString(fileData.Contents),
		ModTs:  fileData.ModTs,
	}, nil
}

func (ws *WshServer) WriteAppFileCommand(ctx context.Context, data wshrpc.CommandWriteAppFileData) error {
	if data.AppId == "" {
		return fmt.Errorf("must provide an appId to WriteAppFileCommand")
	}
	contents, err := base64.StdEncoding.DecodeString(data.Data64)
	if err != nil {
		return fmt.Errorf("failed to decode data64: %w", err)
	}
	return waveappstore.WriteAppFile(data.AppId, data.FileName, contents)
}

func (ws *WshServer) WaveFileReadStreamCommand(ctx context.Context, data wshrpc.CommandWaveFileReadStreamData) (*wshrpc.WaveFileInfo, error) {
	const maxStreamFileSize = 5 * 1024 * 1024

	waveFile, err := filestore.WFS.Stat(ctx, data.ZoneId, data.Name)
	if err != nil {
		return nil, fmt.Errorf("error statting wavefile: %w", err)
	}

	dataLength := waveFile.DataLength()
	if dataLength > maxStreamFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum streaming size of %d bytes", dataLength, maxStreamFileSize)
	}

	wshRpc := wshutil.GetWshRpcFromContext(ctx)
	if wshRpc == nil || wshRpc.StreamBroker == nil {
		return nil, fmt.Errorf("no stream broker available")
	}

	writer, err := wshRpc.StreamBroker.CreateStreamWriter(&data.StreamMeta)
	if err != nil {
		return nil, fmt.Errorf("error creating stream writer: %w", err)
	}

	_, fileData, err := filestore.WFS.ReadFile(ctx, data.ZoneId, data.Name)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("error reading wavefile: %w", err)
	}

	go func() {
		defer func() {
			panichandler.PanicHandler("WaveFileReadStreamCommand", recover())
		}()
		defer writer.Close()

		_, err := writer.Write(fileData)
		if err != nil {
			log.Printf("error writing to stream for wavefile %s:%s: %v\n", data.ZoneId, data.Name, err)
		}
	}()

	rtnInfo := &wshrpc.WaveFileInfo{
		ZoneId:    waveFile.ZoneId,
		Name:      waveFile.Name,
		Opts:      waveFile.Opts,
		CreatedTs: waveFile.CreatedTs,
		Size:      waveFile.Size,
		ModTs:     waveFile.ModTs,
		Meta:      waveFile.Meta,
	}
	return rtnInfo, nil
}

func (ws *WshServer) WriteAppGoFileCommand(ctx context.Context, data wshrpc.CommandWriteAppGoFileData) (*wshrpc.CommandWriteAppGoFileRtnData, error) {
	if data.AppId == "" {
		return nil, fmt.Errorf("must provide an appId to WriteAppGoFileCommand")
	}
	contents, err := base64.StdEncoding.DecodeString(data.Data64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode data64: %w", err)
	}

	formattedOutput := waveapputil.FormatGoCode(contents)

	err = waveappstore.WriteAppFile(data.AppId, "app.go", formattedOutput)
	if err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(formattedOutput)
	return &wshrpc.CommandWriteAppGoFileRtnData{Data64: encoded}, nil
}

func (ws *WshServer) DeleteAppFileCommand(ctx context.Context, data wshrpc.CommandDeleteAppFileData) error {
	if data.AppId == "" {
		return fmt.Errorf("must provide an appId to DeleteAppFileCommand")
	}
	return waveappstore.DeleteAppFile(data.AppId, data.FileName)
}

func (ws *WshServer) RenameAppFileCommand(ctx context.Context, data wshrpc.CommandRenameAppFileData) error {
	if data.AppId == "" {
		return fmt.Errorf("must provide an appId to RenameAppFileCommand")
	}
	return waveappstore.RenameAppFile(data.AppId, data.FromFileName, data.ToFileName)
}

func (ws *WshServer) WriteAppSecretBindingsCommand(ctx context.Context, data wshrpc.CommandWriteAppSecretBindingsData) error {
	if data.AppId == "" {
		return fmt.Errorf("must provide an appId to WriteAppSecretBindingsCommand")
	}
	return waveappstore.WriteAppSecretBindings(data.AppId, data.Bindings)
}

func (ws *WshServer) DeleteBuilderCommand(ctx context.Context, builderId string) error {
	if builderId == "" {
		return fmt.Errorf("must provide a builderId to DeleteBuilderCommand")
	}
	buildercontroller.DeleteController(builderId)
	return nil
}

func (ws *WshServer) StartBuilderCommand(ctx context.Context, data wshrpc.CommandStartBuilderData) error {
	if data.BuilderId == "" {
		return fmt.Errorf("must provide a builderId to StartBuilderCommand")
	}
	bc := buildercontroller.GetOrCreateController(data.BuilderId)
	rtInfo := wstore.GetRTInfo(waveobj.MakeORef("builder", data.BuilderId))
	if rtInfo == nil {
		return fmt.Errorf("builder rtinfo not found for builderid: %s", data.BuilderId)
	}
	appId := rtInfo.BuilderAppId
	if appId == "" {
		return fmt.Errorf("builder appid not set for builderid: %s", data.BuilderId)
	}
	return bc.Start(ctx, appId, rtInfo.BuilderEnv)
}

func (ws *WshServer) StopBuilderCommand(ctx context.Context, builderId string) error {
	if builderId == "" {
		return fmt.Errorf("must provide a builderId to StopBuilderCommand")
	}
	bc := buildercontroller.GetController(builderId)
	if bc == nil {
		return nil
	}
	return bc.Stop()
}

func (ws *WshServer) RestartBuilderAndWaitCommand(ctx context.Context, data wshrpc.CommandRestartBuilderAndWaitData) (*wshrpc.RestartBuilderAndWaitResult, error) {
	if data.BuilderId == "" {
		return nil, fmt.Errorf("must provide a builderId to RestartBuilderAndWaitCommand")
	}

	bc := buildercontroller.GetOrCreateController(data.BuilderId)
	rtInfo := wstore.GetRTInfo(waveobj.MakeORef("builder", data.BuilderId))
	if rtInfo == nil {
		return nil, fmt.Errorf("builder rtinfo not found for builderid: %s", data.BuilderId)
	}

	appId := rtInfo.BuilderAppId
	if appId == "" {
		return nil, fmt.Errorf("builder appid not set for builderid: %s", data.BuilderId)
	}

	result, err := bc.RestartAndWaitForBuild(ctx, appId, rtInfo.BuilderEnv)
	if err != nil {
		return nil, err
	}

	return &wshrpc.RestartBuilderAndWaitResult{
		Success:      result.Success,
		ErrorMessage: result.ErrorMessage,
		BuildOutput:  result.BuildOutput,
	}, nil
}

func (ws *WshServer) GetBuilderStatusCommand(ctx context.Context, builderId string) (*wshrpc.BuilderStatusData, error) {
	if builderId == "" {
		return nil, fmt.Errorf("must provide a builderId to GetBuilderStatusCommand")
	}
	bc := buildercontroller.GetOrCreateController(builderId)
	status := bc.GetStatus()
	return &status, nil
}

func (ws *WshServer) GetBuilderOutputCommand(ctx context.Context, builderId string) ([]string, error) {
	if builderId == "" {
		return nil, fmt.Errorf("must provide a builderId to GetBuilderOutputCommand")
	}
	bc := buildercontroller.GetOrCreateController(builderId)
	return bc.GetOutput(), nil
}

func (ws *WshServer) CheckGoVersionCommand(ctx context.Context) (*wshrpc.CommandCheckGoVersionRtnData, error) {
	watcher := wconfig.GetWatcher()
	fullConfig := watcher.GetFullConfig()
	goPath := fullConfig.Settings.TsunamiGoPath

	result := build.CheckGoVersion(goPath)

	return &wshrpc.CommandCheckGoVersionRtnData{
		GoStatus:    result.GoStatus,
		GoPath:      result.GoPath,
		GoVersion:   result.GoVersion,
		ErrorString: result.ErrorString,
	}, nil
}

func (ws *WshServer) PublishAppCommand(ctx context.Context, data wshrpc.CommandPublishAppData) (*wshrpc.CommandPublishAppRtnData, error) {
	publishedAppId, err := waveappstore.PublishDraft(data.AppId)
	if err != nil {
		return nil, fmt.Errorf("error publishing app: %w", err)
	}
	return &wshrpc.CommandPublishAppRtnData{
		PublishedAppId: publishedAppId,
	}, nil
}

func (ws *WshServer) MakeDraftFromLocalCommand(ctx context.Context, data wshrpc.CommandMakeDraftFromLocalData) (*wshrpc.CommandMakeDraftFromLocalRtnData, error) {
	draftAppId, err := waveappstore.MakeDraftFromLocal(data.LocalAppId)
	if err != nil {
		return nil, fmt.Errorf("error making draft from local: %w", err)
	}
	return &wshrpc.CommandMakeDraftFromLocalRtnData{
		DraftAppId: draftAppId,
	}, nil
}

func (ws *WshServer) RecordTEventCommand(ctx context.Context, data telemetrydata.TEvent) error {
	err := telemetry.RecordTEvent(ctx, &data)
	if err != nil {
		log.Printf("error recording telemetry event: %v", err)
	}
	return err
}

func (ws WshServer) SendTelemetryCommand(ctx context.Context) error {
	return wcloud.SendAllTelemetry(wstore.GetClientId())
}

func (ws *WshServer) WaveAIEnableTelemetryCommand(ctx context.Context) error {
	// Enable telemetry in config
	meta := waveobj.MetaMapType{
		wconfig.ConfigKey_TelemetryEnabled: true,
	}
	err := wconfig.SetBaseConfigValue(meta)
	if err != nil {
		return fmt.Errorf("error setting telemetry enabled: %w", err)
	}

	// Record the telemetry event
	event := telemetrydata.MakeTEvent("waveai:enabletelemetry", telemetrydata.TEventProps{})
	err = telemetry.RecordTEvent(ctx, event)
	if err != nil {
		log.Printf("error recording waveai:enabletelemetry event: %v", err)
	}

	// Immediately send telemetry to cloud
	err = wcloud.SendAllTelemetry(wstore.GetClientId())
	if err != nil {
		log.Printf("error sending telemetry after enabling: %v", err)
	}

	return nil
}

func (ws *WshServer) GetWaveAIChatCommand(ctx context.Context, data wshrpc.CommandGetWaveAIChatData) (*uctypes.UIChat, error) {
	aiChat := chatstore.DefaultChatStore.Get(data.ChatId)
	if aiChat == nil {
		return nil, nil
	}
	uiChat, err := aiusechat.ConvertAIChatToUIChat(aiChat)
	if err != nil {
		return nil, fmt.Errorf("error converting AI chat to UI chat: %w", err)
	}
	return uiChat, nil
}

func (ws *WshServer) GetWaveAIRateLimitCommand(ctx context.Context) (*uctypes.RateLimitInfo, error) {
	return aiusechat.GetGlobalRateLimit(), nil
}

func (ws *WshServer) WaveAIToolApproveCommand(ctx context.Context, data wshrpc.CommandWaveAIToolApproveData) error {
	return aiusechat.UpdateToolApproval(data.ToolCallId, data.Approval)
}

func (ws *WshServer) WaveAIGetToolDiffCommand(ctx context.Context, data wshrpc.CommandWaveAIGetToolDiffData) (*wshrpc.CommandWaveAIGetToolDiffRtnData, error) {
	originalContent, modifiedContent, err := aiusechat.CreateWriteTextFileDiff(ctx, data.ChatId, data.ToolCallId)
	if err != nil {
		return nil, err
	}

	return &wshrpc.CommandWaveAIGetToolDiffRtnData{
		OriginalContents64: base64.StdEncoding.EncodeToString(originalContent),
		ModifiedContents64: base64.StdEncoding.EncodeToString(modifiedContent),
	}, nil
}

var wshActivityRe = regexp.MustCompile(`^[a-z:#]+$`)

func (ws *WshServer) WshActivityCommand(ctx context.Context, data map[string]int) error {
	if len(data) == 0 {
		return nil
	}
	props := telemetrydata.TEventProps{}
	for key, value := range data {
		if len(key) > 20 {
			delete(data, key)
		}
		if !wshActivityRe.MatchString(key) {
			delete(data, key)
		}
		if value != 1 {
			delete(data, key)
		}
		if strings.HasSuffix(key, "#error") {
			props.WshHadError = true
		} else {
			props.WshCmd = key
		}
	}
	activityUpdate := wshrpc.ActivityUpdate{
		WshCmds: data,
	}
	telemetry.GoUpdateActivityWrap(activityUpdate, "wsh-activity")
	telemetry.GoRecordTEventWrap(&telemetrydata.TEvent{
		Event: "wsh:run",
		Props: props,
	})
	return nil
}

func (ws *WshServer) ActivityCommand(ctx context.Context, activity wshrpc.ActivityUpdate) error {
	telemetry.GoUpdateActivityWrap(activity, "wshrpc-activity")
	return nil
}

func (ws *WshServer) GetVarCommand(ctx context.Context, data wshrpc.CommandVarData) (*wshrpc.CommandVarResponseData, error) {
	_, fileData, err := filestore.WFS.ReadFile(ctx, data.ZoneId, data.FileName)
	if err == fs.ErrNotExist {
		return &wshrpc.CommandVarResponseData{Key: data.Key, Exists: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error reading blockfile: %w", err)
	}
	envMap := envutil.EnvToMap(string(fileData))
	value, ok := envMap[data.Key]
	return &wshrpc.CommandVarResponseData{Key: data.Key, Exists: ok, Val: value}, nil
}

func (ws *WshServer) GetAllVarsCommand(ctx context.Context, data wshrpc.CommandVarData) ([]wshrpc.CommandVarResponseData, error) {
	_, fileData, err := filestore.WFS.ReadFile(ctx, data.ZoneId, data.FileName)
	if err == fs.ErrNotExist {
		return []wshrpc.CommandVarResponseData{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error reading blockfile: %w", err)
	}
	envMap := envutil.EnvToMap(string(fileData))
	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]wshrpc.CommandVarResponseData, 0, len(keys))
	for _, k := range keys {
		result = append(result, wshrpc.CommandVarResponseData{
			Key:    k,
			Val:    envMap[k],
			Exists: true,
		})
	}
	return result, nil
}

func (ws *WshServer) SetVarCommand(ctx context.Context, data wshrpc.CommandVarData) error {
	_, fileData, err := filestore.WFS.ReadFile(ctx, data.ZoneId, data.FileName)
	if err == fs.ErrNotExist {
		fileData = []byte{}
		err = filestore.WFS.MakeFile(ctx, data.ZoneId, data.FileName, nil, wshrpc.FileOpts{})
		if err != nil {
			return fmt.Errorf("error creating blockfile: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("error reading blockfile: %w", err)
	}
	envMap := envutil.EnvToMap(string(fileData))
	if data.Remove {
		delete(envMap, data.Key)
	} else {
		envMap[data.Key] = data.Val
	}
	envStr := envutil.MapToEnv(envMap)
	return filestore.WFS.WriteFile(ctx, data.ZoneId, data.FileName, []byte(envStr))
}

func (ws *WshServer) PathCommand(ctx context.Context, data wshrpc.PathCommandData) (string, error) {
	pathType := data.PathType
	openInternal := data.Open
	openExternal := data.OpenExternal
	var path string
	switch pathType {
	case "config":
		path = wavebase.GetWaveConfigDir()
	case "data":
		path = wavebase.GetWaveDataDir()
	case "log":
		path = filepath.Join(wavebase.GetWaveDataDir(), "waveapp.log")
	}

	if openInternal && openExternal {
		return "", fmt.Errorf("open and openExternal cannot both be true")
	}

	if openInternal {
		_, err := ws.CreateBlockCommand(ctx, wshrpc.CommandCreateBlockData{
			TabId: data.TabId,
			BlockDef: &waveobj.BlockDef{Meta: map[string]any{
				waveobj.MetaKey_View: "preview",
				waveobj.MetaKey_File: path,
			}},
			Ephemeral: true,
			Focused:   true,
		})

		if err != nil {
			return path, fmt.Errorf("error opening path: %w", err)
		}
	} else if openExternal {
		err := open.Run(path)
		if err != nil {
			return path, fmt.Errorf("error opening path: %w", err)
		}
	}
	return path, nil
}

func (ws *WshServer) FetchSuggestionsCommand(ctx context.Context, data wshrpc.FetchSuggestionsData) (*wshrpc.FetchSuggestionsResponse, error) {
	return suggestion.FetchSuggestions(ctx, data)
}

func (ws *WshServer) DisposeSuggestionsCommand(ctx context.Context, widgetId string) error {
	suggestion.DisposeSuggestions(ctx, widgetId)
	return nil
}

func (ws *WshServer) GetTabCommand(ctx context.Context, tabId string) (*waveobj.Tab, error) {
	tab, err := wstore.DBGet[*waveobj.Tab](ctx, tabId)
	if err != nil {
		return nil, fmt.Errorf("error getting tab: %w", err)
	}
	return tab, nil
}

func (ws *WshServer) GetAllBadgesCommand(ctx context.Context) ([]baseds.BadgeEvent, error) {
	return wcore.GetAllBadges(), nil
}

func (ws *WshServer) GetSecretsCommand(ctx context.Context, names []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, name := range names {
		value, exists, err := secretstore.GetSecret(name)
		if err != nil {
			return nil, fmt.Errorf("error getting secret %q: %w", name, err)
		}
		if exists {
			result[name] = value
		}
	}
	return result, nil
}

func (ws *WshServer) GetSecretsNamesCommand(ctx context.Context) ([]string, error) {
	names, err := secretstore.GetSecretNames()
	if err != nil {
		return nil, fmt.Errorf("error getting secret names: %w", err)
	}
	return names, nil
}

func (ws *WshServer) SetSecretsCommand(ctx context.Context, secrets map[string]*string) error {
	for name, value := range secrets {
		if value == nil {
			err := secretstore.DeleteSecret(name)
			if err != nil {
				return fmt.Errorf("error deleting secret %q: %w", name, err)
			}
		} else {
			err := secretstore.SetSecret(name, *value)
			if err != nil {
				return fmt.Errorf("error setting secret %q: %w", name, err)
			}
		}
	}
	return nil
}

func (ws *WshServer) GetSecretsLinuxStorageBackendCommand(ctx context.Context) (string, error) {
	backend, err := secretstore.GetLinuxStorageBackend()
	if err != nil {
		return "", fmt.Errorf("error getting linux storage backend: %w", err)
	}
	return backend, nil
}

func (ws *WshServer) JobCmdExitedCommand(ctx context.Context, data wshrpc.CommandJobCmdExitedData) error {
	return jobcontroller.HandleCmdJobExited(ctx, data.JobId, data)
}

func (ws *WshServer) JobControllerListCommand(ctx context.Context) ([]*waveobj.Job, error) {
	return wstore.DBGetAllObjsByType[*waveobj.Job](ctx, waveobj.OType_Job)
}

func (ws *WshServer) JobControllerDeleteJobCommand(ctx context.Context, jobId string) error {
	return jobcontroller.DeleteJob(ctx, jobId)
}

func (ws *WshServer) JobControllerStartJobCommand(ctx context.Context, data wshrpc.CommandJobControllerStartJobData) (string, error) {
	params := jobcontroller.StartJobParams{
		ConnName: data.ConnName,
		JobKind:  data.JobKind,
		Cmd:      data.Cmd,
		Args:     data.Args,
		Env:      data.Env,
		TermSize: data.TermSize,
	}
	return jobcontroller.StartJob(ctx, params)
}

func (ws *WshServer) JobControllerExitJobCommand(ctx context.Context, jobId string) error {
	return jobcontroller.TerminateJobManager(ctx, jobId)
}

func (ws *WshServer) JobControllerDisconnectJobCommand(ctx context.Context, jobId string) error {
	return jobcontroller.DisconnectJob(ctx, jobId)
}

func (ws *WshServer) JobControllerReconnectJobCommand(ctx context.Context, jobId string) error {
	return jobcontroller.ReconnectJob(ctx, jobId, nil)
}

func (ws *WshServer) JobControllerReconnectJobsForConnCommand(ctx context.Context, connName string) error {
	return jobcontroller.ReconnectJobsForConn(ctx, connName)
}

func (ws *WshServer) JobControllerConnectedJobsCommand(ctx context.Context) ([]string, error) {
	return jobcontroller.GetConnectedJobIds(), nil
}

func (ws *WshServer) JobControllerAttachJobCommand(ctx context.Context, data wshrpc.CommandJobControllerAttachJobData) error {
	return jobcontroller.AttachJobToBlock(ctx, data.JobId, data.BlockId)
}

func (ws *WshServer) JobControllerDetachJobCommand(ctx context.Context, jobId string) error {
	return jobcontroller.DetachJobFromBlock(ctx, jobId, true)
}

func (ws *WshServer) BlockJobStatusCommand(ctx context.Context, blockId string) (*wshrpc.BlockJobStatusData, error) {
	return jobcontroller.GetBlockJobStatus(ctx, blockId)
}

func (ws *WshServer) TeamCreateMemberCommand(ctx context.Context, data wshrpc.TeamCreateMemberData) (*wshrpc.TeamMember, error) {
	m := convertRpcMemberToDb(&data)
	err := team.CreateMember(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("error creating team member: %w", err)
	}
	team.PublishMemberUpdate()
	return convertDbMemberToRpc(m), nil
}

func (ws *WshServer) TeamGetMemberCommand(ctx context.Context, memberId string) (*wshrpc.TeamMember, error) {
	m, err := team.GetMember(ctx, memberId)
	if err != nil {
		return nil, fmt.Errorf("error getting team member: %w", err)
	}
	return convertDbMemberToRpc(m), nil
}

func (ws *WshServer) TeamUpdateMemberCommand(ctx context.Context, data wshrpc.TeamUpdateMemberData) (*wshrpc.TeamMember, error) {
	m, err := team.GetMember(ctx, data.MemberID)
	if err != nil {
		return nil, fmt.Errorf("error getting team member: %w", err)
	}
	applyMemberUpdate(m, &data)
	err = team.UpdateMember(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("error updating team member: %w", err)
	}
	team.PublishMemberUpdate()
	return convertDbMemberToRpc(m), nil
}

func (ws *WshServer) TeamDeleteMemberCommand(ctx context.Context, memberId string) error {
	err := team.DeleteMember(ctx, memberId)
	if err != nil {
		return fmt.Errorf("error deleting team member: %w", err)
	}
	team.PublishMemberUpdate()
	return nil
}

func (ws *WshServer) TeamListMembersCommand(ctx context.Context) ([]*wshrpc.TeamMember, error) {
	members, err := team.ListMembers(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing team members: %w", err)
	}
	var result []*wshrpc.TeamMember
	for _, m := range members {
		result = append(result, convertDbMemberToRpc(m))
	}
	return result, nil
}

func (ws *WshServer) TeamForkWorkerCommand(ctx context.Context, memberId string) (*wshrpc.TeamWorker, error) {
	w, err := team.ForkWorker(ctx, memberId)
	if err != nil {
		return nil, fmt.Errorf("error forking worker: %w", err)
	}
	team.PublishWorkerUpdate()
	return convertDbWorkerToRpc(w), nil
}

func (ws *WshServer) TeamGetWorkerCommand(ctx context.Context, workerId string) (*wshrpc.TeamWorker, error) {
	w, err := team.GetWorker(ctx, workerId)
	if err != nil {
		return nil, fmt.Errorf("error getting team worker: %w", err)
	}
	return convertDbWorkerToRpc(w), nil
}

func (ws *WshServer) TeamUpdateWorkerCommand(ctx context.Context, data wshrpc.TeamUpdateWorkerData) (*wshrpc.TeamWorker, error) {
	w, err := team.GetWorker(ctx, data.WorkerID)
	if err != nil {
		return nil, fmt.Errorf("error getting team worker: %w", err)
	}
	if data.Status != "" {
		if err := team.ValidateWorkerTransition(w.Status, data.Status); err != nil {
			return nil, err
		}
		w.Status = data.Status
	}
	if data.Name != "" {
		w.Name = data.Name
	}
	if data.AssignedTaskID != "" {
		w.AssignedTaskID = data.AssignedTaskID
	}
	if data.BlockID != "" {
		w.BlockID = data.BlockID
	}
	if data.TabID != "" {
		w.TabID = data.TabID
	}
	if data.PID > 0 {
		w.PID = data.PID
	}
	w.ProjectID = data.ProjectID
	err = team.UpdateWorker(ctx, w)
	if err != nil {
		return nil, fmt.Errorf("error updating team worker: %w", err)
	}
	team.PublishWorkerUpdate()
	return convertDbWorkerToRpc(w), nil
}

func (ws *WshServer) TeamDeleteWorkerCommand(ctx context.Context, workerId string) error {
	err := team.DeleteWorker(ctx, workerId)
	if err != nil {
		return fmt.Errorf("error deleting team worker: %w", err)
	}
	team.PublishWorkerUpdate()
	return nil
}

func (ws *WshServer) TeamListWorkersCommand(ctx context.Context, memberId string) ([]*wshrpc.TeamWorker, error) {
	workers, err := team.ListWorkers(ctx, memberId)
	if err != nil {
		return nil, fmt.Errorf("error listing team workers: %w", err)
	}
	var result []*wshrpc.TeamWorker
	for _, w := range workers {
		result = append(result, convertDbWorkerToRpc(w))
	}
	return result, nil
}

func (ws *WshServer) TeamRecycleWorkerCommand(ctx context.Context, workerId string) error {
	err := team.RecycleWorker(ctx, workerId)
	if err != nil {
		return fmt.Errorf("error recycling worker: %w", err)
	}
	team.PublishWorkerUpdate()
	return nil
}

func (ws *WshServer) TeamCreateTaskCommand(ctx context.Context, data wshrpc.TeamCreateTaskData) (*wshrpc.TeamTask, error) {
	t := &team.TeamTask{
		Title:       data.Title,
		Description: data.Description,
		Priority:    data.Priority,
		DependsOn:   data.DependsOn,
	}
	err := team.CreateTask(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("error creating team task: %w", err)
	}
	team.PublishTaskUpdate()
	return convertDbTaskToRpc(t), nil
}

func (ws *WshServer) TeamGetTaskCommand(ctx context.Context, taskId string) (*wshrpc.TeamTask, error) {
	t, err := team.GetTask(ctx, taskId)
	if err != nil {
		return nil, fmt.Errorf("error getting team task: %w", err)
	}
	return convertDbTaskToRpc(t), nil
}

func (ws *WshServer) TeamUpdateTaskCommand(ctx context.Context, data wshrpc.TeamUpdateTaskData) (*wshrpc.TeamTask, error) {
	t, err := team.GetTask(ctx, data.TaskID)
	if err != nil {
		return nil, fmt.Errorf("error getting team task: %w", err)
	}
	if data.Title != "" {
		t.Title = data.Title
	}
	if data.Description != "" {
		t.Description = data.Description
	}
	if data.Priority != "" {
		t.Priority = data.Priority
	}
	if data.Status != "" {
		if err := team.ValidateTaskTransition(t.Status, data.Status); err != nil {
			return nil, err
		}
		t.Status = data.Status
		if data.Status == team.TaskStatusDone || data.Status == team.TaskStatusFailed {
			t.CompletedAt = time.Now().Unix()
		}
	}
	if data.AssignedMemberID != "" {
		t.AssignedMemberID = data.AssignedMemberID
	}
	if data.AssignedWorkerID != "" {
		t.AssignedWorkerID = data.AssignedWorkerID
	}
	if data.Result != "" {
		t.Result = data.Result
	}
	if data.Error != "" {
		t.Error = data.Error
	}
	if data.Progress > 0 {
		t.Progress = data.Progress
	}
	err = team.UpdateTask(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("error updating team task: %w", err)
	}
	team.PublishTaskUpdate()
	return convertDbTaskToRpc(t), nil
}

func (ws *WshServer) TeamDeleteTaskCommand(ctx context.Context, taskId string) error {
	err := team.DeleteTask(ctx, taskId)
	if err != nil {
		return fmt.Errorf("error deleting team task: %w", err)
	}
	team.PublishTaskUpdate()
	return nil
}

func (ws *WshServer) TeamListTasksCommand(ctx context.Context, data wshrpc.TeamListTasksData) ([]*wshrpc.TeamTask, error) {
	tasks, err := team.ListTasks(ctx, data.Status, data.Priority, data.MemberID)
	if err != nil {
		return nil, fmt.Errorf("error listing team tasks: %w", err)
	}
	var result []*wshrpc.TeamTask
	for _, t := range tasks {
		result = append(result, convertDbTaskToRpc(t))
	}
	return result, nil
}

func (ws *WshServer) TeamExecuteTaskCommand(ctx context.Context, data wshrpc.TeamExecuteTaskData) (*wshrpc.TeamExecuteTaskResponse, error) {
	worker, err := team.GetWorker(ctx, data.WorkerID)
	if err != nil {
		return nil, fmt.Errorf("error getting worker: %w", err)
	}

	task, err := team.GetTask(ctx, data.TaskID)
	if err != nil {
		return nil, fmt.Errorf("error getting task: %w", err)
	}

	var cmd string
	member, _ := team.GetMember(ctx, worker.MemberID)
	if data.Command != "" {
		cmd = data.Command
	} else {
		tool := team.ToolClaude
		if member != nil && member.Tool != "" {
			tool = member.Tool
		}
		cmd = getWorkerStartCommand(tool)
	}

	var projectPath string
	if worker.ProjectID != "" {
		proj, projErr := team.GetProject(ctx, worker.ProjectID)
		if projErr == nil && proj.Path != "" {
			projectPath = proj.Path
		}
	}

	if member != nil {
		if injectErr := team.InjectWorkerConfig(worker, member); injectErr != nil {
			log.Printf("warning: failed to inject worker config for %s: %v\n", worker.Name, injectErr)
		}
	}

	blockDef := &waveobj.BlockDef{
		Meta: waveobj.MetaMapType{
			"view":       "term",
			"controller": "shell",
			"term:title": fmt.Sprintf("%s — %s", worker.Name, task.Title),
		},
	}

	tabId := worker.TabID
	if tabId == "" {
		client, clientErr := wstore.DBGetSingleton[*waveobj.Client](ctx)
		if clientErr == nil && len(client.WindowIds) > 0 {
			window, windowErr := wstore.DBMustGet[*waveobj.Window](ctx, client.WindowIds[0])
			if windowErr == nil {
				workspace, wsErr := wstore.DBMustGet[*waveobj.Workspace](ctx, window.WorkspaceId)
				if wsErr == nil {
					tabId = workspace.ActiveTabId
				}
			}
		}
		if tabId == "" {
			return nil, fmt.Errorf("worker %s has no tab assigned and no active tab found", worker.Name)
		}
		worker.TabID = tabId
	}

	blockRef, err := ws.CreateBlockCommand(ctx, wshrpc.CommandCreateBlockData{
		TabId:    tabId,
		BlockDef: blockDef,
		Focused:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating terminal block: %w", err)
	}
	blockId := blockRef.OID

	if projectPath != "" {
		cdInput := &blockcontroller.BlockInputUnion{
			InputData: []byte("cd " + projectPath + "\n"),
		}
		for attempt := 1; attempt <= 5; attempt++ {
			err = blockcontroller.SendInput(blockId, cdInput)
			if err == nil {
				break
			}
			if attempt < 5 {
				time.Sleep(300 * time.Millisecond)
			}
		}
		time.Sleep(300 * time.Millisecond)
	}

	envInput := &blockcontroller.BlockInputUnion{
		InputData: []byte(fmt.Sprintf("export WAVE_WORKER_ID=%s WAVE_TASK_ID=%s WAVE_TEAM_MCP=1\n", worker.WorkerID, task.TaskID)),
	}
	for attempt := 1; attempt <= 3; attempt++ {
		err = blockcontroller.SendInput(blockId, envInput)
		if err == nil {
			break
		}
		if attempt < 3 {
			time.Sleep(200 * time.Millisecond)
		}
	}
	time.Sleep(200 * time.Millisecond)

	inputUnion := &blockcontroller.BlockInputUnion{
		InputData: []byte(cmd + "\n"),
	}
	for attempt := 1; attempt <= 10; attempt++ {
		err = blockcontroller.SendInput(blockId, inputUnion)
		if err == nil {
			break
		}
		if attempt == 10 {
			fmt.Printf("warning: failed to send command to worker terminal after %d attempts: %v\n", attempt, err)
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if task.Description != "" {
		go func() {
			time.Sleep(5 * time.Second)
			taskText := &blockcontroller.BlockInputUnion{
				InputData: []byte(task.Description),
			}
			blockcontroller.SendInput(blockId, taskText)
			time.Sleep(200 * time.Millisecond)
			taskEnter := &blockcontroller.BlockInputUnion{
				InputData: []byte("\r"),
			}
			blockcontroller.SendInput(blockId, taskEnter)
		}()
	}

	if err := team.ValidateWorkerTransition(worker.Status, team.WorkerStatusWorking); err != nil {
		return nil, fmt.Errorf("cannot set worker to working: %w", err)
	}
	worker.Status = team.WorkerStatusWorking
	worker.AssignedTaskID = task.TaskID
	worker.BlockID = blockId
	err = team.UpdateWorker(ctx, worker)
	if err != nil {
		return nil, fmt.Errorf("error updating worker: %w", err)
	}

	if err := team.ValidateTaskTransition(task.Status, team.TaskStatusWorking); err != nil {
		return nil, fmt.Errorf("cannot set task to working: %w", err)
	}
	task.Status = team.TaskStatusWorking
	err = team.UpdateTask(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("error updating task: %w", err)
	}

	team.PublishWorkerUpdate()
	team.PublishTaskUpdate()

	return &wshrpc.TeamExecuteTaskResponse{
		BlockID: blockId,
		TabID:   worker.TabID,
		Success: true,
	}, nil
}

func (ws *WshServer) TeamPauseTaskCommand(ctx context.Context, taskId string) error {
	task, err := team.GetTask(ctx, taskId)
	if err != nil {
		return fmt.Errorf("error getting team task: %w", err)
	}
	if err := team.ValidateTaskTransition(task.Status, team.TaskStatusPaused); err != nil {
		return err
	}
	task.Status = team.TaskStatusPaused
	return team.UpdateTask(ctx, task)
}

func (ws *WshServer) TeamResumeTaskCommand(ctx context.Context, taskId string) error {
	task, err := team.GetTask(ctx, taskId)
	if err != nil {
		return fmt.Errorf("error getting team task: %w", err)
	}
	if err := team.ValidateTaskTransition(task.Status, team.TaskStatusPending); err != nil {
		return err
	}
	task.Status = team.TaskStatusPending
	return team.UpdateTask(ctx, task)
}

func (ws *WshServer) TeamRetryTaskCommand(ctx context.Context, taskId string) error {
	task, err := team.GetTask(ctx, taskId)
	if err != nil {
		return fmt.Errorf("error getting team task: %w", err)
	}
	if task.Status != team.TaskStatusFailed {
		return fmt.Errorf("can only retry failed tasks, current status: %s", task.Status)
	}
	if task.RetryCount >= task.MaxRetries {
		return fmt.Errorf("task has exceeded max retries (%d/%d)", task.RetryCount, task.MaxRetries)
	}
	task.RetryCount++
	delaySeconds := math.Pow(2, float64(task.RetryCount))
	task.NextRetryAt = time.Now().Add(time.Duration(delaySeconds) * time.Second).Unix()
	task.Status = team.TaskStatusPending
	task.Error = ""
	return team.UpdateTask(ctx, task)
}

func (ws *WshServer) TeamGetTaskOutputHistoryCommand(ctx context.Context, taskId string) ([]wshrpc.TeamTaskOutput, error) {
	task, err := team.GetTask(ctx, taskId)
	if err != nil {
		return nil, fmt.Errorf("error getting team task: %w", err)
	}
	var result []wshrpc.TeamTaskOutput
	for _, o := range task.OutputHistory {
		result = append(result, wshrpc.TeamTaskOutput{
			Timestamp: o.Timestamp,
			Type:      o.Type,
			Content:   o.Content,
		})
	}
	return result, nil
}

func (ws *WshServer) TeamGetStatusCommand(ctx context.Context) (*wshrpc.TeamStatusData, error) {
	status, err := team.GetStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting team status: %w", err)
	}
	return (*wshrpc.TeamStatusData)(status), nil
}

func (ws *WshServer) TeamDetectRuntimesCommand(ctx context.Context) (*wshrpc.TeamDetectRuntimesReturn, error) {
	providers := []struct {
		name        string
		displayName string
		command     string
		flag        string
	}{
		{"claude", "Claude Code", "claude", "--version"},
		{"opencode", "OpenCode", "opencode", "--version"},
		{"cursor", "Cursor Agent", "cursor-agent", "--version"},
		{"aider", "Aider", "aider", "--version"},
	}

	var runtimes []wshrpc.AIRuntime
	for _, p := range providers {
		cmd := exec.Command(p.command, p.flag)
		output, err := cmd.CombinedOutput()
		if err != nil {
			runtimes = append(runtimes, wshrpc.AIRuntime{
				Name: p.name, DisplayName: p.displayName,
				Command: p.command, Version: "", Status: "offline",
			})
			continue
		}
		version := strings.TrimSpace(string(output))
		if idx := strings.Index(version, "\n"); idx != -1 {
			version = version[:idx]
		}
		runtimes = append(runtimes, wshrpc.AIRuntime{
			Name: p.name, DisplayName: p.displayName,
			Command: p.command, Version: version, Status: "online",
		})
	}

	return &wshrpc.TeamDetectRuntimesReturn{Runtimes: runtimes}, nil
}

func (ws *WshServer) TeamAddActivityCommand(ctx context.Context, data wshrpc.TeamAddActivityData) error {
	a := &team.TeamActivity{
		TaskID:      data.TaskID,
		WorkerID:    data.WorkerID,
		MemberID:    data.MemberID,
		Type:        data.Type,
		Description: data.Description,
		Meta:        data.Meta,
	}
	err := team.AddActivity(ctx, a)
	if err != nil {
		return fmt.Errorf("error adding team activity: %w", err)
	}
	return nil
}

func (ws *WshServer) TeamListActivityCommand(ctx context.Context, data wshrpc.TeamListActivityData) ([]*wshrpc.TeamActivity, error) {
	activities, err := team.ListActivities(ctx, data.Limit, data.TaskID, data.WorkerID, data.MemberID)
	if err != nil {
		return nil, fmt.Errorf("error listing team activities: %w", err)
	}
	var result []*wshrpc.TeamActivity
	for _, a := range activities {
		result = append(result, convertDbActivityToRpc(a))
	}
	return result, nil
}

func (ws *WshServer) TeamCreateProjectCommand(ctx context.Context, data wshrpc.TeamCreateProjectData) (*wshrpc.TeamProject, error) {
	p := convertRpcProjectToDb(&data)
	err := team.CreateProject(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("error creating team project: %w", err)
	}
	team.PublishProjectUpdate()
	return convertDbProjectToRpc(p), nil
}

func (ws *WshServer) TeamGetProjectCommand(ctx context.Context, projectId string) (*wshrpc.TeamProject, error) {
	p, err := team.GetProject(ctx, projectId)
	if err != nil {
		return nil, fmt.Errorf("error getting team project: %w", err)
	}
	return convertDbProjectToRpc(p), nil
}

func (ws *WshServer) TeamUpdateProjectCommand(ctx context.Context, data wshrpc.TeamUpdateProjectData) (*wshrpc.TeamProject, error) {
	p, err := team.GetProject(ctx, data.ProjectId)
	if err != nil {
		return nil, fmt.Errorf("error getting team project: %w", err)
	}
	if data.Name != "" {
		p.Name = data.Name
	}
	if data.Path != "" {
		p.Path = data.Path
	}
	if data.Spec != "" {
		p.Spec = data.Spec
	}
	err = team.UpdateProject(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("error updating team project: %w", err)
	}
	team.PublishProjectUpdate()
	return convertDbProjectToRpc(p), nil
}

func (ws *WshServer) TeamDeleteProjectCommand(ctx context.Context, projectId string) error {
	err := team.DeleteProject(ctx, projectId)
	if err != nil {
		return fmt.Errorf("error deleting team project: %w", err)
	}
	team.PublishProjectUpdate()
	return nil
}

func (ws *WshServer) TeamListProjectsCommand(ctx context.Context) ([]*wshrpc.TeamProject, error) {
	projects, err := team.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing team projects: %w", err)
	}
	var result []*wshrpc.TeamProject
	for _, p := range projects {
		result = append(result, convertDbProjectToRpc(p))
	}
	return result, nil
}

func (ws *WshServer) TeamListTemplatesCommand(ctx context.Context) ([]*wshrpc.TeamMember, error) {
	defaults, err := team.LoadDefaultTemplates()
	if err != nil {
		return nil, fmt.Errorf("error loading default templates: %w", err)
	}
	global, err := team.LoadGlobalTemplates()
	if err != nil {
		return nil, fmt.Errorf("error loading global templates: %w", err)
	}
	seen := make(map[string]bool)
	var result []*wshrpc.TeamMember
	for i := range defaults {
		if !seen[defaults[i].Name] {
			seen[defaults[i].Name] = true
			result = append(result, convertDbMemberToRpc(&defaults[i]))
		}
	}
	for i := range global {
		if !seen[global[i].Name] {
			seen[global[i].Name] = true
			result = append(result, convertDbMemberToRpc(&global[i]))
		}
	}
	return result, nil
}

func (ws *WshServer) TeamSaveTemplateCommand(ctx context.Context, member *wshrpc.TeamMember) error {
	m, err := rpcMemberToDb(member)
	if err != nil {
		return fmt.Errorf("error converting template: %w", err)
	}
	err = team.SaveGlobalTemplate(m)
	if err != nil {
		return fmt.Errorf("error saving template: %w", err)
	}
	return nil
}

func (ws *WshServer) TeamDeleteTemplateCommand(ctx context.Context, templateName string) error {
	err := team.DeleteGlobalTemplate(templateName)
	if err != nil {
		return fmt.Errorf("error deleting template: %w", err)
	}
	return nil
}

// --- Conversion helpers between pkg/team and wshrpc types ---

func getWorkerStartCommand(tool string) string {
	switch tool {
	case "claude":
		return "claude"
	case "opencode":
		return "opencode"
	case "cursor":
		return "cursor-agent"
	case "aider":
		return "aider"
	default:
		return tool
	}
}

func convertRpcMemberToDb(data *wshrpc.TeamCreateMemberData) *team.TeamMember {
	return &team.TeamMember{
		Name:           data.Name,
		Tool:           data.Tool,
		CustomCmd:      data.CustomCmd,
		Description:    data.Description,
		Persona:        data.Persona,
		PersonaPath:    data.PersonaPath,
		Skills:         data.Skills,
		McpServers:     convertRpcMcpServers(data.McpServers),
		Capabilities:   data.Capabilities,
		Model:          data.Model,
		MaxConcurrency: data.MaxConcurrency,
		MaxRetries:     data.MaxRetries,
		Memory:         data.Memory,
		Color:          data.Color,
		ProjectID:      data.ProjectID,
	}
}

func applyMemberUpdate(m *team.TeamMember, data *wshrpc.TeamUpdateMemberData) {
	if data.Name != "" {
		m.Name = data.Name
	}
	if data.Tool != "" {
		m.Tool = data.Tool
	}
	if data.CustomCmd != "" {
		m.CustomCmd = data.CustomCmd
	}
	if data.Description != "" {
		m.Description = data.Description
	}
	if data.Persona != "" {
		m.Persona = data.Persona
	}
	if data.PersonaPath != "" {
		m.PersonaPath = data.PersonaPath
	}
	if len(data.Skills) > 0 {
		m.Skills = data.Skills
	}
	if len(data.McpServers) > 0 {
		m.McpServers = convertRpcMcpServers(data.McpServers)
	}
	if len(data.Capabilities) > 0 {
		m.Capabilities = data.Capabilities
	}
	if data.Model != "" {
		m.Model = data.Model
	}
	if data.MaxConcurrency > 0 {
		m.MaxConcurrency = data.MaxConcurrency
	}
	if data.MaxRetries > 0 {
		m.MaxRetries = data.MaxRetries
	}
	if data.Memory != "" {
		m.Memory = data.Memory
	}
	if data.Color != "" {
		m.Color = data.Color
	}
}

func convertRpcMcpServers(servers []wshrpc.TeamMCPConfig) []team.MCPConfig {
	var result []team.MCPConfig
	for _, s := range servers {
		result = append(result, team.MCPConfig{
			Name: s.Name, Type: s.Type,
			Command: s.Command, Args: s.Args,
			Env: s.Env, URL: s.URL, Headers: s.Headers,
		})
	}
	return result
}

func convertDbMemberToRpc(m *team.TeamMember) *wshrpc.TeamMember {
	return &wshrpc.TeamMember{
		MemberID:       m.MemberID,
		Name:           m.Name,
		Tool:           m.Tool,
		CustomCmd:      m.CustomCmd,
		Description:    m.Description,
		Persona:        m.Persona,
		PersonaPath:    m.PersonaPath,
		Skills:         m.Skills,
		McpServers:     convertDbMcpServers(m.McpServers),
		Capabilities:   m.Capabilities,
		Model:          m.Model,
		MaxConcurrency: m.MaxConcurrency,
		MaxRetries:     m.MaxRetries,
		Memory:         m.Memory,
		Color:          m.Color,
		ProjectID:      m.ProjectID,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func rpcMemberToDb(m *wshrpc.TeamMember) (*team.TeamMember, error) {
	var mcpservers []team.MCPConfig
	for _, s := range m.McpServers {
		mcpservers = append(mcpservers, team.MCPConfig{
			Name: s.Name, Type: s.Type,
			Command: s.Command, Args: s.Args,
			Env: s.Env, URL: s.URL, Headers: s.Headers,
		})
	}
	return &team.TeamMember{
		Name:           m.Name,
		Tool:           m.Tool,
		CustomCmd:      m.CustomCmd,
		Description:    m.Description,
		Persona:        m.Persona,
		PersonaPath:    m.PersonaPath,
		Skills:         m.Skills,
		McpServers:     mcpservers,
		Capabilities:   m.Capabilities,
		Model:          m.Model,
		MaxConcurrency: m.MaxConcurrency,
		MaxRetries:     m.MaxRetries,
		Memory:         m.Memory,
		Color:          m.Color,
		ProjectID:      m.ProjectID,
	}, nil
}

func convertDbMcpServers(servers []team.MCPConfig) []wshrpc.TeamMCPConfig {
	var result []wshrpc.TeamMCPConfig
	for _, s := range servers {
		result = append(result, wshrpc.TeamMCPConfig{
			Name: s.Name, Type: s.Type,
			Command: s.Command, Args: s.Args,
			Env: s.Env, URL: s.URL, Headers: s.Headers,
		})
	}
	return result
}

func convertDbWorkerToRpc(w *team.TeamWorker) *wshrpc.TeamWorker {
	return &wshrpc.TeamWorker{
		WorkerID:       w.WorkerID,
		MemberID:       w.MemberID,
		Name:           w.Name,
		Status:         w.Status,
		AssignedTaskID: w.AssignedTaskID,
		BlockID:        w.BlockID,
		TabID:          w.TabID,
		PID:            w.PID,
		ProjectID:      w.ProjectID,
		CreatedAt:      w.CreatedAt,
		UpdatedAt:      w.UpdatedAt,
		LastHeartbeat:  w.LastHeartbeat,
	}
}

func convertDbTaskToRpc(t *team.TeamTask) *wshrpc.TeamTask {
	var outputHistory []wshrpc.TeamTaskOutput
	for _, o := range t.OutputHistory {
		outputHistory = append(outputHistory, wshrpc.TeamTaskOutput{
			Timestamp: o.Timestamp, Type: o.Type, Content: o.Content,
		})
	}
	return &wshrpc.TeamTask{
		TaskID:           t.TaskID,
		Title:            t.Title,
		Description:      t.Description,
		Priority:         t.Priority,
		Status:           t.Status,
		AssignedMemberID: t.AssignedMemberID,
		AssignedWorkerID: t.AssignedWorkerID,
		DependsOn:        t.DependsOn,
		Result:           t.Result,
		Error:            t.Error,
		OutputHistory:    outputHistory,
		Progress:         t.Progress,
		RetryCount:       t.RetryCount,
		MaxRetries:       t.MaxRetries,
		NextRetryAt:      t.NextRetryAt,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
		CompletedAt:      t.CompletedAt,
	}
}

func convertDbActivityToRpc(a *team.TeamActivity) *wshrpc.TeamActivity {
	return &wshrpc.TeamActivity{
		Id:          a.Id,
		TaskID:      a.TaskID,
		WorkerID:    a.WorkerID,
		MemberID:    a.MemberID,
		Type:        a.Type,
		Description: a.Description,
		Meta:        a.Meta,
		CreatedAt:   a.CreatedAt,
	}
}

func convertRpcProjectToDb(data *wshrpc.TeamCreateProjectData) *team.TeamProject {
	return &team.TeamProject{
		Name: data.Name,
		Path: data.Path,
		Spec: data.Spec,
	}
}

func convertDbProjectToRpc(p *team.TeamProject) *wshrpc.TeamProject {
	return &wshrpc.TeamProject{
		ProjectID: p.ProjectID,
		Name:      p.Name,
		Path:      p.Path,
		Spec:      p.Spec,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

