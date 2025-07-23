package agent

import (
	"container/heap"
	"sync"
	"time"
)

// TaskQueue manages the prioritized task queue
type TaskQueue struct {
	items    []*queueItem
	itemMap  map[string]*queueItem
	mu       sync.Mutex
	notEmpty chan struct{}
}

// queueItem wraps a task with priority queue metadata
type queueItem struct {
	task     *Task
	priority int
	index    int
}

// NewTaskQueue creates a new task queue
func NewTaskQueue() *TaskQueue {
	q := &TaskQueue{
		items:    make([]*queueItem, 0),
		itemMap:  make(map[string]*queueItem),
		notEmpty: make(chan struct{}, 1),
	}
	heap.Init(q)
	return q
}

// Add adds a task to the queue
func (q *TaskQueue) Add(task *Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if task already exists
	if _, exists := q.itemMap[task.ID]; exists {
		return nil // Skip duplicate
	}

	item := &queueItem{
		task:     task,
		priority: task.Priority,
	}

	heap.Push(q, item)
	q.itemMap[task.ID] = item

	// Signal that queue is not empty
	select {
	case q.notEmpty <- struct{}{}:
	default:
	}

	return nil
}

// Next retrieves the next highest priority task
func (q *TaskQueue) Next() *Task {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.Len() == 0 {
		return nil
	}

	item := heap.Pop(q).(*queueItem)
	delete(q.itemMap, item.task.ID)

	return item.task
}

// NextWithWait retrieves the next task, waiting if queue is empty
func (q *TaskQueue) NextWithWait(timeout time.Duration) *Task {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		if task := q.Next(); task != nil {
			return task
		}

		select {
		case <-q.notEmpty:
			continue
		case <-timer.C:
			return nil
		}
	}
}

// Remove removes a task from the queue
func (q *TaskQueue) Remove(taskID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	item, exists := q.itemMap[taskID]
	if !exists {
		return false
	}

	heap.Remove(q, item.index)
	delete(q.itemMap, taskID)

	return true
}

// Size returns the number of tasks in the queue
func (q *TaskQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.Len()
}

// Implement heap.Interface
func (q *TaskQueue) Len() int { return len(q.items) }

func (q *TaskQueue) Less(i, j int) bool {
	// Higher priority = lower number = comes first
	if q.items[i].priority != q.items[j].priority {
		return q.items[i].priority > q.items[j].priority
	}
	// If same priority, older tasks come first
	return q.items[i].task.CreatedAt.Before(q.items[j].task.CreatedAt)
}

func (q *TaskQueue) Swap(i, j int) {
	q.items[i], q.items[j] = q.items[j], q.items[i]
	q.items[i].index = i
	q.items[j].index = j
}

func (q *TaskQueue) Push(x interface{}) {
	item := x.(*queueItem)
	item.index = len(q.items)
	q.items = append(q.items, item)
}

func (q *TaskQueue) Pop() interface{} {
	old := q.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	q.items = old[0 : n-1]
	return item
}
