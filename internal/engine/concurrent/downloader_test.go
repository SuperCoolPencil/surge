package concurrent

import (
	"testing"

	"github.com/surge-downloader/surge/internal/engine/types"
)

func TestGetInitialConnections(t *testing.T) {
	tests := []struct {
		name         string
		fileSize     int64
		maxConns     int
		minChunkSize int64
		expected     int
	}{
		{
			name:         "Zero Size",
			fileSize:     0,
			maxConns:     16,
			minChunkSize: types.MB,
			expected:     1,
		},
		{
			name:         "Negative Size",
			fileSize:     -1,
			maxConns:     16,
			minChunkSize: types.MB,
			expected:     1,
		},
		{
			name:         "Small File (1MB)",
			fileSize:     1 * types.MB,
			maxConns:     16,
			minChunkSize: types.MB,
			expected:     1, // sqrt(1) = 1
		},
		{
			name:         "Medium File (100MB)",
			fileSize:     100 * types.MB,
			maxConns:     16,
			minChunkSize: types.MB,
			expected:     10, // sqrt(100) = 10
		},
		{
			name:         "Max Connections Limit",
			fileSize:     100 * types.MB,
			maxConns:     4,
			minChunkSize: types.MB,
			expected:     4, // limited by maxConns
		},
		{
			name:         "Min Chunk Size Constraint",
			fileSize:     10 * types.MB,
			maxConns:     16,
			minChunkSize: 5 * types.MB,
			expected:     2, // max chunks = 10/5 = 2. sqrt(10) = 3. capped at 2.
		},
		{
			name:         "Min Chunk Size Larger Than File",
			fileSize:     4 * types.MB,
			maxConns:     16,
			minChunkSize: 5 * types.MB,
			expected:     1, // max chunks = 4/5 = 0 -> 1.
		},
		{
			name:         "Large File (1GB)",
			fileSize:     1024 * types.MB,
			maxConns:     64,
			minChunkSize: 10 * types.MB,
			expected:     32, // sqrt(1024) = 32. 1024/10 = 102 chunks max. 32 < 64.
		},
		{
			name:         "Very Large File (10GB) with High Max Conns",
			fileSize:     10 * 1024 * types.MB,
			maxConns:     100,
			minChunkSize: 10 * types.MB,
			expected:     100, // sqrt(10240) = ~101. Limited by maxConns 100.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := &types.RuntimeConfig{
				MaxConnectionsPerHost: tt.maxConns,
				MinChunkSize:          tt.minChunkSize,
			}
			d := &ConcurrentDownloader{
				Runtime: runtime,
			}

			got := d.getInitialConnections(tt.fileSize)
			if got != tt.expected {
				t.Errorf("getInitialConnections(%d) = %d; want %d", tt.fileSize, got, tt.expected)
			}
		})
	}
}
