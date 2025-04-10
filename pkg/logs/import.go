package logs

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	var importCmd = &cobra.Command{
		Use:   "import",
		Short: "Import logs from a tar archive",
		Run: func(cmd *cobra.Command, args []string) {
			source, _ := cmd.Flags().GetString("source")
			dest, _ := cmd.Flags().GetString("dest")

			// Create destination directory if it doesn't exist
			if err := os.MkdirAll(dest, 0755); err != nil {
				log.Fatal(err)
			}

			// Implement your tar extraction logic here
			if err := extractTar(source, dest); err != nil {
				log.Fatal(err)
			}
			fmt.Println("Successfully imported logs to:", dest)
		},
	}
	// Import command flags
	importCmd.Flags().StringP("source", "s", "", "Source tar file to import (required)")
	importCmd.Flags().StringP("dest", "d", "/tmp/k8sdebug/logs", "Destination directory for extraction")
	importCmd.MarkFlagRequired("source")
	return importCmd
}

func newExportCmd() *cobra.Command {
	var exportCmd = &cobra.Command{
		Use:   "export",
		Short: "Export logs to a tar archive",
		Run: func(cmd *cobra.Command, args []string) {
			source, _ := cmd.Flags().GetString("source")
			dest, _ := cmd.Flags().GetString("dest")

			// Implement your tar creation logic here
			if err := createTar(source, dest); err != nil {
				log.Fatal(err)
			}
			fmt.Println("Successfully exported logs to:", dest)
		},
	}
	// Export command flags
	exportCmd.Flags().StringP("source", "s", "/tmp/k8sdebug/logs", "Source directory to export")
	exportCmd.Flags().StringP("dest", "d", "", "Destination tar file path (required)")
	exportCmd.MarkFlagRequired("dest")
	return exportCmd
}

func extractTar(source string, dest string) error {
	// Open source file
	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	// Create tar reader
	tr := tar.NewReader(gzr)

	// Process each entry in the tar archive
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("tar read error: %w", err)
		}

		// Sanitize file path to prevent path traversal
		targetPath := filepath.Join(dest, header.Name)
		if !strings.HasPrefix(targetPath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory with original permissions
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			// Create parent directories if needed
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directories: %w", err)
			}

			// Create file with original permissions
			f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			// Copy file contents
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("failed to copy file contents: %w", err)
			}
			f.Close()

			// Preserve modification time
			if err := os.Chtimes(targetPath, header.AccessTime, header.ModTime); err != nil {
				return fmt.Errorf("failed to set file times: %w", err)
			}

		default:
			return fmt.Errorf("unsupported file type: %v", header.Typeflag)
		}
	}
	return nil
}

func createTar(source string, dest string) error {
	// Create destination file
	file, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer file.Close()

	// Create gzip writer
	gzw := gzip.NewWriter(file)
	defer gzw.Close()

	// Create tar writer
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// Walk the source directory
	baseDir := filepath.Clean(source)
	err = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil // Skip root directory
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create header: %w", err)
		}
		header.Name = relPath

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}

		// Skip directories (header only)
		if info.IsDir() {
			return nil
		}

		// Open file for reading
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer f.Close()

		// Copy file contents
		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("failed to copy file contents: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("directory walk failed: %w", err)
	}

	return nil
}
