package cmd

import (
	"context"
	"fmt"
	"os"

	"surge/internal/downloader"
	"surge/internal/messages"
	"surge/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// Shared channel for download events (start/complete/error)
var eventCh chan tea.Msg
var program *tea.Program

// initTUI sets up the shared event channel and BubbleTea program
func initTUI() {
	eventCh = make(chan tea.Msg, DefaultProgressChannelBuffer)
	program = tea.NewProgram(tui.InitialRootModel(), tea.WithAltScreen())

	// Pump events to TUI
	go func() {
		for msg := range eventCh {
			program.Send(msg)
		}
	}()
}

// runTUI starts the TUI and blocks until quit
func runTUI() error {
	_, err := program.Run()
	return err
}

var getCmd = &cobra.Command{
	Use:   "get [url]",
	Short: "get downloads a file from a URL",
	Long:  `get downloads a file from a URL and saves it to the local filesystem.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]
		outPath, _ := cmd.Flags().GetString("path")
		verbose, _ := cmd.Flags().GetBool("verbose")
		// concurrent, _ := cmd.Flags().GetInt("concurrent") Have to implement this later
		md5sum, _ := cmd.Flags().GetString("md5")
		sha256sum, _ := cmd.Flags().GetString("sha256")

		if outPath == "" {
			outPath = "."
		}

		initTUI()
		ctx := context.Background()
		go func() {
			defer close(eventCh)
			if err := downloader.Download(ctx, url, outPath, verbose, md5sum, sha256sum, eventCh, 1); err != nil {
				program.Send(messages.DownloadErrorMsg{DownloadID: 1, Err: err})
			}
		}()

		if err := runTUI(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	getCmd.Flags().StringP("path", "p", "", "the path to the download folder")
	getCmd.Flags().IntP("concurrent", "c", DefaultConcurrentConnections, "number of concurrent connections (1 = single thread)")
	getCmd.Flags().BoolP("verbose", "v", false, "enable verbose output")
	getCmd.Flags().String("md5", "", "MD5 checksum for verification")
	getCmd.Flags().String("sha256", "", "SHA256 checksum for verification")
}
