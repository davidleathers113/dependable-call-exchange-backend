package performance

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// WorkerPool manages a pool of workers for async task processing
type WorkerPool struct {
	logger *zap.Logger
	config *WorkerPoolConfig
	
	// Worker management
	workers   []*Worker
	taskQueue chan Task
	
	// Load balancing
	balancer LoadBalancer
	
	// Statistics
	stats      *WorkerPoolStats
	statsMutex sync.RWMutex
	
	// State management
	running int32
	stopped chan struct{}
	wg      sync.WaitGroup
}

// WorkerPoolConfig configures the worker pool
type WorkerPoolConfig struct {
	PoolSize         int
	QueueSize        int
	IdleTimeout      time.Duration
	TaskTimeout      time.Duration
	LoadBalanceStrategy LoadBalancingStrategy
	WorkerRecycleAge time.Duration
	MaxTaskRetries   int
}

// LoadBalancer interface for distributing tasks among workers
type LoadBalancer interface {
	SelectWorker(workers []*Worker, task Task) *Worker
	UpdateWorkerStats(worker *Worker, taskDuration time.Duration, success bool)
	GetStrategy() LoadBalancingStrategy
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(config *WorkerPoolConfig, logger *zap.Logger) *WorkerPool {
	pool := &WorkerPool{
		logger:    logger,
		config:    config,
		taskQueue: make(chan Task, config.QueueSize),
		workers:   make([]*Worker, 0, config.PoolSize),
		stats: &WorkerPoolStats{
			Total: config.PoolSize,
		},
		stopped: make(chan struct{}),
	}
	
	// Initialize load balancer
	pool.balancer = pool.createLoadBalancer()
	
	return pool
}

// Start initializes and starts all workers
func (wp *WorkerPool) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&wp.running, 0, 1) {
		return fmt.Errorf("worker pool already running")
	}
	
	wp.logger.Info("Starting worker pool",
		zap.Int("pool_size", wp.config.PoolSize),
		zap.Int("queue_size", wp.config.QueueSize),
		zap.String("load_balance_strategy", wp.config.LoadBalanceStrategy.String()),
	)
	
	// Create and start workers
	for i := 0; i < wp.config.PoolSize; i++ {
		worker := wp.createWorker(i)
		wp.workers = append(wp.workers, worker)
		
		wp.wg.Add(1)
		go wp.runWorker(ctx, worker)
	}
	
	// Start task dispatcher
	wp.wg.Add(1)
	go wp.runTaskDispatcher(ctx)
	
	// Start stats collector
	wp.wg.Add(1)
	go wp.runStatsCollector(ctx)
	
	wp.logger.Info("Worker pool started successfully")
	return nil
}

// Stop gracefully shuts down the worker pool
func (wp *WorkerPool) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&wp.running, 1, 0) {
		return nil
	}
	
	wp.logger.Info("Stopping worker pool")
	
	// Close task queue to signal shutdown
	close(wp.taskQueue)
	
	// Signal all workers to stop
	close(wp.stopped)
	
	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		wp.logger.Info("Worker pool stopped gracefully")
	case <-ctx.Done():
		wp.logger.Warn("Worker pool stop timed out")
		return ctx.Err()
	}
	
	return nil
}

// SubmitTask submits a task to the worker pool
func (wp *WorkerPool) SubmitTask(task Task) error {
	if atomic.LoadInt32(&wp.running) == 0 {
		return fmt.Errorf("worker pool not running")
	}
	
	select {
	case wp.taskQueue <- task:
		atomic.AddInt64(&wp.stats.TotalTasks, 1)
		return nil
	default:
		return fmt.Errorf("task queue full")
	}
}

// GetWorker returns an available worker (for direct task assignment)
func (wp *WorkerPool) GetWorker(ctx context.Context) (*Worker, error) {
	if atomic.LoadInt32(&wp.running) == 0 {
		return nil, fmt.Errorf("worker pool not running")
	}
	
	// Find the best worker using load balancer
	worker := wp.balancer.SelectWorker(wp.workers, Task{})
	if worker == nil {
		return nil, fmt.Errorf("no available workers")
	}
	
	return worker, nil
}

// GetStats returns current worker pool statistics
func (wp *WorkerPool) GetStats() *WorkerPoolStats {
	wp.statsMutex.RLock()
	defer wp.statsMutex.RUnlock()
	
	stats := *wp.stats
	
	// Calculate real-time stats
	var active, idle int
	for _, worker := range wp.workers {
		if worker.Active {
			active++
		} else {
			idle++
		}
	}
	
	stats.Active = active
	stats.Idle = idle
	stats.Queued = len(wp.taskQueue)
	
	return &stats
}

// createWorker creates a new worker
func (wp *WorkerPool) createWorker(id int) *Worker {
	return &Worker{
		ID:       id,
		Active:   false,
		TaskChan: make(chan Task, 1),
		QuitChan: make(chan bool, 1),
		Stats: WorkerStats{
			Created: time.Now(),
		},
	}
}

// createLoadBalancer creates the appropriate load balancer
func (wp *WorkerPool) createLoadBalancer() LoadBalancer {
	switch wp.config.LoadBalanceStrategy {
	case LoadBalancingRoundRobin:
		return NewRoundRobinBalancer()
	case LoadBalancingLeastConnections:
		return NewLeastConnectionsBalancer()
	case LoadBalancingWeightedRoundRobin:
		return NewWeightedRoundRobinBalancer()
	case LoadBalancingLatencyBased:
		return NewLatencyBasedBalancer()
	case LoadBalancingResourceBased:
		return NewResourceBasedBalancer()
	default:
		return NewRoundRobinBalancer()
	}
}

// runWorker runs a single worker
func (wp *WorkerPool) runWorker(ctx context.Context, worker *Worker) {
	defer wp.wg.Done()
	
	wp.logger.Debug("Starting worker",
		zap.Int("worker_id", worker.ID),
	)
	
	idleTimer := time.NewTimer(wp.config.IdleTimeout)
	defer idleTimer.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-wp.stopped:
			return
		case <-worker.QuitChan:
			return
		case task := <-worker.TaskChan:
			wp.processTask(worker, task)
			idleTimer.Reset(wp.config.IdleTimeout)
		case <-idleTimer.C:
			// Worker has been idle too long, check if we should recycle it
			if wp.shouldRecycleWorker(worker) {
				wp.recycleWorker(worker)
				return
			}
			idleTimer.Reset(wp.config.IdleTimeout)
		}
	}
}

// runTaskDispatcher dispatches tasks from the queue to workers
func (wp *WorkerPool) runTaskDispatcher(ctx context.Context) {
	defer wp.wg.Done()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-wp.stopped:
			return
		case task, ok := <-wp.taskQueue:
			if !ok {
				return // Queue closed
			}
			wp.dispatchTask(task)
		}
	}
}

// runStatsCollector collects worker pool statistics
func (wp *WorkerPool) runStatsCollector(ctx context.Context) {
	defer wp.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-wp.stopped:
			return
		case <-ticker.C:
			wp.collectStats()
		}
	}
}

// processTask processes a single task
func (wp *WorkerPool) processTask(worker *Worker, task Task) {
	start := time.Now()
	worker.Active = true
	worker.Stats.LastActive = start
	
	defer func() {
		worker.Active = false
		duration := time.Since(start)
		worker.Stats.TotalDuration += duration
		
		if r := recover(); r != nil {
			atomic.AddInt64(&worker.Stats.TasksFailed, 1)
			atomic.AddInt64(&wp.stats.FailedTasks, 1)
			
			wp.logger.Error("Worker panic",
				zap.Int("worker_id", worker.ID),
				zap.String("task_id", task.ID),
				zap.Any("panic", r),
			)
			
			// Send error result if channel is available
			if task.ResultCh != nil {
				select {
				case task.ResultCh <- TaskResult{
					Success:  false,
					Error:    fmt.Errorf("worker panic: %v", r),
					Duration: duration,
					Worker:   worker.ID,
				}:
				default:
				}
			}
		}
	}()
	
	// Create task context with timeout
	taskCtx := task.Context
	if wp.config.TaskTimeout > 0 {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithTimeout(task.Context, wp.config.TaskTimeout)
		defer cancel()
	}
	
	// Process the task
	result := wp.executeTask(taskCtx, task)
	
	// Update statistics
	if result.Success {
		atomic.AddInt64(&worker.Stats.TasksCompleted, 1)
		atomic.AddInt64(&wp.stats.CompletedTasks, 1)
	} else {
		atomic.AddInt64(&worker.Stats.TasksFailed, 1)
		atomic.AddInt64(&wp.stats.FailedTasks, 1)
	}
	
	// Update load balancer
	wp.balancer.UpdateWorkerStats(worker, result.Duration, result.Success)
	
	// Send result if channel is available
	if task.ResultCh != nil {
		select {
		case task.ResultCh <- result:
		case <-time.After(time.Second):
			wp.logger.Warn("Result channel send timed out",
				zap.String("task_id", task.ID),
				zap.Int("worker_id", worker.ID),
			)
		}
	}
	
	wp.logger.Debug("Task processed",
		zap.String("task_id", task.ID),
		zap.Int("worker_id", worker.ID),
		zap.Duration("duration", result.Duration),
		zap.Bool("success", result.Success),
	)
}

// executeTask executes the actual task logic
func (wp *WorkerPool) executeTask(ctx context.Context, task Task) TaskResult {
	start := time.Now()
	
	result := TaskResult{
		Duration: time.Since(start),
		Worker:   -1, // Will be set by caller
	}
	
	// Task execution logic based on task type
	switch task.Type {
	case TaskTypeDNCQuery:
		result.Data, result.Error = wp.processDNCQuery(ctx, task)
	case TaskTypeCacheWarmup:
		result.Data, result.Error = wp.processCacheWarmup(ctx, task)
	case TaskTypeCacheEviction:
		result.Data, result.Error = wp.processCacheEviction(ctx, task)
	case TaskTypeMetricsCollection:
		result.Data, result.Error = wp.processMetricsCollection(ctx, task)
	case TaskTypeConnectionMaintenance:
		result.Data, result.Error = wp.processConnectionMaintenance(ctx, task)
	default:
		result.Error = fmt.Errorf("unknown task type: %v", task.Type)
	}
	
	result.Success = result.Error == nil
	result.Duration = time.Since(start)
	
	return result
}

// dispatchTask assigns a task to the best available worker
func (wp *WorkerPool) dispatchTask(task Task) {
	worker := wp.balancer.SelectWorker(wp.workers, task)
	if worker == nil {
		wp.logger.Warn("No available workers for task",
			zap.String("task_id", task.ID),
			zap.String("task_type", task.Type.String()),
		)
		
		// Send error result
		if task.ResultCh != nil {
			select {
			case task.ResultCh <- TaskResult{
				Success: false,
				Error:   fmt.Errorf("no available workers"),
				Worker:  -1,
			}:
			default:
			}
		}
		return
	}
	
	// Try to send task to worker
	select {
	case worker.TaskChan <- task:
		// Task dispatched successfully
	default:
		// Worker is busy, try to find another one
		wp.logger.Debug("Worker busy, finding alternative",
			zap.Int("worker_id", worker.ID),
			zap.String("task_id", task.ID),
		)
		
		// Put task back in queue for retry
		select {
		case wp.taskQueue <- task:
		default:
			// Queue is full, drop the task
			wp.logger.Warn("Task dropped - queue full",
				zap.String("task_id", task.ID),
			)
		}
	}
}

// shouldRecycleWorker determines if a worker should be recycled
func (wp *WorkerPool) shouldRecycleWorker(worker *Worker) bool {
	// Check age
	if time.Since(worker.Stats.Created) > wp.config.WorkerRecycleAge {
		return true
	}
	
	// Check error rate
	total := worker.Stats.TasksCompleted + worker.Stats.TasksFailed
	if total > 100 && float64(worker.Stats.TasksFailed)/float64(total) > 0.1 {
		return true
	}
	
	return false
}

// recycleWorker replaces an old worker with a new one
func (wp *WorkerPool) recycleWorker(worker *Worker) {
	wp.logger.Info("Recycling worker",
		zap.Int("worker_id", worker.ID),
		zap.Int64("tasks_completed", worker.Stats.TasksCompleted),
		zap.Int64("tasks_failed", worker.Stats.TasksFailed),
		zap.Duration("uptime", time.Since(worker.Stats.Created)),
	)
	
	// Find worker in slice and replace
	for i, w := range wp.workers {
		if w.ID == worker.ID {
			newWorker := wp.createWorker(worker.ID)
			wp.workers[i] = newWorker
			
			// Start new worker
			wp.wg.Add(1)
			go wp.runWorker(context.Background(), newWorker)
			break
		}
	}
}

// collectStats collects and updates worker pool statistics
func (wp *WorkerPool) collectStats() {
	wp.statsMutex.Lock()
	defer wp.statsMutex.Unlock()
	
	var totalDuration time.Duration
	var totalTasks int64
	
	for _, worker := range wp.workers {
		totalDuration += worker.Stats.TotalDuration
		totalTasks += worker.Stats.TasksCompleted + worker.Stats.TasksFailed
	}
	
	if totalTasks > 0 {
		wp.stats.AverageTaskDuration = totalDuration / time.Duration(totalTasks)
	}
}

// Task processing methods (these would be implemented based on specific requirements)

func (wp *WorkerPool) processDNCQuery(ctx context.Context, task Task) (interface{}, error) {
	// Implementation would handle DNC query processing
	return nil, nil
}

func (wp *WorkerPool) processCacheWarmup(ctx context.Context, task Task) (interface{}, error) {
	// Implementation would handle cache warmup
	return nil, nil
}

func (wp *WorkerPool) processCacheEviction(ctx context.Context, task Task) (interface{}, error) {
	// Implementation would handle cache eviction
	return nil, nil
}

func (wp *WorkerPool) processMetricsCollection(ctx context.Context, task Task) (interface{}, error) {
	// Implementation would handle metrics collection
	return nil, nil
}

func (wp *WorkerPool) processConnectionMaintenance(ctx context.Context, task Task) (interface{}, error) {
	// Implementation would handle connection maintenance
	return nil, nil
}