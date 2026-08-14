// Package usi は USI (Universal Shogi Interface) のマス座標と手表記を
// 各プロジェクト共通で扱うための仕様パッケージ。
//
// 内部座標 (x,y) は 1..9。USI のマス表記は筋(file)が 10-x の数字、
// 段(rank)が 'a'+y-1 のアルファベット。例: (x=3,y=7) <-> "7g"。
//
// engine のホットパス(指し手生成・探索の置換表)からも呼ばれるため、
// 実装は switch/バイト演算ベースで確保(alloc)を最小に保つこと。
package usi

// Resign は投了を表す手文字列。
const Resign = "resign"

// FormatSquare は内部座標 (x,y in 1..9) を USI マス文字列に変換する。
// 例: (3,7) -> "7g"。
func FormatSquare(x, y int) string {
	fx := byte(10-x) + '0'
	ry := byte(y-1) + 'a'
	return string([]byte{fx, ry})
}

// ParseSquare は USI マス文字列を内部座標 (x,y in 1..9) に変換する。
// 長さが 2 でない、または筋 '1'..'9' / 段 'a'..'i' の範囲外なら ok=false。
func ParseSquare(s string) (x, y int, ok bool) {
	if len(s) != 2 {
		return 0, 0, false
	}
	f := s[0]
	r := s[1]
	if f < '1' || f > '9' || r < 'a' || r > 'i' {
		return 0, 0, false
	}
	return 10 - int(f-'0'), int(r-'a') + 1, true
}

// Move は USI の一手を表す中立表現。
//   - Resign が true なら投了。
//   - Drop != 0 なら打ち(その大文字の駒文字を打つ)、ToX/ToY が打ち先。
//   - それ以外は FromX/FromY -> ToX/ToY の移動で、Promote が成り。
type Move struct {
	Resign  bool
	Drop    byte // 0 または打つ駒の大文字 ('P','L',...)
	FromX   int
	FromY   int
	ToX     int
	ToY     int
	Promote bool
}

// ParseMove は USI 手文字列を Move に変換する。
// "7g7f"(移動) / "7g7f+"(成り) / "P*5e"(打ち) / "resign"(投了) に対応する。
// 長さ 2 の文字列は移動元のみ(engine の従来挙動)として From に格納する。
// パースできない場合は ok=false。
func ParseMove(s string) (Move, bool) {
	var m Move
	if s == Resign {
		m.Resign = true
		return m, true
	}
	n := len(s)
	if n == 2 {
		x, y, ok := ParseSquare(s)
		if !ok {
			return Move{}, false
		}
		m.FromX, m.FromY = x, y
		return m, true
	}
	if n >= 4 {
		if s[1] == '*' {
			x, y, ok := ParseSquare(s[2:4])
			if !ok {
				return Move{}, false
			}
			m.Drop = s[0]
			m.ToX, m.ToY = x, y
			return m, true
		}
		fx, fy, ok := ParseSquare(s[0:2])
		if !ok {
			return Move{}, false
		}
		tx, ty, ok2 := ParseSquare(s[2:4])
		if !ok2 {
			return Move{}, false
		}
		m.FromX, m.FromY = fx, fy
		m.ToX, m.ToY = tx, ty
		if n == 5 {
			m.Promote = true
		}
		return m, true
	}
	return Move{}, false
}

// String は Move を USI 手文字列に変換する。
func (m Move) String() string {
	if m.Resign {
		return Resign
	}
	if m.Drop != 0 {
		return string(m.Drop) + "*" + FormatSquare(m.ToX, m.ToY)
	}
	g := ""
	if m.Promote {
		g = "+"
	}
	return FormatSquare(m.FromX, m.FromY) + FormatSquare(m.ToX, m.ToY) + g
}
