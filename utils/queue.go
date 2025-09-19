package utils

import "container/list"

type Queue[T any] struct {
	items *list.List
}

func NewQueue[T any]() *Queue[T] {
	return &Queue[T]{
		items: list.New(),
	}
}

func (q *Queue[T]) PushFront(item T) {
	q.items.PushFront(item) // O(1)
}

func (q *Queue[T]) Enqueue(item T) {
	q.items.PushBack(item) // O(1)
}

func (q *Queue[T]) Dequeue() *T {
	front := q.items.Front()
	if front != nil {
		value := q.items.Remove(front).(T)
		return &value
	}
	return nil
}

func (q *Queue[T]) RemoveValue(targetValue T) bool {
	for e := q.items.Front(); e != nil; e = e.Next() {
		if any(e.Value) == any(targetValue) {
			q.items.Remove(e)
			return true
		}
	}
	return false
}

func (q *Queue[T]) GetFront() *T {
	front := q.items.Front()
	if front != nil {
		value := front.Value.(T)
		return &value
	}
	return nil
}

func (q *Queue[T]) Has(targetValue T, delete bool) bool {
	for e := q.items.Front(); e != nil; e = e.Next() {
		if any(e.Value) == any(targetValue) {
			if delete {
				q.items.Remove(e)
			}
			return true
		}
	}
	return false
}

func (q *Queue[T]) DeepClone() *Queue[T] {
	newQueue := NewQueue[T]()
	for e := q.items.Front(); e != nil; e = e.Next() {
		newQueue.Enqueue(e.Value.(T))
	}
	return newQueue
}

func (q *Queue[T]) IsEmpty() bool {
	return q.items == nil || q.items.Len() == 0
}

func (q *Queue[T]) Size() int {
	return q.items.Len()
}
