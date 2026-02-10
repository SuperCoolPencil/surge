package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/surge-downloader/surge/internal/engine/events"
)

func BenchmarkUpdateProgress(b *testing.B) {
	// Initialize a RootModel with some active downloads
	activeDownload := NewDownloadModel("active-id", "http://example.com/file", "file.zip", 1024*1024)
	activeDownload.Speed = 100 // Active
	activeDownload.Connections = 1

	downloads := []*DownloadModel{activeDownload}

	// Add some dummy active downloads to make the list populated
	for i := 0; i < 50; i++ {
		dm := NewDownloadModel(fmt.Sprintf("id-%d", i), "url", fmt.Sprintf("file-%d", i), 100)
		dm.Speed = 100 // Active
		dm.Connections = 1
		downloads = append(downloads, dm)
	}

	m := RootModel{
		downloads:    downloads,
		list:         NewDownloadList(100, 20),
		activeTab:    TabActive,
		speedBuffer:  make([]float64, 0, 10),
		SpeedHistory: make([]float64, 10),
		// Initial speed history update time
		lastSpeedHistoryUpdate: time.Now(),
	}

	// Ensure list items are initially populated
	m.UpdateListItems()

	msg := events.ProgressMsg{
		DownloadID:        "active-id",
		Downloaded:        50,
		Total:             100,
		Speed:             200, // Still active
		ActiveConnections: 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// We call Update. Note that Update returns a new model, but since we are testing
		// the performance of the function call itself and its side effects (like UpdateListItems which might happen),
		// we can discard the result or use it.
		// However, RootModel methods modify the receiver's pointer fields (like downloads slice elements),
		// but Update takes value receiver.
		// So m itself (the struct) is copied.
		// But m.downloads is a slice of pointers, so the underlying DownloadModels are modified.
		// m.list is a struct, so it is copied. m.UpdateListItems modifies m.list.SetItems().
		// If we don't update m, the next iteration starts with original m.list.
		// This means m.UpdateListItems is called on the original list every time.
		// This is what we want to measure: the cost of calling UpdateListItems.

		_, _ = m.Update(msg)
	}
}
