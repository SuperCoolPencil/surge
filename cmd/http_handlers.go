package cmd

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/surge-downloader/surge/internal/config"
	"github.com/surge-downloader/surge/internal/core"
	"github.com/surge-downloader/surge/internal/engine/events"
	"github.com/surge-downloader/surge/internal/utils"
)

// DownloadRequest represents a download request from the browser extension
type DownloadRequest struct {
	URL                  string            `json:"url"`
	Filename             string            `json:"filename,omitempty"`
	Path                 string            `json:"path,omitempty"`
	RelativeToDefaultDir bool              `json:"relative_to_default_dir,omitempty"`
	Mirrors              []string          `json:"mirrors,omitempty"`
	SkipApproval         bool              `json:"skip_approval,omitempty"` // Extension validated request, skip TUI prompt
	Headers              map[string]string `json:"headers,omitempty"`       // Custom HTTP headers from browser (cookies, auth, etc.)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	}); err != nil {
		utils.Debug("Failed to encode response: %v", err)
	}
}

func EventsHandler(service core.DownloadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		// Access-Control-Allow-Origin handled by corsMiddleware

		// Get event stream
		stream, cleanup, err := service.StreamEvents(r.Context())
		if err != nil {
			http.Error(w, "Failed to subscribe to events", http.StatusInternalServerError)
			return
		}
		defer cleanup()

		// Flush headers immediately
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}
		flusher.Flush()

		// Send events
		// Create a closer notifier
		done := r.Context().Done()

		for {
			select {
			case <-done:
				return
			case msg, ok := <-stream:
				if !ok {
					return
				}

				// Encode message to JSON
				data, err := json.Marshal(msg)
				if err != nil {
					utils.Debug("Error marshaling event: %v", err)
					continue
				}

				// Determine event type name based on struct
				// Events are in internal/engine/events package
				eventType := "unknown"
				switch msg.(type) {
				case events.DownloadStartedMsg:
					eventType = "started"
				case events.DownloadCompleteMsg:
					eventType = "complete"
				case events.DownloadErrorMsg:
					eventType = "error"
				case events.ProgressMsg:
					eventType = "progress"
				case events.DownloadPausedMsg:
					eventType = "paused"
				case events.DownloadResumedMsg:
					eventType = "resumed"
				case events.DownloadQueuedMsg:
					eventType = "queued"
				case events.DownloadRemovedMsg:
					eventType = "removed"
				case events.DownloadRequestMsg:
					eventType = "request"
				}

				// SSE Format:
				// event: <type>
				// data: <json>
				// \n
				_, _ = fmt.Fprintf(w, "event: %s\n", eventType)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}

func DownloadHandler(defaultOutputDir string, service core.DownloadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handleDownload(w, r, defaultOutputDir, service)
	}
}

func PauseHandler(service core.DownloadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}

		if err := service.Pause(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "paused", "id": id}); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
	}
}

func PauseAllHandler(service core.DownloadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := service.PauseAll(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "paused_all"}); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
	}
}

func ResumeHandler(service core.DownloadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}

		if err := service.Resume(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "resumed", "id": id}); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
	}
}

func DeleteHandler(service core.DownloadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete && r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}

		if err := service.Delete(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": id}); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
	}
}

func ListHandler(service core.DownloadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		statuses, err := service.List()
		if err != nil {
			http.Error(w, "Failed to list downloads: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(statuses); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
	}
}

func HistoryHandler(service core.DownloadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		history, err := service.History()
		if err != nil {
			http.Error(w, "Failed to retrieve history: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(history); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		origin := r.Header.Get("Origin")
		if checkOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS, PUT, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("X-Surge-Server", "true")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func checkOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	// Allow browser extensions
	if strings.HasPrefix(origin, "chrome-extension://") ||
		strings.HasPrefix(origin, "moz-extension://") ||
		strings.HasPrefix(origin, "safari-web-extension://") {
		return true
	}
	// Allow local development/web UI
	if origin == "http://localhost" || strings.HasPrefix(origin, "http://localhost:") {
		return true
	}
	if origin == "http://127.0.0.1" || strings.HasPrefix(origin, "http://127.0.0.1:") {
		return true
	}
	return false
}

func authMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow OPTIONS for CORS preflight
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		// Check for Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			if strings.HasPrefix(authHeader, "Bearer ") {
				providedToken := strings.TrimPrefix(authHeader, "Bearer ")
				if len(providedToken) == len(token) && subtle.ConstantTimeCompare([]byte(providedToken), []byte(token)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

func ensureAuthToken() string {
	tokenFile := filepath.Join(config.GetSurgeDir(), "token")
	data, err := os.ReadFile(tokenFile)
	if err == nil {
		return strings.TrimSpace(string(data))
	}

	// Generate new token
	token := uuid.New().String()
	if err := os.WriteFile(tokenFile, []byte(token), 0600); err != nil {
		utils.Debug("Failed to write token file: %v", err)
	}
	return token
}

func handleDownload(w http.ResponseWriter, r *http.Request, defaultOutputDir string, service core.DownloadService) {
	// GET request to query status
	if r.Method == http.MethodGet {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if service == nil {
			http.Error(w, "Service unavailable", http.StatusInternalServerError)
			return
		}

		status, err := service.GetStatus(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		if err := json.NewEncoder(w).Encode(status); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Load settings once for use throughout the function
	settings, err := config.LoadSettings()
	if err != nil {
		// Fallback to defaults if loading fails (though LoadSettings handles missing file)
		settings = config.DefaultSettings()
	}

	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			utils.Debug("Error closing body: %v", err)
		}
	}()

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	if strings.Contains(req.Path, "..") || strings.Contains(req.Filename, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	if strings.Contains(req.Filename, "/") || strings.Contains(req.Filename, "\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	utils.Debug("Received download request: URL=%s, Path=%s", utils.SanitizeURL(req.URL), req.Path)

	downloadID := uuid.New().String()
	if service == nil {
		http.Error(w, "Service unavailable", http.StatusInternalServerError)
		return
	}

	// Resolve base directory
	baseDir := settings.General.DefaultDownloadDir
	if baseDir == "" {
		baseDir = defaultOutputDir
	}
	if baseDir == "" {
		baseDir = "."
	}
	baseDir = utils.EnsureAbsPath(baseDir)

	// Prepare output path
	var outPath string
	if req.RelativeToDefaultDir && req.Path != "" {
		outPath = filepath.Join(baseDir, req.Path)
	} else if req.Path != "" {
		outPath = req.Path
	} else {
		outPath = baseDir
	}

	// Enforce absolute path to ensure resume works even if CWD changes
	outPath = utils.EnsureAbsPath(outPath)

	// SECURITY: Ensure the download directory is within the base directory
	// This prevents arbitrary file writes via the API
	rel, err := filepath.Rel(baseDir, outPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		http.Error(w, "Invalid path: download path must be within the default download directory", http.StatusForbidden)
		return
	}

	// Create directory
	if err := os.MkdirAll(outPath, 0o755); err != nil {
		http.Error(w, "Failed to create directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check settings for extension prompt and duplicates
	// Logic modified to distinguish between ACTIVE (corruption risk) and COMPLETED (overwrite safe)
	isDuplicate := false
	isActive := false

	urlForAdd := req.URL
	mirrorsForAdd := req.Mirrors
	if len(mirrorsForAdd) == 0 && strings.Contains(req.URL, ",") {
		urlForAdd, mirrorsForAdd = ParseURLArg(req.URL)
	}

	if GlobalPool.HasDownload(urlForAdd) {
		isDuplicate = true
		// Check if specifically active\
		allActive := GlobalPool.GetAll()
		for _, c := range allActive {
			if c.URL == urlForAdd {
				if c.State != nil && !c.State.Done.Load() {
					isActive = true
				}
				break
			}
		}
	}

	utils.Debug("Download request: URL=%s, SkipApproval=%v, isDuplicate=%v, isActive=%v", utils.SanitizeURL(urlForAdd), req.SkipApproval, isDuplicate, isActive)

	// EXTENSION VETTING SHORTCUT:
	// If SkipApproval is true, we trust the extension completely.
	// The backend will auto-rename duplicate files, so no need to reject.
	if req.SkipApproval {
		// Trust extension -> Skip all prompting logic, proceed to download
		utils.Debug("Extension request: skipping all prompts, proceeding with download")
	} else {
		// Logic for prompting:
		// 1. If ExtensionPrompt is enabled
		// 2. OR if WarnOnDuplicate is enabled AND it is a duplicate
		shouldPrompt := settings.General.ExtensionPrompt || (settings.General.WarnOnDuplicate && isDuplicate)

		// Only prompt if we have a UI running (serverProgram != nil)
		if shouldPrompt {
			if serverProgram != nil {
				utils.Debug("Requesting TUI confirmation for: %s (Duplicate: %v)", utils.SanitizeURL(req.URL), isDuplicate)

				// Send request to TUI
				if err := service.Publish(events.DownloadRequestMsg{
					ID:       downloadID,
					URL:      urlForAdd,
					Filename: req.Filename,
					Path:     outPath, // Use the path we resolved (default or requested)
					Mirrors:  mirrorsForAdd,
					Headers:  req.Headers,
				}); err != nil {
					http.Error(w, "Failed to notify TUI: "+err.Error(), http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				// Return 202 Accepted to indicate it's pending approval
				w.WriteHeader(http.StatusAccepted)
				if err := json.NewEncoder(w).Encode(map[string]string{
					"status":  "pending_approval",
					"message": "Download request sent to TUI for confirmation",
					"id":      downloadID, // ID might change if user modifies it, but useful for tracking
				}); err != nil {
					utils.Debug("Failed to encode response: %v", err)
				}
				return
			} else {
				// Headless mode check
				if settings.General.ExtensionPrompt || (settings.General.WarnOnDuplicate && isDuplicate) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusConflict)
					if err := json.NewEncoder(w).Encode(map[string]string{
						"status":  "error",
						"message": "Download rejected: Duplicate download or approval required (Headless mode)",
					}); err != nil {
						utils.Debug("Failed to encode response: %v", err)
					}
					return
				}
			}
		}
	}

	// Add via service
	newID, err := service.Add(urlForAdd, outPath, req.Filename, mirrorsForAdd, req.Headers)
	if err != nil {
		http.Error(w, "Failed to add download: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Increment active downloads counter
	atomic.AddInt32(&activeDownloads, 1)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "queued",
		"message": "Download queued successfully",
		"id":      newID,
	}); err != nil {
		utils.Debug("Failed to encode response: %v", err)
	}
}
