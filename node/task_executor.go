// task_executor.go
package main

import "github.com/panjf2000/ants/v2"

// TaskFunc 是 Goroutine 池中执行的任务函数类型
type TaskFunc func()

// TaskExecutor 是 Goroutine 池的接口
type TaskExecutor interface {
	Submit(task TaskFunc) error
	Release()
}

// AntsExecutor 是 ants.Pool 的封装，实现 TaskExecutor 接口
type AntsExecutor struct {
	executor *ants.Pool
}

// NewAntsExecutor 创建一个新的 AntsExecutor
func NewAntsExecutor(size int) (TaskExecutor, error) {
	pool, err := ants.NewPool(size)
	if err != nil {
		return nil, err
	}
	return &AntsExecutor{executor: pool}, nil
}

// Submit 提交任务到 Goroutine 池
func (e *AntsExecutor) Submit(task TaskFunc) error {
	return e.executor.Submit(task)
}

// Release 释放 Goroutine 池
func (e *AntsExecutor) Release() {
	e.executor.Release()
}
