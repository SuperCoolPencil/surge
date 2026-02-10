package concurrent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/surge-downloader/surge/internal/engine/types"
)

func TestReportMirrorError(t *testing.T) {
	// Setup
	state := types.NewProgressState("test-id", 1000)
	mirrors := []types.MirrorStatus{
		{URL: "http://mirror1.com", Active: true, Error: false},
		{URL: "http://mirror2.com", Active: true, Error: false},
	}
	state.SetMirrors(mirrors)

	d := &ConcurrentDownloader{
		State: state,
	}

	// Test case 1: Report error for existing mirror
	d.ReportMirrorError("http://mirror1.com")
	updatedMirrors := d.State.GetMirrors()
	assert.True(t, updatedMirrors[0].Error, "Mirror 1 should be marked as error")
	assert.False(t, updatedMirrors[1].Error, "Mirror 2 should not be marked as error")

	// Test case 2: Report error for non-existent mirror (should not panic or change anything)
	d.ReportMirrorError("http://mirror3.com")
	updatedMirrors = d.State.GetMirrors()
	assert.Len(t, updatedMirrors, 2)
	assert.True(t, updatedMirrors[0].Error)
	assert.False(t, updatedMirrors[1].Error)

	// Test case 3: Report error for already errored mirror (should stay errored)
	d.ReportMirrorError("http://mirror1.com")
	updatedMirrors = d.State.GetMirrors()
	assert.True(t, updatedMirrors[0].Error)

	// Test case 4: Nil state (should return early without panic)
	dNil := &ConcurrentDownloader{State: nil}
	assert.NotPanics(t, func() {
		dNil.ReportMirrorError("http://mirror1.com")
	})
}
