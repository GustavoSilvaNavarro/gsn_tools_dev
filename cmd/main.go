package main

import (
	"fmt"
	"os"

	"gsn-dev-tools/internals/files"
	"gsn-dev-tools/pkg/gh"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "gsn",
		Short: "A small cli program to run my most usual tools and commands in my day to day as a Software Engineer",
	}

	// Define a command that accepts one argument
	var showCmd = &cobra.Command{
		Use:   "show [text]",
		Short: "Print the provided text",
		Run: func(cmd *cobra.Command, args []string) {
			name, _ := cmd.Flags().GetString("name")
			if name == "" {
				fmt.Println("No name provided. Use -n or --name.")
				return
			}
			fmt.Printf("Hello, %s!\n", name)
		},
	}

	showCmd.Flags().StringP("name", "n", "", "Name to print")

	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(gh.ApproveGhPrs())
	rootCmd.AddCommand(files.FileUpdateCmd())
	rootCmd.AddCommand(files.CompressionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
