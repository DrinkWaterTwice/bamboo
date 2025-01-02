import torch
import torch.nn as nn
import torch.nn.functional as F
from torch.distributions import Categorical
import numpy as np

class A2CModel(nn.Module):
    def __init__(self, state_dim, action_dim):
        super(A2CModel, self).__init__()


        # Shared layers for feature extraction
        self.shared_fc = nn.Sequential(
            nn.Linear(state_dim, 128),  # Directly use state_dim
            nn.ReLU(),
            nn.Linear(128, 128),
            nn.ReLU()
        )

        # Actor network (policy head)
        self.actor = nn.Sequential(
            nn.Linear(128, 64),
            nn.ReLU(),
            nn.Linear(64, action_dim),
            nn.Softmax(dim=-1)
        )

        # Critic network (value head)
        self.critic = nn.Sequential(
            nn.Linear(128, 64),
            nn.ReLU(),
            nn.Linear(64, 1)
        )

    def forward(self, state):
        # Directly use the state as a 1D vector
        state_features = torch.tensor(state, dtype=torch.float)

        # Handle missing values in the state by replacing NaNs with zeros
        state_features = torch.nan_to_num(state_features, nan=0.0)

        # Shared feature extraction
        features = self.shared_fc(state_features)

        # Actor output: action probabilities
        action_probs = self.actor(features)

        # Critic output: state value
        state_value = self.critic(features)

        return action_probs, state_value
