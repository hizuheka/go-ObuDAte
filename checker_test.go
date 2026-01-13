package main

import (
	"bytes"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

// MockFileSystem はテスト用のファイルシステムモックです。
type MockFileSystem struct {
	Files map[string]string // Filename -> Content
}

// ReadDir は指定されたディレクトリ内のファイルを模倣して返します。
func (m *MockFileSystem) ReadDir(dirname string) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry
	for name := range m.Files {
		entries = append(entries, &mockDirEntry{name: name})
	}
	return entries, nil
}

// Open は指定されたファイルの内容を読み取ります。
func (m *MockFileSystem) Open(name string) (io.ReadCloser, error) {
	parts := strings.Split(filepath.ToSlash(name), "/")
	base := parts[len(parts)-1]

	content, ok := m.Files[base]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

// mockDirEntry implementation...
type mockDirEntry struct{ name string }

func (e *mockDirEntry) Name() string               { return e.name }
func (e *mockDirEntry) IsDir() bool                { return false }
func (e *mockDirEntry) Type() fs.FileMode          { return 0 }
func (e *mockDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func TestProcessor_Run(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name           string
		cfg            Config
		files          map[string]string
		expectedOutput []string
		expectedStats  Stats
	}{
		{
			name: "正常系: 10桁IDとint64比較",
			cfg: Config{
				Dir: ".", IDColIdx: 0, FlagColIdx: 1, InsertF: "INS", UpdateF: "UPD",
				MinID: 2000000000,
			},
			files: map[string]string{
				"INS_01.csv": "ID,Flag\n2000000000,1\n9999999999,1", // Flag=1はOK
			},
			expectedOutput: []string{
				"追加ファイル: 0件 更新ファイル:0件",
			},
			expectedStats: Stats{InsertErrors: 0, UpdateErrors: 0},
		},
		{
			name: "異常系: フラグ=0のエラー（メッセージは追加ファイルで2回目）",
			cfg: Config{
				Dir: ".", IDColIdx: 0, FlagColIdx: 1, InsertF: "INS", UpdateF: "UPD",
				MinID: 100,
			},
			files: map[string]string{
				"INS_01.csv": "ID,Flag\n100,0\n101,1", // 100はFlag=0エラー, 101はOK
			},
			expectedOutput: []string{
				"INS_01.csv(2) - 追加ファイルで2回目 : 100,0",
				"追加ファイル: 1件 更新ファイル:0件",
			},
			expectedStats: Stats{InsertErrors: 1, UpdateErrors: 0},
		},
		{
			name: "異常系: フラグ=0エラー後の再出現は「エラー対象者の2回目以降」",
			cfg: Config{
				Dir: ".", IDColIdx: 0, FlagColIdx: 1, InsertF: "INS", UpdateF: "UPD",
				MinID: 100,
			},
			files: map[string]string{
				"INS_01.csv": "ID,Flag\n100,0", // Error: 追加ファイルで2回目
				"INS_02.csv": "ID,Flag\n100,1", // Error: エラー対象者の2回目以降
			},
			expectedOutput: []string{
				"INS_01.csv(2) - 追加ファイルで2回目 : 100,0",
				"INS_02.csv(2) - エラー対象者の2回目以降 : 100,1",
			},
			expectedStats: Stats{InsertErrors: 2, UpdateErrors: 0},
		},
		{
			name: "異常系: 10桁IDでの再転入エラー",
			cfg: Config{
				Dir: ".", IDColIdx: 0, FlagColIdx: 1, InsertF: "INS", UpdateF: "UPD",
				MinID: 5000000000,
			},
			files: map[string]string{
				"INS_01.csv": "ID,Flag\n4999999999,NG\n5000000000,OK",
			},
			expectedOutput: []string{
				"INS_01.csv(2) - 再転入エラー : 4999999999,NG",
				"追加ファイル: 1件 更新ファイル:0件",
			},
			expectedStats: Stats{InsertErrors: 1, UpdateErrors: 0},
		},
		{
			name: "異常系: 正常IDの2回目出現（追加ファイルで2回目）",
			cfg: Config{
				Dir: ".", IDColIdx: 0, FlagColIdx: 1, InsertF: "INS", UpdateF: "UPD",
				MinID: 100, // ID 200 は正常
			},
			files: map[string]string{
				"INS_01.csv": "ID,Flag\n200,OK\n200,Dup",
			},
			expectedOutput: []string{
				"INS_01.csv(3) - 追加ファイルで2回目 : 200,Dup",
				"追加ファイル: 1件 更新ファイル:0件",
			},
			expectedStats: Stats{InsertErrors: 1, UpdateErrors: 0},
		},
		{
			name: "優先順位確認: 重複 > Min値未満",
			cfg: Config{
				Dir: ".", IDColIdx: 0, FlagColIdx: 1, InsertF: "INS", UpdateF: "UPD", MinID: 200,
			},
			files: map[string]string{
				"INS_01.csv": "ID,Flag\n100,F", // Min未満 -> 再転入エラー
				"INS_02.csv": "ID,Flag\n100,T", // 既エラーID -> エラー対象者の2回目以降
			},
			expectedOutput: []string{
				"INS_01.csv(2) - 再転入エラー : 100,F",
				"INS_02.csv(2) - エラー対象者の2回目以降 : 100,T",
				"追加ファイル: 2件 更新ファイル:0件",
			},
			expectedStats: Stats{InsertErrors: 2, UpdateErrors: 0},
		},
		{
			name: "更新ファイル: 未登録IDは無視",
			cfg: Config{
				Dir: ".", IDColIdx: 0, FlagColIdx: 1, InsertF: "INS", UpdateF: "UPD", MinID: 100,
			},
			files: map[string]string{
				"UPD_01.csv": "ID,Flag\n999,T",
			},
			expectedOutput: []string{
				"追加ファイル: 0件 更新ファイル:0件",
			},
			expectedStats: Stats{InsertErrors: 0, UpdateErrors: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsMock := &MockFileSystem{Files: tt.files}
			outBuf := new(bytes.Buffer)

			p := NewProcessor(tt.cfg, fsMock, outBuf, logger)
			stats, err := p.Run()

			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}

			if stats.InsertErrors != tt.expectedStats.InsertErrors {
				t.Errorf("InsertErrors mismatch: got %d, want %d", stats.InsertErrors, tt.expectedStats.InsertErrors)
			}
			if stats.UpdateErrors != tt.expectedStats.UpdateErrors {
				t.Errorf("UpdateErrors mismatch: got %d, want %d", stats.UpdateErrors, tt.expectedStats.UpdateErrors)
			}

			outputStr := outBuf.String()
			for _, want := range tt.expectedOutput {
				if !strings.Contains(outputStr, want) {
					t.Errorf("Output missing expected string: %q. Got:\n%s", want, outputStr)
				}
			}
		})
	}
}

func TestProcessor_EdgeCases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name           string
		cfg            Config
		files          map[string]string
		expectedOutput []string
		expectedStats  Stats
	}{
		{
			name: "列指定の変更: IDが2列目にある場合",
			cfg: Config{
				Dir: ".", IDColIdx: 1, FlagColIdx: 0, InsertF: "INS", UpdateF: "UPD", MinID: 100,
			},
			files: map[string]string{
				"INS_01.csv": "Flag,ID\nF,200\nT,50",
			},
			expectedOutput: []string{
				"INS_01.csv(3) - 再転入エラー : T,50",
			},
			expectedStats: Stats{InsertErrors: 1, UpdateErrors: 0},
		},
		{
			name: "数値変換エラー: ID列が数値以外の場合（スキップ確認）",
			cfg: Config{
				Dir: ".", IDColIdx: 0, FlagColIdx: 1, InsertF: "INS", UpdateF: "UPD", MinID: 100,
			},
			files: map[string]string{
				"INS_01.csv": "ID,Flag\nABC,F\n50,T",
			},
			expectedOutput: []string{
				"INS_01.csv(3) - 再転入エラー : 50,T",
			},
			expectedStats: Stats{InsertErrors: 1, UpdateErrors: 0},
		},
		{
			name: "更新ファイル: 追加ファイルで正常登録済みのIDが出現（正常系）",
			cfg: Config{
				Dir: ".", IDColIdx: 0, FlagColIdx: 1, InsertF: "INS", UpdateF: "UPD", MinID: 100,
			},
			files: map[string]string{
				"INS_01.csv": "ID,Flag\n100,Ok",
				"UPD_01.csv": "ID,Flag\n100,Update",
			},
			expectedOutput: []string{
				"追加ファイル: 0件 更新ファイル:0件",
			},
			expectedStats: Stats{InsertErrors: 0, UpdateErrors: 0},
		},
		{
			name: "ファイル名のソート順序: 文字列としての昇順",
			cfg: Config{
				Dir: ".", IDColIdx: 0, FlagColIdx: 1, InsertF: "INS", UpdateF: "UPD", MinID: 100,
			},
			files: map[string]string{
				// 文字列ソート順: INS_1.csv -> INS_10.csv -> INS_2.csv
				// INS_1.csv: 100 (1回目) -> OK
				// INS_10.csv: 100 (2回目) -> Error (追加ファイルで2回目)
				// INS_2.csv: 100 (3回目) -> Error (エラー対象者の2回目以降)
				"INS_1.csv":  "ID,Flag\n100,First",
				"INS_2.csv":  "ID,Flag\n100,Third",
				"INS_10.csv": "ID,Flag\n100,Second",
			},
			expectedOutput: []string{
				"INS_10.csv(2) - 追加ファイルで2回目 : 100,Second",
				"INS_2.csv(2) - エラー対象者の2回目以降 : 100,Third",
			},
			expectedStats: Stats{InsertErrors: 2, UpdateErrors: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsMock := &MockFileSystem{Files: tt.files}
			outBuf := new(bytes.Buffer)

			p := NewProcessor(tt.cfg, fsMock, outBuf, logger)
			stats, err := p.Run()

			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}

			if stats.InsertErrors != tt.expectedStats.InsertErrors {
				t.Errorf("InsertErrors mismatch: got %d, want %d", stats.InsertErrors, tt.expectedStats.InsertErrors)
			}
			if stats.UpdateErrors != tt.expectedStats.UpdateErrors {
				t.Errorf("UpdateErrors mismatch: got %d, want %d", stats.UpdateErrors, tt.expectedStats.UpdateErrors)
			}

			outputStr := outBuf.String()
			for _, want := range tt.expectedOutput {
				if !strings.Contains(outputStr, want) {
					t.Errorf("Output missing expected string: %q. Got:\n%s", want, outputStr)
				}
			}

			if strings.Contains(outputStr, "ABC") {
				t.Errorf("Output contained skipped record content: %s", outputStr)
			}
		})
	}
}
