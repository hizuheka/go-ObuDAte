package main

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// dateTimePattern は `"YYYY-MM-DD","HH:` の形式をキャプチャします。
// 日付と時間の間にカンマがあるCSV形式を想定しています。
var dateTimePattern = regexp.MustCompile(`"(\d{4}-\d{2}-\d{2})","([2-4][0-9]):`)

const dateFormat = "2006-01-02"

// ReplaceTime はテキスト内の日付と時間を検証し、時間が 24〜47 の場合に
// 日付を1日加算、時間を -24 してゼロ埋め置換します。
func ReplaceTime(input string) (string, bool) {
	replaced := false
	result := dateTimePattern.ReplaceAllStringFunc(input, func(match string) string {
		submatches := dateTimePattern.FindStringSubmatch(match)
		if len(submatches) != 3 {
			return match
		}

		dateStr := submatches[1]
		hourStr := submatches[2]

		hour, err := strconv.Atoi(hourStr)
		if err != nil {
			return match // 万が一のパースエラー時は置換しない
		}

		// 時間が仕様の範囲(24〜47)であるか確認
		if hour >= 24 && hour <= 47 {
			// 日付の妥当性検証とパース
			parsedDate, err := time.Parse(dateFormat, dateStr)
			if err != nil {
				return match // 存在しない日付("2023-13-45"など)の場合は置換しない
			}

			// 日付に1日加算し、時間から24を引く
			newDate := parsedDate.AddDate(0, 0, 1)
			newHour := hour - 24
			replaced = true

			return fmt.Sprintf(`"%s","%02d:`, newDate.Format(dateFormat), newHour)
		}
		return match
	})

	return result, replaced
}
