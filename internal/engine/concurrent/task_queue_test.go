package concurrent

import (
	"testing"

	"github.com/surge-downloader/surge/internal/engine/types"
)

func TestTaskQueue_PushPop(t *testing.T) {
	q := NewTaskQueue()

	task := types.Task{Offset: 0, Length: 1000}
	q.Push(task)

	if q.Len() != 1 {
		t.Errorf("Len = %d, want 1", q.Len())
	}

	got, ok := q.Pop()
	if !ok {
		t.Error("Pop returned false, expected true")
	}
	if got.Offset != task.Offset || got.Length != task.Length {
		t.Errorf("Pop = %+v, want %+v", got, task)
	}
}

func TestTaskQueue_PushMultiple(t *testing.T) {
	q := NewTaskQueue()

	tasks := []types.Task{
		{Offset: 0, Length: 100},
		{Offset: 100, Length: 100},
		{Offset: 200, Length: 100},
	}
	q.PushMultiple(tasks)

	if q.Len() != 3 {
		t.Errorf("Len = %d, want 3", q.Len())
	}
}

func TestTaskQueue_IdleWorkers(t *testing.T) {
	q := NewTaskQueue()

	// Initially 0 idle workers
	if q.IdleWorkers() != 0 {
		t.Errorf("IdleWorkers = %d, want 0", q.IdleWorkers())
	}
}

func TestTaskQueue_Close(t *testing.T) {
	q := NewTaskQueue()
	q.Push(types.Task{Offset: 0, Length: 100})
	q.Close()

	// After close, Pop should still return existing tasks
	if _, ok := q.Pop(); !ok {
		t.Error("Pop should return existing task after Close")
	}

	// Additional Pop should return false
	if _, ok := q.Pop(); ok {
		t.Error("Pop should return false after draining closed queue")
	}
}

func TestTaskQueue_DrainRemaining(t *testing.T) {
	q := NewTaskQueue()

	tasks := []types.Task{
		{Offset: 0, Length: 100},
		{Offset: 100, Length: 100},
		{Offset: 200, Length: 100},
	}
	q.PushMultiple(tasks)

	remaining := q.DrainRemaining()

	if len(remaining) != 3 {
		t.Errorf("DrainRemaining returned %d tasks, want 3", len(remaining))
	}
	if q.Len() != 0 {
		t.Errorf("Queue should be empty after drain, Len = %d", q.Len())
	}
}

func TestAlignedSplitSize(t *testing.T) {
	tests := []struct {
		name      string
		remaining int64
		wantSize  int64
	}{
		{
			name:      "Too small (< MinChunk)",
			remaining: types.MinChunk,
			wantSize:  0,
		},
		{
			name:      "Exact min split",
			remaining: 2 * types.MinChunk,
			wantSize:  types.MinChunk,
		},
		{
			name:      "Larger split",
			remaining: 4 * types.MinChunk,
			wantSize:  2 * types.MinChunk,
		},
		{
			name:      "Odd split (aligns down)",
			remaining: 2*types.MinChunk + types.AlignSize,
			wantSize:  types.MinChunk,
		},
		{
			name:      "Odd split (aligns up)",
			remaining: 2*types.MinChunk + 2*types.AlignSize,
			wantSize:  types.MinChunk + types.AlignSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alignedSplitSize(tt.remaining)
			if got != tt.wantSize {
				t.Errorf("alignedSplitSize(%d) = %d, want %d", tt.remaining, got, tt.wantSize)
			}
			// Verify alignment
			if got != 0 && got%types.AlignSize != 0 {
				t.Errorf("alignedSplitSize(%d) = %d, not aligned to %d", tt.remaining, got, types.AlignSize)
			}
		})
	}
}
