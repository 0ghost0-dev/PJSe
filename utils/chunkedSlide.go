package utils

import "sync"

type ChunkedStore[T any] struct {
	chunks    [][]T
	chunkSize int
	totalSize int
	maxMemory int64
	mutex     sync.RWMutex
}

func NewChunkedStore[T any](chunkSize int) *ChunkedStore[T] {
	return &ChunkedStore[T]{
		chunks:    make([][]T, 0),
		chunkSize: chunkSize,
	}
}

func (cs *ChunkedStore[T]) Append(item T) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if len(cs.chunks) == 0 || len(cs.chunks[len(cs.chunks)-1]) >= cs.chunkSize {
		newChunk := make([]T, 0, cs.chunkSize)
		cs.chunks = append(cs.chunks, newChunk)
	}

	lastIndex := len(cs.chunks) - 1
	cs.chunks[lastIndex] = append(cs.chunks[lastIndex], item)
	cs.totalSize++
}

func (cs *ChunkedStore[T]) GetLatest(count int) []T {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	if count >= cs.totalSize {
		return cs.getAllData()
	}

	result := make([]T, 0, count)
	remaining := count

	for i := len(cs.chunks) - 1; i >= 0 && remaining > 0; i-- {
		chunk := cs.chunks[i]
		chunkLen := len(chunk)

		if remaining >= chunkLen {
			for j := chunkLen - 1; j >= 0; j-- {
				result = append(result, chunk[j])
			}
			remaining -= chunkLen
		} else {
			for j := chunkLen - 1; j >= chunkLen-remaining; j-- {
				result = append(result, chunk[j])
			}
			break
		}
	}

	return result
}

func (cs *ChunkedStore[T]) GetMostRecent() T {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	var zero T
	if cs.totalSize == 0 {
		return zero
	}

	lastChunk := cs.chunks[len(cs.chunks)-1]
	mostRecent := lastChunk[len(lastChunk)-1]
	return mostRecent
}

func (cs *ChunkedStore[T]) GetRange(start, end int) []T {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	if start < 0 || start >= cs.totalSize || end <= start {
		return nil
	}

	if end > cs.totalSize {
		end = cs.totalSize
	}

	result := make([]T, 0, end-start)
	currentPos := 0

	for _, chunk := range cs.chunks {
		chunkLen := len(chunk)
		chunkEnd := currentPos + chunkLen

		if currentPos >= end {
			break
		}

		if chunkEnd > start {
			chunkStart := max(0, start-currentPos)
			chunkEndPos := min(chunkLen, end-currentPos)

			for i := chunkStart; i < chunkEndPos; i++ {
				result = append(result, chunk[i])
			}
		}

		currentPos = chunkEnd
	}

	return result
}

func (cs *ChunkedStore[T]) Size() int {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	return cs.totalSize
}

func (cs *ChunkedStore[T]) getAllData() []T {
	result := make([]T, 0, cs.totalSize)
	for _, chunk := range cs.chunks {
		result = append(result, chunk...)
	}
	return result
}
