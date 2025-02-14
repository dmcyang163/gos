package utils

import (
	"container/heap"
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
)

// TaskFunc 是 Goroutine 池中执行的任务函数类型
type TaskFunc func()

// TaskExecutor 是 Goroutine 池的接口
type TaskExecutor interface {
	Submit(task TaskFunc) error
	SubmitWithPriority(task TaskFunc, priority int) error         // 支持任务优先级
	SubmitWithTimeout(task TaskFunc, timeout time.Duration) error // 支持任务超时
	SubmitWithRetry(task TaskFunc, retries int) error             // 支持任务重试
	Resize(size int) error                                        // 动态调整池大小
	Stats() PoolStats                                             // 监控 Goroutine 池状态
	Release()                                                     // 释放 Goroutine 池
}

// AntsExecutor 是 ants.Pool 的封装，实现 TaskExecutor 接口
type AntsExecutor struct {
	pool      *ants.Pool
	mu        sync.Mutex
	taskQueue *PriorityQueue // 优先级队列
	stats     PoolStats      // 统计信息
	logger    Logger         // 使用项目中的 Logger 模块
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

// getFunctionName 获取函数的名称
func getFunctionName(fn interface{}) string {
	// 使用反射获取函数指针
	fnValue := reflect.ValueOf(fn)
	if fnValue.Kind() != reflect.Func {
		return "unknown"
	}

	// 获取函数的完整名称（包括包路径）
	fullName := runtime.FuncForPC(fnValue.Pointer()).Name()

	// 如果是匿名函数，返回一个默认名称
	if strings.Contains(fullName, "func") {
		return "anonymous-" + fullName
	}

	// 提取函数名（去掉包路径）
	parts := strings.Split(fullName, ".")
	return parts[len(parts)-1]
}

// 通用的任务包装函数
func (e *AntsExecutor) wrapTask(task TaskFunc, taskName string, priority int) func() {
	return func() {
		start := time.Now()
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
				e.stats.FailedTasks++
			}
			durationMsg := fmt.Sprintf("Task %s finished in %v", taskName, duration)
			if priority > 0 {
				durationMsg = fmt.Sprintf("Task %s (priority %d) finished in %v", taskName, priority, duration)
			}
			e.logger.Debug(durationMsg)
			e.stats.TaskDuration = duration
		}()
		task()
	}
}

// Submit 提交任务到 Goroutine 池
func (e *AntsExecutor) Submit(task TaskFunc) error {
	taskName := getFunctionName(task)
	wrappedTask := e.wrapTask(task, taskName, 0)
	return e.pool.Submit(wrappedTask)
}

// SubmitWithPriority 提交带优先级的任务到 Goroutine 池
func (e *AntsExecutor) SubmitWithPriority(task TaskFunc, priority int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	taskName := getFunctionName(task)
	wrappedTask := e.wrapTask(task, taskName, priority)

	// 将任务添加到优先级队列
	heap.Push(e.taskQueue, &PriorityTask{Task: wrappedTask, Priority: priority})

	// 从队列中取出最高优先级的任务并提交
	if e.taskQueue.Len() > 0 {
		pt := heap.Pop(e.taskQueue).(*PriorityTask)
		return e.pool.Submit(pt.Task)
	}
	return nil
}

// SubmitWithTimeout 提交带超时的任务到 Goroutine 池
func (e *AntsExecutor) SubmitWithTimeout(task TaskFunc, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	taskName := getFunctionName(task)
	wrappedTask := e.wrapTask(task, taskName, 0)

	var wg sync.WaitGroup
	wg.Add(1)
	var err error

	go func() {
		defer wg.Done()
		err = e.pool.Submit(wrappedTask)
	}()

	select {
	case <-ctx.Done():
		e.stats.TimeoutTasks++
		e.logger.Warnf("Task %s timed out", taskName)
		return ctx.Err()
	case <-time.After(timeout):
		wg.Wait()
		return err
	}
}

// SubmitWithRetry 提交带重试的任务到 Goroutine 池
func (e *AntsExecutor) SubmitWithRetry(task TaskFunc, retries int) error {
	taskName := getFunctionName(task)

	var lastErr error
	for i := 0; i < retries; i++ {
		wrappedTask := e.wrapTask(task, taskName, 0)
		err := e.pool.Submit(wrappedTask)
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(time.Duration(1<<i) * time.Second) // 指数退避
	}
	return fmt.Errorf("task %s failed after %d retries: %w", taskName, retries, lastErr)
}

// Resize 动态调整 Goroutine 池的大小
func (e *AntsExecutor) Resize(size int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if size < 1 {
		return fmt.Errorf("pool size must be greater than 0")
	}
	e.pool.Tune(size)
	return nil
}

// Stats 返回 Goroutine 池的当前状态
func (e *AntsExecutor) Stats() PoolStats {
	e.mu.Lock()
	defer e.mu.Unlock()
	return PoolStats{
		Running:      e.pool.Running(),
		Waiting:      e.pool.Waiting(),
		TaskDuration: e.stats.TaskDuration,
		FailedTasks:  e.stats.FailedTasks,
		TimeoutTasks: e.stats.TimeoutTasks,
	}
}

// Release 释放 Goroutine 池
func (e *AntsExecutor) Release() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 等待所有任务完成
	for e.pool.Running() > 0 {
		time.Sleep(100 * time.Millisecond)
	}

	e.pool.Release()
}

// PoolStats 是 Goroutine 池的状态
type PoolStats struct {
	Running      int           // 活跃 Goroutine 数量
	Waiting      int           // 等待任务数量
	TaskDuration time.Duration // 平均任务执行时间
	FailedTasks  int           // 失败任务数量
	TimeoutTasks int           // 超时任务数量
}

// PriorityTask 是带优先级的任务
type PriorityTask struct {
	Task     TaskFunc
	Priority int // 优先级，值越小优先级越高
}

// PriorityQueue 是优先级队列的实现
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
