package kifu

import (
	"strings"
	"testing"
	"time"
)

// TestDocumentStringComments はコメントが KIF の `*` 行として出ることを見る。
//
// KIF ではコメントは**注釈する手の直後**に置く。初期局面のコメントだけが
// 列ヘッダと初手の間に来る。
func TestDocumentStringComments(t *testing.T) {
	d := Document{
		Comment: "振り駒の結果、先手は藤井竜王。",
		Moves: []Move{
			{Num: 1, Name: "２六歩", FromX: 2, FromY: 7, Comment: "飛車先を突いた。"},
			{Num: 2, Name: "８四歩", FromX: 8, FromY: 3},
			{Num: 3, Name: "２五歩", FromX: 2, FromY: 6, Comment: "1行目\n2行目"},
		},
	}
	want := strings.Join([]string{
		"手合割：平手",
		MoveColumnHeader,
		"*振り駒の結果、先手は藤井竜王。",
		"1 ２六歩(27)",
		"*飛車先を突いた。",
		"2 ８四歩(83)",
		"3 ２五歩(26)",
		"*1行目",
		"*2行目",
		"",
	}, "\n")
	if got := d.String(); got != want {
		t.Errorf("String() =\n%q\nwant\n%q", got, want)
	}
}

// TestParseComments は `*` 行を直前の手に付けて読むことを見る。
//
// `#` 行(`# --- Kifu for Windows ...`)はコメントではないので拾わない。
func TestParseComments(t *testing.T) {
	src := strings.Join([]string{
		"# --- Kifu for Windows ---",
		"手合割：平手",
		MoveColumnHeader,
		"*開始前のコメント",
		"   1 ２六歩(27)   ( 0:16/00:00:16)",
		"*1手目のコメント",
		"*その続き",
		"   2 ８四歩(83)   ( 0:20/00:00:20)",
		"   3 投了",
	}, "\n")

	d, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if d.Comment != "開始前のコメント" {
		t.Errorf("Document.Comment = %q", d.Comment)
	}
	if len(d.Moves) != 3 {
		t.Fatalf("moves = %d, want 3", len(d.Moves))
	}
	if want := "1手目のコメント\nその続き"; d.Moves[0].Comment != want {
		t.Errorf("Moves[0].Comment = %q, want %q", d.Moves[0].Comment, want)
	}
	if d.Moves[1].Comment != "" {
		t.Errorf("Moves[1].Comment = %q, want empty", d.Moves[1].Comment)
	}
}

// TestParseFormatCommentRoundTrip は Parse → String でコメントが保たれることを見る。
func TestParseFormatCommentRoundTrip(t *testing.T) {
	d := Document{
		ShowTime: true,
		Comment:  "開始前",
		Moves: []Move{
			{Num: 1, Name: "２六歩", FromX: 2, FromY: 7, Spend: 16 * time.Second, Comment: "コメント\n2行目"},
			{Num: 2, Name: "８四歩", FromX: 8, FromY: 3, Spend: 20 * time.Second},
		},
	}
	got, err := Parse(d.String())
	if err != nil {
		t.Fatal(err)
	}
	if got.Comment != d.Comment {
		t.Errorf("Comment = %q, want %q", got.Comment, d.Comment)
	}
	for i, m := range d.Moves {
		if got.Moves[i].Comment != m.Comment {
			t.Errorf("Moves[%d].Comment = %q, want %q", i, got.Moves[i].Comment, m.Comment)
		}
	}
}
