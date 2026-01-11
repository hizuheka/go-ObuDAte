package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Config はチェック処理の設定を保持します。
type Config struct {
	Dir      string // 処理対象ディレクトリ
	IDColIdx int    // 識別番号の列インデックス (0-based)
	InsertF  string // 追加ファイルの接頭辞
	UpdateF  string // 更新ファイルの接頭辞
	MinID    int64  // 識別番号の最小値
}

// Stats は処理結果の統計情報を保持します。
type Stats struct {
	InsertErrors int
	UpdateErrors int
}

// CheckResult は1行のチェック結果を表します。
type CheckResult struct {
	IsError bool
	Message string
}

// FileSystem はファイル操作を抽象化するインターフェースです。
type FileSystem interface {
	ReadDir(dirname string) ([]fs.DirEntry, error)
	Open(name string) (io.ReadCloser, error)
}

// Processor はチェック処理の状態を管理します。
type Processor struct {
	cfg      Config
	fs       FileSystem
	out      io.Writer
	logger   *slog.Logger
	seenIDs  map[string]bool // 追加ファイルで出現済みの識別番号
	errorIDs map[string]bool // エラーとなった識別番号
}

// NewProcessor はProcessorのインスタンスを生成します。
func NewProcessor(cfg Config, fs FileSystem, out io.Writer, logger *slog.Logger) *Processor {
	return &Processor{
		cfg:      cfg,
		fs:       fs,
		out:      out,
		logger:   logger,
		seenIDs:  make(map[string]bool),
		errorIDs: make(map[string]bool),
	}
}

// Run はディレクトリ内のファイルを順次処理し、チェックを実行します。
func (p *Processor) Run() (*Stats, error) {
	entries, err := p.fs.ReadDir(p.cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read dir: %w", err)
	}

	// ファイル名昇順で処理するためにソート
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	stats := &Stats{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		if filepath.Ext(filename) != ".csv" {
			continue
		}

		isInsert := strings.HasPrefix(filename, p.cfg.InsertF)
		isUpdate := strings.HasPrefix(filename, p.cfg.UpdateF)

		if !isInsert && !isUpdate {
			p.logger.Debug("skipping file", slog.String("file", filename))
			continue
		}

		fileErrCount, err := p.processFile(filename, isInsert)
		if err != nil {
			return nil, fmt.Errorf("failed to process file %s: %w", filename, err)
		}

		if isInsert {
			stats.InsertErrors += fileErrCount
		} else {
			stats.UpdateErrors += fileErrCount
		}
	}

	// 統計情報の出力
	fmt.Fprintf(p.out, "追加ファイル: %d件 更新ファイル:%d件\n", stats.InsertErrors, stats.UpdateErrors)

	return stats, nil
}

// processFile は単一のファイルを処理します。
func (p *Processor) processFile(filename string, isInsert bool) (int, error) {
	path := filepath.Join(p.cfg.Dir, filename)
	f, err := p.fs.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)

	// ヘッダー行を読み飛ばす
	_, err = reader.Read()
	if err != nil {
		if err == io.EOF {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read header: %w", err)
	}

	rowNum := 1 // ヘッダーを1行目とする
	errCount := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errCount, fmt.Errorf("csv read error at line %d: %w", rowNum+1, err)
		}
		rowNum++

		// 行に対するバリデーション
		res := p.validateRow(record, isInsert)

		if res.IsError {
			errCount++
			// 仕様：メッセージ : エラーデータの内容（行全体）
			lineContent := strings.Join(record, ",")
			fmt.Fprintf(p.out, "%s(%d) - %s : %s\n", filename, rowNum, res.Message, lineContent)
		}
	}

	return errCount, nil
}

// validateRow は1行のデータに対してビジネスロジックを適用します。
func (p *Processor) validateRow(record []string, isInsert bool) CheckResult {
	// 列不足チェック
	// 識別番号の列が存在するか確認
	if len(record) <= p.cfg.IDColIdx {
		p.logger.Warn("invalid column length", slog.Int("len", len(record)), slog.Int("required_idx", p.cfg.IDColIdx), slog.Any("record", record))
		return CheckResult{IsError: false}
	}

	idStr := record[p.cfg.IDColIdx]

	// 共通チェック: 既にエラーとなっているIDか？
	if p.errorIDs[idStr] {
		return CheckResult{IsError: true, Message: "エラー対象者の2回目以降"}
	}

	if isInsert {
		// 追加ファイルのルール
		// 優先順位: 重複 > Min

		// 1. 2回目の出現チェック
		if p.seenIDs[idStr] {
			p.markError(idStr)
			return CheckResult{IsError: true, Message: "追加ファイルで2回目"}
		}

		// 2. 識別番号の最小値チェック (int64で比較)
		// 10桁の数値に変換可能な文字列前提
		idInt, err := strconv.ParseInt(idStr, 10, 64)
		if err == nil && idInt < p.cfg.MinID {
			p.markError(idStr)
			return CheckResult{IsError: true, Message: "再転入エラー"}
		} else if err != nil {
			p.logger.Warn("failed to parse ID as int64", slog.String("id", idStr), slog.String("error", err.Error()))
		}

		// 正常な追加
		p.seenIDs[idStr] = true

	} else {
		// 更新ファイルのルール（未登録IDは無視）
	}

	return CheckResult{IsError: false}
}

// markError はIDをエラーとして記録します。
func (p *Processor) markError(id string) {
	p.errorIDs[id] = true
}
