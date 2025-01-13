import gym
from gym import spaces
import numpy as np
import random

class CustomEnv(gym.Env):
    def __init__(self, state):
        super(CustomEnv, self).__init__()
        
        # 定义动作空间，假设有两个离散动作
        self.action_space = spaces.Discrete(2)
        
        
        # 初始化状态
        self.state = state
        self.done = False

    def reset(self):
        # 重置环境并返回初始状态
        # self.state = np.random.randint(0, 256, size=(10,), dtype=np.uint8)
        self.done = False
        return self.state

    def step(self, action):
        # 执行一个动作，返回新的状态、奖励、是否完成以及额外信息
        if self.done:
            raise Exception("Episode already done")
        
        # 根据动作更新状态
        if action == 0:
            self.state += 1
        elif action == 1:
            self.state -= 1
        
        # 确保状态在合法范围内
        self.state = np.clip(self.state, 0, 255)
        
        # 计算奖励
        reward = 1.0 if np.sum(self.state) > 1000 else -1.0
        
        # 判断是否完成
        self.done = np.sum(self.state) > 1000
        
        return self.state, reward, self.done, {}

    def render(self, mode='human'):
        # 可选方法，用于渲染环境的当前状态
        print(f"Current State: {self.state}")

    def close(self):
        # 可选方法，用于关闭环境
        pass

# 示例使用
if __name__ == "__main__":
    env = CustomEnv()
    state = env.reset()
    done = False
    
    while not done:
        action = env.action_space.sample()  # 随机选择一个动作
        state, reward, done, info = env.step(action)
        env.render()
        print(f"Action: {action}, Reward: {reward}, Done: {done}")