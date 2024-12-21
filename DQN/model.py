import torch
import torch.nn as nn
import torch.nn.functional as F
from torch.distributions import Categorical
import numpy as np


class A2CModel(nn.Module):
    def __init__(self, state_dim, action_dim, embedding_dims = 32):
        super(A2CModel, self).__init__()
        
        # Embedding layers for discrete features
        self.role_embedding = nn.Embedding(10, embedding_dims)  # Assuming max 10 roles
        self.consensus_stage_embedding = nn.Embedding(10, embedding_dims)  # Assuming max 10 stages
        
        # Shared layers for feature extraction
        self.shared_fc = nn.Sequential(
            nn.Linear(state_dim + 2 * embedding_dims - 2, 128),  # Adjust input size after embedding
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
        # Extract discrete features and continuous features
        node_id, primary_id, role,preViewRole, byz_ratio, consensus_stage, vote_ratio, block_gen_rate, block_commit_rate, fork_rate, throughput, latency = state

        # Handle embeddings for discrete features
        role_embedded = self.role_embedding(torch.tensor(role).long())
        consensus_stage_embedded = self.consensus_stage_embedding(torch.tensor(consensus_stage).long())
        # preViewRole = np.array(preViewRole)
        # Concatenate all features
        preViewRole_tensor = torch.tensor(preViewRole)
        continuous_features = torch.tensor([node_id, primary_id, byz_ratio, vote_ratio, block_gen_rate, block_commit_rate, fork_rate, throughput, latency],dtype=torch.float32)
        continuous_features = torch.cat([continuous_features, preViewRole_tensor], dim=-1)
        state_features = torch.cat([continuous_features, role_embedded, consensus_stage_embedded], dim=-1)

        # Handle missing values in the state by replacing NaNs with zeros
        state_features = torch.nan_to_num(state_features, nan=0.0)

        # Shared feature extraction
        features = self.shared_fc(state_features)
        
        # Actor output: action probabilities
        action_probs = self.actor(features)
        
        # Critic output: state value
        state_value = self.critic(features)
        # print(action_probs)
        return action_probs, state_value

# # Define the state space dimensions and action space dimensions
# state_dim = 9  # Number of continuous state variables
# embedding_dims = 8  # Dimension of embeddings for discrete variables
# action_dim = 5  # Example: replace with desired number of actions

# # Initialize the A2C model
# a2c_model = A2CModel(state_dim, action_dim, embedding_dims)

# # Example of a single forward pass
# example_state = torch.rand((1, 11))  # Simulated input state with 11 features
# # Introduce some NaN values to simulate missing data
# example_state[0, 2] = float('nan')
# example_state[0, 4] = float('nan')

# # Forward pass
# action_probs, state_value = a2c_model(example_state)

# print("Action probabilities:", action_probs)
# print("State value:", state_value)
