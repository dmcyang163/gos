package utils

import (
	"container/heap"
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"
)

// TaskFunc 是 Goroutine 池中执行的任务函数类型
type TaskFunc func()

// TaskExecutor 是 Goroutine 池的接口
type TaskExecutor interface {
	Submit(task TaskFunc) error
	SubmitWithPriority(task TaskFunc, priority int) error
	SubmitWithTimeout(task TaskFunc, timeout time.Duration) error
	SubmitWithRetry(task TaskFunc, retries int) error
	Resize(size int) error
	Stats() PoolStats
	Release()
}

// AntsExecutor 是 ants.Pool 的封装，实现 TaskExecutor 接口
type AntsExecutor struct {
	pool      *ants.Pool
	mu        sync.RWMutex
	taskQueue *PriorityQueue
	stats     PoolStats
	logger    Logger
}

// NewAntsExecutor 创建一个新的 AntsExecutor
func NewAntsExecutor(size int, logger Logger) (TaskExecutor, error) {
	pool, err := ants.NewPool(size)
	if err != nil {
		return nil, err
	}
	taskQueue := &PriorityQueue{}
	heap.Init(taskQueue)
	return &AntsExecutor{
		pool:      pool,
		taskQueue: taskQueue,
		stats:     PoolStats{},
		logger:    logger,
	}, nil
}

// getFunctionName 获取函数的名称（通过反射）
var functionNameCache sync.Map

func getFunctionName(fn interface{}) string {
	fnPointer := reflect.ValueOf(fn).Pointer()
	if name, ok := functionNameCache.Load(fnPointer); ok {
		return name.(string)
	}

	fnValue := reflect.ValueOf(fn)
	if fnValue.Kind() != reflect.Func {
		return "unknown"
	}
	fullName := runtime.FuncForPC(fnValue.Pointer()).Name()
	if strings.Contains(fullName, "func") {
		fullName = "anonymous-" + fullName
	}
	parts := strings.Split(fullName, ".")
	name := parts[len(parts)-1]
	functionNameCache.Store(fnPointer, name)
	return name
}

// 任务包装函数
func (e *AntsExecutor) wrapTask(task TaskFunc, priority int) func() {
	taskName := getFunctionName(task)
	return func() {
		start := time.Now()
		atomic.AddInt32(&e.stats.TotalTasks, 1) // 更新总任务数

		logMsg := fmt.Sprintf("Task %s started", taskName)
		if priority > 0 {
			logMsg = fmt.Sprintf("Task %s (priority %d) started", taskName, priority)
		}
		e.logger.Debug(logMsg)

		defer func() {
			duration := time.Since(start)
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("Task %s panic: %v", taskName, r)
				if priority > 0 {
					errMsg = fmt.Sprintf("Task %s (priority %d) panic: %v", taskName, priority, r)
				}
				e.logger.Error(errMsg)
				atomic.AddInt32(&e.stats.FailedTasks, 1) // 更新失败任务数
			} else {
				atomic.AddInt32(&e.stats.CompletedTasks, 1) // 更新成功任务数
			}

			// 更新任务执行时间统计
			e.mu.Lock()
			e.stats.TaskDuration = duration
			if duration > e.stats.MaxTaskDuration {
				e.stats.MaxTaskDuration = duration
			}
			e.stats.AvgTaskDuration = time.Duration(
				(int64(e.stats.AvgTaskDuration)*int64(e.stats.CompletedTasks-1) + int64(duration)) / int64(e.stats.CompletedTasks),
			)
			e.mu.Unlock()

			durationMsg := fmt.Sprintf("Task %s finished in %v", taskName, duration)
			if priority > 0 {
				durationMsg = fmt.Sprintf("Task %s (priority %d) finished in %v", taskName, priority, duration)
			}
			e.logger.Debug(durationMsg)
			atomic.StoreInt32(&e.stats.Running, int32(e.pool.Running()))
		}()
		task()
	}
}

// Submit 提交任务到 Goroutine 池
func (e *AntsExecutor) Submit(task TaskFunc) error {
	wrappedTask := e.wrapTask(task, 0)
	return e.pool.Submit(wrappedTask)
}

// SubmitWithPriority 提交带优先级的任务
var taskPool = sync.Pool{
	New: func() interface{} {
		return &PriorityTask{}
	},
}

func (e *AntsExecutor) SubmitWithPriority(task TaskFunc, priority int) error {
	wrappedTask := e.wrapTask(task, priority)

	e.mu.Lock()
	pt := taskPool.Get().(*PriorityTask)
	pt.Task = wrappedTask
	pt.Priority = priority
	heap.Push(e.taskQueue, pt)
	e.mu.Unlock()

	e.mu.Lock()
	if e.taskQueue.Len() > 0 {
		pt := heap.Pop(e.taskQueue).(*PriorityTask)
		e.mu.Unlock()
		return e.pool.Submit(pt.Task)
	}
	e.mu.Unlock()
	return nil
}

// SubmitWithTimeout 优化后的超时实现
func (e *AntsExecutor) SubmitWithTimeout(task TaskFunc, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	wrappedTask := e.wrapTask(task, 0)
	errChan := make(chan error, 1)

	go func() {
		errChan <- e.pool.Submit(wrappedTask)
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		atomic.AddInt32(&e.stats.TimeoutTasks, 1)
		e.logger.Warnf("Task %s timed out", getFunctionName(task))
		return ctx.Err()
	}
}

// SubmitWithRetry 内部重试实现
func (e *AntsExecutor) SubmitWithRetry(task TaskFunc, retries int) error {
	wrappedFunc := func() {
		var lastErr error
		for i := 0; i < retries; i++ {
			func() {
				defer func() {
					if r := recover(); r != nil {
						lastErr = fmt.Errorf("panic: %v", r)
					}
				}()
				task()
				lastErr = nil
			}()
			if lastErr == nil {
				return
			}
			time.Sleep(time.Duration(1<<i) * time.Second) // 指数退避
		}
		e.logger.Errorf("Task %s failed after %d retries: %v", getFunctionName(task), retries, lastErr)
	}

	return e.pool.Submit(e.wrapTask(wrappedFunc, 0))
}

// Resize 动态调整池大小
func (e *AntsExecutor) Resize(size int) error {
	if size < 1 {
		return fmt.Errorf("pool size must be greater than 0")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	currentSize := e.pool.Cap()
	if abs(size-currentSize)*100/currentSize > 20 {
		e.pool.Tune(size)
	}
	return nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Stats 返回池状态
func (e *AntsExecutor) Stats() PoolStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return PoolStats{
		Running: atomic.LoadInt32(&e.stats.Running),
		Waiting: int(e.pool.Waiting()),

		TimeoutTasks:   atomic.LoadInt32(&e.stats.TimeoutTasks),
		RetryTasks:     atomic.LoadInt32(&e.stats.RetryTasks),
		FailedTasks:    atomic.LoadInt32(&e.stats.FailedTasks),
		TotalTasks:     atomic.LoadInt32(&e.stats.TotalTasks),
		CompletedTasks: atomic.LoadInt32(&e.stats.CompletedTasks),

		TaskDuration:    e.stats.TaskDuration,
		AvgTaskDuration: e.stats.AvgTaskDuration,
		MaxTaskDuration: e.stats.MaxTaskDuration,

		PoolSize:  e.pool.Cap(),
		QueueSize: e.taskQueue.Len(),
	}
}

// Release 释放池
func (e *AntsExecutor) Release() {
	e.mu.Lock()
	defer e.mu.Unlock()

	timeout := time.After(5 * time.Second)
	for e.pool.Running() > 0 {
		select {
		case <-timeout:
			e.logger.Error("Pool release timed out, force exiting")
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
	e.pool.Release()
}

// PoolStats 状态统计
type PoolStats struct {
	Running int32 // 当前正在运行的 Goroutine 数量
	Waiting int   // 当前正在等待执行的任务数量

	FailedTasks    int32 // 失败的任务数量
	TimeoutTasks   int32 // 超时的任务数量
	RetryTasks     int32 // 重试的任务数量
	TotalTasks     int32 // 提交的任务总数
	CompletedTasks int32 // 成功完成的任务数量

	TaskDuration    time.Duration // 最近一个任务的执行时间
	AvgTaskDuration time.Duration // 任务的平均执行时间
	MaxTaskDuration time.Duration // 任务的最大执行时间

	QueueSize int // 任务队列的当前大小
	PoolSize  int // Goroutine 池的当前大小
}

// PriorityQueue 及相关实现
type PriorityTask struct {
	Task     func()
	Priority int
}

type PriorityQueue []*PriorityTask

func (pq PriorityQueue) Len() int           { return len(pq) }
func (pq PriorityQueue) Less(i, j int) bool { return pq[i].Priority < pq[j].Priority }
func (pq PriorityQueue) Swap(i, j int)      { pq[i], pq[j] = pq[j], pq[i] }

func (pq *PriorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*PriorityTask))
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}
