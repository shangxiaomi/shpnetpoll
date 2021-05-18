package queue

// Task is a asynchronous function.
type Task func() error

// AsyncTaskQueue is a queue storing asynchronous tasks.
type AsyncTaskQueue interface {
	Enqueue(Task)
	Dequeue() Task
	Empty() bool
}
