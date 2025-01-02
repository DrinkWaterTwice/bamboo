import torch
import torch.nn as nn
import torch.nn.functional as F
import random
from torch.distributions import Categorical

# 经验回放池
class exp_replay:
    def __init__(self, capacity):
        self.capacity = capacity
        self.buffer = []
        self.position = 0

    def store(self, buffer):
        self.buffer.append(buffer)
        if len(self.buffer) > self.capacity:
            self.buffer.pop(0)
    
    def sample(self, batch_size):
        if len(self.buffer) < batch_size:
            return self.buffer
        indices = random.sample(range(len(self.buffer)), batch_size)
        return [self.buffer[i] for i in indices]

pool = exp_replay(10000)
global epsilon 
epsilon = 0.2
# 在线训练 A2C
def train_a2c_online(model, optimizer, state, reward, buffer, done, view_reward=0, gamma=0.99):
    global epsilon
    buffer.store_reward(reward, done)
    if done and not buffer.done and len(buffer.states) > 0:
        buffer.done = True
        buffer.states.append(state)
        buffer.total_reward = view_reward
        buffer.rewards = buffer.rewards[1:]
        pool.store(buffer)
        return
    
    # 转换 state 为张量
    # state = torch.tensor(state, dtype=torch.float32)

    # 前向传播
    action_probs, state_value = model(state)
    local_epsilon = epsilon
    epsilon = local_epsilon * 1
    # Epsilon-greedy探索
    ran = random.random()
    if ran < epsilon:  # 随机选择动作
        action = random.randint(0, len(action_probs) - 1)
    else:  # 根据模型输出选择最优动作
        dist = Categorical(action_probs)
        action = dist.sample().item()  # 根据概率分布选择动作
    # 存储当前状态、动作、log_prob 和状态值
    buffer.store(state, action,  None, state_value[0].detach())
    return action 

# 训练缓冲区
class TrainingBuffer:
    def __init__(self):
        self.done = False
        self.states = []
        self.actions = []
        self.log_probs = []
        self.values = []
        self.rewards = []
        self.done_flags = []
        self.total_reward = 0

    def store(self, state, action, log_prob, value):
        self.states.append(state)
        self.actions.append(action)
        self.log_probs.append(log_prob)
        self.values.append(value)

    def store_reward(self, reward, done):
        self.rewards.append(reward)
        self.done_flags.append(done)

    def reset(self):
        self.states.clear()
        self.actions.clear()
        self.log_probs.clear()
        self.values.clear()
        self.rewards.clear()
        self.done_flags.clear()

    def __str__(self):
        return f"States: {self.states}, Actions: {self.actions}, Log Probs: {self.log_probs}, Values: {self.values}, Rewards: {self.rewards}, Done Flags: {self.done_flags}"


# 更新模型
def update_model(model, optimizer, buffer, gamma):
    # 批量化训练
    for i in range(len(buffer.states) - 1):
        states = buffer.states[i]
        actions = torch.tensor([buffer.actions[i]], dtype=torch.long)  # 确保 actions 是一个张量
        next_states = buffer.states[i + 1]
        reward = buffer.rewards[i]
        done = buffer.done_flags[i + 1]

        # 前向传播
        action_probs, state_values = model(states)
        _, next_state_values = model(next_states)
        dist = Categorical(action_probs)
        log_probs_new = dist.log_prob(actions)

        # 计算每个状态的优势函数 A = R + γ * V(s') - V(s)
        td_target = reward + gamma * next_state_values.item() * (1 - done)
        advantages = td_target - state_values.item()

        # 计算 actor 和 critic 的损失
        actor_loss = -(log_probs_new * advantages)
        critic_loss = F.mse_loss(state_values.squeeze().unsqueeze(0), torch.tensor([td_target], dtype=torch.float32))

        # 总损失
        loss = actor_loss + critic_loss

        # 打印调试信息
        ran = random.random()
        if ran < 0.05:
            print(f"loss: {loss}, actor_loss: {actor_loss}, critic_loss: {critic_loss}, state_value: {state_values.squeeze()}, td_target: {td_target}")
            print(f"rewards: {buffer.rewards}")
            print(f"advantages: {advantages}")
            print(f"state_values: {state_values}")
            print(f"action_probs: {action_probs}")
            print(f"state: {states}")
            print(f"buffer: {buffer}")

        # 反向传播和优化
        optimizer.zero_grad()
        loss.backward()
        optimizer.step()

# 训练函数
def train(model, optimizer, size=100, gamma=0.99):
    buffers = pool.sample(size)
    for buffer in buffers:
        if len(buffer.states) > 0 and buffer.done:
            # distribute_rewards(buffer, buffer.total_reward, gamma)
            update_model(model, optimizer, buffer, gamma)
