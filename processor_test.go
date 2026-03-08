package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessDirectory(t *testing.T) {
	// テスト時はログ出力を捨てる
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("置換対象があり、正しく.cs_ファイルが作成される", func(t *testing.T) {
		tempDir := t.TempDir()

		// テスト用CSVの作成
		targetCsv := filepath.Join(tempDir, "test1.csv")
		content := "id,name,date,time\r\n\"1\",\"山田　太郎\",\"2024-02-28\",\"24:30\"\r\n\"2\",\"佐藤　花子\",\"2023-01-02\",\"12:00\"\r\n"
		err := os.WriteFile(targetCsv, []byte(content), 0644)
		if err != nil {
			t.Fatalf("テストファイルの作成に失敗: %v", err)
		}

		processor := &Processor{Logger: logger}
		replaced, err := processor.ProcessDirectory(tempDir)

		if err != nil {
			t.Fatalf("予期せぬエラー: %v", err)
		}
		if !replaced {
			t.Errorf("replaced = false, want true")
		}

		// 出力ファイルの検証（閏年の翌日になり、00時になっていること）
		targetCs_ := filepath.Join(tempDir, "test1.cs_")
		outData, err := os.ReadFile(targetCs_)
		if err != nil {
			t.Fatalf("ファイル %s が作成されていません: %v", targetCs_, err)
		}

		expected := "id,name,date,time\r\n\"1\",\"山田　太郎\",\"2024-02-29\",\"00:30\"\r\n\"2\",\"佐藤　花子\",\"2023-01-02\",\"12:00\"\r\n"
		if string(outData) != expected {
			t.Errorf("生成ファイル内容:\n%v\n想定内容:\n%v", string(outData), expected)
		}
	})

	t.Run("存在しないディレクトリを指定した場合はエラー", func(t *testing.T) {
		processor := &Processor{Logger: logger}
		_, err := processor.ProcessDirectory("dummy_not_exists_dir")
		if err == nil {
			t.Errorf("エラーが返るべきです")
		}
	})
}
