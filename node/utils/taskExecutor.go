package utils

import (
	"container/heap"
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"
)

// TaskFunc 是 Goroutine 池中执行的任务函数类型 (任务函数类型)
type TaskFunc func()

// TaskExecutor 是 Goroutine 池的接口 (任务执行器接口)
type TaskExecutor interface {
	Submit(task TaskFunc) error                                   // 提交任务
	SubmitWithPriority(task TaskFunc, priority int) error         // 提交带优先级的任务
	SubmitWithTimeout(task TaskFunc, timeout time.Duration) error // 提交带超时的任务
	SubmitWithRetry(task TaskFunc, retries int) error             // 提交带重试的任务
	Resize(size int) error                                        // 调整池大小
	Stats() PoolStats                                             // 获取池状态
	PrintPoolStats()
	Release() // 释放池
}

// AntsExecutor 是 ants.Pool 的封装，实现 TaskExecutor 接口 (Ants执行器，封装了ants.Pool)
type AntsExecutor struct {
	pool          *ants.Pool     // ants 池
	mu            sync.RWMutex   // 读写锁
	priorityQueue *PriorityQueue // 优先级队列
	stats         PoolStats      // 统计信息
	logger        Logger         // 日志记录器
}

// NewAntsExecutor 创建一个新的 AntsExecutor (创建新的Ants执行器)
func NewAntsExecutor(size int, logger Logger) (TaskExecutor, error) {
	pool, err := ants.NewPool(size) // 创建 ants 池
	if err != nil {
		return nil, err
	}
	taskQueue := &PriorityQueue{} // 创建优先级队列
	heap.Init(taskQueue)          // 初始化优先级队列

	return &AntsExecutor{
		pool:          pool,
		priorityQueue: taskQueue,
		stats:         PoolStats{},
		logger:        logger,
	}, nil
}

// getFunctionName 获取函数的名称（通过反射）(获取函数名称，使用反射)
var functionNameCache sync.Map // 函数名称缓存

func getFunctionName(fn interface{}) string {
	fnPointer := reflect.ValueOf(fn).Pointer()             // 获取函数指针
	if name, ok := functionNameCache.Load(fnPointer); ok { // 从缓存中获取
		return name.(string)
	}

	fnValue := reflect.ValueOf(fn)      // 获取函数值
	if fnValue.Kind() != reflect.Func { // 检查是否是函数
		return "unknown"
	}
	fullName := runtime.FuncForPC(fnValue.Pointer()).Name() // 获取完整函数名
	if strings.Contains(fullName, "func") {                 // 匿名函数处理
		fullName = "anonymous-" + fullName
	}

	// 避免每次都分配新的切片，重用执行器的切片 (注释已删除，因为没有实际重用)
	parts := strings.Split(fullName, ".")
	name := parts[len(parts)-1]
	functionNameCache.Store(fnPointer, name) // 缓存函数名
	return name
}

// wrapTask 任务包装函数 (包装任务函数，添加统计和日志)
func (e *AntsExecutor) wrapTask(task TaskFunc, priority int) func() {
	taskName := getFunctionName(task)
	logPrefix := fmt.Sprintf("Task %s", taskName)
	if priority > 0 {
		logPrefix = fmt.Sprintf("Task %s (priority %d)", taskName, priority)
	}

	logStart := func() {
		e.logger.Debug(logPrefix + " started")
	}

	logFinish := func(duration time.Duration) {
		e.logger.Debug(fmt.Sprintf("%s finished in %v", logPrefix, duration))
	}

	return func() {
		start := time.Now()
		atomic.AddInt32(&e.stats.TotalTasks, 1)
		logStart()

		defer func() {
			duration := time.Since(start)
			logFinish(duration)

			if r := recover(); r != nil {
				e.logger.Errorf("%s panic: %v", logPrefix, r)
				atomic.AddInt32(&e.stats.FailedTasks, 1)
			} else {
				atomic.AddInt32(&e.stats.CompletedTasks, 1)
			}

			e.updateStats(duration)
			atomic.StoreInt32(&e.stats.Running, int32(e.pool.Running()))
		}()

		task() // 执行任务
	}
}

// updateStats 更新统计信息 (更新统计信息)
func (e *AntsExecutor) updateStats(duration time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.TaskDuration = duration
	if duration > e.stats.MaxTaskDuration {
		e.stats.MaxTaskDuration = duration
	}

	completedTasks := atomic.LoadInt32(&e.stats.CompletedTasks)
	if completedTasks > 0 {
		e.stats.AvgTaskDuration = time.Duration(
			(int64(e.stats.AvgTaskDuration)*int64(completedTasks-1) + int64(duration)) / int64(completedTasks),
		)
	}
}

// Submit 提交任务到 Goroutine 池 (提交任务)
func (e *AntsExecutor) Submit(task TaskFunc) error {
	wrappedTask := e.wrapTask(task, 0) // 包装任务
	return e.pool.Submit(wrappedTask)  // 提交到 ants 池
}

func (e *AntsExecutor) SubmitWithPriority(task TaskFunc, priority int) error {
	wrappedTask := e.wrapTask(task, priority)

	e.mu.Lock()
	defer e.mu.Unlock()

	// 直接创建对象，避免复用
	heap.Push(e.priorityQueue, &PriorityTask{
		Task:       wrappedTask,
		Priority:   priority,
		SubmitTime: time.Now(),
	})

	go e.executePriorityTask()
	return nil
}

// executePriorityTask attempts to execute a task from the priority queue (尝试从优先级队列执行任务)
func (e *AntsExecutor) executePriorityTask() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.priorityQueue.Len() == 0 {
		return
	}

	pt := heap.Pop(e.priorityQueue).(*PriorityTask)
	if err := e.pool.Submit(pt.Task); err != nil {
		e.logger.Errorf("提交优先级任务失败: %v", err)
	}
}

// SubmitWithTimeout 优化后的超时实现 (提交带超时的任务)
func (e *AntsExecutor) SubmitWithTimeout(task TaskFunc, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout) // 创建带超时的 context
	defer cancel()                                                    // 延迟取消 context

	wrappedTask := e.wrapTask(task, 0) // 包装任务
	errChan := make(chan error, 1)     // 创建错误通道

	go func() {
		errChan <- e.pool.Submit(wrappedTask) // 提交到 ants 池，并将结果发送到错误通道
	}()

	select {
	case err := <-errChan: // 从错误通道接收结果
		return err
	case <-ctx.Done(): // 超时
		atomic.AddInt32(&e.stats.TimeoutTasks, 1)                  // 更新超时任务数
		e.logger.Warnf("Task %s timed out", getFunctionName(task)) // 记录警告日志
		return ctx.Err()                                           // 返回超时错误
	}
}

// SubmitWithRetry 内部重试实现 (提交带重试的任务)
func (e *AntsExecutor) SubmitWithRetry(task TaskFunc, retries int) error {
	wrappedFunc := func() { // 包装任务，添加重试逻辑
		var lastErr error
		for i := 0; i < retries; i++ { // 重试循环
			func() {
				defer func() { // 延迟执行，捕获 panic
					if r := recover(); r != nil { // 捕获 panic
						lastErr = fmt.Errorf("panic: %v", r) // 记录错误信息
					}
				}()
				task()        // 执行任务
				lastErr = nil // 重置错误
			}()
			if lastErr == nil { // 成功执行，退出循环
				return
			}
			time.Sleep(time.Duration(1<<i) * time.Second) // 指数退避
		}
		e.logger.Errorf("Task %s failed after %d retries: %v", getFunctionName(task), retries, lastErr) // 记录错误日志
		atomic.AddInt32(&e.stats.RetryTasks, 1)                                                         // 更新重试任务数
	}

	return e.pool.Submit(e.wrapTask(wrappedFunc, 0)) // 提交到 ants 池
}

// Resize 动态调整池大小 (调整池大小)
func (e *AntsExecutor) Resize(size int) error {
	if size < 1 { // 检查大小是否合法
		return fmt.Errorf("pool size must be greater than 0")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	currentSize := e.pool.Cap()                     // 获取当前池大小
	if abs(size-currentSize)*100/currentSize > 20 { // 如果变化超过 20%，则调整
		e.pool.Tune(size) // 调整池大小
	}
	return nil
}

func abs(x int) int { // 绝对值函数
	if x < 0 {
		return -x
	}
	return x
}

// Stats 返回池状态 (获取池状态)
func (e *AntsExecutor) Stats() PoolStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	stats := PoolStats{ // 构造池状态信息
		Running:           atomic.LoadInt32(&e.stats.Running),        // 获取正在运行的 Goroutine 数量
		Waiting:           int(e.pool.Waiting()),                     // 获取等待执行的任务数量
		TimeoutTasks:      atomic.LoadInt32(&e.stats.TimeoutTasks),   // 获取超时任务数
		RetryTasks:        atomic.LoadInt32(&e.stats.RetryTasks),     // 获取重试任务数
		FailedTasks:       atomic.LoadInt32(&e.stats.FailedTasks),    // 获取失败任务数
		TotalTasks:        atomic.LoadInt32(&e.stats.TotalTasks),     // 获取总任务数
		CompletedTasks:    atomic.LoadInt32(&e.stats.CompletedTasks), // 获取完成任务数
		TaskDuration:      e.stats.TaskDuration,                      // 获取最近一次任务执行时间
		AvgTaskDuration:   e.stats.AvgTaskDuration,                   // 获取平均任务执行时间
		MaxTaskDuration:   e.stats.MaxTaskDuration,                   // 获取最大任务执行时间
		PoolSize:          e.pool.Cap(),                              // 获取池大小
		PriorityQueueSize: e.priorityQueue.Len(),                     // 获取优先级队列大小 (新增)
	}
	return stats
}

// PrintPoolStats 格式化 PoolStats 信息并将其打印到标准输出。
func (e *AntsExecutor) PrintPoolStats() {
	stats := e.Stats() // Get the stats from the executor
	fmt.Println(stats.FormatPoolStats())
}

// Release 释放池 (释放池)
func (e *AntsExecutor) Release() {
	e.mu.Lock()
	defer e.mu.Unlock()

	timeout := time.After(5 * time.Second) // 设置超时时间
	for e.pool.Running() > 0 {             // 等待所有任务完成
		select {
		case <-timeout: // 超时
			e.logger.Error("Pool release timed out, force exiting") // 记录错误日志
			return
		default:
			time.Sleep(100 * time.Millisecond) // 短暂休眠
		}
	}
	e.pool.Release() // 释放 ants 池
}

// PoolStats 状态统计 (池状态统计)
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

	PoolSize          int // Goroutine 池的当前大小
	PriorityQueueSize int // 优先级队列中的任务数量 (新增)
}

// FormatPoolStats 自动格式化 PoolStats 信息并返回字符串。
func (s PoolStats) FormatPoolStats() string {
	var sb strings.Builder
	sb.WriteString("池统计信息:\n")

	// 使用反射遍历结构体字段
	v := reflect.ValueOf(s)
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// 格式化字段值
		formattedValue := formatFieldValue(field, value)

		// 将字段名和值写入字符串构建器
		sb.WriteString(fmt.Sprintf("  %s: %s\n", field.Name, formattedValue))
	}

	return sb.String()
}

// formatFieldValue 格式化字段值
func formatFieldValue(field reflect.StructField, value reflect.Value) string {
	switch {
	// 判断字段类型是否为 time.Duration
	case field.Type == reflect.TypeOf(time.Duration(0)):
		return formatDuration(value.Interface().(time.Duration))

	// 处理其他类型
	case value.Kind() == reflect.Int, value.Kind() == reflect.Int32, value.Kind() == reflect.Int64:
		return strconv.FormatInt(value.Int(), 10)
	case value.Kind() == reflect.String:
		return value.String()
	default:
		return fmt.Sprintf("%v", value.Interface())
	}
}

// formatDuration 将 time.Duration 格式化为易读的时间字符串
func formatDuration(d time.Duration) string {
	// 如果时间小于 1 秒，显示毫秒
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	// 否则显示小时、分钟、秒
	return d.String()
}

// PriorityQueue 及相关实现 (优先级队列及相关实现)
type PriorityTask struct {
	Task       func()    // 任务函数
	Priority   int       // 优先级
	index      int       // 在堆中的索引
	SubmitTime time.Time // 任务提交时间
}

type PriorityQueue []*PriorityTask

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	if pq[i].Priority == pq[j].Priority {
		return pq[i].SubmitTime.Before(pq[j].SubmitTime) // 如果优先级相同，按提交时间排序
	}
	return pq[i].Priority < pq[j].Priority // 否则按优先级排序
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i // 更新索引
	pq[j].index = j // 更新索引
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*PriorityTask)
	item.index = n // 设置索引
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // 避免内存泄漏
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}
