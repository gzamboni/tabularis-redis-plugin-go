//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
)

func main() {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0, // use default DB
	})

	fmt.Println("Seeding Redis with test data...")

	// 1. Strings
	err := rdb.Set(ctx, "user:1:name", "Giovanni Zamboni", 0).Err()
	if err != nil { log.Printf("Error setting key: %v", err) }
	rdb.Set(ctx, "user:1:email", "giovanni@example.com", 0)

	// 2. Hashes
	rdb.HSet(ctx, "user:1:profile", map[string]interface{}{
		"age":      30,
		"location": "Italy",
		"status":   "active",
	})

	rdb.HSet(ctx, "project:tabularis:meta", map[string]interface{}{
		"version": "0.9.0",
		"author":  "debba",
		"stars":   "1200",
	})

	// 3. Lists
	rdb.Del(ctx, "logs:system")
	rdb.RPush(ctx, "logs:system", "booting", "connecting_db", "ready", "shutdown")

	// 4. Sets
	rdb.Del(ctx, "tags:golang")
	rdb.SAdd(ctx, "tags:golang", "programming", "backend", "concurrency", "cloud")

	// 5. Sorted Sets (ZSets)
	rdb.Del(ctx, "leaderboard:points")
	rdb.ZAdd(ctx, "leaderboard:points", &redis.Z{Score: 100, Member: "player1"})
	rdb.ZAdd(ctx, "leaderboard:points", &redis.Z{Score: 250, Member: "player2"})
	rdb.ZAdd(ctx, "leaderboard:points", &redis.Z{Score: 175, Member: "player3"})

	fmt.Println("✅ Successfully seeded Redis!")
	fmt.Println("")
	fmt.Println("Try these queries in Tabularis:")
	fmt.Println("1. SELECT * FROM keys")
	fmt.Println("2. SELECT * FROM hashes WHERE key = 'user:1:profile'")
	fmt.Println("3. SELECT * FROM lists WHERE key = 'logs:system'")
	fmt.Println("4. SELECT * FROM sets WHERE key = 'tags:golang'")
	fmt.Println("5. SELECT * FROM zsets WHERE key = 'leaderboard:points'")
}
