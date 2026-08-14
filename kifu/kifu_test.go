package kifu

import (
	"strings"
	"testing"
	"time"
)

// core/web/test.mjs の kifu ケースと同じ期待値。片方だけ直さないこと。
func TestFormatLine(t *testing.T) {
	cases := []struct {
		name string
		num  int
		move string
		x, y int
		want string
	}{
		{"normal", 1, "７六歩", 7, 7, "1 ７六歩(77)"},
		{"strips modifier", 3, "７八銀右", 6, 9, "3 ７八銀(69)"},
		{"drop -> (00), keeps 打", 5, "５五角打", 8, 8, "5 ５五角打(00)"},
		// 読売のペイロードは終局を "投了打" として返す。座標なしの終局行にする。
		{"terminal 投了打", 104, "投了打", 0, 0, "104 投了"},
		{"terminal 投了", 104, "投了", 0, 0, "104 投了"},
		{"terminal 中断", 51, "中断", 0, 0, "51 中断"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FormatLine(c.num, c.move, c.x, c.y); got != c.want {
				t.Errorf("FormatLine() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestStripMoveModifiers(t *testing.T) {
	cases := map[string]string{
		"７八銀右":  "７八銀",
		"２二角不成": "２二角",
		"７六歩":   "７六歩",
	}
	for in, want := range cases {
		if got := StripMoveModifiers(in); got != want {
			t.Errorf("StripMoveModifiers(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTerminalMarker(t *testing.T) {
	for _, in := range []string{"投了", "投了打", "中断", "千日手打", "持将棋"} {
		if _, ok := TerminalMarker(in); !ok {
			t.Errorf("TerminalMarker(%q) = false, want true", in)
		}
	}
	// 通常の打ち手を終局と誤判定しないこと。
	for _, in := range []string{"５三銀打", "２三歩打", "７六歩", "同桂成"} {
		if m, ok := TerminalMarker(in); ok {
			t.Errorf("TerminalMarker(%q) = %q, want false", in, m)
		}
	}
}

func TestDocumentDefaultsToHirate(t *testing.T) {
	d := Document{Moves: []Move{{Num: 1, Name: "７六歩", FromX: 7, FromY: 7}}}
	got := d.String()

	want := "手合割：平手\n" + MoveColumnHeader + "\n1 ７六歩(77)\n"
	if got != want {
		t.Errorf("String() =\n%q\nwant\n%q", got, want)
	}
}

func TestDocumentHeaders(t *testing.T) {
	d := Document{
		Event:     "第37期竜王戦七番勝負第２局",
		Place:     "福井県あわら市",
		StartedAt: time.Date(2024, 10, 19, 9, 0, 0, 0, time.UTC),
		Black:     "藤井聡太竜王",
		White:     "佐々木勇気八段",
		Moves: []Move{
			{Num: 1, Name: "７六歩", FromX: 7, FromY: 7},
			{Num: 2, Name: "８四歩", FromX: 8, FromY: 3},
		},
	}
	got := d.String()

	for _, want := range []string{
		"開始日時：2024/10/19 09:00:00\n",
		"棋戦：第37期竜王戦七番勝負第２局\n",
		"場所：福井県あわら市\n",
		"手合割：平手\n",
		"先手：藤井聡太竜王\n",
		"後手：佐々木勇気八段\n",
		MoveColumnHeader + "\n",
		"1 ７六歩(77)\n",
		"2 ８四歩(83)\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("String() missing %q\ngot:\n%s", want, got)
		}
	}

	// ヘッダは列ヘッダより前、指し手は列ヘッダより後。
	if strings.Index(got, "先手：") > strings.Index(got, MoveColumnHeader) {
		t.Error("先手 header must come before the move column header")
	}
}

func TestFormatSpendAndTotal(t *testing.T) {
	spend := map[time.Duration]string{
		0:                                     " 0:00",
		16 * time.Second:                      " 0:16",
		4 * time.Minute:                       " 4:00",
		75 * time.Minute:                      "75:00", // 分は 60 を超えても繰り上げない
		-1 * time.Second:                      " 0:00",
		90*time.Second + 500*time.Millisecond: " 1:30",
	}
	for d, want := range spend {
		if got := FormatSpend(d); got != want {
			t.Errorf("FormatSpend(%v) = %q, want %q", d, got, want)
		}
	}

	total := map[time.Duration]string{
		0:                             "00:00:00",
		16 * time.Second:              "00:00:16",
		time.Hour + 2*time.Minute + 3: "01:02:00",
		7*time.Hour + 42*time.Minute:  "07:42:00",
	}
	for d, want := range total {
		if got := FormatTotal(d); got != want {
			t.Errorf("FormatTotal(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestFormatTimes(t *testing.T) {
	got := FormatTimes(4*time.Minute, time.Hour)
	if got != "( 4:00/01:00:00)" {
		t.Errorf("FormatTimes = %q", got)
	}
}

func TestFormatTimeLimit(t *testing.T) {
	cases := map[time.Duration]string{
		0:                "",
		8 * time.Hour:    "各8時間",
		25 * time.Minute: "各25分",
	}
	for d, want := range cases {
		if got := FormatTimeLimit(d); got != want {
			t.Errorf("FormatTimeLimit(%v) = %q, want %q", d, got, want)
		}
	}
}

// 累計消費時間は対局者ごとに積む(奇数手が先手、偶数手が後手)。
func TestDocumentShowTime(t *testing.T) {
	d := Document{
		Black:     "先手 太郎",
		White:     "後手 次郎",
		TimeLimit: 8 * time.Hour,
		Countdown: 60 * time.Second,
		ShowTime:  true,
		Moves: []Move{
			{Num: 1, Name: "７六歩", FromX: 7, FromY: 7, Spend: 0},
			{Num: 2, Name: "８四歩", FromX: 8, FromY: 3, Spend: time.Minute},
			{Num: 3, Name: "６八銀", FromX: 7, FromY: 9, Spend: time.Minute},
			{Num: 4, Name: "３四歩", FromX: 3, FromY: 3, Spend: 2 * time.Minute},
		},
	}
	got := d.String()

	for _, want := range []string{
		"持ち時間：各8時間\n",
		"秒読み：60秒\n",
		"1 ７六歩(77)   ( 0:00/00:00:00)\n",
		"2 ８四歩(83)   ( 1:00/00:01:00)\n",
		"3 ６八銀(79)   ( 1:00/00:01:00)\n", // 先手の累計は 0:00 + 1:00
		"4 ３四歩(33)   ( 2:00/00:03:00)\n", // 後手の累計は 1:00 + 2:00
	} {
		if !strings.Contains(got, want) {
			t.Errorf("String() missing %q\ngot:\n%s", want, got)
		}
	}
}

// ShowTime が false なら消費時間欄を出さない
// (取得元が消費時間を持たないとき "( 0:00/00:00:00)" が並ぶのを避ける)。
func TestDocumentWithoutShowTime(t *testing.T) {
	d := Document{
		Moves: []Move{{Num: 1, Name: "７六歩", FromX: 7, FromY: 7, Spend: time.Minute}},
	}
	got := d.String()
	if strings.Contains(got, "/") || strings.Contains(got, "0:00") {
		t.Errorf("String() should omit the time column, got:\n%s", got)
	}
	if !strings.Contains(got, "1 ７六歩(77)\n") {
		t.Errorf("got:\n%s", got)
	}
}

// 終局行にも消費時間を出す(元データに入っているため)。
func TestDocumentShowTimeOnTerminalMove(t *testing.T) {
	d := Document{
		ShowTime: true,
		Moves: []Move{
			{Num: 1, Name: "７六歩", FromX: 7, FromY: 7, Spend: time.Minute},
			{Num: 2, Name: "投了打", Spend: 4 * time.Minute},
		},
	}
	got := d.String()
	if !strings.Contains(got, "2 投了   ( 4:00/00:04:00)\n") {
		t.Errorf("terminal move should carry its time, got:\n%s", got)
	}
}

func TestDocumentOmitsEmptyHeaders(t *testing.T) {
	d := Document{Handicap: "平手"}
	got := d.String()
	for _, ng := range []string{"棋戦：", "場所：", "先手：", "後手：", "開始日時："} {
		if strings.Contains(got, ng) {
			t.Errorf("String() should omit %q, got:\n%s", ng, got)
		}
	}
}
