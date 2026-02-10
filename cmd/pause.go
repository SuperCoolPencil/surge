package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/surge-downloader/surge/internal/config"
	"github.com/surge-downloader/surge/internal/core"
	"github.com/surge-downloader/surge/internal/engine/state"
)

var pauseCmd = &cobra.Command{
	Use:   "pause <ID>",
	Short: "Pause a download",
	Long:  `Pause a download by its ID. Use --all to pause all downloads.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initializeGlobalState()

		all, _ := cmd.Flags().GetBool("all")

		if !all && len(args) == 0 {
			fmt.Fprintln(os.Stderr, "Error: provide a download ID or use --all")
			os.Exit(1)
		}

		port := readActivePort()

		var service *core.RemoteDownloadService
		if port > 0 {
			tokenFile := filepath.Join(config.GetSurgeDir(), "token")
			tokenBytes, _ := os.ReadFile(tokenFile)
			token := strings.TrimSpace(string(tokenBytes))
			service = core.NewRemoteDownloadService(fmt.Sprintf("http://127.0.0.1:%d", port), token)
		}

		if all {
			// Pause all downloads
			if port > 0 {
				// Send to running server
				if err := service.PauseAll(); err != nil {
					fmt.Fprintf(os.Stderr, "Error pausing all downloads: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("All active downloads paused.")
			} else {
				// Offline mode: update DB directly
				if err := state.PauseAllDownloads(); err != nil {
					fmt.Fprintf(os.Stderr, "Error pausing downloads: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("All downloads paused.")
			}
			return
		}

		id := args[0]

		// Resolve partial ID to full ID
		id, err := resolveDownloadID(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if port > 0 {
			// Send to running server
			if err := service.Pause(id); err != nil {
				fmt.Fprintf(os.Stderr, "Error pausing download: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Paused download %s\n", id[:8])
		} else {
			// Offline mode: update DB directly
			if err := state.UpdateStatus(id, "paused"); err != nil {
				fmt.Fprintf(os.Stderr, "Error pausing download: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Paused download %s (offline mode)\n", id[:8])
		}
	},
}

func init() {
	rootCmd.AddCommand(pauseCmd)
	pauseCmd.Flags().Bool("all", false, "Pause all downloads")
}
