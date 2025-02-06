import json
import subprocess
import time
import os
import sys
from colorama import Fore, Style, init

# 初始化 colorama
init(autoreset=True)

def handle_error(message, error=None):
    """
    处理错误信息并退出程序
    :param message: 错误信息
    :param error: 异常对象（可选）
    """
    if error:
        print(f"{Fore.RED}{message}: {error}{Style.RESET_ALL}")
    else:
        print(f"{Fore.RED}{message}{Style.RESET_ALL}")
    sys.exit(1)

def load_config(config_file):
    """
    加载配置文件
    :param config_file: 配置文件路径
    :return: 配置字典
    """
    try:
        with open(config_file, "r") as f:
            return json.load(f)
    except FileNotFoundError:
        handle_error(f"Config file {config_file} not found.")
    except json.JSONDecodeError as e:
        handle_error("Error decoding config file", e)

def write_temp_config(node_config):
    """
    将节点配置写入临时文件
    :param node_config: 节点配置
    :return: 临时配置文件路径或 None
    """
    node_id = node_config.get("id", "unknown")
    temp_config_file = f"temp_config_{node_id}.json"
    try:
        with open(temp_config_file, "w") as f:
            json.dump(node_config, f, indent=4)
        return temp_config_file
    except Exception as e:
        handle_error("Error writing temporary config file", e)
        return None

def start_node_process(temp_config_file):
    """
    启动节点进程
    :param temp_config_file: 临时配置文件路径
    :return: 进程对象或 None
    """
    try:
        if sys.platform == "win32":
            # Windows 使用 start 命令打开新终端
            command = f'start cmd /k go run . {temp_config_file}'
            process = subprocess.Popen(command, shell=True)
        elif sys.platform.startswith('linux'):
            # Linux 使用 x-terminal-emulator 打开新终端
            command = f'x-terminal-emulator -e "bash -c \'go run . {temp_config_file}; exec bash\'"'
            process = subprocess.Popen(command, shell=True)
        elif sys.platform == "darwin":
            # Mac 使用 osascript 打开新终端
            command = f'osascript -e \'tell app "Terminal" to do script "go run . {temp_config_file}"\''
            process = subprocess.Popen(command, shell=True)
        else:
            handle_error(f"Unsupported operating system: {sys.platform}")
            return None
        return process
    except Exception as e:
        handle_error("Error starting node", e)
        return None

def start_node(node_config):
    """
    启动一个节点
    :param node_config: 节点配置
    :return: (进程对象, 临时配置文件路径)
    """
    node_id = node_config.get("id", "unknown")
    print(f"{Fore.GREEN}Starting node {node_id} with config:{Style.RESET_ALL}")
    # 使用 json.dumps 将 JSON 显示为美化格式
    print(json.dumps(node_config, indent=4))

    temp_config_file = write_temp_config(node_config)
    if not temp_config_file:
        return None, None

    process = start_node_process(temp_config_file)
    if not process:
        return None, None

    # 启动节点后延迟 1 秒
    time.sleep(1)

    return process, temp_config_file

def stop_nodes(processes):
    """
    停止所有节点
    :param processes: 进程列表
    """
    print(f"\n{Fore.YELLOW}Stopping all nodes...{Style.RESET_ALL}")
    for process in processes:
        try:
            process.terminate()
            process.wait()  # 等待进程终止
        except Exception as e:
            handle_error(f"Error stopping process", e)
    print(f"{Fore.GREEN}All nodes stopped.{Style.RESET_ALL}")

def clean_temp_files(temp_files):
    """
    清理临时配置文件
    :param temp_files: 临时文件列表
    """
    for temp_file in temp_files:
        try:
            if os.path.exists(temp_file):
                os.remove(temp_file)
                print(f"{Fore.BLUE}Removed temporary config file: {temp_file}{Style.RESET_ALL}")
        except Exception as e:
            handle_error(f"Error removing temporary config file {temp_file}", e)

def main():
    # 检查命令行参数
    if len(sys.argv) < 2:
        handle_error("Usage: python run-nodes.py <config_file>")

    # 加载配置文件
    config_file = sys.argv[1]
    config = load_config(config_file)

    # 读取节点配置
    nodes = config.get("nodes", [])
    if not nodes:
        handle_error("No nodes found in config.json")

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
        print(f"{Fore.GREEN}All nodes started. Press Ctrl+C to stop.{Style.RESET_ALL}")
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        stop_nodes(processes)
    finally:
        clean_temp_files(temp_files)

if __name__ == "__main__":
    main()