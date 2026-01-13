package main

import (
	"flag"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"strconv"
)

// RealFileSystem は実際のOSのファイルシステム操作を実装します。
type RealFileSystem struct{}

func (RealFileSystem) ReadDir(dirname string) ([]fs.DirEntry, error) {
	return os.ReadDir(dirname)
}

func (RealFileSystem) Open(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

func main() {
	// 引数の定義
	dir := flag.String("dir", ".", "処理対象フォルダのパス")
	idCol := flag.Int("id", 1, "識別番号の列番号（1始まり）")
	flagCol := flag.Int("flag", 0, "フラグの列番号（1始まり、0の場合はチェックしない）") // 追加
	insertF := flag.String("insertF", "insert", "追加ファイルの識別子")
	updateF := flag.String("updateF", "update", "更新ファイルの識別子")
	minStr := flag.String("min", "0", "識別番号の最小値")

	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// idのバリデーション
	if *idCol < 1 {
		logger.Error("invalid id argument: must be >= 1", slog.Int("id", *idCol))
		os.Exit(1)
	}

	// flagのバリデーション (指定されている場合のみ)
	flagColIdx := -1
	if *flagCol > 0 {
		flagColIdx = *flagCol - 1
	}

	// minのパース
	minID, err := strconv.ParseInt(*minStr, 10, 64)
	if err != nil {
		logger.Error("invalid min argument", slog.String("min", *minStr), slog.String("error", err.Error()))
		os.Exit(1)
	}

	// 設定の構築
	cfg := Config{
		Dir:        *dir,
		IDColIdx:   *idCol - 1, // 1-based to 0-based
		FlagColIdx: flagColIdx, // -1 if not specified
		InsertF:    *insertF,
		UpdateF:    *updateF,
		MinID:      minID,
	}

	fsSys := RealFileSystem{}
	processor := NewProcessor(cfg, fsSys, os.Stdout, logger)

	if _, err := processor.Run(); err != nil {
		logger.Error("process failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
