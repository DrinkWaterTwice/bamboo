#!/bin/bash

# 定义接口名称
INTERFACE="lo"

# 定义带宽限制（单位：kbps）
RATE="1000"  # 1 Mbps = 1000 kbps

# 清除所有现有的 tc 配置
sudo tc qdisc del dev $INTERFACE root 2>/dev/null
sudo tc qdisc del dev $INTERFACE ingress 2>/dev/null

# 添加根队列规则
sudo tc qdisc add dev $INTERFACE root handle 1: htb default 10

# 添加类
sudo tc class add dev $INTERFACE parent 1: classid 1:1 htb rate ${RATE}kbps
sudo tc class add dev $INTERFACE parent 1:1 classid 1:10 htb rate ${RATE}kbps

# 添加过滤器
sudo tc filter add dev $INTERFACE protocol ip parent 1:0 prio 1 u32 match ip src 0.0.0.0/0 flowid 1:10

echo "带宽限制已设置为 ${RATE} kbps 对于接口 ${INTERFACE}"