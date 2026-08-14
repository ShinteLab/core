package kifu

import "testing"

// 平手の初期局面。
const startpos = "lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL b - 1"

// TestNotationSequence は「開始局面から手を順に流す」という本来の使い方を通す。
//
// **駒種は手文字列に書いていない**ので、8h が角であることは盤から読むしかない。
// ここが崩れると全部の表記が壊れるので、角換わりの出だしで固定しておく。
func TestNotationSequence(t *testing.T) {
	n, err := NewNotation(startpos)
	if err != nil {
		t.Fatalf("NewNotation: %v", err)
	}
	moves := []struct{ usi, want string }{
		{"7g7f", "▲７六歩"},
		{"3c3d", "△３四歩"},
		// 8h の角が 2b へ成る。**利用者から出た例そのもの。**
		{"8h2b+", "▲２二角成"},
		// 直前と同じマスなので「同」。**全角スペースが入る。**
		{"3a2b", "△同　銀"},
	}
	for i, tc := range moves {
		got, err := n.Next(tc.usi)
		if err != nil {
			t.Fatalf("%d手目 Next(%q): %v", i+1, tc.usi, err)
		}
		if got.Text != tc.want {
			t.Errorf("%d手目 Next(%q) = %q, want %q", i+1, tc.usi, got.Text, tc.want)
		}
		if !got.OK {
			t.Errorf("%d手目 Next(%q) の OK が false", i+1, tc.usi)
		}
	}
}

// TestNotationFromKifuMove は MoveText がそのまま kifu.Move に入ることを見る。
// **移動元は KIF の数え方（筋・段）**で、内部座標 x とは向きが逆。
func TestNotationFromKifuMove(t *testing.T) {
	n, _ := NewNotation(startpos)
	got, err := n.Next("7g7f")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got.Name != "７六歩" {
		t.Errorf("Name = %q, want %q", got.Name, "７六歩")
	}
	if got.FromX != 7 || got.FromY != 7 {
		t.Errorf("From = (%d,%d), want (7,7)", got.FromX, got.FromY)
	}
	if line := FormatLine(1, got.Name, got.FromX, got.FromY); line != "1 ７六歩(77)" {
		t.Errorf("FormatLine = %q, want %q", line, "1 ７六歩(77)")
	}
}

// TestNotationDropUsesUchi は打ちに必ず「打」が付くことを見る。
//
// ⚠️ **落とすと KIF に出したときに盤上の手として読まれる**（FormatLine が
// 「打」の有無で移動元座標を 0 にするかを決めているため）。
func TestNotationDropUsesUchi(t *testing.T) {
	n, err := NewNotation("9/9/9/9/4k4/9/9/9/4K4 b P 1")
	if err != nil {
		t.Fatalf("NewNotation: %v", err)
	}
	got, err := n.Next("P*5d")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got.Name != "５四歩打" {
		t.Errorf("Name = %q, want %q", got.Name, "５四歩打")
	}
	if got.FromX != 0 || got.FromY != 0 {
		t.Errorf("打ちなのに移動元が入っている: (%d,%d)", got.FromX, got.FromY)
	}
	if line := FormatLine(1, got.Name, got.FromX, got.FromY); line != "1 ５四歩打(00)" {
		t.Errorf("FormatLine = %q, want %q", line, "1 ５四歩打(00)")
	}
}

// TestNotationTurnMark は後手番から始まる読み筋でも記号が正しく付くことを見る。
//
// **エンジンの読み筋は解析した局面の手番から始まる**ので、先手番だけを前提にすると
// 半分の局面で先後が入れ替わる。
func TestNotationTurnMark(t *testing.T) {
	n, err := NewNotation("lnsgkgsnl/1r5b1/ppppppppp/9/9/2P6/PP1PPPPPP/1B5R1/LNSGKGSNL w - 2")
	if err != nil {
		t.Fatalf("NewNotation: %v", err)
	}
	got, _ := n.Next("3c3d")
	if got.Text != "△３四歩" {
		t.Errorf("Text = %q, want %q", got.Text, "△３四歩")
	}
	if got.Black {
		t.Error("後手番なのに Black が true")
	}
}

// TestNotationPromotion は成・不成の書き分けを見る。
//
// **不成は「成れたのに成らなかった」ときだけ。** 敵陣に関係ない手に付くと嘘になる。
func TestNotationPromotion(t *testing.T) {
	tests := []struct {
		name  string
		board string
		move  string
		want  string
	}{
		// 2d の飛が 2b へ。敵陣なので成れる。
		{"成る", "4k4/9/9/1R7/9/9/9/9/4K4 b - 1", "8d8b+", "８二飛成"},
		{"不成", "4k4/9/9/1R7/9/9/9/9/4K4 b - 1", "8d8b", "８二飛不成"},
		// 敵陣に入らない手には何も付かない。
		{"関係なし", "4k4/9/9/9/1R7/9/9/9/4K4 b - 1", "8e8f", "８六飛"},
		// 金は成れないので不成も付かない。
		{"金は成れない", "4k4/9/9/1G7/9/9/9/9/4K4 b - 1", "8d8c", "８三金"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, err := NewNotation(tc.board)
			if err != nil {
				t.Fatalf("NewNotation: %v", err)
			}
			got, err := n.Next(tc.move)
			if err != nil {
				t.Fatalf("Next: %v", err)
			}
			if got.Name != tc.want {
				t.Errorf("Name = %q, want %q", got.Name, tc.want)
			}
		})
	}
}

// TestNotationPromotedPieceNames は成駒の名前を固定する。
func TestNotationPromotedPieceNames(t *testing.T) {
	tests := []struct {
		board string
		move  string
		want  string
	}{
		{"4k4/9/9/9/1+P7/9/9/9/4K4 b - 1", "8e8f", "８六と"},
		{"4k4/9/9/9/1+L7/9/9/9/4K4 b - 1", "8e8f", "８六成香"},
		{"4k4/9/9/9/1+N7/9/9/9/4K4 b - 1", "8e8f", "８六成桂"},
		{"4k4/9/9/9/1+S7/9/9/9/4K4 b - 1", "8e8f", "８六成銀"},
		{"4k4/9/9/9/1+R7/9/9/9/4K4 b - 1", "8e8f", "８六龍"},
		{"4k4/9/9/9/1+B7/9/9/9/4K4 b - 1", "8e8f", "８六馬"},
	}
	for _, tc := range tests {
		n, err := NewNotation(tc.board)
		if err != nil {
			t.Fatalf("NewNotation(%q): %v", tc.board, err)
		}
		got, err := n.Next(tc.move)
		if err != nil {
			t.Fatalf("Next(%q): %v", tc.move, err)
		}
		if got.Name != tc.want {
			t.Errorf("%q: Name = %q, want %q", tc.board, got.Name, tc.want)
		}
	}
}

// TestNotationQualifier は「同じ駒が複数行ける」ときの修飾を見る。
//
// **ここが表記の一番むずかしいところ。** 駒種と移動先だけでは手が決まらないので、
// 盤全体を見て他の候補を数える必要がある。
func TestNotationQualifier(t *testing.T) {
	tests := []struct {
		name  string
		board string
		move  string
		want  string
	}{
		// 6九と4九の金がどちらも5八へ行ける → 左右で区別する。
		// 先手から見て 1 筋が右なので、4九（筋が小さい）から来たほうが「右」。
		{"金が左右", "4k4/9/9/9/9/9/9/9/3G1G2K b - 1", "6i5h", "５八金左"},
		{"金が左右(右)", "4k4/9/9/9/9/9/9/9/3G1G2K b - 1", "4i5h", "５八金右"},
		// 同じ筋に飛が2枚 → 上下で区別する。
		// ⚠️ **段は 1 が上（後手陣）。** 先手が前へ進むと段の数字は小さくなるので、
		// 8 段目から 6 段目へ動くのが「上」。逆に書くと上下が丸ごと入れ替わる。
		{"飛が上下(上)", "4k4/9/9/7R1/9/9/9/7R1/4K4 b - 1", "2h2f", "２六飛上"},
		{"飛が上下(引)", "4k4/9/9/7R1/9/9/9/7R1/4K4 b - 1", "2d2f", "２六飛引"},
		// 真下から来た銀は「直」、斜めから来た銀は左右。
		{"銀が直", "4k4/9/9/9/9/9/9/5SS2/4K4 b - 1", "3h3g", "３七銀直"},
		{"銀が左", "4k4/9/9/9/9/9/9/5SS2/4K4 b - 1", "4h3g", "３七銀左"},
		// 候補が 1 枚なら修飾は付かない。
		{"1枚なら無し", "4k4/9/9/9/9/9/6S2/9/4K4 b - 1", "3g3f", "３六銀"},
		// と金と成銀は名前が違うので互いに紛れない（どちらも金の動きだが修飾は不要）。
		{"名前が違えば紛れない", "4k4/9/9/9/9/3+P1+S3/9/9/4K4 b - 1", "6f5f", "５六と"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, err := NewNotation(tc.board)
			if err != nil {
				t.Fatalf("NewNotation: %v", err)
			}
			got, err := n.Next(tc.move)
			if err != nil {
				t.Fatalf("Next: %v", err)
			}
			if got.Name != tc.want {
				t.Errorf("Name = %q, want %q", got.Name, tc.want)
			}
		})
	}
}

// TestNotationQualifierWhite は後手の左右が反転することを見る。
//
// **後手は盤を逆から見ている**ので、同じ位置関係でも右左が入れ替わる。
// ⚠️ ここを間違えると、後手の手だけ静かに左右が逆になる（読めてしまうので気づきにくい）。
func TestNotationQualifierWhite(t *testing.T) {
	board := "1K2k1g1g/9/9/9/9/9/9/9/4K4 w - 1"
	n, err := NewNotation(board)
	if err != nil {
		t.Fatalf("NewNotation: %v", err)
	}
	// 3a と 1a の金がどちらも 2b へ行ける。
	// **後手から見て 9 筋が右**なので、筋の数字が大きい 3a から来たほうが「右」。
	// ⚠️ 先手なら同じ位置関係で「左」になる。ここを取り違えると、後手の手だけ
	// 静かに左右が逆になる。
	got, err := n.Next("3a2b")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got.Name != "２二金右" {
		t.Errorf("Name = %q, want %q", got.Name, "２二金右")
	}

	n2, _ := NewNotation(board)
	got2, err := n2.Next("1a2b")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got2.Name != "２二金左" {
		t.Errorf("Name = %q, want %q", got2.Name, "２二金左")
	}
}

// TestNotationSlidingIsBlocked は飛角香の利きが駒で遮られることを見る。
//
// **遮りを見ないと、届かない駒まで候補に数えて要らない修飾が付く。**
func TestNotationSlidingIsBlocked(t *testing.T) {
	// 2g と 2c に飛。あいだの 2e に歩があるので、2c の飛は 2f へ届かない。
	n, err := NewNotation("4k4/9/7R1/9/7P1/9/7R1/9/4K4 b - 1")
	if err != nil {
		t.Fatalf("NewNotation: %v", err)
	}
	got, err := n.Next("2g2f")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got.Name != "２六飛" {
		t.Errorf("Name = %q, want %q（遮られた飛を候補に数えている）", got.Name, "２六飛")
	}
}

// TestFormatMovesDegrades は読めない手があっても残りを捨てないことを見る。
//
// **設計原則: 段階的に劣化する。** 途中で 1 手読めなくなっても、生の USI を並べれば
// まだ読める。丸ごとエラーにして何も出さないのが一番困る。
func TestFormatMovesDegrades(t *testing.T) {
	// 2 手目の移動元（5e）は空マスなので名付けられない。
	got, err := FormatMoves(startpos, []string{"7g7f", "5e5d", "3c3d"})
	if err == nil {
		t.Error("読めない手があるのに error が nil")
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Text != "▲７六歩" || !got[0].OK {
		t.Errorf("1手目が壊れている: %+v", got[0])
	}
	if got[1].OK || got[1].Text != "5e5d" {
		t.Errorf("2手目は USI のまま返すこと: %+v", got[1])
	}
	// 盤が信用できなくなったので 3 手目以降も素通しになる。
	if got[2].OK || got[2].Text != "3c3d" {
		t.Errorf("3手目は素通しにすること: %+v", got[2])
	}
}

// TestFormatMovesEmpty は手が無いときに何も返らないことを見る。
func TestFormatMovesEmpty(t *testing.T) {
	got, err := FormatMoves(startpos, nil)
	if err != nil {
		t.Fatalf("FormatMoves: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

// TestNotationBoardOnly は盤面部分だけの SFEN でも動くことを見る（手番は先手）。
func TestNotationBoardOnly(t *testing.T) {
	n, err := NewNotation("lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL")
	if err != nil {
		t.Fatalf("NewNotation: %v", err)
	}
	got, err := n.Next("7g7f")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got.Text != "▲７六歩" {
		t.Errorf("Text = %q, want %q", got.Text, "▲７六歩")
	}
}

// TestNotationFirstMoveIsNeverSame は最初の 1 手が「同」にならないことを見る。
//
// **開始局面より前の手は知らない**（履歴を持たない）。エンジンの読み筋では
// これが正しい —— 直前の手を知らないのに「同」と書いたら嘘になる。
func TestNotationFirstMoveIsNeverSame(t *testing.T) {
	n, err := NewNotation("4k4/9/9/9/4p4/9/9/9/4K4 b - 1")
	if err != nil {
		t.Fatalf("NewNotation: %v", err)
	}
	got, _ := n.Next("5i5h")
	if got.Name != "５八玉" {
		t.Errorf("Name = %q, want %q", got.Name, "５八玉")
	}
}

// TestNotationResign は投了を通す。
func TestNotationResign(t *testing.T) {
	n, _ := NewNotation(startpos)
	got, err := n.Next("resign")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got.Name != "投了" {
		t.Errorf("Name = %q, want %q", got.Name, "投了")
	}
	// FormatLine 側の終局判定と噛み合うこと（座標を付けない）。
	if line := FormatLine(1, got.Name, 0, 0); line != "1 投了" {
		t.Errorf("FormatLine = %q, want %q", line, "1 投了")
	}
}
