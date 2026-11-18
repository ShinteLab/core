package kifu

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func sampleKI2Doc() KI2Document {
	return KI2Document{
		Document: Document{
			Handicap:  HirateHandicap,
			Event:     "王将戦 挑決L",
			StartedAt: time.Date(2026, 11, 6, 10, 0, 0, 0, jst),
			Black:     "永瀬拓矢",
			White:     "伊藤匠",
			TimeLimit: 4 * time.Hour,
			Comment:   "ふわりと将棋から",
		},
		Moves: []KI2Move{
			{Text: "▲２六歩"},
			{Text: "△８四歩", Comment: "居飛車\n2 行目"},
			{Text: "▲２五歩"},
			{Text: "△８五歩"},
			{Text: "▲２四歩"},
			{Text: "△同　歩"},
			{Text: "▲同　飛"},
			{Text: "△２三歩"},
		},
		End: "投了",
	}
}

func TestKI2DocumentString(t *testing.T) {
	got := sampleKI2Doc().String()
	want := `開始日時：2026/11/06 10:00:00
棋戦：王将戦 挑決L
手合割：平手
先手：永瀬拓矢
後手：伊藤匠
持ち時間：各4時間
*ふわりと将棋から
▲２六歩
△８四歩
*居飛車
*2 行目
▲２五歩
△８五歩
▲２四歩
△同　歩
▲同　飛
△２三歩
まで8手で後手の勝ち
`
	if got != want {
		t.Fatalf("String():\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, MoveColumnHeader) {
		t.Error("KI2 に指し手の列ヘッダを書いている")
	}
}

func TestKI2DocumentRoundTrip(t *testing.T) {
	for _, d := range []KI2Document{
		sampleKI2Doc(),
		// 対局中（終局なし）・手の欠け（置き場）・千日手など。
		{Document: Document{Handicap: HirateHandicap}, Moves: []KI2Move{{Text: "▲２六歩"}, {Text: "△？", Comment: "欠け"}}},
		{Document: Document{Handicap: HirateHandicap}, Moves: []KI2Move{{Text: "▲２六歩"}}, End: "千日手"},
		{Document: Document{Handicap: HirateHandicap}},
	} {
		got, err := ParseKI2(d.String())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, d) {
			t.Errorf("round trip:\n got %+v\nwant %+v", got, d)
		}
	}
}

// 標準の KI2（1 行に複数手、"同　" の全角空白は手の一部）を読む。
func TestParseKI2Standard(t *testing.T) {
	text := "\ufeff手合割：平手\r\n先手：A\r\n後手：B\r\n" +
		"▲７六歩    △３四歩    ▲２二角成  △同　銀\r\n" +
		"*角換わり\r\n" +
		"▲４五角\r\n" +
		"まで5手で先手の勝ち\r\n"
	d, err := ParseKI2(text)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, m := range d.Moves {
		texts = append(texts, m.Text)
	}
	if want := []string{"▲７六歩", "△３四歩", "▲２二角成", "△同　銀", "▲４五角"}; !reflect.DeepEqual(texts, want) {
		t.Fatalf("moves = %q, want %q", texts, want)
	}
	if d.Moves[3].Comment != "角換わり" {
		t.Errorf("comment = %q", d.Moves[3].Comment)
	}
	if d.End != "投了" || d.Black != "A" || d.White != "B" {
		t.Errorf("End = %q, Black = %q, White = %q", d.End, d.Black, d.White)
	}
	if d.EndMark() != "投了" || d.Empty() {
		t.Errorf("EndMark = %q, Empty = %v", d.EndMark(), d.Empty())
	}
}

func TestParseKI2End(t *testing.T) {
	for _, tt := range []struct{ text, end string }{
		{"▲７六歩\nまで1手で先手の勝ち\n", "投了"},
		{"▲７六歩\nまで1手で上手の勝ち\n", "投了"},
		{"▲７六歩\nまで1手で千日手\n", "千日手"},
		{"▲７六歩\n△投了\n", "投了"},
		{"▲７六歩\nまで1手で何かの理由\n", ""},
	} {
		d, err := ParseKI2(tt.text)
		if err != nil {
			t.Fatal(err)
		}
		if d.End != tt.end || len(d.Moves) != 1 {
			t.Errorf("%q: End = %q, 手数 = %d", tt.text, d.End, len(d.Moves))
		}
	}
}

// 投了の勝者は「n 手のあとの手番の相手」。駒落ちは上手から指し、呼び名は 下手/上手。
func TestKI2ResultLine(t *testing.T) {
	moves := func(n int) []KI2Move { return make([]KI2Move, n) }
	for _, tt := range []struct {
		d    KI2Document
		want string
	}{
		{KI2Document{Document: Document{Handicap: HirateHandicap}, Moves: moves(97), End: "投了"}, "まで97手で先手の勝ち"},
		{KI2Document{Document: Document{Handicap: HirateHandicap}, Moves: moves(96), End: "投了"}, "まで96手で後手の勝ち"},
		{KI2Document{Document: Document{Handicap: "角落ち"}, Moves: moves(51), End: "投了"}, "まで51手で上手の勝ち"},
		{KI2Document{Document: Document{Handicap: "角落ち"}, Moves: moves(50), End: "投了"}, "まで50手で下手の勝ち"},
		{KI2Document{Document: Document{Handicap: HirateHandicap}, Moves: moves(120), End: "千日手"}, "まで120手で千日手"},
	} {
		if got := tt.d.resultLine(); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}

func TestKI2DocumentKIF(t *testing.T) {
	doc, errMove, err := sampleKI2Doc().KIF()
	if err != nil || errMove != 0 {
		t.Fatalf("errMove = %d, err = %v", errMove, err)
	}
	if len(doc.Moves) != 9 || doc.EndMark() != "投了" {
		t.Fatalf("手数 = %d, EndMark = %q", len(doc.Moves), doc.EndMark())
	}
	kif := doc.String()
	for _, line := range []string{"棋戦：王将戦 挑決L", "*ふわりと将棋から", "2 ８四歩(83)", "*居飛車", "6 同　歩(23)", "8 ２三歩打(00)", "9 投了"} {
		if !strings.Contains(kif, line) {
			t.Errorf("KIF に %q が無い:\n%s", line, kif)
		}
	}
}

// 盤に当てられない手（中継の誤記）で止まり、そこまでを返す。終局は付けない。
func TestKI2DocumentKIFStopsAtTypo(t *testing.T) {
	d := sampleKI2Doc()
	d.Moves[4].Text = "▲８九飛" // 8九には桂がいて、飛はそこへ行けない
	doc, errMove, err := d.KIF()
	if !errors.Is(err, ErrKI2Move) {
		t.Fatalf("err = %v", err)
	}
	if errMove != 5 || len(doc.Moves) != 4 || doc.EndMark() != "" {
		t.Fatalf("errMove = %d, 手数 = %d, EndMark = %q", errMove, len(doc.Moves), doc.EndMark())
	}
}

// 欠けた手の置き場（"▲？"）でも止まる。
func TestKI2DocumentKIFStopsAtPlaceholder(t *testing.T) {
	d := sampleKI2Doc()
	d.Moves[2].Text = "▲？"
	parsed, err := ParseKI2(d.String())
	if err != nil {
		t.Fatal(err)
	}
	_, errMove, err := parsed.KIF()
	if !errors.Is(err, ErrKI2Move) || errMove != 3 {
		t.Fatalf("errMove = %d, err = %v", errMove, err)
	}
}

func TestKI2DocumentKIFUnknownHandicap(t *testing.T) {
	d := sampleKI2Doc()
	d.Handicap = "謎落ち"
	if _, errMove, err := d.KIF(); err == nil || errMove != 1 {
		t.Fatalf("errMove = %d, err = %v", errMove, err)
	}
}

func TestNormalizeKI2Move(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"▲55金", "▲５五金"},
		{"▲５５金", "▲５五金"},
		{"▲53銀引成", "▲５三銀引成"},
		{"△同玉", "△同　玉"},
		{"△同 玉", "△同　玉"},
		{"△同　玉", "△同　玉"},
		{"▲７六歩", "▲７六歩"},
		{"▲89竜", "▲８九竜"}, // 駒名は触らない
		{"26歩", "２六歩"},
		{"▲？", "▲？"},
		{"▲同", "▲同"},
		{"投了", "投了"},
	} {
		if got := NormalizeKI2Move(tt.in); got != tt.want {
			t.Errorf("NormalizeKI2Move(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsKI2(t *testing.T) {
	for _, tt := range []struct {
		name string
		text string
		want bool
	}{
		{"KI2", sampleKI2Doc().String(), true},
		{"1 行に複数手", "▲７六歩    △３四歩\n", true},
		{"同から", "△同　歩\n", true},
		{"KIF", sampleKIF, false},
		{"KIF（Document.String）", sampleKI2Doc().Document.String() + "1 ７六歩(77)\n", false},
		{"CSA", sampleCSA, false},
		{"ただの文章", "こんにちは\n▲です\n", false},
		{"空", "", false},
	} {
		if got := IsKI2(tt.text); got != tt.want {
			t.Errorf("%s: IsKI2 = %v, want %v", tt.name, got, tt.want)
		}
	}
}
