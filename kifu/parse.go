package kifu

import (
	"strconv"
	"strings"
	"time"
)

// Parse は KIF テキストを Document に読み取る。
//
// 対応するのは「ヘッダ行 + 指し手行」の基本構造。コメント行(`*`)、`#` 行、
// 指し手の列ヘッダ、`まで～手で…` の結果行は読み飛ばす。
//
// **変化(`変化：N手`)は未対応**で、そこで読み取りを打ち切る(本譜のみを取る)。
// kicho は棋譜を保存して外部ツールへ渡すのが目的なので、本譜が取れれば足りる。
// 分岐を扱うなら Document 側にツリー表現を足すところから必要になる。
//
// **指し手が 0 手でもエラーにしない**(finish を参照)。error を返す余地は
// 将来の解析エラーのために残してあるが、現状は常に nil。
func Parse(s string) (Document, error) {
	var d Document
	lines := strings.Split(normalizeNewlines(stripBOM(s)), "\n")

	sawTime := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "*"), strings.HasPrefix(line, "#"):
			// コメント・メタ行
			continue
		case strings.HasPrefix(line, "手数"):
			// 指し手の列ヘッダ
			continue
		case strings.HasPrefix(line, "変化："):
			// 分岐は未対応。本譜だけ取って打ち切る。
			return finish(d, sawTime)
		case strings.HasPrefix(line, "まで"):
			// "まで104手で先手の勝ち" のような結果行
			continue
		}

		// 数字始まりなら指し手行として読む(ヘッダより先に判定する。
		// "1 ７六歩(77)" のような行に "：" は無いが、順序を明確にしておく)。
		if m, ok := parseMoveLine(line); ok {
			if m.Spend > 0 {
				sawTime = true
			}
			d.Moves = append(d.Moves, m)
			continue
		}

		if key, value, ok := splitHeader(line); ok {
			applyHeader(&d, key, value)
			continue
		}
		// 解釈できない行は読み飛ばす(KIF には方言が多いため落とさない)。
	}
	return finish(d, sawTime)
}

// finish は読み取り結果を仕上げる。
//
// **指し手が 0 手でもエラーにしない。** 対局前の中継棋譜(ヘッダだけで
// 指し手がまだ無い .kif)が正当に存在するため。「1 手も無い」は
// 読み取りの失敗ではなく、まだ指されていないという事実。
// 棋譜として成立しているかの判断は呼び出し側が行う。
func finish(d Document, sawTime bool) (Document, error) {
	d.ShowTime = sawTime
	return d, nil
}

// utf8BOM は UTF-8 のバイトオーダーマーク。
// KIF ファイルは BOM 付きで保存されていることが多い。
const utf8BOM = "\ufeff"

func stripBOM(s string) string {
	return strings.TrimPrefix(s, utf8BOM)
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// splitHeader は "キー：値" を分割する。全角コロンのみを区切りとする。
func splitHeader(line string) (key, value string, ok bool) {
	i := strings.Index(line, "：")
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+len("："):]), true
}

func applyHeader(d *Document, key, value string) {
	switch key {
	case "開始日時", "対局日":
		if t, ok := parseKifTime(value); ok {
			d.StartedAt = t
		}
	case "棋戦", "棋戦名":
		d.Event = value
	case "場所", "対局場所":
		d.Place = value
	case "手合割":
		d.Handicap = value
	case "先手", "下手":
		d.Black = value
	case "後手", "上手":
		d.White = value
	case "持ち時間":
		d.TimeLimit = parseTimeLimit(value)
	case "秒読み":
		d.Countdown = parseCountdown(value)
	}
}

// kifTimeLayouts は開始日時に使われる書式。KIF は方言が多いので複数試す。
var kifTimeLayouts = []string{
	"2006/01/02 15:04:05",
	"2006/01/02 15:04",
	"2006/01/02",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

// jst は日時にタイムゾーンが書かれていない場合の解釈。
var jst = time.FixedZone("JST", 9*60*60)

func parseKifTime(v string) (time.Time, bool) {
	// "2024/10/19 09:00:00" の後ろに補足が付くことがあるので前から順に試す。
	v = strings.TrimSpace(v)
	for _, layout := range kifTimeLayouts {
		if len(v) < len(layout) {
			continue
		}
		if t, err := time.ParseInLocation(layout, v[:len(layout)], jst); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseTimeLimit は "各8時間" / "各25分" / "8時間" を読む。
func parseTimeLimit(v string) time.Duration {
	v = strings.TrimPrefix(strings.TrimSpace(v), "各")
	if i := strings.Index(v, "時間"); i > 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(v[:i])); err == nil {
			return time.Duration(n) * time.Hour
		}
	}
	if i := strings.Index(v, "分"); i > 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(v[:i])); err == nil {
			return time.Duration(n) * time.Minute
		}
	}
	return 0
}

// parseCountdown は "60秒" を読む。
func parseCountdown(v string) time.Duration {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, "秒"); i > 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(v[:i])); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return 0
}

// parseMoveLine は指し手行を読む。
//
//	  1 ７六歩(77)   ( 0:16/00:00:16)
//	 14 同　歩(23)
//	104 投了
//
// 指し手名に全角空白を含むこと(`同　歩`)があるので、
// 「手数 → 末尾の消費時間 → 末尾の移動元座標 → 残りが指し手名」の順に削っていく。
func parseMoveLine(line string) (Move, bool) {
	num, rest, ok := cutMoveNumber(line)
	if !ok {
		return Move{}, false
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return Move{}, false
	}

	var m Move
	m.Num = num

	// 末尾の消費時間欄を切り離す。
	if body, spend, ok := cutTimes(rest); ok {
		rest = body
		m.Spend = spend
	}
	rest = strings.TrimSpace(rest)

	// 末尾の移動元座標 "(77)" を切り離す。
	if body, x, y, ok := cutFromSquare(rest); ok {
		rest = body
		m.FromX, m.FromY = x, y
	}

	name := strings.TrimSpace(rest)
	if name == "" {
		return Move{}, false
	}
	m.Name = name
	return m, true
}

// cutMoveNumber は行頭の手数を切り出す。
func cutMoveNumber(line string) (num int, rest string, ok bool) {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, "", false
	}
	// 数字の直後は区切り(半角/全角空白かタブ)でなければ手数ではない。
	if i < len(line) {
		c := line[i]
		if c != ' ' && c != '\t' && !strings.HasPrefix(line[i:], "　") {
			return 0, "", false
		}
	}
	n, err := strconv.Atoi(line[:i])
	if err != nil {
		return 0, "", false
	}
	return n, line[i:], true
}

// cutTimes は末尾の "( 0:16/00:00:16)" を切り離して消費時間を返す。
func cutTimes(s string) (rest string, spend time.Duration, ok bool) {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, ")") {
		return s, 0, false
	}
	i := strings.LastIndex(s, "(")
	if i < 0 {
		return s, 0, false
	}
	inner := s[i+1 : len(s)-1]
	slash := strings.Index(inner, "/")
	if slash < 0 {
		return s, 0, false // 移動元座標 "(77)" などは対象外
	}
	d, ok := parseClock(strings.TrimSpace(inner[:slash]))
	if !ok {
		return s, 0, false
	}
	return s[:i], d, true
}

// parseClock は "4:00" / "01:00:00" を Duration にする。
func parseClock(v string) (time.Duration, bool) {
	parts := strings.Split(v, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 {
			return 0, false
		}
		nums = append(nums, n)
	}
	if len(parts) == 2 {
		return time.Duration(nums[0])*time.Minute + time.Duration(nums[1])*time.Second, true
	}
	return time.Duration(nums[0])*time.Hour +
		time.Duration(nums[1])*time.Minute +
		time.Duration(nums[2])*time.Second, true
}

// cutFromSquare は末尾の "(77)" を切り離して移動元を返す。
func cutFromSquare(s string) (rest string, x, y int, ok bool) {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, ")") {
		return s, 0, 0, false
	}
	i := strings.LastIndex(s, "(")
	if i < 0 {
		return s, 0, 0, false
	}
	inner := s[i+1 : len(s)-1]
	if len(inner) != 2 {
		return s, 0, 0, false
	}
	if inner[0] < '0' || inner[0] > '9' || inner[1] < '0' || inner[1] > '9' {
		return s, 0, 0, false
	}
	return s[:i], int(inner[0] - '0'), int(inner[1] - '0'), true
}

// EndMark は Document の最終手が終局を表していればその名称を返す。
func (d Document) EndMark() string {
	if len(d.Moves) == 0 {
		return ""
	}
	if marker, ok := TerminalMarker(d.Moves[len(d.Moves)-1].Name); ok {
		return marker
	}
	return ""
}
