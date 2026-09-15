package kifu

// KIF の指し手（"▲７六歩" の側）を USI の手文字列（"7g7f"）にする方向の変換。
//
// **notation.go の逆向き。** あちらは USI → 日本語表記で、盤が要る（手文字列に
// 駒種が書いていないため）。こちらは**盤が要らない**:
//
//   - 移動元は KIF 行に数字で書いてある（`７六歩(77)` の "(77)"）
//   - 成/不成・打ちは指し手名に書いてある
//   - 打つ駒の種類も指し手名に書いてある
//
// 唯一の状態が「同」（直前の手の移動先）なので、Decoder が 1 つ覚えるだけで済む。
//
// ⚠️ **合法性は見ない。** 読めた手をそのまま USI にするだけで、その手が指せるかは
// 呼び出し側（合法手生成を持っている層）の問題。ここは表記の変換に徹する。

import (
	"fmt"
	"strings"

	"github.com/ShinteLab/core/usi"
)

// 手合割ごとの初期局面。**駒を落とすのは上手（後手）側**なので、盤面の 1〜2 段目
// （SFEN の先頭 2 フィールド）だけが平手と違う。
//
// ⚠️ **駒落ちは上手（後手）が初手を指す**ので手番は "w"。手数は SFEN の数え方で 1。
// これは「偶奇が合わない」ように見えるが正当な局面（`w` + 手数 1）。
//
// ⚠️ **「左」は上手から見た左＝1 筋側。** 香落ちが落とすのは 1一 の香で、
// 9一 のほうは「右香落ち」。逆にすると盤が左右反転した棋譜になる。
var handicapBoards = map[string]string{
	HirateHandicap: "lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL b - 1",
	"香落ち":         "lnsgkgsn1/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1",
	"右香落ち":        "1nsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1",
	"角落ち":         "lnsgkgsnl/1r7/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1",
	"飛車落ち":        "lnsgkgsnl/7b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1",
	"飛香落ち":        "lnsgkgsn1/7b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1",
	"二枚落ち":        "lnsgkgsnl/9/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1",
	"三枚落ち":        "lnsgkgsn1/9/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1",
	"四枚落ち":        "1nsgkgsn1/9/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1",
	"五枚落ち":        "1nsgkgs2/9/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1",
	"六枚落ち":        "2sgkgs2/9/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1",
	"八枚落ち":        "3gkg3/9/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1",
	"十枚落ち":        "4k4/9/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1",
}

// handicapAliases は同じ手合割の別名。
var handicapAliases = map[string]string{
	"":     HirateHandicap,
	"飛落ち":  "飛車落ち",
	"左香落ち": "香落ち",
}

// StartSFEN は手合割名に対応する初期局面の SFEN（完全形）を返す。
//
// 知らない手合割はエラー。**平手に倒さないこと** —— 別の初期配置の棋譜を
// 平手として読むと、そこから先の手が全部でたらめになる（しかも棋譜としては
// 読めてしまうので画面では気づけない）。
func StartSFEN(handicap string) (string, error) {
	name := strings.TrimSpace(handicap)
	if alias, ok := handicapAliases[name]; ok {
		name = alias
	}
	if s, ok := handicapBoards[name]; ok {
		return s, nil
	}
	return "", fmt.Errorf("kifu: 未対応の手合割です: %q", handicap)
}

// StartSFEN は Document の開始局面の SFEN を返す。
//
// ⚠️ **盤面図（`Start`）があればそちらが勝つ**（2026-09-16）。途中の局面から
// 始まる棋譜は手合割では表せないので、**両方あるなら書いてある局面のほうが本物**。
func (d Document) StartSFEN() (string, error) {
	if s := strings.TrimSpace(d.Start); s != "" {
		return s, nil
	}
	return StartSFEN(d.Handicap)
}

// 筋（全角数字。KIF は全角だが半角で書かれたものも読む）。
var fileNumbers = map[rune]int{
	'１': 1, '２': 2, '３': 3, '４': 4, '５': 5, '６': 6, '７': 7, '８': 8, '９': 9,
	'1': 1, '2': 2, '3': 3, '4': 4, '5': 5, '6': 6, '7': 7, '8': 8, '9': 9,
}

// 段（漢数字）。
var rankNumbers = map[rune]int{
	'一': 1, '二': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9,
}

// pieceLetters は駒の日本語名 → USI の駒文字（打ちに使う大文字）。
//
// **打てるのは成っていない駒だけ**なので、ここに載せるのは表の駒種のみ。
// 盤上の駒を動かす手は移動元・移動先だけで表せるので駒種を要らない。
var pieceLetters = map[string]byte{
	"歩": 'P', "香": 'L', "桂": 'N', "銀": 'S', "金": 'G',
	"角": 'B', "飛": 'R', "玉": 'K', "王": 'K',
}

// promotedNames は成駒の名前（打ちには使えないが、指し手名としては出てくる）。
//
// ⚠️ **1 文字の略記（杏・圭・全）も受けること。** 中継の KIF は
// `８二杏(92)` のように略記で書くことがある（成香・成桂・成銀は 2 文字だと
// 他の駒より横に広くなるため）。受けないとその 1 手で読み取りが止まる。
// **書き出すのは常に正式名**（notation.go）で、こちらは読む側だけの話。
var promotedNames = map[string]bool{
	"と": true, "成香": true, "成桂": true, "成銀": true, "馬": true, "龍": true, "竜": true,
	"杏": true, "圭": true, "全": true,
}

// Decoder は KIF の指し手を USI に変換していく変換器。
//
// **状態は「直前の手の移動先」だけ**（"同　歩" のため）。棋譜 1 本につき 1 つ作る。
type Decoder struct {
	// prevFile/prevRank は直前の手の移動先（KIF の筋・段）。0 なら「同」は読めない。
	prevFile, prevRank int
}

// NewDecoder は変換器を作る。
func NewDecoder() *Decoder { return &Decoder{} }

// Next は 1 手を USI 手文字列にする。
//
// 終局（投了・中断など）は手ではないので ok=false を返す（エラーではない）。
func (d *Decoder) Next(m Move) (move string, ok bool, err error) {
	name := strings.TrimSpace(m.Name)
	if _, terminal := TerminalMarker(name); terminal {
		return "", false, nil
	}

	file, rank, rest, err := d.cutDestination(name)
	if err != nil {
		return "", false, err
	}

	drop := false
	promote := false
	switch {
	case strings.HasSuffix(rest, "打"):
		drop = true
		rest = strings.TrimSuffix(rest, "打")
	case strings.HasSuffix(rest, "不成"):
		rest = strings.TrimSuffix(rest, "不成")
	case strings.HasSuffix(rest, "成"):
		// ⚠️ **成駒の名前と紛れない。** "成銀" は "銀" で終わるので、
		// 末尾の "成" は必ず「成った」の意味。
		promote = true
		rest = strings.TrimSuffix(rest, "成")
	}
	piece := strings.TrimSpace(StripMoveModifiers(rest))
	if piece == "" {
		return "", false, fmt.Errorf("kifu: 駒の名前が読めません: %q", m.Name)
	}

	// 移動元が無ければ打ち（KIF は "打" を省くことがあるので座標でも判定する）。
	if m.FromX == 0 || m.FromY == 0 {
		drop = true
	}
	if drop {
		letter, ok := pieceLetters[piece]
		if !ok {
			if promotedNames[piece] {
				return "", false, fmt.Errorf("kifu: 成駒は打てません: %q", m.Name)
			}
			return "", false, fmt.Errorf("kifu: 打つ駒が読めません: %q", m.Name)
		}
		d.prevFile, d.prevRank = file, rank
		return string(letter) + "*" + usiSquare(file, rank), true, nil
	}

	if _, ok := pieceLetters[piece]; !ok && !promotedNames[piece] {
		return "", false, fmt.Errorf("kifu: 駒の名前が読めません: %q", m.Name)
	}
	if m.FromX < 1 || m.FromX > 9 || m.FromY < 1 || m.FromY > 9 {
		return "", false, fmt.Errorf("kifu: 移動元が盤の外です: %q(%d%d)", m.Name, m.FromX, m.FromY)
	}

	out := usiSquare(m.FromX, m.FromY) + usiSquare(file, rank)
	if promote {
		out += "+"
	}
	d.prevFile, d.prevRank = file, rank
	return out, true, nil
}

// cutDestination は指し手名の先頭から移動先を切り出す（"同" なら直前の手の移動先）。
func (d *Decoder) cutDestination(name string) (file, rank int, rest string, err error) {
	if strings.HasPrefix(name, "同") {
		if d.prevFile == 0 || d.prevRank == 0 {
			return 0, 0, "", fmt.Errorf("kifu: 直前の手が分からないので「同」を読めません: %q", name)
		}
		rest = strings.TrimPrefix(name, "同")
		// KIF は "同　歩" と全角スペースを入れる（半角のこともある）。
		rest = strings.TrimLeft(rest, " 　")
		return d.prevFile, d.prevRank, rest, nil
	}

	rs := []rune(name)
	if len(rs) < 2 {
		return 0, 0, "", fmt.Errorf("kifu: 指し手が読めません: %q", name)
	}
	f, okF := fileNumbers[rs[0]]
	r, okR := rankNumbers[rs[1]]
	if !okF || !okR {
		return 0, 0, "", fmt.Errorf("kifu: 移動先が読めません: %q", name)
	}
	return f, r, string(rs[2:]), nil
}

// usiSquare は KIF の筋・段を USI のマス文字列にする。
//
// ⚠️ **core/usi の内部 x は筋番号ではない**（FormatSquare が 10-x を書く）ので、
// 筋をそのまま渡さないこと。x = 10 - 筋。
func usiSquare(file, rank int) string { return usi.FormatSquare(10-file, rank) }

// DecodeMoves は KIF の指し手の並びをまとめて USI にする。
//
// ⚠️ **エラーを返しても、そこまでに読めた手は返す**（設計原則: 段階的に劣化する）。
// 途中の 1 手が読めなくても、そこまでを並べたほうが使える。
//
// 終局（投了・中断など）はそこで打ち切る（指し手ではないため）。
func DecodeMoves(moves []Move) ([]string, error) {
	d := NewDecoder()
	out := make([]string, 0, len(moves))
	for _, m := range moves {
		move, ok, err := d.Next(m)
		if err != nil {
			return out, fmt.Errorf("kifu: %d手目: %w", m.Num, err)
		}
		if !ok {
			break // 終局
		}
		out = append(out, move)
	}
	return out, nil
}
