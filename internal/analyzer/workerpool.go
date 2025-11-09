package analyzer

import (
	"sync"
)

const MaxWorkers = 100        // Tune based on CPU/RAM
const MaxQueue = 1000         // Prevent memory explosion

var (
	workerPool chan struct{}
	once       sync.Once
)

func initWorkerPool() {
	once.Do(func() {
		workerPool = make(chan struct{}, MaxWorkers)
		for i := 0; i < MaxWorkers; i++ {
			workerPool <- struct{}{}
		}
	})
}

// Acquire blocks if pool is full
func Acquire() {
	initWorkerPool()
	<-workerPool
}

func Release() {
	workerPool <- struct{}{}
}