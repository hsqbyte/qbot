---
name: claude-code
description: 使用 Claude Code 完成编程任务或操作电脑。当用户要求写代码、修改文件、修复 bug、创建项目、查看系统信息、打开应用、管理文件、执行命令、Git 操作、截图等一切涉及操作电脑或编程的需求时使用。
compatibility: macOS / Linux, Claude Code CLI 已安装并登录
metadata:
  parameters_schema: '{"type":"object","properties":{"directory":{"type":"string","description":"工作目录的绝对路径，默认: /Users/helwd"},"prompt":{"type":"string","description":"要 Claude Code 执行的任务描述"}},"required":["prompt"]}'
---

# Claude Code

在指定目录下调用 Claude Code CLI 完成编程任务或操作电脑。

## 使用场景

### 编程类
- 写代码、创建文件
- 修复 bug
- 重构代码
- 创建新项目
- 代码审查

### 操作电脑类
- 查看系统信息（磁盘空间、内存、进程等）
- 打开应用程序（浏览器、编辑器等）
- 文件管理（列出、查找、删除文件）
- Git 操作（commit、push、pull 等）
- 截屏、播放声音等系统操作
- 任意 shell 命令

## 执行方式

运行 `scripts/execute.py`，传入 JSON 格式参数：

```bash
python3 scripts/execute.py '{"directory": "/path/to/project", "prompt": "实现一个 HTTP 服务器"}'
```

### 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| directory | 否 | 工作目录的绝对路径，默认为 /Users/helwd |
| prompt | 是 | 要 Claude Code 执行的任务描述 |

### 示例

```bash
# 编程任务
python3 scripts/execute.py '{"directory": "/Users/helwd/projects/myapp", "prompt": "创建一个 main.go 入口文件，实现简单的 HTTP 服务器"}'

# 操作电脑
python3 scripts/execute.py '{"prompt": "查看当前磁盘空间使用情况"}'
python3 scripts/execute.py '{"prompt": "查看 ~/Desktop 下有哪些文件"}'
python3 scripts/execute.py '{"prompt": "打开 Safari 浏览器"}'
python3 scripts/execute.py '{"prompt": "用 git status 查看当前仓库状态"}'
```

## 注意事项

- 执行超时时间为 10 分钟
- 输出超过 3000 字符会被截断
- 确保目录路径正确且存在
- 不提供 directory 时默认使用用户 home 目录
