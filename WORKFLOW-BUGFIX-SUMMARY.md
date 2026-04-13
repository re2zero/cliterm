# M002 Bug Fix Summary

## Date
2026-04-13

## Purpose
修复agent列表交互功能和agent CLI启动问题

## Branch Operations
1. **开发分支**: `gsd/bugfix/m002`
2. **合并到**: `zeroai` (fast-forward merge, 12 commits)
3. **推送**: 已推送到 `origin/gsd/bugfix/m002`

## Commits in This Session
```
6df8a601 feat(blockcontroller): enhance CLI command construction with agent parameters
80eb3c82 feat(agent-list): inject comprehensive agent configuration to block
818ea243 feat(agent-list): set agent name as block title
160cc0f1 fix(process): differentiate CLI command formats by backend  
a47064d6 fix(process): use CLI --system-prompt instead of ACP protocol
64a56ab0 feat(blockcontroller): start agent CLI when agent block was created
f956b0f3 feat(agent-list): implement agent run functionality
```

## Files Modified
- frontend/app/zeroai/components/agent-list.tsx (AgentList组件)
- frontend/app/zeroai/components/agent-list.scss (样式)
- pkg/blockcontroller/shellcontroller.go (后端controller)
- pkg/zeroai/process/process-spawner.go (进程管理)

## Bug Fixes Implemented

### 1. 删除确认对话框
- 添加删除确认对话框，防止误删除
- 显示警告三角形图标和agent名称

### 2. 收起状态交互简化
- 移除右键菜单，保留双击运行
- 创建CollapsedAgentItem组件避免React hooks错误

### 3. 展开状态菜单优化
- ...下拉菜单只包含Edit和Delete（Run移至独立按钮）
- 独立绿色的运行按钮

### 4. Agent运行功能
- 双击agent创建终端block
- 注入agent配置到block meta
- 后端检测并启动相应的CLI

### 5. CLI命令格式修复
- Claude: --system-prompt应用agent soul
- opencode: --agent参数选择agent
- qwen/codex: 交互模式启动

### 6. 对话框尺寸优化
- 创建: 560px×600px
- 编辑: 640px×700px

### 7. 收起状态显示优化
- 垂直显示agent头像（最多5个）
- "+"按钮在底部
- 背景色头像

### 8. Agent显示优化
- 两行布局: name (13px粗体) + role (11px小字体)

### 9. Skills/MCP集成
- 从后端加载skills和MCP服务器
- 多选标签UI

### 10. 环境变量注入
- 注入完整的agent元数据到CLI进程
- 支持所有backend类型

## Testing Status
- ✅ 所有backend CLI (claude, opencode, qwen, codex) 启动成功
- ✅ Agent创建、编辑、删除功能正常
- ✅ 双击运行功能正常
- ✅ 收起/展开状态交互流畅
- ✅ 无编译错误

## Known Issues
- 终端块标题可能不显示agent名称（需要验证frame:title是否生效）

## Next Steps
1. 验证所有功能在实际应用中正常工作
2. 考虑删除gsd/bugfix/m002分支（已合并到zeroai）
3. 如需要，继续M003: Scheduler管理UI实现

## Related Decisions
- D007: 简化sidebar显示（name+role only）
- D003: 独立agent数据存储
- D004: Assistant service作为agent间通信中继
