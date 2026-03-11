package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Service 元数据服务，基于 K8s ConfigMap + Gossip
type Service struct {
	nodeID    string
	shardID   int
	namespace string
	configMap string
	client    *kubernetes.Clientset
	cache     *ClusterState
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// ClusterState 集群状态缓存
type ClusterState struct {
	Shards map[int]*ShardInfo `json:"shards"`
	Nodes  map[string]*NodeInfo `json:"nodes"`
}

// ShardInfo 分片信息
type ShardInfo struct {
	Leader    string   `json:"leader"`
	Followers []string `json:"followers"`
}

// NodeInfo 节点信息
type NodeInfo struct {
	Status   string `json:"status"`
	Address  string `json:"address"`
	ShardID  int    `json:"shard_id"`
	IsLeader bool   `json:"is_leader"`
}

// NewService 创建新的元数据服务
func NewService(nodeID string, shardID int) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建 K8s 客户端
	config, err := rest.InClusterConfig()
	if err != nil {
		// 不在集群内运行时，返回错误（开发环境可能需要特殊处理）
		return nil, fmt.Errorf("failed to load K8s config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create K8s client: %w", err)
	}

	svc := &Service{
		nodeID:    nodeID,
		shardID:   shardID,
		namespace: "default", // TODO: 从环境变量读取
		configMap: "dbf-cluster",
		client:    clientset,
		cache:     &ClusterState{},
		ctx:       ctx,
		cancel:    cancel,
	}

	// TODO: 启动 Gossip 协议
	// 使用 HashiCorp Memberlist 实现节点状态同步

	return svc, nil
}

// GetLeader 获取指定分片的 Leader
func (s *Service) GetLeader(shardID int) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	shard, ok := s.cache.Shards[shardID]
	if !ok {
		return "", errors.New("shard not found")
	}

	return shard.Leader, nil
}

// GetFollowers 获取指定分片的 Followers
func (s *Service) GetFollowers(shardID int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	shard, ok := s.cache.Shards[shardID]
	if !ok {
		return nil, errors.New("shard not found")
	}

	return shard.Followers, nil
}

// GetNode 获取节点信息
func (s *Service) GetNode(nodeID string) (*NodeInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.cache.Nodes[nodeID]
	if !ok {
		return nil, errors.New("node not found")
	}

	return node, nil
}

// GetAllNodes 获取所有节点
func (s *Service) GetAllNodes() map[string]*NodeInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*NodeInfo)
	for k, v := range s.cache.Nodes {
		result[k] = v
	}
	return result
}

// RegisterNode 注册节点
func (s *Service) RegisterNode(nodeID string, info *NodeInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache.Nodes[nodeID] = info

	// TODO: 更新 ConfigMap
	return s.updateConfigMap()
}

// DeregisterNode 注销节点
func (s *Service) DeregisterNode(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.cache.Nodes, nodeID)

	// TODO: 更新 ConfigMap
	return s.updateConfigMap()
}

// updateConfigMap 更新 ConfigMap
func (s *Service) updateConfigMap() error {
	// TODO: 实现 ConfigMap 更新逻辑
	// 1. 序列化 ClusterState 到 JSON
	// 2. 更新 K8s ConfigMap
	// 3. 处理并发更新冲突

	data, err := json.Marshal(s.cache)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster state: %w", err)
	}

	cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(
		s.ctx,
		s.configMap,
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data["cluster-state.json"] = string(data)

	_, err = s.client.CoreV1().ConfigMaps(s.namespace).Update(
		s.ctx,
		cm,
		metav1.UpdateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	return nil
}

// loadFromConfigMap 从 ConfigMap 加载集群状态
func (s *Service) loadFromConfigMap() error {
	cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(
		s.ctx,
		s.configMap,
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	data, ok := cm.Data["cluster-state.json"]
	if !ok {
		return errors.New("cluster state not found in ConfigMap")
	}

	var state ClusterState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return fmt.Errorf("failed to unmarshal cluster state: %w", err)
	}

	s.cache = &state
	return nil
}

// Close 关闭元数据服务
func (s *Service) Close() error {
	s.cancel()
	// TODO: 注销当前节点
	return nil
}

// StartGossip 启动 Gossip 协议（占位）
func (s *Service) StartGossip() error {
	// TODO: 使用 HashiCorp Memberlist 实现
	// 1. 创建 Memberlist 配置
	// 2. 加入集群
	// 3. 监听节点状态变化
	// 4. 定期同步到 ConfigMap
	return nil
}
