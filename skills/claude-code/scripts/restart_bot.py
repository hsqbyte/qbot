#!/usr/bin/env python3
"""restart_bot.py — 跨平台 QQ Bot 重启脚本

被 Bot 自身通过 nohup 调用，在独立进程中完成重启。
用法: python3 restart_bot.py <mode> <project_dir>
"""
import sys
import os
import time
import subprocess
import signal
import platform
import fcntl
from pathlib import Path
from datetime import datetime


def log(msg: str, log_file: Path):
    """打印并写入日志"""
    ts = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    line = f"[{ts}] {msg}"
    print(line, flush=True)
    with open(log_file, "a") as f:
        f.write(line + "\n")


def find_bot_pids() -> list[int]:
    """查找所有 Bot 相关进程的 PID（通过 ps -eo pid,command 避免列变长导致解析故障）"""
    patterns = [
        "task dev",
        "main.go dev",
        "bin/qbot",
        "go-build",  # 匹配任何 go run 产生的临时二进制
    ]
    pids = set()

    try:
        # ps -eo pid,command 在 macOS 和大部分 Linux 都支持
        result = subprocess.run(["ps", "-eo", "pid,command"], capture_output=True, text=True, timeout=5)
        if result.returncode == 0:
            lines = result.stdout.strip().split("\n")
            for line in lines[1:]: # 第 0 行是 PID COMMAND 表头
                line = line.strip()
                if not line:
                    continue
                
                parts = line.split(maxsplit=1)
                if len(parts) < 2:
                    continue
                    
                pid_str, cmd_str = parts

                if not pid_str.isdigit():
                    continue

                for pattern in patterns:
                    if pattern in cmd_str:
                        pid = int(pid_str)
                        # 不要杀自己或者 ps 进程
                        if pid != os.getpid() and pid != os.getppid() and "ps -eo" not in cmd_str:
                            pids.add(pid)
                        break
    except Exception as e:
        log(f"⚠️ 获取进程列表时异常: {e}", Path("logs/restart.log"))

    return list(pids)


def kill_pids(pids: list[int], force: bool = False):
    """杀掉指定的进程列表"""
    sig = signal.SIGKILL if force else signal.SIGTERM
    for pid in pids:
        try:
            os.kill(pid, sig)
        except (ProcessLookupError, PermissionError):
            pass


def acquire_lock(lock_file: Path) -> bool:
    """尝试获取文件锁，防止多个重启脚本同时运行"""
    # 检查锁文件是否存在且未过期（30秒内）
    if lock_file.exists():
        try:
            age = time.time() - lock_file.stat().st_mtime
            if age < 30:
                return False
        except OSError:
            pass

    # 写入当前 PID 作为锁
    lock_file.write_text(str(os.getpid()))
    return True


def release_lock(lock_file: Path):
    """释放文件锁"""
    try:
        lock_file.unlink(missing_ok=True)
    except OSError:
        pass


def start_process(cmd: list[str], cwd: str, log_path: Path) -> int:
    """启动后台进程，返回 PID"""
    with open(log_path, "a") as out:
        proc = subprocess.Popen(
            cmd,
            cwd=cwd,
            stdout=out,
            stderr=out,
            stdin=subprocess.DEVNULL,
            start_new_session=True,  # 完全脱离当前进程组（跨平台的 setsid）
        )
    return proc.pid


def main():
    mode = sys.argv[1] if len(sys.argv) > 1 else "dev"
    project_dir = sys.argv[2] if len(sys.argv) > 2 else os.path.expanduser("~/opt/helwd/qq")

    project = Path(project_dir)
    logs_dir = project / "logs"
    logs_dir.mkdir(exist_ok=True)

    log_file = logs_dir / "restart.log"
    lock_file = logs_dir / ".restart.lock"

    # 1. 获取锁
    if not acquire_lock(lock_file):
        log("⚠️ 重启脚本已在运行中，跳过", log_file)
        return

    try:
        log(f"🔄 开始重启流程 (mode={mode}, dir={project_dir})", log_file)

        # 2. 等待 2 秒让调用方发完消息
        time.sleep(2)

        # 3. 杀掉所有 Bot 进程
        log("🛑 正在停止所有 Bot 进程...", log_file)
        pids = find_bot_pids()
        if pids:
            log(f"   找到进程: {pids}", log_file)
            kill_pids(pids, force=False)
            time.sleep(1)

            # 确认是否都死了
            remaining = find_bot_pids()
            if remaining:
                log(f"⚠️ 强杀残留进程: {remaining}", log_file)
                kill_pids(remaining, force=True)
                time.sleep(1)
        else:
            log("   未找到正在运行的 Bot 进程", log_file)

        # 4. 根据模式重启
        if mode == "prod":
            # 编译
            log("🔨 重新编译...", log_file)
            build = subprocess.run(
                ["go", "build", "-o", "bin/qbot", "main.go"],
                cwd=project_dir, capture_output=True, text=True, timeout=120,
            )
            if build.returncode != 0:
                log(f"❌ 编译失败: {build.stderr}", log_file)
                return

            # 启动 prod
            log("🚀 启动 prod 进程...", log_file)
            new_pid = start_process(
                [str(project / "bin" / "qbot"), "prod"],
                cwd=project_dir,
                log_path=logs_dir / "bot_prod.log",
            )
        else:
            # 找到 task 命令的路径
            task_path = subprocess.run(
                ["which", "task"], capture_output=True, text=True,
            ).stdout.strip()
            if not task_path:
                task_path = "task"

            log(f"🚀 启动 dev 进程 ({task_path} dev)...", log_file)
            new_pid = start_process(
                [task_path, "dev"],
                cwd=project_dir,
                log_path=logs_dir / "bot_dev.log",
            )

        # 5. 验证
        time.sleep(3)
        try:
            os.kill(new_pid, 0)  # 检查进程是否存活
            log(f"✅ 重启成功！新进程 PID={new_pid} (mode={mode})", log_file)
        except ProcessLookupError:
            log(f"❌ 新进程 PID={new_pid} 未存活，重启可能失败", log_file)

    finally:
        release_lock(lock_file)


if __name__ == "__main__":
    main()
