package analyzer

import (
	"context"
	"encoding/json"
	"time"
	"os"
	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client
var CACHE_TTL = 1 // Redis TTL

func init() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379" // fallback if env var not set
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr: addr,
	})
}

// GetCachedResult checks if we already analyzed this URL before
// It looks in Redis (fast in-memory database) under key: "result:https://example.com"
func GetCachedResult(url string) (*AnalysisResult, bool) {
	ctx := context.Background()
	data, err := redisClient.Get(ctx, "result:"+url).Bytes()
	if err != nil {
		return nil, false
	}
	var res AnalysisResult
	json.Unmarshal(data, &res)
	return &res, true
}

// SetCachedResult saves the analysis result in Redis for 1 hour
// So next time someone asks for the same URL → instant answer!
func SetCachedResult(url string, res *AnalysisResult) {
	ctx := context.Background()
	data, _ := json.Marshal(res)
	redisClient.Set(ctx, "result:"+url, data, time.Duration(CACHE_TTL)*time.Hour)
}