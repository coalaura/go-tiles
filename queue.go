package gotiles

import (
	"fmt"
	"runtime"
	"sync"
)

type Queue struct {
	workers int
	jobs    chan func()
	wg      sync.WaitGroup
}

func NewQueue() *Queue {
	workers := runtime.NumCPU() * 2

	q := &Queue{
		workers: workers,
		jobs:    make(chan func(), workers),
	}

	for i := 0; i < workers; i++ {
		go func() {
			var job func()

			for job = range q.jobs {
				job()

				q.wg.Done()
			}
		}()
	}

	fmt.Printf("Queue ready with %d workers\n", workers)

	return q
}

func (q *Queue) Work(fn func()) {
	q.wg.Add(1)

	q.jobs <- fn
}

func (q *Queue) Wait() {
	q.wg.Wait()
}
