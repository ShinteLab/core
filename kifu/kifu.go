// Package kifu は KIF 形式(将棋の棋譜テキスト)の組み立てを
// 各プロジェクト共通で扱うための仕様パッケージ。
//
// スクレイピング等の I/O は含まない純粋関数のみ。
//
// JS 側の同等実装は core/web/kifu.js にある(ブラウザの棋譜ビューア向け)。
// 指し手行の書式(StripMoveModifiers / FormatLine)は両者で挙動を揃えること。
// ヘッダ付きのドキュメント組み立て(Document)は Go 側にのみあり、
// KIF ファイルを書き出す用途(kicho)はこちらを使う。
package kifu

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MoveModifiers は KIF の相対表記修飾(同じ移動先に複数駒が動ける場合の区別)。
// KIF の (筋段) 座標表記を使う場合は不要なので指し手名から除去する。
var MoveModifiers = []string{"右", "左", "上", "寄", "引", "直", "不成"}

// KIF ドキュメントの定型行。
const (
	HirateHandicap   = "平手"
	MoveColumnHeader = "手数----指手---------消費時間--"
)

// TerminalMarkers は対局終了を表す指し手名(投了・中断など)。
// これらは駒の移動ではないので、KIF では移動元座標を付けずに出力する。
//
// 読売のペイロードは終局を `move:"投了打"` のように "打" 付きで返してくるため、
// 末尾の "打" を除いた形でも判定する(そのまま出すと ShogiHome 等が
// 「５三銀打」と同じ打ち手として解釈しようとして壊れる)。
var TerminalMarkers = []string{
	"投了", "中断", "千日手", "持将棋", "詰み", "切れ負け",
	"反則勝ち", "反則負け", "入玉勝ち", "不戦勝", "不戦敗",
}

// TerminalMarker は指し手名が対局終了を表すならその名称を返す。
func TerminalMarker(move string) (string, bool) {
	name := strings.TrimSuffix(strings.TrimSpace(move), "打")
	for _, m := range TerminalMarkers {
		if name == m {
			return m, true
		}
	}
	return "", false
}

// StripMoveModifiers は指し手の漢字表記から相対表記修飾(右/左/上/寄/引/直/不成)を除去する。
func StripMoveModifiers(move string) string {
	name := move
	for _, m := range MoveModifiers {
		name = strings.ReplaceAll(name, m, "")
	}
	return name
}

// FormatLine は KIF の1手行を組み立てる。
//
//	`{num} {移動先の漢字表記}({移動元の筋}{移動元の段})`
//
// 指し手名に "打" を含む場合は打ちなので移動元座標を 0,0 にする("打" 自体は残す)。
// 投了・中断などの終局行は座標を付けずに `{num} 投了` の形で出力する。
func FormatLine(num int, move string, fromX, fromY int) string {
	if marker, ok := TerminalMarker(move); ok {
		return strconv.Itoa(num) + " " + marker
	}
	x, y := fromX, fromY
	if strings.Contains(move, "打") {
		x, y = 0, 0
	}
	var sb strings.Builder
	sb.WriteString(strconv.Itoa(num))
	sb.WriteByte(' ')
	sb.WriteString(StripMoveModifiers(move))
	sb.WriteByte('(')
	sb.WriteString(strconv.Itoa(x))
	sb.WriteString(strconv.Itoa(y))
	sb.WriteByte(')')
	return sb.String()
}

// Move は KIF 1手分の情報。
type Move struct {
	Num   int    // 手数(1 始まり)
	Name  string // 指し手の漢字表記(例 "７六歩", "５五角打", "７八銀右")
	FromX int    // 移動元の筋(打ちの場合は無視される)
	FromY int    // 移動元の段(同上)
	// Spend はこの手の消費時間。Document.ShowTime が true のときだけ出力する。
	// 取得元によっては分単位までしか分からない(読売の棋譜は 60 秒刻み)。
	Spend time.Duration
	// Comment はこの手に付くコメント(KIF の `*` 行)。複数行なら "\n" で連結する。
	// KIF ではコメントは**注釈する手の直後**に置く。
	Comment string
}

// FormatSpend は消費時間を KIF の "分:秒" 表記にする(例 " 4:00")。
// 分は 60 を超えても繰り上げない(KIF の1手消費時間欄の慣例)。
func FormatSpend(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d / time.Second)
	return fmt.Sprintf("%2d:%02d", total/60, total%60)
}

// FormatTotal は累計消費時間を KIF の "時:分:秒" 表記にする(例 "01:00:00")。
func FormatTotal(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d / time.Second)
	return fmt.Sprintf("%02d:%02d:%02d", total/3600, (total/60)%60, total%60)
}

// FormatTimes は指し手行の末尾に付く消費時間欄を組み立てる。
//
//	( 4:00/01:00:00)   消費時間 / その対局者の累計消費時間
func FormatTimes(spend, total time.Duration) string {
	return "(" + FormatSpend(spend) + "/" + FormatTotal(total) + ")"
}

// Document は KIF ドキュメント1局分。ヘッダ項目は空なら出力されない。
type Document struct {
	Handicap  string        // 手合割(空なら "平手")
	Event     string        // 棋戦名
	Place     string        // 対局場所
	StartedAt time.Time     // 開始日時(ゼロ値なら出力しない)
	Black     string        // 先手(下手)
	White     string        // 後手(上手)
	TimeLimit time.Duration // 持ち時間(0 なら出力しない)
	Countdown time.Duration // 秒読み(0 なら出力しない)
	// ShowTime が true なら各手に消費時間欄を出力する。
	// 取得元が消費時間を持たない場合に "( 0:00/00:00:00)" が並ぶのを避けるため、
	// 明示的なフラグにしている(0 秒の手は正当に存在しうるので値からは判定できない)。
	ShowTime bool
	// Comment は初期局面に付くコメント(KIF の `*` 行)。複数行なら "\n" で連結する。
	// 初手より前に置かれた `*` 行がここに入る(読売のペイロードなら num:0 のコメント)。
	Comment string
	Moves   []Move
}

// Empty はヘッダも指し手も1つも読み取れなかったことを表す。
//
// Parse は**指し手が 0 手でもエラーにしない**(対局前の中継棋譜は
// ヘッダだけで指し手がまだ無い)。そのため「そもそも KIF ではない」の判定は
// 手数ではなくこちらで行う —— HTML やただの文章を読ませた場合は
// ヘッダも指し手も取れないので true になる。
func (d Document) Empty() bool {
	return len(d.Moves) == 0 &&
		d.Handicap == "" && d.Event == "" && d.Place == "" &&
		d.Black == "" && d.White == "" &&
		d.StartedAt.IsZero() && d.TimeLimit == 0 && d.Countdown == 0
}

// FormatTimeLimit は持ち時間を KIF のヘッダ表記にする(例 "各8時間", "各25分")。
func FormatTimeLimit(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("各%d時間", int(d/time.Hour))
	}
	return fmt.Sprintf("各%d分", int(d/time.Minute))
}

// String は Document を KIF テキストに変換する(行末は "\n"、末尾にも改行)。
//
// ヘッダは KIF の慣例に従い "開始日時：" → "棋戦：" → "場所：" → "手合割：" →
// "先手：" → "後手：" の順に出力し、続けて指し手の列ヘッダと各手を並べる。
func (d Document) String() string {
	var sb strings.Builder
	writeHeader := func(key, value string) {
		if value == "" {
			return
		}
		sb.WriteString(key)
		sb.WriteString("：")
		sb.WriteString(value)
		sb.WriteByte('\n')
	}

	if !d.StartedAt.IsZero() {
		writeHeader("開始日時", d.StartedAt.Format("2006/01/02 15:04:05"))
	}
	writeHeader("棋戦", d.Event)
	writeHeader("場所", d.Place)

	handicap := d.Handicap
	if handicap == "" {
		handicap = HirateHandicap
	}
	writeHeader("手合割", handicap)

	writeHeader("先手", d.Black)
	writeHeader("後手", d.White)
	writeHeader("持ち時間", FormatTimeLimit(d.TimeLimit))
	if d.Countdown > 0 {
		writeHeader("秒読み", fmt.Sprintf("%d秒", int(d.Countdown/time.Second)))
	}

	sb.WriteString(MoveColumnHeader)
	sb.WriteByte('\n')

	// 初期局面のコメントは列ヘッダの直後、初手より前に置く。
	writeComment(&sb, d.Comment)

	//累計消費時間は対局者ごとに積む(奇数手が先手、偶数手が後手)。
	var totalBlack, totalWhite time.Duration
	for _, m := range d.Moves {
		sb.WriteString(FormatLine(m.Num, m.Name, m.FromX, m.FromY))
		if d.ShowTime {
			total := &totalWhite
			if m.Num%2 == 1 {
				total = &totalBlack
			}
			*total += m.Spend
			sb.WriteString("   ")
			sb.WriteString(FormatTimes(m.Spend, *total))
		}
		sb.WriteByte('\n')
		// コメントは注釈する手の**直後**に置く。
		writeComment(&sb, m.Comment)
	}
	return sb.String()
}

// writeComment はコメントを KIF の `*` 行として書き出す(空なら何も書かない)。
// 複数行のコメントは1行ずつ `*` を付ける。
func writeComment(sb *strings.Builder, comment string) {
	if comment == "" {
		return
	}
	for _, line := range strings.Split(normalizeNewlines(comment), "\n") {
		sb.WriteByte('*')
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
}
