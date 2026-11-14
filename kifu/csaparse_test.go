package kifu

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const sampleCSA = `'CSA encoding=UTF-8
V2.2
N+先手の人
N-後手の人
$EVENT:テスト棋戦
$SITE:東京
$START_TIME:2026/11/15 10:00:00
$TIME_LIMIT:01:30+60
P1-KY-KE-GI-KI-OU-KI-GI-KE-KY
P2 * -HI *  *  *  *  * -KA *
P3-FU-FU-FU-FU-FU-FU-FU-FU-FU
P4 *  *  *  *  *  *  *  *  *
P5 *  *  *  *  *  *  *  *  *
P6 *  *  *  *  *  *  *  *  *
P7+FU+FU+FU+FU+FU+FU+FU+FU+FU
P8 * +KA *  *  *  *  * +HI *
P9+KY+KE+GI+KI+OU+KI+GI+KE+KY
P+
P-
+
'*初期局面のコメント
+7776FU
T12
'*角道を開ける
-3334FU,T5
+8822UM
T3
-3122GI
T1
%TORYO
`

// TestParseCSA は対局情報・平手の判定・指し手名・消費時間・コメント・終局を見る。
func TestParseCSA(t *testing.T) {
	d, err := ParseCSA(sampleCSA)
	if err != nil {
		t.Fatalf("ParseCSA: %v", err)
	}
	if d.Black != "先手の人" || d.White != "後手の人" || d.Event != "テスト棋戦" || d.Place != "東京" {
		t.Errorf("対局情報 = %q %q %q %q", d.Black, d.White, d.Event, d.Place)
	}
	if d.StartedAt.Format("2006/01/02 15:04:05") != "2026/11/15 10:00:00" {
		t.Errorf("StartedAt = %v", d.StartedAt)
	}
	if d.TimeLimit != 90*time.Minute || d.Countdown != 60*time.Second {
		t.Errorf("持ち時間 = %v / %v", d.TimeLimit, d.Countdown)
	}
	// 平手の初期局面なら手合割の名前にする（盤面図にしない）。
	if d.Handicap != HirateHandicap || d.Start != "" {
		t.Errorf("Handicap = %q, Start = %q", d.Handicap, d.Start)
	}
	if !d.ShowTime {
		t.Error("ShowTime が立っていない")
	}
	if d.Comment != "初期局面のコメント" {
		t.Errorf("Comment = %q", d.Comment)
	}
	want := []struct {
		name   string
		fx, fy int
		spend  time.Duration
	}{
		{"７六歩", 7, 7, 12 * time.Second},
		{"３四歩", 3, 3, 5 * time.Second},
		{"２二角成", 8, 8, 3 * time.Second},
		{"同　銀", 3, 1, 1 * time.Second},
		{"投了", 0, 0, 0},
	}
	if len(d.Moves) != len(want) {
		t.Fatalf("手数 = %d, want %d: %+v", len(d.Moves), len(want), d.Moves)
	}
	for i, w := range want {
		m := d.Moves[i]
		if m.Num != i+1 || m.Name != w.name || m.FromX != w.fx || m.FromY != w.fy || m.Spend != w.spend {
			t.Errorf("%d手目 = %+v, want %+v", i+1, m, w)
		}
	}
	if d.Moves[0].Comment != "角道を開ける" {
		t.Errorf("1手目のコメント = %q", d.Moves[0].Comment)
	}
	if d.EndMark() != "投了" {
		t.Errorf("EndMark = %q", d.EndMark())
	}
	// **KIF と同じ経路で USI に戻ること**（使う側は CSA を知らなくてよい）。
	got, err := DecodeMoves(d.Moves)
	if err != nil {
		t.Fatalf("DecodeMoves: %v", err)
	}
	if strings.Join(got, " ") != "7g7f 3c3d 8h2b+ 3a2b" {
		t.Errorf("DecodeMoves = %v", got)
	}
	// KIF として書き出して読み戻しても同じ手になる（CSA → KIF の変換）。
	back, err := Parse(d.String())
	if err != nil {
		t.Fatalf("Parse(String()): %v", err)
	}
	again, _ := DecodeMoves(back.Moves)
	if strings.Join(again, " ") != strings.Join(got, " ") {
		t.Errorf("KIF に書き出して読み戻した手 = %v", again)
	}
}

// TestParseCSAHandicap は PI の駒落ちが手合割の名前になることを見る。
func TestParseCSAHandicap(t *testing.T) {
	d, err := ParseCSA("PI82HI\n-\n-5142OU\n")
	if err != nil {
		t.Fatalf("ParseCSA: %v", err)
	}
	if d.Handicap != "飛車落ち" || d.Start != "" {
		t.Errorf("Handicap = %q, Start = %q", d.Handicap, d.Start)
	}
	// ⚠️ 手合割に無い局面（駒落ちなのに先手番）は盤面図にする。
	d, err = ParseCSA("PI82HI\n+\n")
	if err != nil {
		t.Fatalf("ParseCSA: %v", err)
	}
	if d.Handicap != "" || !strings.HasSuffix(d.Start, " b - 1") {
		t.Errorf("Handicap = %q, Start = %q", d.Handicap, d.Start)
	}
}

// TestParseCSAMatePosition は詰将棋の形（P+ / P- で駒を置き、00AL で残りを配る）を見る。
func TestParseCSAMatePosition(t *testing.T) {
	d, err := ParseCSA("P-51OU\nP+52KI00KI\nP-00AL\n+\n+0042KI\n")
	if err != nil {
		t.Fatalf("ParseCSA: %v", err)
	}
	const want = "4k4/4G4/9/9/9/9/9/9/9 b G2r2b2g4s4n4l18p 1"
	if d.Start != want {
		t.Errorf("Start = %q, want %q", d.Start, want)
	}
	if len(d.Moves) != 1 || d.Moves[0].Name != "４二金打" {
		t.Errorf("Moves = %+v", d.Moves)
	}
}

// TestParseCSAStopsMidway は途中で読めなくなっても、そこまでの手を返すことを見る。
func TestParseCSAStopsMidway(t *testing.T) {
	d, err := ParseCSA("PI\n+\n+7776FU\n+3334FU\n")
	if !errors.Is(err, ErrCSAMoves) {
		t.Fatalf("err = %v, want ErrCSAMoves", err)
	}
	if len(d.Moves) != 1 || d.Moves[0].Name != "７六歩" {
		t.Errorf("Moves = %+v", d.Moves)
	}
}

// TestParseCSABrokenStart は開始局面が読めなければ平手に倒さないことを見る。
func TestParseCSABrokenStart(t *testing.T) {
	for _, in := range []string{
		"N+a\n+7776FU\n",           // 局面の行が無い
		"PI55FU\n+\n",              // 取り除く駒が無い
		"P1-KY-KE-XX\n+\n",         // 駒が読めない
		"P+00OU\n+\n",              // 玉は駒台に置けない
		"PI\n+\n+7776FU\nP+00FU\n", // 指し手のあとに局面
	} {
		if _, err := ParseCSA(in); err == nil || errors.Is(err, ErrCSAMoves) {
			t.Errorf("ParseCSA(%q) err = %v, want 開始局面のエラー", in, err)
		}
	}
}

// TestIsCSA は KIF と CSA を見分けられることを見る。
func TestIsCSA(t *testing.T) {
	for in, want := range map[string]bool{
		sampleCSA:            true,
		"+7776FU\n-3334FU\n": true,
		"PI\n+\n":            true,
		"手合割：平手\n   1 ７六歩(77)\n":                                           false,
		"後手の持駒：なし\n+---------------------------+\n|v香v桂v銀v金v玉v金v銀v桂v香|一\n": false,
		"<html><body>hello</body></html>":                                  false,
		"":                                                                 false,
	} {
		if got := IsCSA(in); got != want {
			t.Errorf("IsCSA(%q) = %v, want %v", in, got, want)
		}
	}
}
