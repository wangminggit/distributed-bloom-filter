package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	"github.com/wangminggit/distributed-bloom-filter/internal/raft"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

var (
	configFile = flag.String("config", "config.yaml", "path to config file")
	nodeID     = flag.String("node-id", "", "unique node identifier")
	shardID    = flag.Int("shard-id", 0, "shard ID this node belongs to")
	mode       = flag.String("mode", "standalone", "running mode: standalone|cluster")
)

func main() {
	flag.Parse()

	log.Printf("Starting Distributed Bloom Filter Server...")
	log.Printf("Config: %s, NodeID: %s, ShardID: %d, Mode: %s",
		*configFile, *nodeID, *shardID, *mode)

	// 创建 Counting Bloom Filter
	// 10 亿数据，0.1% 误判率
	cbf := bloom.NewCountingBloomFilter(1000000000, 0.001)
	log.Printf("Created %s", cbf.String())

	// 初始化元数据服务
	metaService, err := metadata.NewService(*nodeID, *shardID)
	if err != nil {
		log.Fatalf("Failed to initialize metadata service: %v", err)
	}

	// 初始化 Raft 节点
	raftNode, err := raft.NewNode(*nodeID, *shardID, cbf)
	if err != nil {
		log.Fatalf("Failed to initialize Raft node: %v", err)
	}

	// 优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %s, shutting down...", sig)

		if err := raftNode.Shutdown(); err != nil {
			log.Printf("Error shutting down Raft node: %v", err)
		}

		if err := metaService.Close(); err != nil {
			log.Printf("Error closing metadata service: %v", err)
		}

		os.Exit(0)
	}()

	log.Printf("Server started successfully")

	// 保持运行
	select {}
}
