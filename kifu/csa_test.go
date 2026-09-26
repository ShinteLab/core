package kifu

import (
	"reflect"
	"strings"
	"testing"
)

const hirate = "lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL b - 1"

// 初期局面からの数手。筋は 10-筋 ではなく USI の数字のまま、段は a..i になること。
func TestDecodeCSA(t *testing.T) {
	got, end, err := DecodeCSA(hirate, []string{"+2726FU", "-8384FU", "+7776FU"})
	if err != nil || end != "" {
		t.Fatalf("DecodeCSA = %v, %q, %v", got, end, err)
	}
	want := []string{"2g2f", "8c8d", "7g7f"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DecodeCSA = %v, want %v", got, want)
	}
}

// ⚠️ CSA は移動後の駒種を書く。**同じ "UM" でも、角が成った手と馬が動いた手がある。**
// 前者だけに "+" が付くこと。
func TestDecodeCSAPromotion(t *testing.T) {
	moves := []string{
		"+7776FU", "-3334FU",
		"+8822UM", // 角が成って 2二 の角を取る → "+" が付く
		"-3142GI",
		"+2211UM", // 既に馬 → "+" は付かない
	}
	got, _, err := DecodeCSA(hirate, moves)
	if err != nil {
		t.Fatalf("DecodeCSA: %v (got %v)", err, got)
	}
	want := []string{"7g7f", "3c3d", "8h2b+", "3a4b", "2b1a"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DecodeCSA = %v, want %v", got, want)
	}
}

// 実戦（伊藤匠 vs 郷田真隆, 王将戦挑決リーグ 2026-09-25。将棋DB2）の角交換。
// 後手の "-2277UM" は成り、先手の "+8877GI" はただの取り返し。
func TestDecodeCSAKakugawari(t *testing.T) {
	moves := strings.Fields("+2726FU -8384FU +2625FU -8485FU +7776FU -4132KI +8877KA -3334FU +7988GI -2277UM +8877GI -3122GI")
	got, _, err := DecodeCSA(hirate, moves)
	if err != nil {
		t.Fatalf("DecodeCSA: %v", err)
	}
	if got[9] != "2b7g+" || got[10] != "8h7g" {
		t.Errorf("10,11手目 = %q, %q, want \"2b7g+\", \"8h7g\"", got[9], got[10])
	}

	// 日本語表記まで通す（kicho はこの経路で KIF を作る）。
	texts, err := FormatMoves(hirate, got)
	if err != nil {
		t.Fatalf("FormatMoves: %v", err)
	}
	names := make([]string, len(texts))
	for i, m := range texts {
		names[i] = m.Text
	}
	want := []string{
		"▲２六歩", "△８四歩", "▲２五歩", "△８五歩", "▲７六歩", "△３二金",
		"▲７七角", "△３四歩", "▲８八銀", "△７七角成", "▲同　銀", "△２二銀",
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("FormatMoves =\n%v\nwant\n%v", names, want)
	}
}

func TestDecodeCSADrop(t *testing.T) {
	start := "4k4/9/9/9/9/9/9/9/4K4 b N 1"
	got, _, err := DecodeCSA(start, []string{"+0044KE"})
	if err != nil || !reflect.DeepEqual(got, []string{"N*4d"}) {
		t.Errorf("DecodeCSA = %v, %v, want [N*4d]", got, err)
	}
}

// 終局表記はそこで止め、KIF の終局の名前を返す。後ろに何かあっても読まない。
func TestDecodeCSATerminal(t *testing.T) {
	got, end, err := DecodeCSA(hirate, []string{"+7776FU", "T3", "", "-3334FU", "%TORYO", "+8822UM"})
	if err != nil {
		t.Fatalf("DecodeCSA: %v", err)
	}
	if end != "投了" || !reflect.DeepEqual(got, []string{"7g7f", "3c3d"}) {
		t.Errorf("DecodeCSA = %v, %q, want [7g7f 3c3d], \"投了\"", got, end)
	}
	if _, ok := TerminalMarker(end); !ok {
		t.Errorf("%q が TerminalMarkers にありません", end)
	}
}

// 写し先がすべて TerminalMarkers にあること（KIF に書いたときに終局として読まれるため）。
func TestCSASpecialsAreTerminalMarkers(t *testing.T) {
	for csa, name := range csaSpecials {
		if _, ok := TerminalMarker(name); !ok {
			t.Errorf("%s → %q が TerminalMarkers にありません", csa, name)
		}
	}
}

// ⚠️ 知らない % は黙って読み飛ばさない（終局した棋譜が対局中に見える）。
func TestDecodeCSAUnknownSpecial(t *testing.T) {
	got, end, err := DecodeCSA(hirate, []string{"+7776FU", "%+ILLEGAL_ACTION"})
	if err == nil {
		t.Fatalf("DecodeCSA = %v, %q, want error", got, end)
	}
	if !reflect.DeepEqual(got, []string{"7g7f"}) {
		t.Errorf("エラーでもそこまでの手を返すこと: got %v", got)
	}
}

// ⚠️ 1 手抜けた棋譜を黙って通さない。エラーでもそこまでの手は返す。
func TestDecodeCSATurnMismatch(t *testing.T) {
	got, _, err := DecodeCSA(hirate, []string{"+7776FU", "+2726FU"})
	if err == nil || !strings.Contains(err.Error(), "2手目") {
		t.Fatalf("err = %v, want 2手目の手番エラー", err)
	}
	if !reflect.DeepEqual(got, []string{"7g7f"}) {
		t.Errorf("got %v, want [7g7f]", got)
	}

	// 後手番から始まる局面（駒落ち）では "-" が先。
	start, _ := StartSFEN("香落ち")
	if _, _, err := DecodeCSA(start, []string{"+7776FU"}); err == nil {
		t.Error("後手番の局面で + を受けた")
	}
}

func TestDecodeCSAInvalid(t *testing.T) {
	for _, s := range []string{
		"+7776F",  // 短い
		"7776FU+", // 符号が無い
		"+7776XX", // 知らない駒
		"+7076FU", // 移動元の段が 0
		"+7770FU", // 移動先が盤の外
		"+5576FU", // 移動元が空
		"+7776KY", // 移動元の駒と合わない
		"+0055TO", // 成駒は打てない
		"+0055OU", // 玉は打てない
	} {
		if got, _, err := DecodeCSA(hirate, []string{s}); err == nil {
			t.Errorf("DecodeCSA(%q) = %v, want error", s, got)
		}
	}
}

// 実戦 1 局（145 手 + %TORYO）を最後まで読めること。
func TestDecodeCSAFullGame(t *testing.T) {
	moves := strings.Fields(`+2726FU -8384FU +2625FU -8485FU +7776FU -4132KI +8877KA -3334FU +7988GI -2277UM
+8877GI -3122GI +6978KI -2233GI +3938GI -7162GI +4746FU -6364FU +3736FU -5142OU +2937KE -7374FU +3847GI
-6263GI +1716FU -9394FU +9796FU -1314FU +4948KI -8173KE +5968OU -6162KI +4756GI -6465FU +2829HI -8281HI
+6766FU -6566FU +7766GI -6354GI +6858OU -8586FU +8786FU -8186HI +5645GI -5463GI +0064FU -6352GI +0087FU
-8681HI +4556GI -4231OU +8977KE -5354FU +7765KE -0044KA +0088KA -1415FU +3745KE -3342GI +6573NK -6273KI
+0026KE -0022KE +1615FU -7364KI +1514FU -0012FU +8897KA -6463KI +6665GI -8161HI +0064FU -6373KI +5847OU
-9495FU +9695FU -0096FU +9786KA -0085FU +8668KA -5455FU +5667GI -7364KI +6564GI -6164HI +6877KA -7475FU
+7675FU -0076FU +6776GI -0028GI +2928HI -6469RY +4837KI -6919RY +2868HI -0061KY +0063FU -5263GI +0064FU
-6352GI +0054KI -5556FU +5444KI -4344FU +7744KA -4243GI +4471UM -0077FU +0053KA -5253GI +4553NK -5657TO
+4757OU -1959RY +0058FU -0039KA +5756OU -0052FU +5343NK -3243KI +0054GI -5968RY +0032GI -3132OU +5443NG
-3243OU +0044KI -4332OU +0043GI -3231OU +7868KI -0066HI +5645OU -0033KE +4433KI -2133KE +4544OU -0041GI
+0051HI -6664HI +4433OU -3966UM +0044KE %TORYO`)
	got, end, err := DecodeCSA(hirate, moves)
	if err != nil {
		t.Fatalf("DecodeCSA: %v (%d手まで)", err, len(got))
	}
	if len(got) != 145 || end != "投了" {
		t.Fatalf("got %d手, %q, want 145手, \"投了\"", len(got), end)
	}
	texts, err := FormatMoves(hirate, got)
	if err != nil {
		t.Fatalf("FormatMoves: %v", err)
	}
	for i, m := range texts {
		if !m.OK {
			t.Errorf("%d手目 %q を日本語表記にできません", i+1, m.USI)
		}
	}
	if last := texts[144].Text; last != "▲４四桂打" {
		t.Errorf("145手目 = %q, want \"▲４四桂打\"", last)
	}
}
