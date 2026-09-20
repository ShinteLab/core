package kifu_test

import (
	"strings"
	"testing"

	"github.com/ShinteLab/core/kifu"
)

const hirateFull = "lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL b - 1"

// 平手の盤面図が KIF の書式どおりに出ること（2026-09-16）。
//
// ⚠️ **ここが崩れると他のソフトで開けない。** 桁（1 マス 2 文字）も罫線の長さも
// 書式で決まっているので、目で見て分かる形のまま固定しておく。
func TestFormatBODHirate(t *testing.T) {
	got, err := kifu.FormatBOD(hirateFull)
	if err != nil {
		t.Fatalf("FormatBOD: %v", err)
	}
	want := strings.Join([]string{
		"後手の持駒：なし",
		"  ９ ８ ７ ６ ５ ４ ３ ２ １",
		"+---------------------------+",
		"|v香v桂v銀v金v玉v金v銀v桂v香|一",
		"| ・v飛 ・ ・ ・ ・ ・v角 ・|二",
		"|v歩v歩v歩v歩v歩v歩v歩v歩v歩|三",
		"| ・ ・ ・ ・ ・ ・ ・ ・ ・|四",
		"| ・ ・ ・ ・ ・ ・ ・ ・ ・|五",
		"| ・ ・ ・ ・ ・ ・ ・ ・ ・|六",
		"| 歩 歩 歩 歩 歩 歩 歩 歩 歩|七",
		"| ・ 角 ・ ・ ・ ・ ・ 飛 ・|八",
		"| 香 桂 銀 金 玉 金 銀 桂 香|九",
		"+---------------------------+",
		"先手の持駒：なし",
		"",
	}, "\n")
	if got != want {
		t.Errorf("盤面図が違います:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	// ⚠️ **先手番では手番の行を書かないこと**（KIF の既定なので、書くと
	// わざわざ指定した棋譜に見える）。
	if strings.Contains(got, "番") {
		t.Errorf("先手番なのに手番の行が出ています:\n%s", got)
	}
}

// 盤面図が SFEN と往復すること（**これが一番の歯止め**）。
//
// ⚠️ **成駒・持ち駒・後手番のどれが落ちても、開き直したときに別の局面になる。**
func TestBODRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		sfen string
	}{
		{"平手", hirateFull},
		{"後手番", "lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL w - 1"},
		{"持ち駒", "lnsgkgsnl/1r5b1/pppppp1pp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL b P2p 1"},
		{"歩を全部持つ", "lnsgkgsnl/1r5b1/9/9/9/9/9/1B5R1/LNSGKGSNL b 9P9p 1"},
		{"成駒", "lnsgkgsnl/1r5b1/ppppppppp/9/4+P4/9/PPPP1PPPP/1B5+r1/LNSGKGSNL w GSnl 1"},
		{"駒が少ない（詰将棋）", "4k4/9/4G4/9/9/9/9/9/9 b G2r2b3g4s4n4l17p 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bod, err := kifu.FormatBOD(c.sfen)
			if err != nil {
				t.Fatalf("FormatBOD: %v", err)
			}
			back, err := kifu.ParseBOD(strings.Split(bod, "\n"))
			if err != nil {
				t.Fatalf("ParseBOD: %v\n%s", err, bod)
			}
			if back != c.sfen {
				t.Errorf("往復しません:\n got = %s\nwant = %s\n%s", back, c.sfen, bod)
			}
		})
	}
}

// ⚠️ **1 マスは必ず 2 文字。** 成駒を「成香」と書くと**そこから右が全部ずれる**ので、
// 盤面図では 1 文字（と杏圭全馬龍）にする。
func TestFormatBODPromotedIsOneLetter(t *testing.T) {
	const s = "9/9/9/9/9/9/9/9/+P+L+N+S+R+B3 b - 1"
	got, err := kifu.FormatBOD(s)
	if err != nil {
		t.Fatalf("FormatBOD: %v", err)
	}
	if !strings.Contains(got, "| と 杏 圭 全 龍 馬 ・ ・ ・|九") {
		t.Errorf("成駒が 1 文字になっていません:\n%s", got)
	}
	// どの段も同じ桁であること（罫線と揃う）。
	for _, line := range strings.Split(got, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if n := len([]rune(line)); n != 21 { // | + 9マス×2 + | + 段の見出し
			t.Errorf("段の桁が違います（%d 文字）: %q", n, line)
		}
	}
}

// 持駒の枚数が漢数字で書かれ、読み戻せること。
func TestBODHandNumbers(t *testing.T) {
	const s = "4k4/9/9/9/9/9/9/9/4K4 b 2G18P 1"
	got, err := kifu.FormatBOD(s)
	if err != nil {
		t.Fatalf("FormatBOD: %v", err)
	}
	if !strings.Contains(got, "先手の持駒：金二　歩十八") {
		t.Errorf("持駒が漢数字になっていません:\n%s", got)
	}
	back, err := kifu.ParseBOD(strings.Split(got, "\n"))
	if err != nil {
		t.Fatalf("ParseBOD: %v", err)
	}
	if back != s {
		t.Errorf("持駒が往復しません: %s, want %s", back, s)
	}
}

// ⚠️ **他のソフトの書き方も読めること**（KIF は方言が多い）。
// 上手／下手・「手番：後手」・算用数字・`竜`/`王`・半角区切り。
func TestParseBODDialects(t *testing.T) {
	text := strings.Join([]string{
		"上手の持駒：飛2",
		"  ９ ８ ７ ６ ５ ４ ３ ２ １",
		"+---------------------------+",
		"| ・ ・ ・ ・v玉 ・ ・ ・ ・|一",
		"| ・ ・ ・ ・ ・ ・ ・ ・ ・|二",
		"| ・ ・ ・ ・ ・ ・ ・ ・ ・|三",
		"| ・ ・ ・ ・ ・ ・ ・ ・ ・|四",
		"| ・ ・ ・ ・ 竜 ・ ・ ・ ・|五",
		"| ・ ・ ・ ・ ・ ・ ・ ・ ・|六",
		"| ・ ・ ・ ・ ・ ・ ・ ・ ・|七",
		"| ・ ・ ・ ・ ・ ・ ・ ・ ・|八",
		"| ・ ・ ・ ・ 王 ・ ・ ・ ・|九",
		"+---------------------------+",
		"下手の持駒：なし",
		"手番：後手",
	}, "\n")
	got, err := kifu.ParseBOD(strings.Split(text, "\n"))
	if err != nil {
		t.Fatalf("ParseBOD: %v", err)
	}
	// ⚠️ **`竜` は龍**（成駒）、**`王` は玉**、手番は後手。
	const want = "4k4/9/9/9/4+R4/9/9/9/4K4 w 2r 1"
	if got != want {
		t.Errorf("方言が読めていません:\n got = %s\nwant = %s", got, want)
	}
}

// ⚠️ **盤面図が無いことと読めないことを分けること**（呼び出し側が判断できるように）。
func TestParseBODMissing(t *testing.T) {
	if _, err := kifu.ParseBOD([]string{"手合割：平手"}); err == nil {
		t.Error("盤面図が無いのにエラーになりません")
	}
	broken := strings.Join([]string{
		"+---------------------------+",
		"| ・ ・ ・|一",
		"+---------------------------+",
	}, "\n")
	if _, err := kifu.ParseBOD(strings.Split(broken, "\n")); err == nil {
		t.Error("壊れた盤面図がエラーになりません")
	}
}

// `Document` に盤面図が乗ること（**書く側**）。
//
// ⚠️ **盤面図があるなら手合割は書かないこと** —— 両方あると、読み手によって
// 別の局面になる（どちらを正とするかは書かれていない）。
func TestDocumentStringWritesBOD(t *testing.T) {
	d := kifu.Document{
		Black: "先手太郎",
		White: "後手花子",
		Start: "4k4/9/4G4/9/9/9/9/9/4K4 b G 1",
		Moves: []kifu.Move{{Num: 1, Name: "５二金", FromX: 5, FromY: 3}},
	}
	got := d.String()
	if !strings.Contains(got, "+---------------------------+") {
		t.Fatalf("盤面図が出ていません:\n%s", got)
	}
	if strings.Contains(got, "手合割") {
		t.Errorf("盤面図と手合割が両方出ています:\n%s", got)
	}
	if !strings.Contains(got, "先手の持駒：金") {
		t.Errorf("持駒が出ていません:\n%s", got)
	}
	if !strings.Contains(got, "５二金") {
		t.Errorf("指し手が出ていません:\n%s", got)
	}
}

// 書いた KIF を読み戻すと、開始局面も手順も戻ること（**往復**）。
func TestDocumentBODRoundTrip(t *testing.T) {
	const start = "ln1g5/1r7/p2pppn2/2ps2p2/1p7/2P6/PPSPPPP1P/2G2K1R1/LN1G3NL b SPbgsn2p 1"
	d := kifu.Document{
		Event: "テスト棋戦",
		Black: "先手太郎",
		White: "後手花子",
		Start: start,
	}
	back, err := kifu.Parse(d.String())
	if err != nil {
		t.Fatalf("Parse: %v\n%s", err, d.String())
	}
	if back.Start != start {
		t.Errorf("開始局面が戻りません:\n got = %s\nwant = %s", back.Start, start)
	}
	// ⚠️ **`StartSFEN()` は盤面図のほうを返すこと**（手合割は空＝平手に見える）。
	got, err := back.StartSFEN()
	if err != nil {
		t.Fatalf("StartSFEN: %v", err)
	}
	if got != start {
		t.Errorf("StartSFEN が手合割に倒れています: %s", got)
	}
	if back.Event != "テスト棋戦" || back.Black != "先手太郎" {
		t.Errorf("ヘッダが落ちています: %+v", back)
	}
}

// ⚠️ **盤面図が読めなければ `Parse` はエラーにすること。**
//
// **黙って手合割へ倒さないこと** —— 盤面図が書いてある以上、手合割の初期局面は
// その棋譜の局面ではない。倒すと「平手の初形に途中の手順が乗った別の対局」として
// 読めてしまい、**棋譜としては通るので画面では気づけない。**
func TestParseRefusesBrokenBOD(t *testing.T) {
	text := strings.Join([]string{
		"手合割：平手",
		"+---------------------------+",
		"|v香v桂v銀|一",
		"+---------------------------+",
		"手数----指手---------消費時間--",
		"   1 ７六歩(77)",
	}, "\n")
	if _, err := kifu.Parse(text); err == nil {
		t.Error("壊れた盤面図の棋譜が読めてしまっています")
	}
}

// 盤面図が無い棋譜は今までどおり（**手合割で開始局面が決まる**）。
func TestParseWithoutBODUnchanged(t *testing.T) {
	text := strings.Join([]string{
		"手合割：二枚落ち",
		"手数----指手---------消費時間--",
		"   1 ３四歩(33)",
	}, "\n")
	d, err := kifu.Parse(text)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if d.Start != "" {
		t.Errorf("盤面図が無いのに開始局面が入っています: %s", d.Start)
	}
	got, err := d.StartSFEN()
	if err != nil {
		t.Fatalf("StartSFEN: %v", err)
	}
	want, _ := kifu.StartSFEN("二枚落ち")
	if got != want {
		t.Errorf("手合割の初期局面が出ません: %s, want %s", got, want)
	}
}
