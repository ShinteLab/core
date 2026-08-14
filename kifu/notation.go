package kifu

import (
	"fmt"
	"strings"

	"github.com/ShinteLab/core/sfen"
	"github.com/ShinteLab/core/usi"
)

// USI の手（"8h2b+"）を日本語の指し手表記（"▲２二角成"）にする。
//
// **駒種は手文字列に書いていない。** "8h2b+" から「角」を出すには
// **8h に何が居るか**を盤から読むしかないので、この変換は必ず局面を要る。
// さらに「同じ駒が他からも行けたか」（→ ７八銀右 の "右"）まで見るので、
// 盤全体が要る。だから USI 側（core/usi）ではなくこちらに置いてある。
//
// 使い方は「開始局面を渡して、手を順に流し込む」:
//
//	n, _ := kifu.NewNotation("lnsgkgsnl/... b - 1")
//	t, _ := n.Next("8h2b+")   // t.Text == "▲２二角成"
//	t, _ = n.Next("3a2b")     // t.Text == "△同　銀"
//
// **1 手だけ名付けたいときも同じ**（Next を 1 回呼ぶ）。
//
// ⚠️ **「同」は Notation が覚えている直前の手からしか出ない。** 開始局面より前の
// 手は知らないので、**最初の 1 手は「同」にならない**（マス名で書かれる）。
// エンジンの読み筋のように履歴を持たない用途ではこれが正しい振る舞い。

// 筋は全角数字、段は漢数字で書く（KIF の慣例）。
var (
	fileKanji = [...]string{"", "１", "２", "３", "４", "５", "６", "７", "８", "９"}
	rankKanji = [...]string{"", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
)

// SameSquare は移動先が直前の手と同じときに使う表記。
//
// **全角スペース込み**（"同　歩"）が KIF の書式。⚠️ 半角にしないこと。
const SameSquare = "同　"

// TurnMark は手番の記号。
const (
	BlackMark = "▲"
	WhiteMark = "△"
)

// pieceName は駒の日本語名を返す。玉/王 は KIF の慣例どおり両方「玉」。
func pieceName(s square) string {
	if s.promoted {
		switch s.base {
		case sfen.Pawn:
			return "と"
		case sfen.Lance:
			return "成香"
		case sfen.Knight:
			return "成桂"
		case sfen.Silver:
			return "成銀"
		case sfen.Rook:
			return "龍"
		case sfen.Bishop:
			return "馬"
		}
	}
	switch s.base {
	case sfen.Pawn:
		return "歩"
	case sfen.Lance:
		return "香"
	case sfen.Knight:
		return "桂"
	case sfen.Silver:
		return "銀"
	case sfen.Gold:
		return "金"
	case sfen.Rook:
		return "飛"
	case sfen.Bishop:
		return "角"
	case sfen.King:
		return "玉"
	}
	return "?"
}

// squareName は内部座標を「２二」のようなマス名にする。
func squareName(x, y int) string {
	if !inBoard(x, y) {
		return ""
	}
	return fileKanji[10-x] + rankKanji[y]
}

// MoveText は 1 手の日本語表記。
//
// **Name / FromX / FromY はそのまま kifu.Move に入る**（KIF 行を組み立てるとき、
// 修飾は FormatLine が StripMoveModifiers で落として座標に置き換える）。
type MoveText struct {
	// USI は元の手文字列（"8h2b+"）。**変換に失敗しても必ず入る。**
	USI string
	// Text は手番記号つきの表記（"▲２二角成"）。**画面に出すのはこちら。**
	// ⚠️ 変換できなかったときは USI がそのまま入る（設計原則: 段階的に劣化する）。
	Text string
	// Name は手番記号なしの表記（"２二角成"）。kifu.Move.Name 用。
	// 変換できなかったときは空。
	Name string
	// FromX/FromY は移動元の筋・段（**KIF の数え方**。打ち・変換失敗なら 0）。
	FromX int
	FromY int
	// Black は指した側。
	Black bool
	// OK は日本語表記にできたか。false なら Text は USI のまま。
	OK bool
}

// Notation は開始局面から手を順に日本語表記へ変換していく変換器。
//
// **状態を持つ**（局面を進めるのと、「同」のために直前の移動先を覚えるため）。
// 使い回さず、読み筋 1 本につき 1 つ作ること。
type Notation struct {
	b *board
	// prevX/prevY は直前の手の移動先（内部座標）。0 なら「同」を出さない。
	prevX, prevY int
	// broken は局面が信用できなくなったか。**一度でも手を進められなかったら立てる。**
	// そこから先の盤は実際と違うので、名付けをやめて USI をそのまま返す
	// （でたらめな表記を出すより、生の手を見せるほうがまし）。
	broken bool
}

// NewNotation は開始局面を指定して変換器を作る。
//
// sfenStr は盤面部分だけでも、手番・持ち駒・手数まで付いた完全形でもよい。
// **手番の欄が無ければ先手番として扱う。**
func NewNotation(sfenStr string) (*Notation, error) {
	b, err := parseBoard(sfenStr)
	if err != nil {
		return nil, err
	}
	return &Notation{b: b}, nil
}

// Black は次に指す側を返す。
func (n *Notation) Black() bool { return n.b.black }

// Next は 1 手を日本語表記にして、局面を進める。
//
// ⚠️ **エラーを返しても MoveText は返す**（Text に USI がそのまま入る）。
// 読み筋の途中で 1 手読めなくなっても、残りを丸ごと捨てるより生の手を並べるほうが
// 使える（設計原則: 段階的に劣化する）。呼び出し側はエラーを無視してよい。
func (n *Notation) Next(move string) (MoveText, error) {
	t := MoveText{USI: move, Text: move, Black: n.b.black}

	m, ok := usi.ParseMove(move)
	if !ok {
		n.broken = true
		return t, fmt.Errorf("kifu: 手が読めません: %q", move)
	}
	if m.Resign {
		t.Name = "投了"
		t.Text = t.Name
		t.OK = true
		return t, nil
	}
	if n.broken {
		// 盤が既に信用できない。名付けずに素通しする。
		return t, nil
	}

	name, err := n.name(m)
	if err != nil {
		n.broken = true
		return t, err
	}
	if err := n.b.apply(m); err != nil {
		n.broken = true
		return t, err
	}

	t.Name = name
	t.Text = mark(t.Black) + name
	t.OK = true
	if m.Drop == 0 {
		// KIF の移動元は筋・段。内部座標の x とは向きが逆なので直す。
		t.FromX, t.FromY = 10-m.FromX, m.FromY
	}
	n.prevX, n.prevY = m.ToX, m.ToY
	return t, nil
}

func mark(black bool) string {
	if black {
		return BlackMark
	}
	return WhiteMark
}

// name は 1 手の表記（手番記号なし）を組み立てる。**局面は進めない。**
func (n *Notation) name(m usi.Move) (string, error) {
	if !inBoard(m.ToX, m.ToY) {
		return "", fmt.Errorf("kifu: 移動先が盤の外です: %s", m.String())
	}

	var sb strings.Builder
	// 移動先。直前の手と同じマスなら「同　」。
	if m.ToX == n.prevX && m.ToY == n.prevY {
		sb.WriteString(SameSquare)
	} else {
		sb.WriteString(squareName(m.ToX, m.ToY))
	}

	if m.Drop != 0 {
		base, _ := sfen.ParsePieceLetter(m.Drop)
		if base == sfen.NotFound {
			return "", fmt.Errorf("kifu: 打つ駒が読めません: %q", string(m.Drop))
		}
		sb.WriteString(pieceName(square{base: base}))
		// ⚠️ **打ちには常に「打」を付ける。** 本来は紛らわしいときだけ付ける表記だが、
		// FormatLine が「打」の有無で移動元座標を 0 にするかを決めているので、
		// 落とすと KIF に出したときに盤上の手として読まれる。
		sb.WriteString("打")
		return sb.String(), nil
	}

	if !inBoard(m.FromX, m.FromY) {
		return "", fmt.Errorf("kifu: 移動元が盤の外です: %s", m.String())
	}
	p := n.b.at(m.FromX, m.FromY)
	if !p.occupied {
		return "", fmt.Errorf("kifu: 移動元に駒がありません: %s", m.String())
	}
	sb.WriteString(pieceName(p))
	sb.WriteString(n.qualifier(p, m.FromX, m.FromY, m.ToX, m.ToY))

	switch {
	case m.Promote:
		sb.WriteString("成")
	case canPromote(p) && (inZone(m.FromY, p.black) || inZone(m.ToY, p.black)):
		// 成れたのに成らなかった手は「不成」と書く（そこが読みの分かれ目なので）。
		sb.WriteString("不成")
	}
	return sb.String(), nil
}

// canPromote はその駒が成れる駒種かを返す（金と玉、および成駒は成れない）。
func canPromote(p square) bool {
	return !p.promoted && p.base != sfen.Gold && p.base != sfen.King
}

// inZone は敵陣（先手なら 1〜3 段目）かを返す。
func inZone(y int, black bool) bool {
	if black {
		return y <= 3
	}
	return y >= 7
}

// qualifier は同じ駒が複数そのマスへ行けるときの区別（右/左/直・上/寄/引）を返す。
//
// **どれも「駒がどこから来たか」を指す**（移動先の位置ではない）。
// 判断は指した側から見た向きなので、後手のときは上下左右をひっくり返す。
//
// 選び方は「区別が付く最小の修飾」:
//
//	1. 左右（右/左/直）だけで他の候補と区別が付くならそれ
//	2. 付かなければ上下（上/寄/引）だけで試す
//	3. どちらも駄目なら両方（"右上" のように左右が先）
//
// ⚠️ **候補にピンは効かない**（board.canReach は合法性を見ない）。実際には動けない
// 駒まで数えるので、**要らない修飾が付くことがある**。指し手そのものは変わらない。
func (n *Notation) qualifier(p square, fx, fy, tx, ty int) string {
	others := n.rivals(p, fx, fy, tx, ty)
	if len(others) == 0 {
		return ""
	}

	h, v := relation(fx, fy, tx, ty, p.black)
	hLabel := horizontalLabel(h, v)
	vLabel := verticalLabel(v)

	// ⚠️ **比べるのは向きの値（h/v）であって、ラベルではない。**
	// 「直」は h==0 のときのラベルだが、**同じ筋から来た他の候補も h==0**。
	// ラベルで比べると、真下から来た駒（"直"）と真上から来た駒（""）が
	// 別扱いになり、**区別が付いていないのに「直」と書いてしまう**（実際に踏んだ）。
	sameH, sameV := false, false
	for _, o := range others {
		oh, ov := relation(o.x, o.y, tx, ty, p.black)
		if oh == h {
			sameH = true
		}
		if ov == v {
			sameV = true
		}
	}
	// hLabel が空（同じ筋から下がってきた）ときは修飾にならないので使えない。
	if hLabel != "" && !sameH {
		return hLabel
	}
	if !sameV {
		return vLabel
	}
	return hLabel + vLabel
}

type origin struct{ x, y int }

// rivals は「同じ名前の駒で、そのマスへ行ける他のマス」を集める。
//
// **同じ名前かどうかで括る**（と金と成銀はどちらも金の動きだが名前が違うので、
// 互いに紛れない）。base と promoted が一致するものだけが候補。
func (n *Notation) rivals(p square, fx, fy, tx, ty int) []origin {
	var out []origin
	for y := 1; y <= 9; y++ {
		for x := 1; x <= 9; x++ {
			if x == fx && y == fy {
				continue
			}
			c := n.b.at(x, y)
			if !c.occupied || c.black != p.black || c.base != p.base || c.promoted != p.promoted {
				continue
			}
			if n.b.canReach(x, y, tx, ty) {
				out = append(out, origin{x, y})
			}
		}
	}
	return out
}

// relation は移動元が移動先から見てどちら側かを、**指した側の視点**で返す。
//
//	h: +1=右 / -1=左 / 0=同じ筋
//	v: +1=上（前へ進んだ） / -1=引（下がった） / 0=寄（真横）
//
// 先手から見て 1 筋が右。内部座標 x は筋と向きが逆（x = 10-筋）なので、
// 「筋の数字が小さい＝右」は「x が大きい＝右」になる。
//
// ⚠️ **段（y）は 1 が上（後手陣）で 9 が下（先手陣）。** 先手が前へ進むと y は
// **小さくなる**ので、`fy - ty` が正なら前進（上）。逆に書くと上下が丸ごと入れ替わる。
func relation(fx, fy, tx, ty int, black bool) (h, v int) {
	h = sign(fx - tx)
	v = sign(fy - ty)
	if !black {
		h, v = -h, -v
	}
	return h, v
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}

// horizontalLabel は左右の修飾。同じ筋から前へ進んだ手だけ「直」になる。
func horizontalLabel(h, v int) string {
	switch {
	case h > 0:
		return "右"
	case h < 0:
		return "左"
	case v > 0:
		return "直"
	}
	return ""
}

func verticalLabel(v int) string {
	switch {
	case v > 0:
		return "上"
	case v < 0:
		return "引"
	}
	return "寄"
}

// FormatMoves は読み筋（USI の手の並び）をまとめて日本語表記にする。
//
// sfenStr はその読み筋の開始局面。**手番も局面から取る**ので、
// 先手番・後手番のどちらから始まる読み筋でも記号（▲△）が正しく付く。
//
// ⚠️ **エラーを返しても、そこまでの結果は返す。** 途中で読めない手があっても
// 残りは USI のまま並ぶので、呼び出し側はエラーを無視してそのまま表示できる
// （設計原則: 段階的に劣化する）。
func FormatMoves(sfenStr string, moves []string) ([]MoveText, error) {
	n, err := NewNotation(sfenStr)
	if err != nil {
		out := make([]MoveText, 0, len(moves))
		for _, m := range moves {
			out = append(out, MoveText{USI: m, Text: m})
		}
		return out, err
	}
	out := make([]MoveText, 0, len(moves))
	var first error
	for _, m := range moves {
		t, err := n.Next(m)
		if err != nil && first == nil {
			first = err
		}
		out = append(out, t)
	}
	return out, first
}
