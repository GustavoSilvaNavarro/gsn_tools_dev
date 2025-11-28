package files

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

func FileUpdateCmd() *cobra.Command {
	updateFilesCmd := cobra.Command{
		Use:   "rename <directory_path>",
		Short: "Renames and updates extensions of files in specific directories",
		Long:  `Applies rules: 1. Clean name (remove prefix/symbols), 2. Space to _, 3. Lowercase, 4. Set new extension.`,
		Args:  cobra.ExactArgs(1),
		Run:   UpdateAndRenameFilesInDirectory,
	}

	updateFilesCmd.Flags().StringP("extension", "e", "txt", "File extension to apply to all files (default: txt)")

	return &updateFilesCmd
}

func UpdateAndRenameFilesInDirectory(cmd *cobra.Command, args []string) {
	directoryPath := args[0]
	extension, _ := cmd.Flags().GetString("extension")

	// Remove leading dot if present
	extension = strings.TrimPrefix(extension, ".")

	// Validate directory exists
	dirInfo, err := os.Stat(directoryPath)
	if err != nil {
		log.Fatalf("Error: Directory '%s' does not exist or cannot be accessed: %v\n", directoryPath, err)
		return
	}

	if !dirInfo.IsDir() {
		log.Fatalf("Error: '%s' is not a directory\n", directoryPath)
		return
	}

	// Read directory contents
	entries, err := os.ReadDir(directoryPath)
	if err != nil {
		log.Fatalf("Error reading directory: %v\n", err)
		return
	}

	renamedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip subdirectories
		}

		oldName := entry.Name()
		oldPath := filepath.Join(directoryPath, oldName)

		baseName := strings.TrimSuffix(oldName, filepath.Ext(oldName))

		// Clean the filename: keep only letters, replace spaces with underscores, convert to lowercase
		cleanedName, err := cleanFileName(baseName)
		if err != nil {
			fmt.Printf("Error cleaning filename '%s': %v\n", oldName, err)
			continue
		}

		// Create new filename with extension
		newName := fmt.Sprintf("%s.%s", cleanedName, extension)
		newPath := filepath.Join(directoryPath, newName)

		if oldName == newName {
			continue
		}

		// Handle case where new filename already exists
		if _, err := os.Stat(newPath); err == nil {
			fmt.Printf("Warning: Skipping '%s' - target name '%s' already exists\n", oldName, newName)
			continue
		}

		renameErr := os.Rename(oldPath, newPath)
		if renameErr != nil {
			fmt.Printf("Error renaming '%s' to '%s': %v\n", oldName, newName, renameErr)
			continue
		}

		fmt.Printf("Renamed: '%s' -> '%s'\n", oldName, newName)
		renamedCount++
	}

	fmt.Printf("\nCompleted! Renamed %d file(s).\n", renamedCount)
}

func cleanFileName(name string) (string, error) {
	var result strings.Builder
	var lastWasSpace bool
	var lastWasLetter bool

	for _, r := range name {
		if unicode.IsLetter(r) {
			result.WriteRune(unicode.ToLower(r))
			lastWasSpace = false
			lastWasLetter = true
		} else if unicode.IsDigit(r) {
			// Only add digit if previous character was a letter
			if lastWasLetter {
				result.WriteRune(r)
				lastWasSpace = false
				lastWasLetter = false // Number is not a letter
			}
		} else if unicode.IsSpace(r) {
			if !lastWasSpace && result.Len() > 0 {
				result.WriteRune('_')
				lastWasSpace = true
				lastWasLetter = false
			}
		} else {
			lastWasLetter = false
		}
	}

	cleanedName := result.String()
	cleanedName = strings.Trim(cleanedName, "_")

	// Handle empty result
	if cleanedName == "" {
		return "", fmt.Errorf("cleaned filename is empty for input: %s", name)
	}

	return cleanedName, nil
}
