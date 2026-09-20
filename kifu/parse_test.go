package kifu

import (
	"strings"
	"testing"
	"time"
)

const sampleKIF = `# ---- Kifu for Windows ----
開始日時：2024/10/19 09:00:00
棋戦：第37期竜王戦七番勝負第２局
場所：福井県あわら市
手合割：平手
先手：佐々木勇気八段
後手：藤井聡太竜王
持ち時間：各8時間
秒読み：60秒
手数----指手---------消費時間--
   1 ７六歩(77)   ( 0:00/00:00:00)
   2 ８四歩(83)   ( 1:00/00:01:00)
   3 ６八銀(79)   ( 2:00/00:02:00)
*珍しい進行
  14 同　歩(23)   ( 0:30/00:03:30)
  15 ２三歩打     ( 1:00/00:03:00)
 104 投了         ( 4:00/07:42:00)
まで104手で先手の勝ち
`

func TestParseHeaders(t *testing.T) {
	d, err := Parse(sampleKIF)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if d.Event != "第37期竜王戦七番勝負第２局" {
		t.Errorf("Event = %q", d.Event)
	}
	if d.Place != "福井県あわら市" {
		t.Errorf("Place = %q", d.Place)
	}
	if d.Handicap != "平手" {
		t.Errorf("Handicap = %q", d.Handicap)
	}
	if d.Black != "佐々木勇気八段" || d.White != "藤井聡太竜王" {
		t.Errorf("Black=%q White=%q", d.Black, d.White)
	}
	if d.TimeLimit != 8*time.Hour {
		t.Errorf("TimeLimit = %v", d.TimeLimit)
	}
	if d.Countdown != 60*time.Second {
		t.Errorf("Countdown = %v", d.Countdown)
	}

	want := time.Date(2024, 10, 19, 9, 0, 0, 0, jst)
	if !d.StartedAt.Equal(want) {
		t.Errorf("StartedAt = %v, want %v", d.StartedAt, want)
	}
}

func TestParseMoves(t *testing.T) {
	d, err := Parse(sampleKIF)
	if err != nil {
		t.Fatal(err)
	}

	if len(d.Moves) != 6 {
		t.Fatalf("len(Moves) = %d, want 6 (%+v)", len(d.Moves), d.Moves)
	}
	if !d.ShowTime {
		t.Error("ShowTime should be true when the source carries times")
	}

	cases := []struct {
		i     int
		num   int
		name  string
		x, y  int
		spend time.Duration
	}{
		{0, 1, "７六歩", 7, 7, 0},
		{1, 2, "８四歩", 8, 3, time.Minute},
		{3, 14, "同　歩", 2, 3, 30 * time.Second}, // 全角空白を含む指し手名
		{4, 15, "２三歩打", 0, 0, time.Minute},     // 打ちは移動元なし
		{5, 104, "投了", 0, 0, 4 * time.Minute},
	}
	for _, c := range cases {
		m := d.Moves[c.i]
		if m.Num != c.num || m.Name != c.name || m.FromX != c.x || m.FromY != c.y || m.Spend != c.spend {
			t.Errorf("Moves[%d] = %+v, want num=%d name=%q from=(%d,%d) spend=%v",
				c.i, m, c.num, c.name, c.x, c.y, c.spend)
		}
	}
}

func TestParseEndMark(t *testing.T) {
	d, err := Parse(sampleKIF)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.EndMark(); got != "投了" {
		t.Errorf("EndMark() = %q, want 投了", got)
	}

	// 対局中(終局手が無い)なら空。
	inProgress := "先手：A\n後手：B\n手数----指手---------消費時間--\n1 ７六歩(77)\n"
	d2, err := Parse(inProgress)
	if err != nil {
		t.Fatal(err)
	}
	if got := d2.EndMark(); got != "" {
		t.Errorf("EndMark() = %q, want empty", got)
	}
}

// 書き出したものを読み直すと元に戻ること(往復)。
func TestParseRoundTrip(t *testing.T) {
	orig := Document{
		Event:     "テスト棋戦",
		Place:     "テスト会館",
		StartedAt: time.Date(2024, 10, 19, 9, 0, 0, 0, jst),
		Handicap:  "平手",
		Black:     "先手 太郎",
		White:     "後手 次郎",
		TimeLimit: 8 * time.Hour,
		Countdown: 60 * time.Second,
		ShowTime:  true,
		Moves: []Move{
			{Num: 1, Name: "７六歩", FromX: 7, FromY: 7, Spend: 0},
			{Num: 2, Name: "８四歩", FromX: 8, FromY: 3, Spend: time.Minute},
			{Num: 3, Name: "５五角打", Spend: 2 * time.Minute},
			{Num: 4, Name: "投了", Spend: 4 * time.Minute},
		},
	}

	got, err := Parse(orig.String())
	if err != nil {
		t.Fatalf("Parse(String()): %v", err)
	}

	if got.Event != orig.Event || got.Place != orig.Place ||
		got.Handicap != orig.Handicap || got.Black != orig.Black || got.White != orig.White {
		t.Errorf("headers differ:\n got %+v\nwant %+v", got, orig)
	}
	if !got.StartedAt.Equal(orig.StartedAt) {
		t.Errorf("StartedAt = %v, want %v", got.StartedAt, orig.StartedAt)
	}
	if got.TimeLimit != orig.TimeLimit || got.Countdown != orig.Countdown {
		t.Errorf("TimeLimit=%v Countdown=%v", got.TimeLimit, got.Countdown)
	}
	if len(got.Moves) != len(orig.Moves) {
		t.Fatalf("len(Moves) = %d, want %d", len(got.Moves), len(orig.Moves))
	}
	for i := range orig.Moves {
		w := orig.Moves[i]
		g := got.Moves[i]
		// 終局手は書き出し時に "投了" へ正規化される。
		if g.Num != w.Num || g.Spend != w.Spend {
			t.Errorf("Moves[%d] = %+v, want %+v", i, g, w)
		}
		if _, terminal := TerminalMarker(w.Name); !terminal {
			if g.Name != w.Name || g.FromX != w.FromX || g.FromY != w.FromY {
				t.Errorf("Moves[%d] = %+v, want %+v", i, g, w)
			}
		}
	}

	// 読み直したものを書き出すと同じテキストになること。
	if orig.String() != got.String() {
		t.Errorf("re-format differs:\n got %q\nwant %q", got.String(), orig.String())
	}
}

// 消費時間の無い KIF は ShowTime が false になること。
func TestParseWithoutTimes(t *testing.T) {
	src := `棋戦：テスト
先手：A
後手：B
手数----指手---------消費時間--
1 ７六歩(77)
2 ８四歩(83)
`
	d, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if d.ShowTime {
		t.Error("ShowTime should be false")
	}
	if len(d.Moves) != 2 {
		t.Fatalf("len(Moves) = %d", len(d.Moves))
	}
	if strings.Contains(d.String(), "/") {
		t.Errorf("re-formatted output should have no time column:\n%s", d.String())
	}
}

// 変化(分岐)は未対応。本譜だけ取って打ち切ること。
func TestParseStopsAtVariation(t *testing.T) {
	src := `先手：A
後手：B
手数----指手---------消費時間--
1 ７六歩(77)
2 ８四歩(83)

変化：2手
2 ３四歩(33)
3 ２二角成(88)
`
	d, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Moves) != 2 {
		t.Errorf("len(Moves) = %d, want 2 (変化 must not be included): %+v", len(d.Moves), d.Moves)
	}
}

func TestParseSkipsNoise(t *testing.T) {
	src := `# コメント
*指し手コメント
先手：A
後手：B
手数----指手---------消費時間--
1 ７六歩(77)
まで1手で中断
`
	d, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Moves) != 1 {
		t.Errorf("len(Moves) = %d, want 1: %+v", len(d.Moves), d.Moves)
	}
}

func TestParseBOMAndCRLF(t *testing.T) {
	src := "\ufeff先手：A\r\n後手：B\r\n手数----指手---------消費時間--\r\n1 ７六歩(77)\r\n"
	d, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if d.Black != "A" || d.White != "B" {
		t.Errorf("Black=%q White=%q", d.Black, d.White)
	}
	if len(d.Moves) != 1 {
		t.Errorf("len(Moves) = %d", len(d.Moves))
	}
}

// KIF でないものを読ませたらヘッダも指し手も取れず Empty になる。
// Parse 自体はエラーにしない(指し手 0 手は対局前として正当なため)。
func TestParseNonKifuIsEmpty(t *testing.T) {
	for _, src := range []string{"", "   ", "これは棋譜ではありません", "<html><body>404</body></html>"} {
		d, err := Parse(src)
		if err != nil {
			t.Errorf("Parse(%q) = %v", src, err)
			continue
		}
		if !d.Empty() {
			t.Errorf("Parse(%q).Empty() = false, want true", src)
		}
	}
}

// 対局前の中継棋譜。ヘッダだけで指し手が無くてもエラーにしない。
func TestParseHeaderOnly(t *testing.T) {
	src := `# --- Kifu for Windows Pro V7.21 棋譜ファイル ---
開始日時：2026/08/18 09:00
棋戦：第67期王位戦七番勝負第４局
手合割：平手
先手：伊藤匠二冠
後手：藤井聡太王位
手数----指手---------消費時間--
*対局前のコメント
`
	d, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Moves) != 0 {
		t.Errorf("len(Moves) = %d, want 0", len(d.Moves))
	}
	if d.Empty() {
		t.Error("Empty() = true, want false (ヘッダは読めている)")
	}
	if d.Black != "伊藤匠二冠" || d.White != "藤井聡太王位" {
		t.Errorf("Black=%q White=%q", d.Black, d.White)
	}
	if d.EndMark() != "" {
		t.Errorf("EndMark() = %q, want empty", d.EndMark())
	}
}

func TestParseTimeLimitVariants(t *testing.T) {
	cases := map[string]time.Duration{
		"各8時間": 8 * time.Hour,
		"8時間":  8 * time.Hour,
		"各25分": 25 * time.Minute,
		"":     0,
		"なし":   0,
	}
	for in, want := range cases {
		if got := parseTimeLimit(in); got != want {
			t.Errorf("parseTimeLimit(%q) = %v, want %v", in, got, want)
		}
	}
}
