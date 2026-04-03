#!/usr/bin/env python3
"""在指定目录下使用 Claude Code 完成编程任务。"""
import sys
import json
import subprocess
import os

MAX_OUTPUT = 3000
TIMEOUT = 600  # 10 分钟


def execute(args_json):
    try:
        args = json.loads(args_json)
        directory = args.get("directory", "").strip()
        prompt = args.get("prompt", "").strip()

        # directory 可选，默认使用用户 home 目录
        if not directory:
            directory = os.path.expanduser("~")
        if not prompt:
            print("错误: 未提供任务描述 (prompt)")
            return
        if not os.path.isdir(directory):
            print(f"错误: 目录不存在: {directory}")
            return

        # 调用 claude -p 获取数据流
        process = subprocess.Popen(
            ["claude", "-p", prompt, "--output-format", "stream-json", "--verbose"],
            cwd=directory,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1, # 行缓冲
        )

        final_response = []
        for line in process.stdout:
            line = line.strip()
            if not line:
                continue
            
            try:
                data = json.loads(line)
                if data.get("type") == "assistant" and "message" in data:
                    contents = data["message"].get("content", [])
                    for content in contents:
                        c_type = content.get("type")
                        if c_type == "tool_use":
                            tool_name = content.get("name", "")
                            # 提取 input 的部分关键信息用于展示
                            tool_input = content.get("input", {})
                            cmd_preview = ""
                            if tool_name == "Bash":
                                cmd_preview = tool_input.get("command", "")
                            elif tool_name == "Grep":
                                cmd_preview = f"搜索关键字 [{tool_input.get('pattern', '')}] in [{tool_input.get('glob', '')}]"
                            elif tool_name == "Glob":
                                cmd_preview = f"扫描路径 [{tool_input.get('path', '')}] 下的 [{tool_input.get('pattern', '')}]"
                            elif tool_name == "Read" or tool_name == "View":
                                cmd_preview = tool_input.get("file_path", "")
                            elif tool_name == "Edit" or tool_name == "Write":
                                cmd_preview = f"修改文件: {tool_input.get('file_path', '')}"
                            else:
                                cmd_preview = json.dumps(tool_input, ensure_ascii=False)
                                
                            # 长度放宽到150，减少多余的截断符号显示
                            cmd_preview = cmd_preview[:150] + ("..." if len(cmd_preview) > 150 else "")
                            p_msg = f"[PROGRESS] ⚙️ 调用工具 [{tool_name}] -> {cmd_preview}"
                            print(p_msg, flush=True)
                        elif c_type == "text":
                            txt = content.get("text", "")
                            final_response.append(txt)
                            print(f"[PROGRESS] 💬 {txt[:50]}...", flush=True)
            except json.JSONDecodeError:
                # 兼容偶尔夹杂的非纯JSON输出
                print(line, flush=True)

        try:
            process.wait(timeout=TIMEOUT)
        except subprocess.TimeoutExpired:
            process.kill()
            print("[PROGRESS] 错误: Claude Code 执行超时")

        # 最后打印合并后的答复给调用方获取结果
        print("\n\n" + "".join(final_response).strip(), flush=True)
    except FileNotFoundError:
        print("错误: 未找到 claude 命令，请确认 Claude Code CLI 已安装")
    except Exception as e:
        print(f"错误: {e}")


if __name__ == "__main__":
    args_json = sys.stdin.read().strip()
    if not args_json:
        sys.exit(1)
    execute(args_json)
