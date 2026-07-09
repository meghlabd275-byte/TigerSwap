package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"

	"tigerswap/indexer/internal/indexer"
)

// @title TigerSwap Indexer
// @version 1.0
// @description Real-time blockchain data indexer for TigerSwap DEX

func main() {
	// Load configuration
	loadConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize indexer
	idx := indexer.NewIndexer(
		viper.GetString("ethereum.rpc_url"),
		viper.GetString("database.dsn"),
		viper.GetString("redis.addr"),
	)

	// Start indexer
	if err := idx.Start(ctx); err != nil {
		log.Fatalf("Failed to start indexer: %v", err)
	}

	log.Println("TigerSwap Indexer started successfully")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down indexer...")
	idx.Stop()
	log.Println("Indexer stopped")
}

func loadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/tigerswap/")

	// Set defaults
	viper.SetDefault("ethereum.rpc_url", "https://eth.llamarpc.com")
	viper.SetDefault("ethereum.ws_url", "wss://eth.llamarpc.com")
	viper.SetDefault("ethereum.start_block", 18000000)
	viper.SetDefault("database.dsn", "host=localhost port=5432 user=tigerswap password= dbname=tigerswap sslmode=disable")
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("indexer.batch_size", 1000)
	viper.SetDefault("indexer.workers", 4)

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: Config file not found, using defaults: %v", err)
	}

	viper.AutomaticEnv()
}
