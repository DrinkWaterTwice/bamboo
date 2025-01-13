#!/bin/bash

# 定义接口名称
INTERFACE="lo"

# 删除根队列规则
sudo tc qdisc del dev $INTERFACE root

echo "带宽限制已移除对于接口 ${INTERFACE}"