package analyzer

import (
	"sync"
)

// MaxWorkers = max number of simultaneous link checks (goroutines)
// Too high → overload server, use too much RAM/CPU
// Too low  → slow analysis
// 100 is a good default for most servers
const MaxWorkers = 100

// MaxQueue = how many links can wait in line
// If more than 1000 links are found, extra ones will be skipped or blocked
// Prevents memory explosion on huge pages
const MaxQueue = 1000

// workerPool is a channel that acts like a "token bucket"
var (
	workerPool chan struct{}
	once       sync.Once
)

// initWorkerPool sets up the token system
// Called automatically the first time someone uses Acquire()
func initWorkerPool() {
	// sync.Once ensures this block runs **only once**, no matter how many goroutines call it
	once.Do(func() {
		// Create a buffered channel with MaxWorkers slots
		// Each slot holds an empty struct{} (uses zero memory)
		workerPool = make(chan struct{}, MaxWorkers)

		// Pre-fill the pool with MaxWorkers "tokens"
		// This means: "Yes, you can start 100 workers right now"
		for i := 0; i < MaxWorkers; i++ {
			workerPool <- struct{}{} // Put a token in
		}
	})
}

// Acquire gets a token → allows one link check to proceed
// If no tokens left → **blocks** (waits) until someone calls Release()
func Acquire() {
	// Make sure pool is ready (first call initializes it)
	initWorkerPool()

	// Wait here until a token is available
	// This blocks the goroutine — prevents too many workers
	<-workerPool
}

// Release returns a token to the pool
// Called with `defer Release()` in goroutines
// Allows another link check to start
func Release() {
	// Put the token back — now one more worker can run
	workerPool <- struct{}{}
}