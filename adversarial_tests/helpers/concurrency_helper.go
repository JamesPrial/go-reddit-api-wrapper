package helpers

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// GoroutineSnapshot captures the state of goroutines at a point in time
type GoroutineSnapshot struct {
	Count     int
	Timestamp time.Time
}

// TakeGoroutineSnapshot captures current goroutine count
func TakeGoroutineSnapshot() *GoroutineSnapshot {
	return &GoroutineSnapshot{
		Count:     runtime.NumGoroutine(),
		Timestamp: time.Now(),
	}
}

// DetectGoroutineLeak compares two snapshots and returns an error if goroutines leaked
func DetectGoroutineLeak(before, after *GoroutineSnapshot, tolerance int) error {
	leaked := after.Count - before.Count
	if leaked > tolerance {
		return fmt.Errorf("goroutine leak detected: started with %d, ended with %d (leaked %d, tolerance %d)",
			before.Count, after.Count, leaked, tolerance)
	}
	return nil
}

// WaitForGoroutineCleanup waits for goroutines to clean up, retrying with GC
func WaitForGoroutineCleanup(maxWait time.Duration, targetCount int, tolerance int) (int, error) {
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		current := runtime.NumGoroutine()
		diff := current - targetCount

		if diff <= tolerance {
			return current, nil
		}

		// Force GC and wait
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
	}

	final := runtime.NumGoroutine()
	return final, fmt.Errorf("goroutines did not clean up within %v: expected %d±%d, got %d",
		maxWait, targetCount, tolerance, final)
}

// MemorySnapshot captures memory statistics at a point in time
type MemorySnapshot struct {
	Alloc      uint64
	TotalAlloc uint64
	Sys        uint64
	NumGC      uint32
	Timestamp  time.Time
}

// TakeMemorySnapshot captures current memory statistics
func TakeMemorySnapshot() *MemorySnapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &MemorySnapshot{
		Alloc:      m.Alloc,
		TotalAlloc: m.TotalAlloc,
		Sys:        m.Sys,
		NumGC:      m.NumGC,
		Timestamp:  time.Now(),
	}
}

// DetectMemoryLeak compares two snapshots and returns an error if memory leaked significantly
func DetectMemoryLeak(before, after *MemorySnapshot, thresholdBytes uint64) error {
	// Force GC before measuring to clean up unreferenced memory
	runtime.GC()
	runtime.GC() // Run twice to be thorough

	// Re-measure after GC
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	leaked := int64(m.Alloc) - int64(before.Alloc)

	if leaked > int64(thresholdBytes) {
		return fmt.Errorf("memory leak detected: started with %d bytes, ended with %d bytes (leaked %d, threshold %d)",
			before.Alloc, m.Alloc, leaked, thresholdBytes)
	}

	return nil
}

// MemoryGrowthMonitor tracks memory growth over time
type MemoryGrowthMonitor struct {
	snapshots []*MemorySnapshot
	mu        sync.Mutex
}

// NewMemoryGrowthMonitor creates a new memory growth monitor
func NewMemoryGrowthMonitor() *MemoryGrowthMonitor {
	return &MemoryGrowthMonitor{
		snapshots: make([]*MemorySnapshot, 0, 100),
	}
}

// Record takes a snapshot and stores it
func (m *MemoryGrowthMonitor) Record() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots = append(m.snapshots, TakeMemorySnapshot())
}

// GetGrowthRate returns the average memory growth rate in bytes per second
func (m *MemoryGrowthMonitor) GetGrowthRate() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.snapshots) < 2 {
		return 0
	}

	first := m.snapshots[0]
	last := m.snapshots[len(m.snapshots)-1]

	memoryDelta := float64(int64(last.Alloc) - int64(first.Alloc))
	timeDelta := last.Timestamp.Sub(first.Timestamp).Seconds()

	if timeDelta == 0 {
		return 0
	}

	return memoryDelta / timeDelta
}

// GetPeakMemory returns the highest memory allocation observed
func (m *MemoryGrowthMonitor) GetPeakMemory() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	var peak uint64
	for _, snap := range m.snapshots {
		if snap.Alloc > peak {
			peak = snap.Alloc
		}
	}

	return peak
}

// ConcurrentExecutor runs a function concurrently with leak detection
type ConcurrentExecutor struct {
	goroutinesBefore *GoroutineSnapshot
	memoryBefore     *MemorySnapshot
	goroutineLeakTolerance int
	memoryLeakThreshold uint64
}

// NewConcurrentExecutor creates a new concurrent executor with leak detection
func NewConcurrentExecutor(goroutineLeakTolerance int, memoryLeakThresholdBytes uint64) *ConcurrentExecutor {
	return &ConcurrentExecutor{
		goroutinesBefore: TakeGoroutineSnapshot(),
		memoryBefore:     TakeMemorySnapshot(),
		goroutineLeakTolerance: goroutineLeakTolerance,
		memoryLeakThreshold: memoryLeakThresholdBytes,
	}
}

// Execute runs the function and checks for leaks afterward
func (ce *ConcurrentExecutor) Execute(fn func() error) error {
	// Run the function
	if err := fn(); err != nil {
		return err
	}

	// Wait for cleanup
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check for goroutine leaks
	goroutinesAfter := TakeGoroutineSnapshot()
	if err := DetectGoroutineLeak(ce.goroutinesBefore, goroutinesAfter, ce.goroutineLeakTolerance); err != nil {
		return err
	}

	// Check for memory leaks
	memoryAfter := TakeMemorySnapshot()
	if err := DetectMemoryLeak(ce.memoryBefore, memoryAfter, ce.memoryLeakThreshold); err != nil {
		return err
	}

	return nil
}

// DeadlockDetector helps detect potential deadlocks in concurrent operations
type DeadlockDetector struct {
	timeout   time.Duration
	completed chan struct{}
}

// NewDeadlockDetector creates a new deadlock detector
func NewDeadlockDetector(timeout time.Duration) *DeadlockDetector {
	return &DeadlockDetector{
		timeout:   timeout,
		completed: make(chan struct{}),
	}
}

// Run executes the function with deadlock detection
func (dd *DeadlockDetector) Run(fn func() error) error {
	errChan := make(chan error, 1)

	go func() {
		err := fn()
		errChan <- err
		close(dd.completed)
	}()

	select {
	case err := <-errChan:
		return err
	case <-time.After(dd.timeout):
		return fmt.Errorf("operation timed out after %v (possible deadlock)", dd.timeout)
	}
}

// GenerateMaliciousRateHeaders creates pathological rate limit header combinations
func GenerateMaliciousRateHeaders() map[string]map[string]string {
	return map[string]map[string]string{
		"nan_remaining": {
			"X-Ratelimit-Remaining": "NaN",
			"X-Ratelimit-Reset":     "60",
		},
		"inf_remaining": {
			"X-Ratelimit-Remaining": "Inf",
			"X-Ratelimit-Reset":     "60",
		},
		"negative_inf_remaining": {
			"X-Ratelimit-Remaining": "-Inf",
			"X-Ratelimit-Reset":     "60",
		},
		"huge_negative_remaining": {
			"X-Ratelimit-Remaining": "-9999999999.99",
			"X-Ratelimit-Reset":     "60",
		},
		"max_float_remaining": {
			"X-Ratelimit-Remaining": "1.7976931348623157e+308",
			"X-Ratelimit-Reset":     "60",
		},
		"negative_reset": {
			"X-Ratelimit-Remaining": "50",
			"X-Ratelimit-Reset":     "-60",
		},
		"zero_reset": {
			"X-Ratelimit-Remaining": "0",
			"X-Ratelimit-Reset":     "0",
		},
		"huge_reset": {
			"X-Ratelimit-Remaining": "50",
			"X-Ratelimit-Reset":     "999999999",
		},
		"invalid_format_remaining": {
			"X-Ratelimit-Remaining": "not_a_number",
			"X-Ratelimit-Reset":     "60",
		},
		"invalid_format_reset": {
			"X-Ratelimit-Remaining": "50",
			"X-Ratelimit-Reset":     "not_a_number",
		},
		"missing_remaining": {
			"X-Ratelimit-Reset": "60",
		},
		"missing_reset": {
			"X-Ratelimit-Remaining": "50",
		},
		"empty_headers": {},
		"special_chars_remaining": {
			"X-Ratelimit-Remaining": "50\r\n\r\nX-Injected: malicious",
			"X-Ratelimit-Reset":     "60",
		},
		"unicode_remaining": {
			"X-Ratelimit-Remaining": "5\u00000",
			"X-Ratelimit-Reset":     "60",
		},
	}
}

// AtomicContentionTester tests atomic operations under high contention
type AtomicContentionTester struct {
	numGoroutines int
	iterations    int
}

// NewAtomicContentionTester creates a new atomic contention tester
func NewAtomicContentionTester(numGoroutines, iterations int) *AtomicContentionTester {
	return &AtomicContentionTester{
		numGoroutines: numGoroutines,
		iterations:    iterations,
	}
}

// TestContention runs the atomic operation under high contention
func (act *AtomicContentionTester) TestContention(atomicOp func(goroutineID int) error) error {
	errors := make(chan error, act.numGoroutines)
	var wg sync.WaitGroup

	wg.Add(act.numGoroutines)
	for i := 0; i < act.numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < act.iterations; j++ {
				if err := atomicOp(id); err != nil {
					errors <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		if err != nil {
			return err
		}
	}

	return nil
}

// SemaphoreStressTester tests semaphore implementations under stress
type SemaphoreStressTester struct {
	maxConcurrent int
	totalOps      int
}

// NewSemaphoreStressTester creates a new semaphore stress tester
func NewSemaphoreStressTester(maxConcurrent, totalOps int) *SemaphoreStressTester {
	return &SemaphoreStressTester{
		maxConcurrent: maxConcurrent,
		totalOps:      totalOps,
	}
}

// Test runs the semaphore test and verifies max concurrency is enforced
func (sst *SemaphoreStressTester) Test(operation func(id int) error) (int, error) {
	var currentConcurrent int32
	var peakConcurrent int32
	var mu sync.Mutex

	sem := make(chan struct{}, sst.maxConcurrent)
	errors := make(chan error, sst.totalOps)
	var wg sync.WaitGroup

	wg.Add(sst.totalOps)
	for i := 0; i < sst.totalOps; i++ {
		go func(id int) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Track concurrency
			mu.Lock()
			currentConcurrent++
			if currentConcurrent > peakConcurrent {
				peakConcurrent = currentConcurrent
			}
			mu.Unlock()

			// Execute operation
			if err := operation(id); err != nil {
				errors <- err
			}

			// Release concurrency counter
			mu.Lock()
			currentConcurrent--
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			return int(peakConcurrent), err
		}
	}

	// Verify peak concurrency didn't exceed limit
	if int(peakConcurrent) > sst.maxConcurrent {
		return int(peakConcurrent), fmt.Errorf("semaphore failed: peak concurrency %d exceeded limit %d",
			peakConcurrent, sst.maxConcurrent)
	}

	return int(peakConcurrent), nil
}
