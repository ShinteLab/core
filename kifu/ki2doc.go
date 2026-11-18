package kifu

import (
	"fmt"
	"regexp"
	"strings"
)

// KI2 形式の棋譜ファイル（ヘッダは KIF と同じ、指し手は**移動元を書かない**表記）。
//
//	手合割：平手
//	先手：永瀬拓矢
//	後手：伊藤匠
//	▲２六歩    △８四歩
//	▲２五歩
//	*コメント
//	まで97手で先手の勝ち
//
// **指し手の表記は推測で直さない。** `KI2Move.Text` は書かれていたとおりに持つ
// （手入力の中継の誤記もそのまま）。盤に当てて KIF にするのは `KIF()` の役目で、
// 当てられなければそこで止まる（`ErrKI2Move`）。原本の誤記を残すのは、
// あとで誰かが直すときの拠り所になるため。

// KI2Document は KI2 形式の棋譜ファイル。ヘッダは KIF と同じ、指し手は移動元を書かない表記。
//
// 埋め込んだ Document はヘッダ（Handicap/Event/Place/StartedAt/Black/White/
// TimeLimit/Countdown/Comment/Start）だけを使う。⚠️ **`Document.Moves` は使わない**（空のまま）。
type KI2Document struct {
	Document
	Moves []KI2Move
	// End は終局名（"投了" など TerminalMarkers のもの）。対局中・不明は空。
	End string
}

// KI2Move は KI2 の 1 手。
type KI2Move struct {
	// Text は手番記号つきの表記そのまま（"▲２六歩" "△同　銀" "▲５三銀引成"）。推測で直さない。
	// 中継に手が欠けていたときは呼び出し側が "▲？" のような置き場を入れてよい（KIF() はそこで止まる）。
	Text string
	// Comment はこの手の直後のコメント（複数行は "\n"）。
	Comment string
}

// Empty はヘッダも指し手も 1 つも読み取れなかったことを表す（埋め込んだ Document.Empty は
// KI2 の指し手を見ないので上書きする）。
func (d KI2Document) Empty() bool {
	return len(d.Moves) == 0 && d.End == "" && d.Document.Empty()
}

// EndMark は終局名を返す（埋め込んだ Document.EndMark は KI2 の指し手を見ないので上書きする）。
func (d KI2Document) EndMark() string { return d.End }

// ki2Marks は指し手の先頭に付く手番記号。
const ki2Marks = "▲△☗☖▽"

// String は KI2 テキストにする（行末は "\n"、末尾にも改行）。
//
// ヘッダは KIF と同じ（`Document.writeHeaders`）。**指し手の列ヘッダは書かない**（KIF だけのもの）。
// 指し手は 1 行に 1 手、コメントはその手の直後の `*` 行。終局は `まで…` の行。
func (d KI2Document) String() string {
	var sb strings.Builder
	d.Document.writeHeaders(&sb)
	writeComment(&sb, d.Comment)
	for _, m := range d.Moves {
		sb.WriteString(m.Text)
		sb.WriteByte('\n')
		writeComment(&sb, m.Comment)
	}
	if d.End != "" {
		sb.WriteString(d.resultLine())
		sb.WriteByte('\n')
	}
	return sb.String()
}

// resultLine は `まで{n}手で…` の行を作る。
//
// 投了は「勝った側の勝ち」と書く。**投了したのは n 手指したあとの手番の側**で、
// 勝ったのはその相手。手番は開始局面から数える（駒落ちは上手から指す）。
// 呼び名は平手（と盤面図）なら 先手/後手、駒落ちなら 下手/上手。
func (d KI2Document) resultLine() string {
	n := len(d.Moves)
	if d.End != "投了" {
		return fmt.Sprintf("まで%d手で%s", n, d.End)
	}
	blackFirst := true
	if start, err := d.Document.StartSFEN(); err == nil {
		if f := strings.Fields(start); len(f) >= 2 && f[1] == "w" {
			blackFirst = false
		}
	}
	// n 手のあとの手番（＝投了した側）が先手か。
	resignedBlack := blackFirst == (n%2 == 0)
	winnerBlack := !resignedBlack
	komaochi := d.Start == "" && d.Handicap != "" && d.Handicap != HirateHandicap
	name := map[[2]bool]string{
		{true, false}: "先手", {false, false}: "後手",
		{true, true}: "下手", {false, true}: "上手",
	}[[2]bool{winnerBlack, komaochi}]
	return fmt.Sprintf("まで%d手で%sの勝ち", n, name)
}

// resultPattern は `まで97手で先手の勝ち` / `まで100手で千日手` の行。
var resultPattern = regexp.MustCompile(`^まで(\d+)手で(.+)$`)

// kifMoveLine は KIF の指し手行（手数で始まる行）。IsKI2 の見分けに使う。
var kifMoveLine = regexp.MustCompile(`^\s*\d+\s+\S`)

// ParseKI2 は KI2 テキストを読む。
//
// ヘッダと盤面図の読み方は `Parse`（KIF）と同じ。`*` 行は直前の手のコメント
// （まだ 1 手も無ければ初期局面のコメント）。1 行に何手あってもよい
// （`▲２六歩    △８四歩`）。`まで…` の行と、`△投了` のような手番記号つきの終局は End に入る。
//
// **指し手が 0 手でもエラーにしない**（対局前の中継）。エラーになるのは別の局面として
// 読めてしまうもの（読めない盤面図）だけ。⚠️ 知らない手合割は `Parse` と同じく
// ここでは断らない（`KIF()` が `StartSFEN` で断る）。
// **変化（`変化：N手`）は未対応**で、そこで打ち切る（`Parse` と同じ）。
func ParseKI2(text string) (KI2Document, error) {
	var d KI2Document
	lines := strings.Split(normalizeNewlines(stripBOM(text)), "\n")
	if HasBOD(text) {
		start, err := ParseBOD(lines)
		if err != nil {
			return KI2Document{}, err
		}
		d.Start = start
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || isBODLine(line) {
			continue
		}
		switch {
		case strings.HasPrefix(line, "*"):
			d.addComment(strings.TrimPrefix(line, "*"))
			continue
		case strings.HasPrefix(line, "#"), strings.HasPrefix(line, "手数"):
			continue
		case strings.HasPrefix(line, "変化："):
			return d, nil
		case strings.HasPrefix(line, "まで"):
			if end, ok := parseResult(line); ok && d.End == "" {
				d.End = end
			}
			continue
		}
		if strings.ContainsRune(ki2Marks, []rune(line)[0]) {
			for _, tok := range splitKI2Moves(line) {
				if end, ok := TerminalMarker(strings.TrimLeft(tok, ki2Marks)); ok {
					if d.End == "" {
						d.End = end
					}
					continue
				}
				d.Moves = append(d.Moves, KI2Move{Text: tok})
			}
			continue
		}
		if key, value, ok := splitHeader(line); ok {
			applyHeader(&d.Document, key, value)
		}
		// 解釈できない行は読み飛ばす（方言が多いため落とさない）。
	}
	return d, nil
}

// addComment は `*` 行の中身を直前の手のコメントに足す（まだ 1 手も無ければ初期局面）。
func (d *KI2Document) addComment(text string) {
	dst := &d.Comment
	if n := len(d.Moves); n > 0 {
		dst = &d.Moves[n-1].Comment
	}
	if *dst != "" {
		*dst += "\n"
	}
	*dst += text
}

// splitKI2Moves は 1 行を手ごとに分ける。**区切りは手番記号**であって空白ではない
// （"同　歩" の全角空白は手の一部なので、空白で割ると 1 手が 2 つに割れる）。
func splitKI2Moves(line string) []string {
	var out []string
	start := -1
	for i, r := range line {
		if strings.ContainsRune(ki2Marks, r) {
			if start >= 0 {
				out = appendMove(out, line[start:i])
			}
			start = i
		}
	}
	if start >= 0 {
		out = appendMove(out, line[start:])
	}
	return out
}

func appendMove(out []string, tok string) []string {
	tok = strings.TrimRight(tok, " \t　")
	if tok == "" {
		return out
	}
	return append(out, tok)
}

// parseResult は `まで…` の行から終局名を読む。「…の勝ち」は投了として読む。
func parseResult(line string) (string, bool) {
	m := resultPattern.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	rest := strings.TrimSpace(m[2])
	for _, side := range []string{"先手", "後手", "下手", "上手"} {
		if rest == side+"の勝ち" {
			return "投了", true
		}
	}
	return TerminalMarker(rest)
}

// IsKI2 は KI2 の棋譜らしいかを返す。手番記号で始まる指し手の行（▲７六歩 / △同　歩）が
// 1 つ以上あり、KIF の指し手行（手数で始まる行）が 1 つも無ければ KI2 とみなす。
func IsKI2(text string) bool {
	found := false
	for _, raw := range strings.Split(normalizeNewlines(stripBOM(text)), "\n") {
		if kifMoveLine.MatchString(raw) {
			return false
		}
		line := strings.TrimSpace(raw)
		rs := []rune(line)
		if len(rs) < 2 || !strings.ContainsRune(ki2Marks, rs[0]) {
			continue
		}
		if _, ok := fileNumbers[rs[1]]; ok || rs[1] == '同' {
			found = true
		}
	}
	return found
}

// KIF は指し手を盤から割り出して KIF の Document にする（`DecodeKI2`）。
//
// ⚠️ **エラー（`ErrKI2Move` を包む）でも、そこまでの手を入れた Document を返す。**
// そのときは**終局を付けない**（読めた所までで終わった棋譜は終局ではない）。
// errMove はエラーの手（1 始まり。エラーが無ければ 0）。
// 開始局面が読めなければ（知らない手合割など）そのエラーを返し、errMove は 1。
func (d KI2Document) KIF() (doc Document, errMove int, err error) {
	doc = d.Document
	doc.Moves = nil
	start, err := d.Document.StartSFEN()
	if err != nil {
		return doc, 1, err
	}
	names := make([]string, len(d.Moves))
	for i, m := range d.Moves {
		names[i] = m.Text
	}
	texts, end, err := DecodeKI2(start, names)
	for i, t := range texts {
		doc.Moves = append(doc.Moves, Move{
			Num: i + 1, Name: t.Name, FromX: t.FromX, FromY: t.FromY,
			Comment: d.Moves[i].Comment,
		})
	}
	if err != nil {
		return doc, len(texts) + 1, err
	}
	if end == "" {
		end = d.End
	}
	if end != "" {
		doc.Moves = append(doc.Moves, Move{Num: len(doc.Moves) + 1, Name: end})
	}
	return doc, 0, nil
}

// NormalizeKI2Move は中継の書き方を KI2 の書き方に寄せる（**情報は落とさない**）。
//
//	"▲55金" → "▲５五金"   移動先の算用数字を 筋は全角・段は漢数字 に
//	"▲同玉" → "▲同　玉"   「同」の後に全角空白
//
// 駒名・修飾・成/不成/打は触らない（"竜" を "龍" にもしない）。読めない形はそのまま返す。
func NormalizeKI2Move(s string) string {
	rs := []rune(strings.TrimSpace(s))
	mark := ""
	if len(rs) > 0 && strings.ContainsRune(ki2Marks, rs[0]) {
		mark, rs = string(rs[0]), rs[1:]
	}
	if len(rs) == 0 {
		return s
	}
	if rs[0] == '同' {
		rest := strings.TrimLeft(string(rs[1:]), " \t　")
		if rest == "" {
			return s
		}
		return mark + SameSquare + rest
	}
	if len(rs) < 2 {
		return s
	}
	f, okF := fileNumbers[rs[0]]
	r, okR := ki2Ranks[rs[1]]
	if !okF || !okR {
		return s
	}
	return mark + fileKanji[f] + rankKanji[r] + string(rs[2:])
}
