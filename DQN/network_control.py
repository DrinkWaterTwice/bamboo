import socket
import json
import numpy as np

from model import DQNAgent
import threading


# 参数设置
n = 4
a = 3
# input_dim = 1 + 1 + n * n + n * a * 9  # 发起请求的节点 + 当前主节点 + 节点间延迟 + 当前策略
input_dim = 1 + 1 + n * n  # 发起请求的节点 + 当前主节点 + 节点间延迟 + 当前策略
output_dim = a * n * 9  # 每个节点的动作数量 * 每个动作的选择范围

# 创建代理
agent = DQNAgent(input_dim, output_dim)

global_delays = [[0.5,0.5,0.5,0.5]] * 4
pre_actions = [[0]] * n
# 暂时先不添加当前节点策略
pre_state = [[0]] * n


# 训练代理
agent.train()



def handle_client(client_socket):
    count = 1
    try:
        while True:
            # 接收客户端发送的数据
            data = client_socket.recv(1024).decode('utf-8')
            print(data)
            # 解析 JSON 数据
            json_data = json.loads(data)
            nodeId = [json_data["nodeId"]]
            if nodeId[0] == 0:
                continue
            print(f"nodeId: {nodeId[0]}")
            primaryId = [json_data["primaryId"]]
            reward = json_data["reward"]
            global_delays[nodeId[0] - 1] = json_data["delays"]
            current_state = np.concatenate((np.array(nodeId),np.array(primaryId),np.array(global_delays).flatten()))
            chose_action = agent.choose_action(current_state)
            # 把上一个state和动作、奖励加入经验池
            if reward != 0:
                agent.add_experience(pre_state[nodeId[0] - 1], pre_actions[nodeId[0] - 1], reward, current_state)
            # 处理数据并生成响应
            pre_actions[nodeId[0] - 1] = np.array(chose_action).reshape(a, n).tolist()
            
            actions = np.array(agent.trans_to_action(chose_action)).reshape(a, n).tolist()
            response_data = {
                "actions": actions,
            }
            pre_state[nodeId[0] - 1] = current_state
            response_json = json.dumps(response_data)
            count += 1
            if count % 10 == 0:
                agent.train()
            # 发送响应给客户端
            client_socket.sendall(response_json.encode('utf-8'))
            
    except Exception as e:
        e.with_traceback()
        # print(f"Error handling client: {e}")
    finally:
        # 关闭客户端连接
        client_socket.close()


def handle_client_thread(client_socket):
    handle_client(client_socket)

def start_server(host='0.0.0.0', port=23309):
    server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server_socket.bind((host, port))
    server_socket.listen(5)
    print(f"Server listening on {host}:{port}")

    try:
        while True:
            client_socket, client_address = server_socket.accept()
            print(f"Accepted connection from {client_address}")

            # 创建新线程处理客户端请求
            client_thread = threading.Thread(target=handle_client_thread, args=(client_socket,))
            client_thread.start()
    except KeyboardInterrupt:
        print("Server shutting down")
    finally:
        server_socket.close()
if __name__ == "__main__":
    start_server()