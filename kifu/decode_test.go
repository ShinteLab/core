package kifu

import (
	"strings"
	"testing"
)

// 指し手名 → USI の基本形。**盤を持たずに変換できることを固定する。**
func TestDecoderNext(t *testing.T) {
	tests := []struct {
		name string
		move Move
		want string
	}{
		{"普通の移動", Move{Num: 1, Name: "７六歩", FromX: 7, FromY: 7}, "7g7f"},
		{"成り", Move{Num: 21, Name: "２二角成", FromX: 8, FromY: 8}, "8h2b+"},
		{"不成", Move{Num: 31, Name: "２二角不成", FromX: 8, FromY: 8}, "8h2b"},
		{"修飾つき", Move{Num: 7, Name: "７八銀右", FromX: 6, FromY: 9}, "6i7h"},
		{"打ち", Move{Num: 41, Name: "５五角打"}, "B*5e"},
		{"成駒を動かす", Move{Num: 51, Name: "３四成銀", FromX: 3, FromY: 5}, "3e3d"},
		{"と金", Move{Num: 53, Name: "３四と", FromX: 3, FromY: 5}, "3e3d"},
		{"龍", Move{Num: 55, Name: "３四龍", FromX: 3, FromY: 5}, "3e3d"},
		{"半角の筋", Move{Num: 1, Name: "7六歩", FromX: 7, FromY: 7}, "7g7f"},
		// ⚠️ 中継の KIF は成香・成桂・成銀を 1 文字の略記で書くことがある
		// （`８二杏(92)`）。受けないとその 1 手で読み取りが止まる。
		{"杏(成香)", Move{Num: 57, Name: "８二杏", FromX: 9, FromY: 2}, "9b8b"},
		{"圭(成桂)", Move{Num: 59, Name: "８二圭", FromX: 9, FromY: 2}, "9b8b"},
		{"全(成銀)", Move{Num: 61, Name: "８二全", FromX: 9, FromY: 2}, "9b8b"},
		{"略記 + 修飾", Move{Num: 63, Name: "８二全右", FromX: 9, FromY: 2}, "9b8b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDecoder()
			got, ok, err := d.Next(tt.move)
			if err != nil || !ok {
				t.Fatalf("Next(%q) = %q, %v, %v", tt.move.Name, got, ok, err)
			}
			if got != tt.want {
				t.Errorf("Next(%q) = %q, want %q", tt.move.Name, got, tt.want)
			}
		})
	}
}

// 「同」は直前の手の移動先から決まる（Decoder が覚えている唯一の状態）。
func TestDecoderSameSquare(t *testing.T) {
	d := NewDecoder()
	if _, _, err := d.Next(Move{Num: 1, Name: "３四歩", FromX: 3, FromY: 5}); err != nil {
		t.Fatalf("1手目: %v", err)
	}
	got, ok, err := d.Next(Move{Num: 2, Name: "同　歩", FromX: 3, FromY: 3})
	if err != nil || !ok {
		t.Fatalf("同: %q, %v, %v", got, ok, err)
	}
	if got != "3c3d" {
		t.Errorf("同 = %q, want %q", got, "3c3d")
	}
}

// ⚠️ 直前の手を知らないまま「同」は読めない（黙って別のマスに解釈しないこと）。
func TestDecoderSameSquareWithoutPrev(t *testing.T) {
	d := NewDecoder()
	if _, _, err := d.Next(Move{Num: 1, Name: "同　歩", FromX: 3, FromY: 3}); err == nil {
		t.Fatal("直前の手が無いのにエラーにならなかった")
	}
}

// 終局は手ではない（エラーでもない）。
func TestDecoderTerminal(t *testing.T) {
	d := NewDecoder()
	move, ok, err := d.Next(Move{Num: 104, Name: "投了"})
	if err != nil {
		t.Fatalf("投了: %v", err)
	}
	if ok || move != "" {
		t.Errorf("投了 = %q, %v; want 手でないこと", move, ok)
	}
}

// "打" が省かれていても、移動元が無ければ打ちとして読む。
func TestDecoderDropWithoutMark(t *testing.T) {
	d := NewDecoder()
	got, ok, err := d.Next(Move{Num: 41, Name: "５五歩"})
	if err != nil || !ok {
		t.Fatalf("打ち: %q, %v, %v", got, ok, err)
	}
	if got != "P*5e" {
		t.Errorf("打ち = %q, want %q", got, "P*5e")
	}
}

// DecodeMoves は投了で打ち切り、そこまでを返す。
func TestDecodeMoves(t *testing.T) {
	got, err := DecodeMoves([]Move{
		{Num: 1, Name: "７六歩", FromX: 7, FromY: 7},
		{Num: 2, Name: "３四歩", FromX: 3, FromY: 3},
		{Num: 3, Name: "２二角成", FromX: 8, FromY: 8},
		{Num: 4, Name: "投了"},
	})
	if err != nil {
		t.Fatalf("DecodeMoves: %v", err)
	}
	want := []string{"7g7f", "3c3d", "8h2b+"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("DecodeMoves = %v, want %v", got, want)
	}
}

// ⚠️ 読めない手が出ても、そこまでの手は返す（設計原則: 段階的に劣化する）。
func TestDecodeMovesKeepsPrefix(t *testing.T) {
	got, err := DecodeMoves([]Move{
		{Num: 1, Name: "７六歩", FromX: 7, FromY: 7},
		{Num: 2, Name: "ほげ", FromX: 3, FromY: 3},
	})
	if err == nil {
		t.Fatal("読めない手でエラーにならなかった")
	}
	if len(got) != 1 || got[0] != "7g7f" {
		t.Errorf("読めたぶんが返らない: %v", got)
	}
	if !strings.Contains(err.Error(), "2手目") {
		t.Errorf("何手目かがエラーに出ない: %v", err)
	}
}

// KIF テキストから通しで USI にできること（Parse との繋ぎ）。
func TestParseThenDecode(t *testing.T) {
	src := "先手：先手太郎\n後手：後手花子\n手数----指手---------消費時間--\n" +
		"   1 ７六歩(77)   ( 0:16/00:00:16)\n" +
		"   2 ３四歩(33)   ( 0:04/00:00:04)\n" +
		"   3 ２二角成(88) ( 0:10/00:00:26)\n" +
		"   4 同　銀(31)   ( 0:02/00:00:06)\n" +
		"   5 投了\n"
	d, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	start, err := d.StartSFEN()
	if err != nil {
		t.Fatalf("StartSFEN: %v", err)
	}
	if !strings.HasSuffix(start, " b - 1") {
		t.Errorf("平手の手番が先手でない: %q", start)
	}
	got, err := DecodeMoves(d.Moves)
	if err != nil {
		t.Fatalf("DecodeMoves: %v", err)
	}
	want := []string{"7g7f", "3c3d", "8h2b+", "3a2b"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("DecodeMoves = %v, want %v", got, want)
	}
	// 変換した手をそのまま日本語表記に戻せる（notation.go と往復すること）。
	texts, err := FormatMoves(start, got)
	if err != nil {
		t.Fatalf("FormatMoves: %v", err)
	}
	if texts[0].Text != "▲７六歩" || texts[3].Text != "△同　銀" {
		t.Errorf("往復しない: %q / %q", texts[0].Text, texts[3].Text)
	}
}

// ⚠️ 駒落ちは上手（後手）が初手。**平手に倒さないこと。**
func TestStartSFENHandicap(t *testing.T) {
	tests := []struct {
		handicap string
		want     string
	}{
		{"", "lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL b - 1"},
		{"平手", "lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL b - 1"},
		{"香落ち", "lnsgkgsn1/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1"},
		{"飛落ち", "lnsgkgsnl/7b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1"},
		{"二枚落ち", "lnsgkgsnl/9/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1"},
		{"六枚落ち", "2sgkgs2/9/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1"},
	}
	for _, tt := range tests {
		got, err := StartSFEN(tt.handicap)
		if err != nil {
			t.Fatalf("StartSFEN(%q): %v", tt.handicap, err)
		}
		if got != tt.want {
			t.Errorf("StartSFEN(%q) = %q, want %q", tt.handicap, got, tt.want)
		}
	}
}

// 知らない手合割は平手に倒さずエラー（倒すとそこから先が全部でたらめになる）。
func TestStartSFENUnknown(t *testing.T) {
	if _, err := StartSFEN("歩三兵"); err == nil {
		t.Fatal("知らない手合割でエラーにならなかった")
	}
}
