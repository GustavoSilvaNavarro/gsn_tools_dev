package files

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

func CompressionCmd() *cobra.Command {
	compressCmd := cobra.Command{
		Use:   "cmp <path_to_compress>",
		Short: "Compresses a file or directory into a .tar.gz archive",
		Long:  "Given a file or directory path, it compresses all the data into a single .tar.gz archive.",
		Args:  cobra.ExactArgs(1),
		Run:   CompressData,
	}

	return &compressCmd
}

func CompressData(cmd *cobra.Command, args []string) {
	// The path to compress (file or directory)
	path := args[0]
	startTime := time.Now()

	dirDetails, err := os.Stat(path)
	if err != nil {
		log.Fatalf("☠️ Error accessing path '%s': %v", path, err)
	}

	// 1. Calculate Total Size for the Progress Bar
	var totalSize int64
	if dirDetails.IsDir() {
		totalSize, err = getDirectorySize(path)
	} else {
		totalSize = dirDetails.Size()
	}

	if err != nil {
		log.Fatalf("☠️ Error calculating size for path '%s': %v", path, err)
	}

	// 2. Initialize Progress Bar
	bar := progressbar.NewOptions64(totalSize,
		progressbar.OptionSetDescription(fmt.Sprintf("📦 Compressing %s", filepath.Base(path))),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionThrottle(65*time.Millisecond), // Update rate for smoother display
		progressbar.OptionClearOnFinish(),
	)

	// 3. Determine the output archive name
	absPath, err := filepath.Abs(path)
	if err != nil {
		log.Fatalf("☠️ Error getting absolute path: %v", err)
	}
	sourceDir := filepath.Dir(absPath)
	outputFileName := filepath.Join(sourceDir, fmt.Sprintf("%s.tar.gz", filepath.Base(path)))

	// 4. Create the output file
	outFile, err := os.Create(outputFileName)
	if err != nil {
		log.Fatalf("☠️ Error creating output file: %v", err)
	}
	defer outFile.Close()

	// 5. Chain the gzip writer to the output file
	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	// 6. Chain the tar writer to the gzip writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// 7. Delegate to the core compression logic, passing the progress bar
	if dirDetails.IsDir() {
		err = compressDirectory(path, path, tarWriter, bar)
	} else {
		err = compressSingleFile(path, tarWriter, bar)
	}

	// 8. Finalize output
	if err != nil {
		log.Fatalf("❌ Compression failed: %v", err)
	}

	// Ensure the progress bar is marked as finished
	bar.Finish()

	fmt.Printf("✅ Compression successful. Archive created: %s (Time: %s)\n", outputFileName, time.Since(startTime))
}

// getDirectorySize recursively walks a directory to calculate the total size of all files.
func getDirectorySize(path string) (int64, error) {
	var totalSize int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	return totalSize, err
}

// compressDirectory walks through a directory and adds files to the tar writer.
func compressDirectory(srcPath string, rootPath string, tw *tar.Writer, bar *progressbar.ProgressBar) error {
	return filepath.Walk(srcPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate the relative path within the archive.
		relativePath := strings.TrimPrefix(filePath, rootPath+string(filepath.Separator))

		// Skip the root directory itself, or the root path of a single file
		if relativePath == "" && info.IsDir() {
			return nil
		}

		// Set a cleaner relative path for the root item
		if srcPath == filePath {
			relativePath = filepath.Base(srcPath)
		} else {
			relativePath = filepath.Base(srcPath) + "/" + relativePath
		}

		// Create the tar header based on file info
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relativePath

		// Write the header to the tar archive
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// If it's a file, copy the contents and update the progress bar
		if !info.IsDir() {
			file, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer file.Close()

			// Wrap the file reader with the progress bar writer
			barReader := io.TeeReader(file, bar)

			if _, err := io.Copy(tw, barReader); err != nil {
				return err
			}
		}

		return nil
	})
}

// compressSingleFile handles compressing a single file by adding it to the tar writer.
func compressSingleFile(filePath string, tw *tar.Writer, bar *progressbar.ProgressBar) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.Base(filePath) // Use just the file name in the archive

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Wrap the file reader with the progress bar writer
	barReader := io.TeeReader(file, bar)

	if _, err := io.Copy(tw, barReader); err != nil {
		return err
	}

	return nil
}
