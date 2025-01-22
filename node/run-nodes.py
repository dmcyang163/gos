import json
import subprocess
import time
import os
import sys

def load_config(config_file):
    """
    加载配置文件
    :param config_file: 配置文件路径
    :return: 配置字典
    """
    try:
        with open(config_file, "r") as f:
            return json.load(f)
    except Exception as e:
        print(f"Error loading config: {e}")
        sys.exit(1)

def start_node(node_config):
    """
    启动一个节点
    :param node_config: 节点配置
    :return: (进程对象, 临时配置文件路径)
    """
    node_id = node_config.get("id", "unknown")
    print(f"Starting node {node_id} with config: {node_config}")

    # 将节点配置写入临时文件
    temp_config_file = f"temp_config_{node_id}.json"
    try:
        with open(temp_config_file, "w") as f:
            json.dump(node_config, f)
    except Exception as e:
        print(f"Error writing temporary config file: {e}")
        return None, None

    # 启动节点
    try:
        if sys.platform == "win32":
            # Windows 使用 start 命令打开新终端
            command = f'start cmd /k go run . {temp_config_file}'
            process = subprocess.Popen(command, shell=True)
        else:
            # Linux/Mac 使用 gnome-terminal 或 xterm 打开新终端
            command = f'gnome-terminal -- bash -c "go run . {temp_config_file}; exec bash"'
            process = subprocess.Popen(command, shell=True)
    except Exception as e:
        print(f"Error starting node {node_id}: {e}")
        return None, None

    # 启动节点后延迟 1 秒
    time.sleep(1)

    return process, temp_config_file

def main():
    # 检查命令行参数
    if len(sys.argv) < 2:
        print("Usage: python run-nodes.py <config_file>")
        sys.exit(1)

    # 加载配置文件
    config_file = sys.argv[1]
    config = load_config(config_file)

    # 读取节点配置
    nodes = config.get("nodes", [])
    if not nodes:
        print("No nodes found in config.json")
        sys.exit(1)

    processes = []
    temp_files = []
    try:
        # 依次启动节点
        for node in nodes:
            process, temp_file = start_node(node)
            if process and temp_file:
                processes.append(process)
                temp_files.append(temp_file)

        # 等待所有节点运行
        print("All nodes started. Press Ctrl+C to stop.")
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        print("\nStopping all nodes...")
        for process in processes:
            try:
                process.terminate()
            except Exception as e:
                print(f"Error stopping process: {e}")
        print("All nodes stopped.")
    finally:
        # 清理临时配置文件
        for temp_file in temp_files:
            try:
                os.remove(temp_file)
                print(f"Removed temporary config file: {temp_file}")
            except Exception as e:
                print(f"Error removing temporary config file {temp_file}: {e}")

if __name__ == "__main__":
    main()