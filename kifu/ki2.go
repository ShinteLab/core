package kifu

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ShinteLab/core/sfen"
	"github.com/ShinteLab/core/usi"
)

// KI2 形式の指し手（**移動元を書かない**表記）を USI にする。
//
//	"▲７六歩"  "△同　銀"  "５三銀引成"  "５五角打"  "55金"
//
// KIF と違って `(77)` の移動元が無いので、**盤を進めながら「どの駒が来たのか」を
// 割り出す**。手掛かりは移動先・駒の名前・修飾（右/左/直/上/寄/引）・成/不成/打だけ。
// そのため盤が要る（`Decoder` は盤が要らないので別物）。
//
// 割り出しは:
//
//  1. その駒の名前で、移動先へ行ける指す側の駒を盤から集める（`board.canReach`）
//  2. 修飾で絞る（向きの値で比べる。`relation`）
//  3. まだ複数なら、動かすと自玉が取られる駒（ピン）を外す
//  4. 盤上に 1 枚も無ければ打ち（KI2 は紛れないとき「打」を省く）
//
// ⚠️ **当てはまる駒が 0 枚・2 枚以上ならエラー**（`ErrKI2Move`）。推測で選ぶと
// **別の局面として読めてしまう**（手入力の中継の誤記を、黙って違う手にする）。
//
// 読むときは方言を広く受ける: 移動先の算用数字（"55" / "５５"）、"同" の後の空白の有無、
// `竜`/`王`、成駒の略記（杏/圭/全）、手番記号（▲△☗☖ と無し）。

// ErrKI2Move は KI2 の指し手を盤に当てられなかったこと（当てはまる駒が無い・絞れない・
// 表記が読めない）。どの手で止まったかはエラーの文に入る。
var ErrKI2Move = errors.New("kifu: KI2 の指し手を盤に当てられません")

// KI2Decoder は KI2 の指し手を開始局面から順に USI へ割り出していく変換器。
//
// **状態を持つ**（局面と「同」のための直前の移動先）。棋譜 1 本につき 1 つ作る。
type KI2Decoder struct {
	n *Notation
}

// NewKI2Decoder は開始局面（SFEN）を指定して変換器を作る。
func NewKI2Decoder(sfenStr string) (*KI2Decoder, error) {
	n, err := NewNotation(sfenStr)
	if err != nil {
		return nil, err
	}
	return &KI2Decoder{n: n}, nil
}

// Next は 1 手を割り出して局面を進める。
//
// 返す MoveText の Name / FromX / FromY はそのまま `kifu.Move` に入る
// （**Name は KIF の正式な書き方に直したもの**。"55金" → "５五金打" のように）。
//
// ⚠️ **終局（投了など）は渡さないこと**（手ではない）。`TerminalMarker` で先に見るか、
// まとめて読むなら `DecodeKI2` を使う。
// ⚠️ エラーのときは局面を進めない（そこから先は読めない）。
func (d *KI2Decoder) Next(text string) (MoveText, error) {
	t := MoveText{Text: text, Black: d.n.b.black}
	m, err := d.resolve(text)
	if err != nil {
		return t, fmt.Errorf("%w: %q: %v", ErrKI2Move, text, err)
	}
	return d.n.Next(m.String())
}

// ki2Move は KI2 の 1 手を読んだもの（盤に当てる前）。
type ki2Move struct {
	same       bool // 「同」
	tx, ty     int  // 移動先（内部座標）。same なら 0
	piece      string
	mods       string // 右/左/直/上/寄/引 の並び
	promote    bool
	notPromote bool
	drop       bool
}

// ki2Aliases は表記ゆれを正式な駒名（pieceName が返すもの）に寄せる。
var ki2Aliases = []struct{ from, to string }{
	{"竜", "龍"}, {"王", "玉"}, {"杏", "成香"}, {"圭", "成桂"}, {"全", "成銀"},
}

// ki2Pieces は駒の名前（pieceName が返すもの）。**2 文字の名前を先に**試す。
var ki2Pieces = []string{"成香", "成桂", "成銀", "歩", "香", "桂", "銀", "金", "角", "飛", "玉", "と", "馬", "龍"}

// ki2Ranks は段。KI2 の中継は "55金" のように段も算用数字で書くことがある。
var ki2Ranks = map[rune]int{
	'一': 1, '二': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9,
	'１': 1, '２': 2, '３': 3, '４': 4, '５': 5, '６': 6, '７': 7, '８': 8, '９': 9,
	'1': 1, '2': 2, '3': 3, '4': 4, '5': 5, '6': 6, '7': 7, '8': 8, '9': 9,
}

// parseKI2 は 1 手の文字列を読む（盤は見ない）。black は指す側（手番記号との照合用）。
func parseKI2(text string, black bool) (ki2Move, error) {
	var m ki2Move
	s := strings.TrimSpace(text)
	for _, mark := range []struct {
		s     string
		black bool
	}{{"▲", true}, {"☗", true}, {"△", false}, {"☖", false}, {"▽", false}} {
		if strings.HasPrefix(s, mark.s) {
			if mark.black != black {
				return m, errors.New("手番が合いません")
			}
			s = strings.TrimPrefix(s, mark.s)
			break
		}
	}

	if strings.HasPrefix(s, "同") {
		m.same = true
		s = strings.TrimLeft(strings.TrimPrefix(s, "同"), " 　")
	} else {
		rs := []rune(s)
		if len(rs) < 2 {
			return m, errors.New("移動先が読めません")
		}
		f, okF := fileNumbers[rs[0]]
		r, okR := ki2Ranks[rs[1]]
		if !okF || !okR {
			return m, errors.New("移動先が読めません")
		}
		m.tx, m.ty = 10-f, r
		s = strings.TrimLeft(string(rs[2:]), " 　")
	}

	for _, a := range ki2Aliases {
		s = strings.ReplaceAll(s, a.from, a.to)
	}
	for _, p := range ki2Pieces {
		if strings.HasPrefix(s, p) {
			m.piece = p
			s = strings.TrimPrefix(s, p)
			break
		}
	}
	if m.piece == "" {
		return m, errors.New("駒の名前が読めません")
	}

	// 末尾から 打 → 不成 → 成。⚠️ **"不成" を先に見る**（末尾の "成" は必ず「成った」）。
	if strings.HasSuffix(s, "打") {
		m.drop = true
		s = strings.TrimSuffix(s, "打")
	}
	switch {
	case strings.HasSuffix(s, "不成"):
		m.notPromote = true
		s = strings.TrimSuffix(s, "不成")
	case strings.HasSuffix(s, "成"):
		m.promote = true
		s = strings.TrimSuffix(s, "成")
	}
	for _, r := range s {
		if !strings.ContainsRune("右左直上寄引", r) {
			return m, fmt.Errorf("読めない文字があります: %q", string(r))
		}
	}
	m.mods = s
	if m.drop && (m.promote || m.notPromote || m.mods != "") {
		return m, errors.New("打ちに成・修飾は付きません")
	}
	return m, nil
}

// resolve は 1 手を盤に当てて USI の手にする。**局面は進めない。**
func (d *KI2Decoder) resolve(text string) (usi.Move, error) {
	b := d.n.b
	m, err := parseKI2(text, b.black)
	if err != nil {
		return usi.Move{}, err
	}
	tx, ty := m.tx, m.ty
	if m.same {
		if d.n.prevX == 0 {
			return usi.Move{}, errors.New("直前の手が分からないので「同」を読めません")
		}
		tx, ty = d.n.prevX, d.n.prevY
	}

	drop := func() (usi.Move, error) {
		letter, ok := pieceLetters[m.piece]
		if !ok || m.piece == "玉" {
			return usi.Move{}, fmt.Errorf("%s は打てません", m.piece)
		}
		if b.at(tx, ty).occupied {
			return usi.Move{}, errors.New("打つマスに駒があります")
		}
		return usi.Move{Drop: letter, ToX: tx, ToY: ty}, nil
	}
	if m.drop {
		return drop()
	}

	var cands []origin
	for y := 1; y <= 9; y++ {
		for x := 1; x <= 9; x++ {
			p := b.at(x, y)
			if !p.occupied || p.black != b.black || pieceName(p) != m.piece || !b.canReach(x, y, tx, ty) {
				continue
			}
			if !matchesMods(m.mods, x, y, tx, ty, b.black) {
				continue
			}
			if m.promote && !(canPromote(p) && (inZone(y, p.black) || inZone(ty, p.black))) {
				continue
			}
			cands = append(cands, origin{x, y})
		}
	}
	if len(cands) == 0 {
		// 盤上から行ける駒が無い。KI2 は紛れないとき「打」を省くので打ちとして読む。
		if m.mods == "" && !m.promote && !m.notPromote {
			mv, err := drop()
			if err != nil {
				// 打ちとして読めなかった理由だけを出すと「盤上から来られない」ことが伝わらない。
				return usi.Move{}, fmt.Errorf("そのマスへ行ける駒がなく、打ちとしても読めません（%v）", err)
			}
			return mv, nil
		}
		return usi.Move{}, errors.New("そのマスへ行ける駒がありません")
	}
	if len(cands) > 1 {
		// 修飾が足りないように見えても、ピンで動けない駒は数えない（書き手は数えないため）。
		var free []origin
		for _, c := range cands {
			if !b.exposesKing(c.x, c.y, tx, ty) {
				free = append(free, c)
			}
		}
		cands = free
	}
	if len(cands) != 1 {
		return usi.Move{}, fmt.Errorf("そのマスへ行ける駒を 1 つに絞れません（%d 枚）", len(cands))
	}
	return usi.Move{FromX: cands[0].x, FromY: cands[0].y, ToX: tx, ToY: ty, Promote: m.promote}, nil
}

// matchesMods は修飾（右/左/直/上/寄/引）がその移動元に当てはまるかを返す。
// **向きの値で比べる**（`qualifier` と同じ。ラベルで比べると「直」を取り違える）。
func matchesMods(mods string, fx, fy, tx, ty int, black bool) bool {
	h, v := relation(fx, fy, tx, ty, black)
	for _, r := range mods {
		ok := false
		switch r {
		case '右':
			ok = h > 0
		case '左':
			ok = h < 0
		case '直':
			ok = h == 0 && v > 0
		case '上':
			ok = v > 0
		case '寄':
			ok = v == 0
		case '引':
			ok = v < 0
		}
		if !ok {
			return false
		}
	}
	return true
}

// exposesKing は (fx,fy) の駒を (tx,ty) へ動かすと自玉が相手の駒に取られる形になるかを返す。
//
// KI2 の手を 1 つに絞るときだけに使う（ピンされた駒を候補から外す）。
// **合法手判定ではない**（打ち歩詰め・王手放置の他の形は見ない）。
func (b *board) exposesKing(fx, fy, tx, ty int) bool {
	nb := *b
	p := nb.at(fx, fy)
	nb.set(tx, ty, p)
	nb.set(fx, fy, square{})
	kx, ky := 0, 0
	for y := 1; y <= 9 && kx == 0; y++ {
		for x := 1; x <= 9; x++ {
			if c := nb.at(x, y); c.occupied && c.black == p.black && c.base == sfen.King {
				kx, ky = x, y
				break
			}
		}
	}
	if kx == 0 {
		return false // 玉が無い（詰将棋の攻め方など）
	}
	for y := 1; y <= 9; y++ {
		for x := 1; x <= 9; x++ {
			if c := nb.at(x, y); c.occupied && c.black != p.black && nb.canReach(x, y, kx, ky) {
				return true
			}
		}
	}
	return false
}

// DecodeKI2 は KI2 の指し手の並びをまとめて割り出す。
//
// 終局（投了など。`TerminalMarker`）が来たらそこで止め、その名前を end に返す。
// ⚠️ **エラーを返しても、そこまでに読めた手は返す**（中継の誤記 1 つで全部を捨てない）。
// エラーの文には何手目（names の 1 始まりの位置）かが入る。
func DecodeKI2(sfenStr string, names []string) (out []MoveText, end string, err error) {
	d, err := NewKI2Decoder(sfenStr)
	if err != nil {
		return nil, "", err
	}
	out = make([]MoveText, 0, len(names))
	for i, name := range names {
		bare := strings.TrimLeft(strings.TrimSpace(name), "▲△☗☖▽")
		if marker, ok := TerminalMarker(bare); ok {
			return out, marker, nil
		}
		t, err := d.Next(name)
		if err != nil {
			return out, "", fmt.Errorf("%d手目: %w", i+1, err)
		}
		out = append(out, t)
	}
	return out, "", nil
}
