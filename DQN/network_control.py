import socket
import json
import numpy as np
import torch
from model import A2CModel
import threading
import train
import time

# 参数设置
n = 4  # 节点数量
a = 4  # 策略数量
input_dim = 14  # 发起请求的节点 + 当前主节点 + 节点间延迟 + 当前策略
output_dim = a   # 每个节点的动作数量 * 每个动作的选择范围
embedding_dims = 32

# 创建代理W
agent = A2CModel(input_dim, output_dim, embedding_dims)
optimizer = torch.optim.Adam(agent.parameters(), lr=0.000001)
trainingBuffers = {}
torch.autograd.set_detect_anomaly(True)
# 创建锁
model_lock = threading.Lock()

# agent.train()
def handle_client(client_socket):
    try:
        # 接收客户端发送的数据
        data = client_socket.recv(1024).decode('utf-8')
        if not data:
            print("Received empty data, closing connection.")
            return

        # 解析 JSON 数据
        try:
            json_data = json.loads(data)
        except json.JSONDecodeError as e:
            print(f"Failed to decode JSON: {e}")
            return

        requestType = json_data["requestType"]
        nodeId = json_data["nodeId"]
        primaryId = json_data["primaryId"]
        view = json_data["view"]
        delays = json_data["delays"]
        role = json_data["role"]
        preViewRole = json_data["preViewRole"]
        byzRatio = json_data["byzRatio"]
        consensusStage = json_data["consensusStage"]
        voteRatio = json_data["voteRatio"]
        blockGenerationRate = json_data["blockGenerationRate"]
        blockCommitRate = json_data["blockCommitRate"]
        forkRate = json_data["forkRate"]
        throughput = json_data["throughput"]
        latency = json_data["latency"]
        forkedNum = json_data["forkNumber"]
        forkedMaliNum = json_data["forkedMaliNumber"]

        action = 0
        if view not in trainingBuffers:
            trainingBuffers[view] = train.TrainingBuffer()
        baseline = 1000
        a1 = 10000
        a2 = -20000
        if requestType == 'request':
            state = [nodeId, primaryId, role,preViewRole, byzRatio, consensusStage, voteRatio, blockGenerationRate, blockCommitRate, forkRate, throughput, latency]
            reward = forkedNum * a1 - baseline + forkedMaliNum * a2
            with model_lock:
                action = train.train_a2c_online(agent, optimizer, state, reward, trainingBuffers[view], False)

        elif requestType == 'viewDone':
            # print(f"view done: " + str(view))
            view_reward = forkedNum * a1 - baseline + forkedMaliNum * a2
            reward = forkedNum * a1 - baseline + forkedMaliNum * a2
            state = [nodeId, primaryId, role,preViewRole, byzRatio, consensusStage, voteRatio, blockGenerationRate, blockCommitRate, forkRate, throughput, latency]
            with model_lock:
                action = train.train_a2c_online(agent, optimizer, state, reward, trainingBuffers[view], True, view_reward)

        response_data = {
            "actions": action,
        }
        if view % 10 == 0:
            train.train(agent, optimizer)
            print(response_data)
            print(json_data)
        response_json = json.dumps(response_data)
        client_socket.sendall(response_json.encode('utf-8'))
        
    except Exception as e:
        print(f"Error handling client: {e.with_traceback()}")
    finally:
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
            # print(f"Accepted connection from {client_address}")

            # 创建新线程处理客户端请求
            client_thread = threading.Thread(target=handle_client_thread, args=(client_socket,))
            client_thread.start()
    except KeyboardInterrupt:
        print("Server shutting down")
    finally:
        server_socket.close()



def start_timer(interval):
    def wrapper():
        while True:
            time.sleep(interval)
            train.train(agent, optimizer)
    
    timer_thread = threading.Thread(target=wrapper, daemon=True)
    timer_thread.start()

if __name__ == "__main__":
    # start_timer(10)
    start_server()