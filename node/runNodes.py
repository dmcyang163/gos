import json
import subprocess
import time
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

def start_node_process(config_file):
    """
    启动节点进程
    :param config_file: 配置文件路径
    :return: 进程对象或 None
    """
    try:
        if sys.platform == "win32":
            # Windows 使用 start 命令打开新终端
            command = f'start cmd /k go run . -c {config_file}'
            process = subprocess.Popen(command, shell=True)
        elif sys.platform == "linux":
            # 检查 gnome-terminal 是否存在
            if os.system("which gnome-terminal") == 0:
                command = f'gnome-terminal -- bash -c \'go run . -c {config_file}; exec bash\''
            elif os.system("which xterm") == 0:
                command = f'xterm -e "go run . -c {config_file}"'
            elif os.system("which konsole") == 0:
                command = f'konsole -e "go run . -c {config_file}"'
            else:
                # 如果没有找到终端，直接在后台运行
                command = f'go run . -c {config_file}'
            process = subprocess.Popen(command, shell=True)
        elif sys.platform == "darwin":
            # Mac 使用 osascript 打开新终端
            command = f'osascript -e \'tell app "Terminal" to do script "go run . -c {config_file}"\''
            process = subprocess.Popen(command, shell=True)
        else:
            handle_error(f"Unsupported operating system: {sys.platform}")
            return None
        return process
    except Exception as e:
        handle_error("Error starting node", e)
        return None

def start_node(config_file):
    """
    启动一个节点
    :param config_file: 配置文件路径
    :return: 进程对象
    """
    config = load_config(config_file)
    node_id = config.get("id", "unknown")
    print(f"{Fore.GREEN}Starting node {node_id} with config:{Style.RESET_ALL}")
    print(json.dumps(config, indent=4))

    process = start_node_process(config_file)
    if not process:
        return None

    # 启动节点后延迟 1 秒
    time.sleep(1)

    return process

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

def main():
    # 检查命令行参数
    if len(sys.argv) < 2:
        handle_error("Usage: python runNodes.py <config1.json> <config2.json> <config3.json> ...")

    processes = []

    try:
        # 遍历所有传入的配置文件
        for config_file in sys.argv[1:]:
            # 启动节点
            process = start_node(config_file)
            if process:
                processes.append(process)

        # 等待所有节点运行
        print(f"{Fore.GREEN}All nodes started. Press Ctrl+C to stop.{Style.RESET_ALL}")
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        stop_nodes(processes)

if __name__ == "__main__":
    main()
