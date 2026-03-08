package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
)

func main() {
	var verbose bool
	flag.BoolVar(&verbose, "v", false, "詳細ログを表示する")
	flag.BoolVar(&verbose, "verbose", false, "詳細ログを表示する")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <target_dir>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "エラー: 処理対象のディレクトリパスを指定してください。")
		flag.Usage()
		os.Exit(2)
	}
	targetDir := args[0]

	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	processor := &Processor{
		Logger: logger,
	}

	logger.Debug("処理を開始します", "target_dir", targetDir)

	anyReplaced, err := processor.ProcessDirectory(targetDir)
	if err != nil {
		logger.Error("例外エラーにより異常終了します", "error", err)
		os.Exit(2)
	}

	if anyReplaced {
		logger.Info("処理が完了しました（置換あり）")
		os.Exit(1)
	}

	logger.Info("置換対象のデータはありませんでした")
	os.Exit(0)
}
