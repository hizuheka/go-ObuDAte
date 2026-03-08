package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Processor は変換処理全体を管理する構造体です。
type Processor struct {
	Logger *slog.Logger
}

// ProcessDirectory は指定ディレクトリ直下のCSVファイルを処理します。
func (p *Processor) ProcessDirectory(targetDir string) (bool, error) {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return false, fmt.Errorf("ディレクトリ読み込みエラー: %w", err)
	}

	anyFileReplaced := false

	for _, entry := range entries {
		// サブディレクトリやCSV以外のファイルはスキップ
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".csv" {
			continue
		}

		filePath := filepath.Join(targetDir, entry.Name())
		replaced, err := p.processFile(filePath)
		if err != nil {
			p.Logger.Error("ファイル処理中にエラーが発生しました", "file", filePath, "error", err)
			return false, err
		}

		if replaced {
			anyFileReplaced = true
		}
	}

	return anyFileReplaced, nil
}

func (p *Processor) processFile(srcPath string) (bool, error) {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return false, fmt.Errorf("ファイルオープンエラー: %w", err)
	}
	defer srcFile.Close()

	scanner := bufio.NewScanner(srcFile)
	var lines []string
	fileReplaced := false
	replaceCount := 0

	for scanner.Scan() {
		newLine, replaced := ReplaceTime(scanner.Text())
		if replaced {
			fileReplaced = true
			replaceCount++
		}
		lines = append(lines, newLine)
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("ファイル読み込みエラー: %w", err)
	}

	// 置換対象がなければ新しいファイルは作成しない
	if !fileReplaced {
		p.Logger.Debug("置換対象なし、スキップします", "file", srcPath)
		return false, nil
	}

	ext := filepath.Ext(srcPath)
	destPath := srcPath[:len(srcPath)-len(ext)] + ".cs_"

	destFile, err := os.Create(destPath)
	if err != nil {
		return false, fmt.Errorf("出力ファイル作成エラー: %w", err)
	}
	defer destFile.Close()

	writer := bufio.NewWriter(destFile)
	for _, line := range lines {
		if _, err := writer.WriteString(line + "\r\n"); err != nil {
			return false, fmt.Errorf("書き込みエラー: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return false, fmt.Errorf("フラッシュエラー: %w", err)
	}

	p.Logger.Info("ファイルを変換・出力しました", "source", srcPath, "output", destPath, "replace_count", replaceCount)
	return true, nil
}
