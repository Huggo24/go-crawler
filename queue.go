package main

import (
	"net/url"
	"sync"
)

type Task struct {
	URL   *url.URL
	depth int
}

type Queue struct {
	items   []Task
	mu      sync.Mutex
	cond    *sync.Cond
	closed  bool
	pending int
}

func (q *Queue) Push(t Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.items = append(q.items, t)
	q.pending++
	q.cond.Signal()
}

func (q *Queue) Pop() (Task, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.items) == 0 {
		if q.closed {
			return Task{}, false
		}
		q.cond.Wait()
	}

	task := q.items[0]
	q.items = q.items[1:]

	return task, true
}

func (q *Queue) Done() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.pending--
	if q.pending == 0 && len(q.items) == 0 {
		q.closed = true
		q.cond.Broadcast()
	}
}
