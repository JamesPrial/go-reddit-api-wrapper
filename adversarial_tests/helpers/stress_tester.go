package helpers

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// StressConfig configures stress test parameters
type StressConfig struct {
	// NumGoroutines is the number of concurrent goroutines to spawn
	NumGoroutines int

	// Duration is how long to run the stress test
	Duration time.Duration

	// OperationFunc is the function to execute concurrently
	OperationFunc func(goroutineID int) error

	// WarmupDuration is an optional warmup period before starting measurement
	WarmupDuration time.Duration

	// CollectMetrics enables detailed metrics collection
	CollectMetrics bool
}

// StressResult contains the results of a stress test
type StressResult struct {
	// TotalOperations is the total number of operations attempted
	TotalOperations uint64

	// SuccessfulOperations is the number of operations that succeeded
	SuccessfulOperations uint64

	// FailedOperations is the number of operations that failed
	FailedOperations uint64

	// Duration is the actual test duration
	Duration time.Duration

	// Errors contains all errors encountered during the test
	Errors []error

	// StartGoroutines is the number of goroutines at test start
	StartGoroutines int

	// EndGoroutines is the number of goroutines at test end
	EndGoroutines int

	// PeakGoroutines is the maximum number of goroutines during the test
	PeakGoroutines int

	// StartMemoryBytes is the memory allocated at test start
	StartMemoryBytes uint64

	// EndMemoryBytes is the memory allocated at test end
	EndMemoryBytes uint64

	// PeakMemoryBytes is the peak memory allocated during the test
	PeakMemoryBytes uint64
}

// StressTester orchestrates concurrent stress testing
type StressTester struct {
	config *StressConfig

	// Metrics
	totalOps     uint64
	successOps   uint64
	failedOps    uint64
	errors       []error
	errorsMutex  sync.Mutex

	// Goroutine tracking
	peakGoroutines   int
	goroutinesMutex  sync.Mutex

	// Memory tracking
	peakMemory       uint64
	memoryMutex      sync.Mutex

	// Control
	ctx              context.Context
	cancel           context.CancelFunc
	monitoringWg     sync.WaitGroup
}

// NewStressTester creates a new stress tester
func NewStressTester(config *StressConfig) *StressTester {
	if config.NumGoroutines <= 0 {
		config.NumGoroutines = 100
	}
	if config.Duration <= 0 {
		config.Duration = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Duration+config.WarmupDuration+5*time.Second)

	return &StressTester{
		config: config,
		errors: make([]error, 0),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Run executes the stress test
func (st *StressTester) Run() *StressResult {
	defer st.cancel()

	// Record starting state
	startTime := time.Now()
	startGoroutines := runtime.NumGoroutine()
	startMemory := getMemStats()

	// Start resource monitoring if metrics collection is enabled
	if st.config.CollectMetrics {
		st.startMonitoring()
	}

	// Warmup phase
	if st.config.WarmupDuration > 0 {
		st.runPhase("warmup", st.config.WarmupDuration, st.config.NumGoroutines)
		// Reset counters after warmup
		atomic.StoreUint64(&st.totalOps, 0)
		atomic.StoreUint64(&st.successOps, 0)
		atomic.StoreUint64(&st.failedOps, 0)
		st.errorsMutex.Lock()
		st.errors = make([]error, 0)
		st.errorsMutex.Unlock()
	}

	// Main stress test phase
	st.runPhase("stress", st.config.Duration, st.config.NumGoroutines)

	// Stop monitoring
	if st.config.CollectMetrics {
		st.stopMonitoring()
	}

	// Record ending state
	endTime := time.Now()
	endMemory := getMemStats()

	// Force garbage collection and wait a moment to see if goroutines clean up
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	finalGoroutines := runtime.NumGoroutine()

	return &StressResult{
		TotalOperations:      atomic.LoadUint64(&st.totalOps),
		SuccessfulOperations: atomic.LoadUint64(&st.successOps),
		FailedOperations:     atomic.LoadUint64(&st.failedOps),
		Duration:             endTime.Sub(startTime),
		Errors:               st.errors,
		StartGoroutines:      startGoroutines,
		EndGoroutines:        finalGoroutines,
		PeakGoroutines:       st.peakGoroutines,
		StartMemoryBytes:     startMemory,
		EndMemoryBytes:       endMemory,
		PeakMemoryBytes:      st.peakMemory,
	}
}

// runPhase runs a single phase of the stress test
func (st *StressTester) runPhase(name string, duration time.Duration, numGoroutines int) {
	ctx, cancel := context.WithTimeout(st.ctx, duration)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			st.workerLoop(ctx, id)
		}(i)
	}

	// Wait for completion
	wg.Wait()
}

// workerLoop is the main loop for each worker goroutine
func (st *StressTester) workerLoop(ctx context.Context, id int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			atomic.AddUint64(&st.totalOps, 1)

			// Execute the operation
			err := st.config.OperationFunc(id)

			if err != nil {
				atomic.AddUint64(&st.failedOps, 1)
				st.recordError(err)
			} else {
				atomic.AddUint64(&st.successOps, 1)
			}
		}
	}
}

// recordError safely records an error
func (st *StressTester) recordError(err error) {
	st.errorsMutex.Lock()
	defer st.errorsMutex.Unlock()

	// Limit the number of errors we store to prevent memory exhaustion
	if len(st.errors) < 1000 {
		st.errors = append(st.errors, err)
	}
}

// startMonitoring starts resource monitoring
func (st *StressTester) startMonitoring() {
	st.monitoringWg.Add(1)
	go func() {
		defer st.monitoringWg.Done()

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-st.ctx.Done():
				return
			case <-ticker.C:
				// Track goroutines
				numGoroutines := runtime.NumGoroutine()
				st.goroutinesMutex.Lock()
				if numGoroutines > st.peakGoroutines {
					st.peakGoroutines = numGoroutines
				}
				st.goroutinesMutex.Unlock()

				// Track memory
				memory := getMemStats()
				st.memoryMutex.Lock()
				if memory > st.peakMemory {
					st.peakMemory = memory
				}
				st.memoryMutex.Unlock()
			}
		}
	}()
}

// stopMonitoring stops resource monitoring
func (st *StressTester) stopMonitoring() {
	st.monitoringWg.Wait()
}

// getMemStats returns current memory allocation
func getMemStats() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

// FormatResult formats a stress test result for display
func (r *StressResult) FormatResult() string {
	successRate := float64(0)
	if r.TotalOperations > 0 {
		successRate = float64(r.SuccessfulOperations) / float64(r.TotalOperations) * 100
	}

	opsPerSec := float64(0)
	if r.Duration.Seconds() > 0 {
		opsPerSec = float64(r.TotalOperations) / r.Duration.Seconds()
	}

	goroutineLeak := r.EndGoroutines - r.StartGoroutines
	memoryLeak := int64(r.EndMemoryBytes) - int64(r.StartMemoryBytes)

	return fmt.Sprintf(`Stress Test Results:
  Total Operations: %d
  Successful: %d (%.2f%%)
  Failed: %d
  Duration: %v
  Ops/sec: %.2f

  Goroutines:
    Start: %d
    End: %d
    Peak: %d
    Leak: %d

  Memory (bytes):
    Start: %d
    End: %d
    Peak: %d
    Leak: %d

  Errors: %d unique errors`,
		r.TotalOperations,
		r.SuccessfulOperations,
		successRate,
		r.FailedOperations,
		r.Duration,
		opsPerSec,
		r.StartGoroutines,
		r.EndGoroutines,
		r.PeakGoroutines,
		goroutineLeak,
		r.StartMemoryBytes,
		r.EndMemoryBytes,
		r.PeakMemoryBytes,
		memoryLeak,
		len(r.Errors),
	)
}

// HasGoroutineLeak returns true if goroutines leaked
func (r *StressResult) HasGoroutineLeak() bool {
	// Allow a small margin for background goroutines
	return (r.EndGoroutines - r.StartGoroutines) > 2
}

// HasMemoryLeak returns true if memory leaked significantly
func (r *StressResult) HasMemoryLeak() bool {
	// Allow 10MB leak margin
	leak := int64(r.EndMemoryBytes) - int64(r.StartMemoryBytes)
	return leak > 10*1024*1024
}

// ConcurrentBurst executes a burst of concurrent operations and waits for all to complete
func ConcurrentBurst(numOps int, opFunc func(id int) error) []error {
	var wg sync.WaitGroup
	errors := make(chan error, numOps)

	wg.Add(numOps)
	for i := 0; i < numOps; i++ {
		go func(id int) {
			defer wg.Done()
			if err := opFunc(id); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Collect errors
	var errList []error
	for err := range errors {
		errList = append(errList, err)
	}

	return errList
}

// RaceDetector helps detect race conditions by coordinating goroutine execution
type RaceDetector struct {
	startSignal chan struct{}
	numWaiters  int32
	targetWaiters int32
}

// NewRaceDetector creates a new race detector
func NewRaceDetector(numGoroutines int) *RaceDetector {
	return &RaceDetector{
		startSignal:   make(chan struct{}),
		targetWaiters: int32(numGoroutines),
	}
}

// WaitForStart makes a goroutine wait until all goroutines are ready
func (rd *RaceDetector) WaitForStart() {
	atomic.AddInt32(&rd.numWaiters, 1)
	if atomic.LoadInt32(&rd.numWaiters) >= rd.targetWaiters {
		close(rd.startSignal)
	}
	<-rd.startSignal
}

// CoordinatedStart executes multiple operations simultaneously to detect races
func CoordinatedStart(numOps int, opFunc func(id int) error) []error {
	detector := NewRaceDetector(numOps)
	errors := make(chan error, numOps)
	var wg sync.WaitGroup

	wg.Add(numOps)
	for i := 0; i < numOps; i++ {
		go func(id int) {
			defer wg.Done()
			detector.WaitForStart() // Synchronize start
			if err := opFunc(id); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	var errList []error
	for err := range errors {
		errList = append(errList, err)
	}

	return errList
}
