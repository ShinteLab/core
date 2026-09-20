package kifu

// 盤面図（BOD）の読み書き（2026-09-16）。
//
// **KIF は本来「手合割 ＋ 初手からの指し手」でしか局面を表せない。** 途中の局面から
// 始まる棋譜（撮った中継の 1 局面・詰将棋・次の一手）はそれでは書けないので、
// KIF には**盤面図**という書き方がある。ここはその読み書き。
//
//	後手の持駒：なし
//	  ９ ８ ７ ６ ５ ４ ３ ２ １
//	+---------------------------+
//	|v香v桂v銀v金v玉v金v銀v桂v香|一
//	（…9 段…）
//	+---------------------------+
//	先手の持駒：なし
//	後手番
//
// ⚠️ **1 マスは必ず 2 文字**（`v歩` / ` 歩` / ` ・`）。だから成駒は
// **1 文字の書き方**（と杏圭全馬龍）になる —— 指し手の表記（成香・成桂・成銀）を
// そのまま持ち込むと**桁がずれて盤が崩れる。**
//
// ⚠️ **駒落ちでは「上手／下手」と書かれる。** 書くときは先手／後手で揃えるが、
// **読むときは両方受けること**（KIF は方言が多い）。

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ShinteLab/core/sfen"
)

// BOD の各行の目印。**読み取りの入口**でもある。
const (
	bodTopLine    = "+---------------------------+"
	bodFileHeader = "  ９ ８ ７ ６ ５ ４ ３ ２ １"
)

// bodRanks は段の見出し（1 段目から）。
var bodRanks = [9]string{"一", "二", "三", "四", "五", "六", "七", "八", "九"}

// bodPieces はベース駒コード → 盤面図の 1 文字（生駒）。
var bodPieces = map[int]string{
	sfen.Pawn: "歩", sfen.Lance: "香", sfen.Knight: "桂", sfen.Silver: "銀",
	sfen.Gold: "金", sfen.Rook: "飛", sfen.Bishop: "角", sfen.King: "玉",
}

// bodPromoted はベース駒コード → 盤面図の 1 文字（成駒）。
//
// ⚠️ **「成香」ではなく「杏」。** 盤面図は 1 マス 2 文字と決まっているので、
// 2 文字の駒名を書くと**そこから右が全部ずれる。**
var bodPromoted = map[int]string{
	sfen.Pawn: "と", sfen.Lance: "杏", sfen.Knight: "圭", sfen.Silver: "全",
	sfen.Rook: "龍", sfen.Bishop: "馬",
}

// bodPiece は盤面図の駒 1 文字が指すもの。
type bodPiece struct {
	base     int
	promoted bool
}

// bodLetters は盤面図の 1 文字 → （ベース駒コード・成っているか）。
//
// ⚠️ **別名も受けること**（`竜`/`王`、成香などの 2 文字表記）。KIF を書く
// ソフトによって揺れるので、**読む側は広く受ける**（書く側は 1 通りに揃える）。
var bodLetters = map[string]bodPiece{
	"歩": {sfen.Pawn, false}, "香": {sfen.Lance, false}, "桂": {sfen.Knight, false},
	"銀": {sfen.Silver, false}, "金": {sfen.Gold, false}, "飛": {sfen.Rook, false},
	"角": {sfen.Bishop, false}, "玉": {sfen.King, false}, "王": {sfen.King, false},
	"と": {sfen.Pawn, true}, "杏": {sfen.Lance, true}, "圭": {sfen.Knight, true},
	"全": {sfen.Silver, true}, "龍": {sfen.Rook, true}, "竜": {sfen.Rook, true},
	"馬": {sfen.Bishop, true},
	// 2 文字の書き方（持駒の行で使われることがある。盤面のマスには入らない）。
	"成香": {sfen.Lance, true}, "成桂": {sfen.Knight, true}, "成銀": {sfen.Silver, true},
}

// bodNumbers は持駒の枚数（漢数字）。**1 枚は書かない**のが KIF の書式。
var bodNumbers = [...]string{
	"", "", "二", "三", "四", "五", "六", "七", "八", "九",
	"十", "十一", "十二", "十三", "十四", "十五", "十六", "十七", "十八",
}

// HasBOD は KIF に盤面図が含まれているか。
//
// **「この棋譜は途中の局面から始まる」を知りたいなら `Document.Start` を見ること。**
// こちらは読み取る前のテキストしか無い場面のためのもの。
func HasBOD(s string) bool { return strings.Contains(s, bodTopLine) }

// FormatBOD は完全形 SFEN を盤面図にする（末尾は改行）。
//
// ⚠️ **手番の行は後手番のときだけ書く**（`後手番`）。先手番は KIF の既定なので、
// 毎回書くと**わざわざ手番を指定した棋譜**に見える。
func FormatBOD(full string) (string, error) {
	b, err := parseBoard(full)
	if err != nil {
		return "", err
	}
	hands, err := parseSFENHands(full)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("後手の持駒：")
	sb.WriteString(formatBODHand(hands[false]))
	sb.WriteByte('\n')
	sb.WriteString(bodFileHeader)
	sb.WriteByte('\n')
	sb.WriteString(bodTopLine)
	sb.WriteByte('\n')
	for y := 1; y <= 9; y++ {
		sb.WriteByte('|')
		// ⚠️ **左から 9 筋 → 1 筋**（画面と同じ並び）。内部座標の x は
		// 1 が 9 筋なので、そのまま 1..9 で回してよい。
		for x := 1; x <= 9; x++ {
			sb.WriteString(bodCell(b.at(x, y)))
		}
		sb.WriteByte('|')
		sb.WriteString(bodRanks[y-1])
		sb.WriteByte('\n')
	}
	sb.WriteString(bodTopLine)
	sb.WriteByte('\n')
	sb.WriteString("先手の持駒：")
	sb.WriteString(formatBODHand(hands[true]))
	sb.WriteByte('\n')
	if !b.black {
		sb.WriteString("後手番\n")
	}
	return sb.String(), nil
}

// bodCell は 1 マスを 2 文字にする。
func bodCell(s square) string {
	if !s.occupied {
		return " ・"
	}
	name := bodPieces[s.base]
	if s.promoted {
		if p, ok := bodPromoted[s.base]; ok {
			name = p
		}
	}
	if s.black {
		return " " + name
	}
	return "v" + name
}

// formatBODHand は駒台を「飛　角　金二　歩三」の形にする（空なら「なし」）。
//
// ⚠️ **並びは `sfen.HandOrder`**（飛角金銀桂香歩）。**独自の順を持たないこと** ——
// 持ち駒の並びは仕様で、ここで決め直すと SFEN 側と食い違う。
func formatBODHand(hand map[int]int) string {
	parts := []string{}
	for _, base := range sfen.HandOrder {
		n := hand[base]
		if n <= 0 {
			continue
		}
		name := bodPieces[base]
		if n < len(bodNumbers) {
			name += bodNumbers[n]
		} else {
			// 上限（歩 18 枚）を超えることは無いはずだが、**黙って落とさない**。
			name += strconv.Itoa(n)
		}
		parts = append(parts, name)
	}
	if len(parts) == 0 {
		return "なし"
	}
	// ⚠️ **区切りは全角スペース**（KIF の書式）。
	return strings.Join(parts, "　")
}

// parseSFENHands は完全形 SFEN の持ち駒欄を（先手か → 駒コード → 枚数）にする。
func parseSFENHands(full string) (map[bool]map[int]int, error) {
	out := map[bool]map[int]int{true: {}, false: {}}
	fields := strings.Fields(strings.TrimSpace(full))
	if len(fields) < 3 || fields[2] == "-" {
		return out, nil
	}
	n := 0
	for i := 0; i < len(fields[2]); i++ {
		c := fields[2][i]
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
			continue
		}
		base, black := sfen.ParsePieceLetter(c)
		if base == sfen.NotFound {
			return nil, fmt.Errorf("kifu: 持ち駒が読めません: %q", fields[2])
		}
		if n == 0 {
			n = 1
		}
		out[black][base] += n
		n = 0
	}
	return out, nil
}

// ParseBOD は盤面図を完全形 SFEN にする（手数は 1）。
//
// lines は棋譜全体の行でよい（盤面図の外は読み飛ばす）。
// **盤面図が無ければエラー** —— 「無い」と「読めない」を呼び出し側で分けられる
// ように、空文字を返して黙らない。
//
// ⚠️ **手数は 1 で返す。** 途中の局面なのに何手目かは盤面図に書かれていない
// （書く欄が無い）。**推測しないこと** —— 手数が要るなら別の欄で持つ。
func ParseBOD(lines []string) (string, error) {
	var cells [81]square
	rank := 0
	inside := false
	hands := map[bool]map[int]int{true: {}, false: {}}
	black := true
	seen := false

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "+---"):
			// ⚠️ **2 本目で閉じること。** 閉じないと、盤面図のあとに
			// たまたま縦棒で始まる行があったときに拾ってしまう。
			inside = !inside
			seen = true
			continue
		case trimmed == "後手番" || trimmed == "上手番":
			black = false
			continue
		case trimmed == "先手番" || trimmed == "下手番":
			black = true
			continue
		}
		if key, value, ok := splitHeader(trimmed); ok {
			switch key {
			case "後手の持駒", "上手の持駒":
				if err := applyBODHand(hands[false], value); err != nil {
					return "", err
				}
				continue
			case "先手の持駒", "下手の持駒":
				if err := applyBODHand(hands[true], value); err != nil {
					return "", err
				}
				continue
			case "手番":
				// ⚠️ **「手番：後手」と書くソフトもある。**
				black = !strings.HasPrefix(value, "後") && !strings.HasPrefix(value, "上")
				continue
			}
		}
		if !inside || !strings.HasPrefix(line, "|") {
			continue
		}
		if rank >= 9 {
			return "", fmt.Errorf("kifu: 盤面図の段が多すぎます")
		}
		if err := parseBODRank(line, rank, &cells); err != nil {
			return "", err
		}
		rank++
	}
	if !seen {
		return "", fmt.Errorf("kifu: 盤面図がありません")
	}
	if rank != 9 {
		return "", fmt.Errorf("kifu: 盤面図の段が %d しかありません", rank)
	}

	board := sfen.FormatBoard(func(r, f int) string {
		// SFEN の記述順（rank=0 が一段目、file=0 が 9 筋）は内部座標の
		// x=file+1 / y=rank+1 と同じ。
		s := cells[r*9+f]
		if !s.occupied {
			return ""
		}
		letter := sfen.Letter(s.base)
		if !s.black {
			letter = strings.ToLower(letter)
		}
		if s.promoted {
			letter = "+" + letter
		}
		return letter
	})
	turn := "b"
	if !black {
		turn = "w"
	}
	return fmt.Sprintf("%s %s %s 1", board, turn, formatSFENHands(hands)), nil
}

// parseBODRank は 1 段（縦棒で挟まれた 9 マス）を読む。
func parseBODRank(line string, rank int, cells *[81]square) error {
	body := strings.TrimPrefix(line, "|")
	if i := strings.LastIndex(body, "|"); i >= 0 {
		body = body[:i]
	}
	runes := []rune(body)
	if len(runes) < 18 {
		return fmt.Errorf("kifu: 盤面図の段が短すぎます: %q", line)
	}
	for file := 0; file < 9; file++ {
		// ⚠️ **1 マスは必ず 2 文字**（先後の印 ＋ 駒）。
		mark, name := runes[file*2], string(runes[file*2+1])
		if name == "・" || name == "." {
			continue
		}
		p, ok := bodLetters[name]
		if !ok {
			return fmt.Errorf("kifu: 盤面図に読めない駒があります: %q", name)
		}
		cells[rank*9+file] = square{
			base: p.base, promoted: p.promoted,
			black: mark != 'v' && mark != 'V', occupied: true,
		}
	}
	return nil
}

// applyBODHand は「飛　角　金二　歩三」を駒台に積む（「なし」は何もしない）。
func applyBODHand(hand map[int]int, value string) error {
	v := strings.TrimSpace(value)
	if v == "" || v == "なし" {
		return nil
	}
	// ⚠️ **区切りは全角スペースだが、半角で書かれることもある。**
	v = strings.ReplaceAll(v, "　", " ")
	for _, part := range strings.Fields(v) {
		runes := []rune(part)
		// 駒名は 1 文字（成香などの 2 文字も受ける）。
		name := string(runes[0])
		rest := string(runes[1:])
		if _, ok := bodLetters[name]; !ok && len(runes) >= 2 {
			name = string(runes[:2])
			rest = string(runes[2:])
		}
		p, ok := bodLetters[name]
		if !ok {
			return fmt.Errorf("kifu: 持駒に読めない駒があります: %q", part)
		}
		n, err := parseBODCount(rest)
		if err != nil {
			return fmt.Errorf("kifu: 持駒の枚数が読めません（%s）: %w", part, err)
		}
		// ⚠️ **成駒は生駒として積むこと**（駒台に成駒は無い）。
		hand[p.base] += n
	}
	return nil
}

// parseBODCount は漢数字の枚数を読む（空なら 1）。
func parseBODCount(s string) (int, error) {
	v := strings.TrimSpace(s)
	if v == "" {
		return 1, nil
	}
	for i, name := range bodNumbers {
		if i >= 2 && name == v {
			return i, nil
		}
	}
	// ⚠️ **算用数字でも受けること**（手で書いた KIF は珍しくない）。
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return n, nil
	}
	return 0, fmt.Errorf("%q", v)
}

// formatSFENHands は駒台を SFEN の持ち駒欄にする（空なら "-"）。
func formatSFENHands(hands map[bool]map[int]int) string {
	var sb strings.Builder
	// ⚠️ **先手が先・`sfen.HandOrder` の順**（SFEN の慣例）。
	// ⚠️ **玉は落ちる**（`HandOrder` に無い）。**駒台の玉は SFEN で表せない**ので
	// 他にやりようが無い —— 盤面図にそう書いてあっても、読み戻したときには消える。
	for _, black := range []bool{true, false} {
		for _, base := range sfen.HandOrder {
			n := hands[black][base]
			if n <= 0 {
				continue
			}
			if n > 1 {
				sb.WriteString(strconv.Itoa(n))
			}
			letter := sfen.Letter(base)
			if !black {
				letter = strings.ToLower(letter)
			}
			sb.WriteString(letter)
		}
	}
	if sb.Len() == 0 {
		return "-"
	}
	return sb.String()
}
