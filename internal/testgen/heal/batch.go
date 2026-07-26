// Purpose: Heals broken selectors across a batch of test files in a directory.
// Why: Separates batch-mode directory scanning from single-file repair and analysis.
package heal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HealTestBatch heals selectors across a batch of test files in a directory.
func HealTestBatch(req TestHealRequest, projectDir string) (*BatchHealResult, error) {
	fullPath, err := ValidateBatchDir(req.TestDir, projectDir)
	if err != nil {
		return nil, err
	}

	testFiles, err := FindTestFiles(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	result := &BatchHealResult{
		FileResults: make([]FileHealResult, 0),
		Warnings:    make([]string, 0),
	}

	if len(testFiles) > MaxFilesPerBatch {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Batch limited to %d files (found %d)", MaxFilesPerBatch, len(testFiles)))
		testFiles = testFiles[:MaxFilesPerBatch]
	}

	var totalBatchSize int64
	for _, filePath := range testFiles {
		_, totalBatchSize = processTestFile(filePath, projectDir, totalBatchSize, result)
	}

	return result, nil
}

func skipFileWithReason(filePath, reason string, result *BatchHealResult) {
	result.FileResults = append(result.FileResults, FileHealResult{
		FilePath: filePath,
		Skipped:  true,
		Reason:   reason,
	})
	result.FilesSkipped++
}

// CheckBatchFileSize validates individual and total batch file sizes.
func CheckBatchFileSize(fileSize, totalBatchSize int64) (string, bool) {
	if fileSize > MaxFileSizeBytes {
		return fmt.Sprintf("File size exceeds %dKB limit", MaxFileSizeBytes/1024), true
	}
	if totalBatchSize+fileSize > MaxTotalBatchSize {
		return fmt.Sprintf("Total batch size would exceed %dMB limit", MaxTotalBatchSize/(1024*1024)), true
	}
	return "", false
}

func processTestFile(filePath, projectDir string, totalBatchSize int64, result *BatchHealResult) (bool, int64) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		skipFileWithReason(filePath, "Failed to read file info", result)
		return true, totalBatchSize
	}

	if reason, skip := CheckBatchFileSize(fileInfo.Size(), totalBatchSize); skip {
		skipFileWithReason(filePath, reason, result)
		return true, totalBatchSize
	}
	totalBatchSize += fileInfo.Size()

	selectors, err := AnalyzeTestFile(TestHealRequest{Action: "analyze", TestFile: filePath}, projectDir)
	if err != nil {
		skipFileWithReason(filePath, "Failed to analyze file: "+err.Error(), result)
		return true, totalBatchSize
	}

	if len(selectors) > MaxSelectorsPerFile {
		result.Warnings = append(result.Warnings, fmt.Sprintf("File %s has %d selectors, limited to %d", filepath.Base(filePath), len(selectors), MaxSelectorsPerFile))
		selectors = selectors[:MaxSelectorsPerFile]
	}

	healResult, err := RepairSelectors(TestHealRequest{Action: "repair", BrokenSelectors: selectors})
	if err != nil {
		skipFileWithReason(filePath, "Failed to repair selectors: "+err.Error(), result)
		return true, totalBatchSize
	}

	result.FileResults = append(result.FileResults, FileHealResult{
		FilePath: filePath,
		Healed:   len(healResult.Healed),
		Unhealed: len(healResult.Unhealed),
	})
	result.FilesProcessed++
	result.TotalSelectors += len(selectors)
	result.TotalHealed += len(healResult.Healed)
	result.TotalUnhealed += len(healResult.Unhealed)
	return false, totalBatchSize
}

var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"dist":         true,
	"build":        true,
}

// IsTestFile checks if a file path matches test file patterns.
func IsTestFile(path string) bool {
	testPatterns := []string{".spec.ts", ".test.ts", ".spec.js", ".test.js"}
	for _, pattern := range testPatterns {
		if strings.HasSuffix(path, pattern) {
			return true
		}
	}
	return false
}

// FindTestFiles recursively finds test files in a directory, skipping common ignored dirs.
func FindTestFiles(dir string) ([]string, error) {
	var testFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if IsTestFile(path) {
			testFiles = append(testFiles, path)
		}
		return nil
	})

	return testFiles, err
}
