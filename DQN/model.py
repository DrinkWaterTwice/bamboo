# 状态空间为
# 发起请求的节点
# 当前主节点
# 节点间的延迟 n * n
# 当前所有节点采取的策略


# 输出为节点n的策略，为a * n 维，a为动作数量，n为针对每个其他节点的单独动作
# 动作从[0.1,0.2,0.3,0.4,0.5,0.6,0.7,0.8,0.9]中进行选择


import torch
import torch.nn as nn
import torch.optim as optim
import numpy as np

class DQN(nn.Module):
    def __init__(self, input_dim, output_dim):
        super(DQN, self).__init__()
        self.fc1 = nn.Linear(input_dim, 128)
        self.fc2 = nn.Linear(128, 256)
        self.fc3 = nn.Linear(256, output_dim)

    def forward(self, x):
        x = torch.relu(self.fc1(x))
        x = torch.relu(self.fc2(x))
        x = self.fc3(x)
        return x

class DQNAgent:
    def __init__(self, input_dim, output_dim, lr=0.0001):
        self.input_dim = input_dim
        self.output_dim = output_dim
        self.model = DQN(input_dim, output_dim)
        self.optimizer = optim.Adam(self.model.parameters(), lr=lr)
        self.criterion = nn.MSELoss()
        self.experience_replay = []

    def train(self):
        self.model.train()

        for state, action, reward, next_state in self.experience_replay:
            state = torch.tensor(state, dtype=torch.float32)
            action = torch.tensor(action, dtype=torch.long)
            reward = torch.tensor(reward, dtype=torch.float32)
            next_state = torch.tensor(next_state, dtype=torch.float32)
            action = action.flatten()
            current_q = self.model(state).reshape(-1, 9)
            current_q = current_q.gather(1, action.unsqueeze(-1))
            current_q = current_q.squeeze()
            # print(current_q.shape)
            next_q = self.model(next_state).view(-1, 9)
            top_k_values, top_k_indices = next_q.topk(1, dim=1)
            top_k_values = top_k_values.squeeze()
            target_q = reward + 0.99 * top_k_values
            loss = self.criterion(current_q, target_q)
            self.optimizer.zero_grad()
            loss.backward()
            self.optimizer.step()

    def add_experience(self, state, action, reward, next_state):
        self.experience_replay.append((state, action, reward, next_state))

    def choose_action(self, state):
        self.model.eval()
        with torch.no_grad():
            state = torch.tensor(state, dtype=torch.float32)
            q_values = self.model(state)
            q_values = q_values.view(-1, 9)  # 将输出重塑为 (a * n, 9)
            probabilities = torch.softmax(q_values, dim=1)
            actions = []
            for prob in probabilities:
                action = np.random.choice(9, p=prob.numpy())
                actions.append((action))
            return actions

    def trans_to_action(self, actions):
        actions = np.array(actions)
        actions = (actions + 0.0) / 10
        return actions

