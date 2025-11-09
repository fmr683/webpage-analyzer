package analyzer

import (
	"context"
	"encoding/json"
	"time"
	"os"
	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client

func init() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379" // fallback if env var not set
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr: addr,
	})
}


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

func SetCachedResult(url string, res *AnalysisResult) {
	ctx := context.Background()
	data, _ := json.Marshal(res)
	redisClient.Set(ctx, "result:"+url, data, 1*time.Hour)
}