package kifu

import (
	"fmt"
	"strings"

	"github.com/ShinteLab/core/sfen"
	"github.com/ShinteLab/core/usi"
)

// board は指し手を日本語表記にするために要る**最小の盤面**。
//
// **持ち駒を持たない。** 表記に要るのは「移動元に何の駒が居たか」と
// 「同じ駒が他にどこから来られたか」だけで、駒台の枚数は要らない。
// 打ちの駒種は USI の手文字列そのものに書いてある。
//
// **合法性も判定しない。** 進めるのは既に決まった手（エンジンの読み筋・棋譜）で、
// 手が正しいかどうかは呼び出し側の問題。ここが調べるのは
// **「そのマスへ行ける同じ駒が他にあるか」**（＝表記の修飾が要るか）だけ。
//
// 座標は core/usi の内部座標 (x,y in 1..9) に揃えてある。
// ⚠️ **sfen の (rank,file) とは別物**（あちらは記述順の 0..8）。変換は x=file+1 / y=rank+1。
type board struct {
	// cells は (y-1)*9 + (x-1) 番目のマス。
	cells [81]square
	// black は手番。
	black bool
}

// square は 1 マス。**ゼロ値が空マス**（sfen.Pawn == 0 なので occupied が要る）。
type square struct {
	base     int // sfen.Pawn..King（成駒でもベース駒）
	promoted bool
	black    bool
	occupied bool
}

func index(x, y int) int { return (y-1)*9 + (x - 1) }

func inBoard(x, y int) bool { return x >= 1 && x <= 9 && y >= 1 && y <= 9 }

func (b *board) at(x, y int) square {
	if !inBoard(x, y) {
		return square{}
	}
	return b.cells[index(x, y)]
}

func (b *board) set(x, y int, s square) {
	if inBoard(x, y) {
		b.cells[index(x, y)] = s
	}
}

// parseBoard は SFEN から盤面と手番を読む。
//
// 盤面部分だけ（"lnsgk..."）でも、手番・持ち駒・手数まで付いた完全形でも受ける。
// **手番の欄が無ければ先手番として扱う**（SFEN の慣例。core/web の setTurn と同じ）。
// 持ち駒と手数は読み飛ばす（表記に要らない）。
func parseBoard(s string) (*board, error) {
	fields := strings.Fields(strings.TrimSpace(s))
	if len(fields) == 0 {
		return nil, fmt.Errorf("kifu: SFEN が空です")
	}
	b := &board{black: true}
	if len(fields) >= 2 && fields[1] == "w" {
		b.black = false
	}
	err := sfen.ParseBoard(fields[0], func(rank, file, base int, black, promoted bool) {
		b.set(file+1, rank+1, square{base: base, promoted: promoted, black: black, occupied: true})
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}

// apply は 1 手を盤に進める（手番も入れ替える）。
//
// **取った駒は捨てる**（持ち駒を持たないため）。表記を作るのに駒台は要らない。
// 移動元が空マスなら、そこから先の局面が信用できなくなるのでエラーにする
// （黙って進めると、以後の表記が全部でたらめになる）。
func (b *board) apply(m usi.Move) error {
	if m.Resign {
		return nil
	}
	if m.Drop != 0 {
		base, _ := sfen.ParsePieceLetter(m.Drop)
		if base == sfen.NotFound {
			return fmt.Errorf("kifu: 打つ駒が読めません: %q", string(m.Drop))
		}
		b.set(m.ToX, m.ToY, square{base: base, black: b.black, occupied: true})
		b.black = !b.black
		return nil
	}
	if !inBoard(m.FromX, m.FromY) || !inBoard(m.ToX, m.ToY) {
		return fmt.Errorf("kifu: マスが盤の外です: %s", m.String())
	}
	from := b.at(m.FromX, m.FromY)
	if !from.occupied {
		return fmt.Errorf("kifu: 移動元に駒がありません: %s", m.String())
	}
	if m.Promote {
		from.promoted = true
	}
	b.set(m.ToX, m.ToY, from)
	b.set(m.FromX, m.FromY, square{})
	b.black = !b.black
	return nil
}

// ---- 「そのマスへ行けるか」の判定 -------------------------------------------
//
// **合法手生成ではない。** ピンも王手も打ち歩詰めも見ない。表記の修飾（右/左/上…）を
// 決めるために「同じ駒が他にもそこへ行けたか」を知りたいだけなので、
// **駒の動き方と、飛角香の利きを遮る駒だけ**を見る。
//
// ⚠️ そのため、ピンされていて実際には動けない駒も候補に数える。結果として
// **本来は要らない修飾が付くことがある**（表記が冗長になるだけで、指し手は変わらない）。

// step は 1 マスの移動量。
type step struct{ dx, dy int }

// canReach は (fx,fy) の駒が (tx,ty) へ動けるかを返す。
func (b *board) canReach(fx, fy, tx, ty int) bool {
	if !inBoard(fx, fy) || !inBoard(tx, ty) {
		return false
	}
	if fx == tx && fy == ty {
		return false
	}
	p := b.at(fx, fy)
	if !p.occupied {
		return false
	}
	// 自分の駒の上には行けない。
	if dst := b.at(tx, ty); dst.occupied && dst.black == p.black {
		return false
	}
	// 先手は上（y が小さくなる方向）へ進む。
	dir := -1
	if !p.black {
		dir = 1
	}
	for _, s := range stepsOf(p, dir) {
		if fx+s.dx == tx && fy+s.dy == ty {
			return true
		}
	}
	for _, s := range slidesOf(p, dir) {
		if b.slides(fx, fy, tx, ty, s) {
			return true
		}
	}
	return false
}

// slides は s の向きへ滑って (tx,ty) に届くかを返す（途中に駒があれば届かない）。
func (b *board) slides(fx, fy, tx, ty int, s step) bool {
	x, y := fx+s.dx, fy+s.dy
	for inBoard(x, y) {
		if x == tx && y == ty {
			return true
		}
		if b.at(x, y).occupied {
			return false
		}
		x += s.dx
		y += s.dy
	}
	return false
}

// goldSteps は金の動き（成駒の歩・香・桂・銀も同じ）。dir は前方向。
func goldSteps(dir int) []step {
	return []step{{0, dir}, {-1, dir}, {1, dir}, {-1, 0}, {1, 0}, {0, -dir}}
}

var (
	diagonals   = []step{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}}
	orthogonals = []step{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
)

// stepsOf は 1 マスだけ動ける方向を返す。
func stepsOf(p square, dir int) []step {
	if p.promoted {
		switch p.base {
		case sfen.Pawn, sfen.Lance, sfen.Knight, sfen.Silver:
			return goldSteps(dir)
		case sfen.Bishop: // 馬 … 角の利き + 縦横 1 マス
			return orthogonals
		case sfen.Rook: // 龍 … 飛の利き + 斜め 1 マス
			return diagonals
		}
	}
	switch p.base {
	case sfen.Pawn:
		return []step{{0, dir}}
	case sfen.Knight:
		return []step{{-1, 2 * dir}, {1, 2 * dir}}
	case sfen.Silver:
		return []step{{0, dir}, {-1, dir}, {1, dir}, {-1, -dir}, {1, -dir}}
	case sfen.Gold:
		return goldSteps(dir)
	case sfen.King:
		return append(append([]step{}, orthogonals...), diagonals...)
	}
	return nil
}

// slidesOf は滑って動ける方向を返す。
func slidesOf(p square, dir int) []step {
	switch p.base {
	case sfen.Lance:
		if p.promoted {
			return nil // 成香は金の動き
		}
		return []step{{0, dir}}
	case sfen.Bishop:
		return diagonals
	case sfen.Rook:
		return orthogonals
	}
	return nil
}
