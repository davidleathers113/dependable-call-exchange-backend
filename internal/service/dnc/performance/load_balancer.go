package performance

import (
	"sort"
	"sync/atomic"
	"time"
)

// Load balancer implementations

// RoundRobinBalancer implements round-robin load balancing
type RoundRobinBalancer struct {
	counter uint64
}

func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

func (rb *RoundRobinBalancer) SelectWorker(workers []*Worker, task Task) *Worker {
	if len(workers) == 0 {
		return nil
	}
	
	index := atomic.AddUint64(&rb.counter, 1) % uint64(len(workers))
	return workers[index]
}

func (rb *RoundRobinBalancer) UpdateWorkerStats(worker *Worker, taskDuration time.Duration, success bool) {
	// No state to update for round-robin
}

func (rb *RoundRobinBalancer) GetStrategy() LoadBalancingStrategy {
	return LoadBalancingRoundRobin
}

// LeastConnectionsBalancer selects worker with least active tasks
type LeastConnectionsBalancer struct{}

func NewLeastConnectionsBalancer() *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{}
}

func (lcb *LeastConnectionsBalancer) SelectWorker(workers []*Worker, task Task) *Worker {
	if len(workers) == 0 {
		return nil
	}
	
	var bestWorker *Worker
	minTasks := int64(-1)
	
	for _, worker := range workers {
		activeTasks := worker.Stats.TasksCompleted + worker.Stats.TasksFailed
		if minTasks == -1 || activeTasks < minTasks {
			minTasks = activeTasks
			bestWorker = worker
		}
	}
	
	return bestWorker
}

func (lcb *LeastConnectionsBalancer) UpdateWorkerStats(worker *Worker, taskDuration time.Duration, success bool) {
	// Stats are updated automatically in worker
}

func (lcb *LeastConnectionsBalancer) GetStrategy() LoadBalancingStrategy {
	return LoadBalancingLeastConnections
}

// WeightedRoundRobinBalancer implements weighted round-robin based on performance
type WeightedRoundRobinBalancer struct {
	weights map[int]int
	counter uint64
}

func NewWeightedRoundRobinBalancer() *WeightedRoundRobinBalancer {
	return &WeightedRoundRobinBalancer{
		weights: make(map[int]int),
	}
}

func (wrb *WeightedRoundRobinBalancer) SelectWorker(workers []*Worker, task Task) *Worker {
	if len(workers) == 0 {
		return nil
	}
	
	// Calculate weights based on performance
	totalWeight := 0
	for _, worker := range workers {
		weight := wrb.calculateWeight(worker)
		wrb.weights[worker.ID] = weight
		totalWeight += weight
	}
	
	if totalWeight == 0 {
		// Fallback to round-robin
		index := atomic.AddUint64(&wrb.counter, 1) % uint64(len(workers))
		return workers[index]
	}
	
	// Select based on weighted distribution
	target := int(atomic.AddUint64(&wrb.counter, 1) % uint64(totalWeight))
	currentWeight := 0
	
	for _, worker := range workers {
		currentWeight += wrb.weights[worker.ID]
		if currentWeight > target {
			return worker
		}
	}
	
	return workers[0] // Fallback
}

func (wrb *WeightedRoundRobinBalancer) calculateWeight(worker *Worker) int {
	totalTasks := worker.Stats.TasksCompleted + worker.Stats.TasksFailed
	if totalTasks == 0 {
		return 10 // Default weight for new workers
	}
	
	// Weight based on success rate and performance
	successRate := float64(worker.Stats.TasksCompleted) / float64(totalTasks)
	avgDuration := worker.Stats.TotalDuration / time.Duration(totalTasks)
	
	// Higher weight for higher success rate and faster performance
	weight := int(successRate * 10)
	if avgDuration > 0 && avgDuration < time.Millisecond {
		weight += 5 // Bonus for fast workers
	}
	
	if weight < 1 {
		weight = 1
	}
	
	return weight
}

func (wrb *WeightedRoundRobinBalancer) UpdateWorkerStats(worker *Worker, taskDuration time.Duration, success bool) {
	// Weights are recalculated on each selection
}

func (wrb *WeightedRoundRobinBalancer) GetStrategy() LoadBalancingStrategy {
	return LoadBalancingWeightedRoundRobin
}

// LatencyBasedBalancer selects worker with lowest average latency
type LatencyBasedBalancer struct{}

func NewLatencyBasedBalancer() *LatencyBasedBalancer {
	return &LatencyBasedBalancer{}
}

func (lbb *LatencyBasedBalancer) SelectWorker(workers []*Worker, task Task) *Worker {
	if len(workers) == 0 {
		return nil
	}
	
	var bestWorker *Worker
	minLatency := time.Duration(-1)
	
	for _, worker := range workers {
		// Skip active workers for this strategy
		if worker.Active {
			continue
		}
		
		totalTasks := worker.Stats.TasksCompleted + worker.Stats.TasksFailed
		if totalTasks == 0 {
			// New worker - give it a chance
			return worker
		}
		
		avgLatency := worker.Stats.TotalDuration / time.Duration(totalTasks)
		if minLatency == -1 || avgLatency < minLatency {
			minLatency = avgLatency
			bestWorker = worker
		}
	}
	
	// If all workers are active, fall back to round-robin
	if bestWorker == nil {
		return workers[0]
	}
	
	return bestWorker
}

func (lbb *LatencyBasedBalancer) UpdateWorkerStats(worker *Worker, taskDuration time.Duration, success bool) {
	// Stats are updated automatically in worker
}

func (lbb *LatencyBasedBalancer) GetStrategy() LoadBalancingStrategy {
	return LoadBalancingLatencyBased
}

// ResourceBasedBalancer selects worker based on resource utilization
type ResourceBasedBalancer struct {
	resourceMetrics map[int]*WorkerResourceMetrics
}

type WorkerResourceMetrics struct {
	CPUUsage    float64
	MemoryUsage int64
	LastUpdated time.Time
}

func NewResourceBasedBalancer() *ResourceBasedBalancer {
	return &ResourceBasedBalancer{
		resourceMetrics: make(map[int]*WorkerResourceMetrics),
	}
}

func (rbb *ResourceBasedBalancer) SelectWorker(workers []*Worker, task Task) *Worker {
	if len(workers) == 0 {
		return nil
	}
	
	// Sort workers by resource utilization
	type workerScore struct {
		worker *Worker
		score  float64
	}
	
	scores := make([]workerScore, 0, len(workers))
	
	for _, worker := range workers {
		score := rbb.calculateResourceScore(worker)
		scores = append(scores, workerScore{
			worker: worker,
			score:  score,
		})
	}
	
	// Sort by score (lower is better)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score < scores[j].score
	})
	
	return scores[0].worker
}

func (rbb *ResourceBasedBalancer) calculateResourceScore(worker *Worker) float64 {
	metrics, exists := rbb.resourceMetrics[worker.ID]
	if !exists || time.Since(metrics.LastUpdated) > time.Minute {
		// No recent metrics, assume average resource usage
		return 50.0
	}
	
	// Combine CPU and memory usage into a single score
	score := metrics.CPUUsage + float64(metrics.MemoryUsage)/1024/1024*0.1 // 0.1 per MB
	
	// Penalty for active workers
	if worker.Active {
		score += 25.0
	}
	
	// Consider task success rate
	totalTasks := worker.Stats.TasksCompleted + worker.Stats.TasksFailed
	if totalTasks > 0 {
		failureRate := float64(worker.Stats.TasksFailed) / float64(totalTasks)
		score += failureRate * 20.0 // Penalty for high failure rate
	}
	
	return score
}

func (rbb *ResourceBasedBalancer) UpdateWorkerStats(worker *Worker, taskDuration time.Duration, success bool) {
	// Update resource metrics (this would typically be done by a monitoring agent)
	metrics := rbb.resourceMetrics[worker.ID]
	if metrics == nil {
		metrics = &WorkerResourceMetrics{}
		rbb.resourceMetrics[worker.ID] = metrics
	}
	
	// Simulate resource usage update based on task performance
	if taskDuration > time.Millisecond*100 {
		metrics.CPUUsage = 70.0 // High CPU for slow tasks
	} else {
		metrics.CPUUsage = 30.0 // Normal CPU for fast tasks
	}
	
	metrics.LastUpdated = time.Now()
}

func (rbb *ResourceBasedBalancer) GetStrategy() LoadBalancingStrategy {
	return LoadBalancingResourceBased
}