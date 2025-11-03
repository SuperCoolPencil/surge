package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	outPath string
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "get downloads a file from a URL",
	Long:  `get downloads a file from a URL and saves it to the local filesystem.`,
	Run: func(cmd *cobra.Command, args []string) {
		get(cmd *cobra.Command, args []string)
	},
}

func init() {
	rootCmd.AddCommand(getCmd)

}

func get(cmd *cobra.Command, args []string) {
	if args[0] == "" {
		outPath = "downloads/"
	} else {
		outPath = args[0]
	}

	fmt.Println("Output Path set to :", outPath)
}
