package gh

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func ApproveGhPrs() *cobra.Command {
	var message string

	approveCmd := &cobra.Command{
		Use:   "approve <PR_URL>",
		Short: "Approve a GitHub PR with optional message",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			prURL := args[0]

			// Construct the gh command
			var ghCmd *exec.Cmd
			if message != "" {
				ghCmd = exec.Command("gh", "pr", "review", prURL, "--approve", "--body", message)
			} else {
				ghCmd = exec.Command("gh", "pr", "review", prURL, "--approve")
			}

			// Attach stdout and stderr so you can see gh output
			ghCmd.Stdout = os.Stdout
			ghCmd.Stderr = os.Stderr

			// Run the command
			if err := ghCmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "☠️ Failed to approve the PR: %v\n", err.Error())
			} else {
				fmt.Println("🎉 Pull Request approved successfully!")
			}
		},
	}

	approveCmd.Flags().StringVarP(&message, "message", "m", "", "Optional review message")
	return approveCmd
}
