package utils

import "container/list"

type Queue struct {
	items *list.List
}

func NewQueue() *Queue {
	return &Queue{
		items: list.New(),
	}
}

func (q *Queue) Enqueue(item interface{}) {
	q.items.PushBack(item) // O(1)
}

func (q *Queue) Dequeue() interface{} {
	front := q.items.Front()
	if front != nil {
		return q.items.Remove(front) // O(1)
	}
	return nil
}

func (q *Queue) RemoveValue(targetValue interface{}) bool {
	for e := q.items.Front(); e != nil; e = e.Next() {
		if e.Value == targetValue {
			q.items.Remove(e)
			return true
		}
	}
	return false
}

func (q *Queue) IsEmpty() bool {
	return q.items.Len() == 0
}

func (q *Queue) Size() int {
	return q.items.Len()
}
