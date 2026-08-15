package shogifont

import (
	"fmt"
	"slices"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// テストは入力フォントを要求しない。手元にどの日本語フォントが入っているかで
// 結果が変わると、壊れたときにツールの問題か環境の問題か切り分けられないため。
// 輪郭は左右非対称にしてあり、反転したかどうかが座標で判る。

const testUPEM = 1000

// fakeGlyph は左右非対称な輪郭を持つ検査用グリフを返す。
// 2 本目の輪郭には off-curve 点を入れてあり、反転時の点順の入れ替わりを見る。
func fakeGlyph() *glyph {
	g := &glyph{adv: testUPEM}
	g.contours = [][]point{
		{{100, 0, true}, {100, 700, true}, {600, 700, true}, {600, 0, true}},
		{{200, 100, true}, {200, 300, false}, {300, 300, true}},
	}
	g.computeBounds()
	return g
}

// fakeSource は全ての文字を fakeGlyph で返すグリフ供給元。
func fakeSource(r rune) (*glyph, error) { return fakeGlyph(), nil }

func TestMirrored(t *testing.T) {
	src := fakeGlyph()
	m := src.mirrored()

	if m.adv != src.adv {
		t.Errorf("送り幅が変わった: got %d, want %d", m.adv, src.adv)
	}
	// 送り幅の中心で折り返すので、左右のサイドベアリングが入れ替わる。
	// 字面の中心で折り返してしまうと bbox が動かず、ここで気づける。
	if m.xmin != src.adv-src.xmax || m.xmax != src.adv-src.xmin {
		t.Errorf("bbox が折り返されていない: got [%d,%d], want [%d,%d]",
			m.xmin, m.xmax, src.adv-src.xmax, src.adv-src.xmin)
	}
	if m.ymin != src.ymin || m.ymax != src.ymax {
		t.Errorf("上下が動いた: got [%d,%d], want [%d,%d]", m.ymin, m.ymax, src.ymin, src.ymax)
	}
	if len(m.contours) != len(src.contours) {
		t.Fatalf("輪郭数が変わった: got %d, want %d", len(m.contours), len(src.contours))
	}

	// 点順は p0 を残して以降を逆順にする。単純に slice を逆順にすると
	// off-curve 点が輪郭の先頭に来てしまう。
	want := []point{{800, 100, true}, {700, 300, true}, {800, 300, false}}
	got := m.contours[1]
	if len(got) != len(want) {
		t.Fatalf("点数が変わった: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("contours[1][%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	for _, c := range m.contours {
		if !c[0].on {
			t.Error("輪郭の先頭が off-curve 点になっている")
		}
	}
}

func TestBuildGlyphSetMapping(t *testing.T) {
	set, err := buildGlyphSet(fakeSource, testUPEM)
	if err != nil {
		t.Fatal(err)
	}

	gid := func(r rune) uint16 {
		t.Helper()
		g, ok := set.cmap[r]
		if !ok {
			t.Fatalf("cmap に %q が無い", r)
		}
		if int(g) >= len(set.glyphs) {
			t.Fatalf("%q の GID %d がグリフ表の外", r, g)
		}
		return g
	}

	// 先手 (大文字) / 後手 (小文字) / 漢字は同じグリフ。反転はしない。
	for _, p := range basePieces {
		if gid(p.letter) != gid(p.letter+32) || gid(p.letter) != gid(p.kanji) {
			t.Errorf("%c / %c / %c の GID が揃っていない", p.letter, p.letter+32, p.kanji)
		}
	}

	// 異体字と反転字は SFEN 文字の向き先を変えず、別グリフとして同居する。
	if gid('王') != gid('K') {
		t.Error("K が王を指していない")
	}
	if gid('玉') == gid('王') {
		t.Error("玉が王と同じグリフになっている (別字として同梱するはず)")
	}
	if gid(leftHorse) == gid('馬') {
		t.Error("左馬が馬と同じグリフになっている (反転していない)")
	}

	// 左馬は馬を折り返したもの。
	uma, hidari := set.glyphs[gid('馬')], set.glyphs[gid(leftHorse)]
	if hidari.xmin != uma.adv-uma.xmax || hidari.xmax != uma.adv-uma.xmin {
		t.Errorf("左馬の bbox = [%d,%d], want [%d,%d]",
			hidari.xmin, hidari.xmax, uma.adv-uma.xmax, uma.adv-uma.xmin)
	}

	// 成駒は "+" + 基本駒 のリガチャ。cmap には成駒の漢字だけが載る。
	if len(set.ligs) != len(promotedPieces) {
		t.Errorf("リガチャ数 = %d, want %d", len(set.ligs), len(promotedPieces))
	}
	for i, p := range promotedPieces {
		if set.ligs[i][0] != gid(p.base) {
			t.Errorf("%c のリガチャが %c から引かれていない", p.kanji, p.base)
		}
		if set.ligs[i][1] != gid(p.kanji) {
			t.Errorf("%c のリガチャ先が %c のグリフでない", p.kanji, p.kanji)
		}
	}
}

// 異体字は「別の符号位置を描く」と「stylistic set で切り替える」の両方で出せる。
// cmap 側は TestBuildGlyphSetMapping、GSUB 側がこれ。
func TestBuildGlyphSetAltFeatures(t *testing.T) {
	set, err := buildGlyphSet(fakeSource, testUPEM)
	if err != nil {
		t.Fatal(err)
	}

	// タグ昇順。FeatureList は昇順に並べる決まりで、buildGSUB は liga/rlig の
	// 後ろにこの並びをそのまま繋ぐ。
	var tags []string
	for _, a := range set.alts {
		tags = append(tags, a.tag)
	}
	if want := []string{"ss01", "ss02"}; !slices.Equal(tags, want) {
		t.Fatalf("feature タグ = %v, want %v", tags, want)
	}

	sub := map[string][2]uint16{}
	for _, a := range set.alts {
		if len(a.subs) != 1 {
			t.Fatalf("%s の置換数 = %d, want 1", a.tag, len(a.subs))
		}
		sub[a.tag] = a.subs[0]
	}
	// ss01: 王 -> 玉。K/k/王 は同じグリフなのでどれで書いても効く。
	if got, want := sub["ss01"], [2]uint16{set.cmap['K'], set.cmap['玉']}; got != want {
		t.Errorf("ss01 = %v, want %v (王 -> 玉)", got, want)
	}
	// ss02: 馬 -> 左馬。⚠️ 置換元は "+B" ではなく**馬のグリフ**。
	// リガチャ(lookup 0)が先に +B を馬にするので、そこを起点にしないと効かない。
	if got, want := sub["ss02"], [2]uint16{set.cmap['馬'], set.cmap[leftHorse]}; got != want {
		t.Errorf("ss02 = %v, want %v (馬 -> 左馬)", got, want)
	}

	// 各 feature は別タグ。まとめると片方だけ切り替えられなくなる。
	if sub["ss01"][0] == sub["ss02"][0] {
		t.Error("玉と左馬の置換元が同じになっている")
	}
}

// 生成した TTF を読み直して、意図した文字が引けることを確認する。
func TestBuildFontRoundTrip(t *testing.T) {
	set, err := buildGlyphSet(fakeSource, testUPEM)
	if err != nil {
		t.Fatal(err)
	}
	out := buildFont(set, fontInfo{
		upem: testUPEM, ascent: 880, descent: 120, capHeight: 700, xHeight: 500,
	})

	f, err := sfnt.Parse(out)
	if err != nil {
		t.Fatalf("生成した TTF を再パースできない: %v", err)
	}
	if int(f.UnitsPerEm()) != testUPEM {
		t.Errorf("unitsPerEm = %d, want %d", f.UnitsPerEm(), testUPEM)
	}

	var buf sfnt.Buffer
	ppem := fixed.Int26_6(testUPEM << 6)
	index := func(r rune) sfnt.GlyphIndex {
		t.Helper()
		gi, err := f.GlyphIndex(&buf, r)
		if err != nil {
			t.Fatalf("%q: %v", r, err)
		}
		if gi == 0 {
			t.Fatalf("%q が .notdef に落ちた", r)
		}
		return gi
	}

	// SFEN 文字・漢字・異体字・反転字が全て引けること。
	runes := []rune{' ', '+'}
	for _, p := range basePieces {
		runes = append(runes, p.letter, p.letter+32, p.kanji)
	}
	for _, p := range promotedPieces {
		runes = append(runes, p.kanji)
	}
	for _, p := range altPieces {
		runes = append(runes, p.kanji)
	}
	for _, p := range mirroredPieces {
		runes = append(runes, p.code)
	}
	for _, r := range runes {
		gi := index(r)
		if int(gi) != int(set.cmap[r]) {
			t.Errorf("%q の GID = %d, want %d", r, gi, set.cmap[r])
		}
		adv, err := f.GlyphAdvance(&buf, gi, ppem, font.HintingNone)
		if err != nil {
			t.Errorf("%q の送り幅を読めない: %v", r, err)
			continue
		}
		if want := set.glyphs[gi].adv; round26_6(adv) != want {
			t.Errorf("%q の送り幅 = %d, want %d", r, round26_6(adv), want)
		}
	}

	// 左馬は馬と別グリフで、輪郭を持っていること (空グリフを書いていない)。
	if index(leftHorse) == index('馬') {
		t.Error("再パース後に左馬と馬が同じ GID になっている")
	}
	segs, err := f.LoadGlyph(&buf, index(leftHorse), ppem, nil)
	if err != nil {
		t.Fatalf("左馬の輪郭を読めない: %v", err)
	}
	if len(segs) == 0 {
		t.Error("左馬の輪郭が空")
	}
}

// 左馬の元になる漢字が入力フォントに無い場合は、黙って空グリフにせず失敗させる。
func TestBuildGlyphSetMissingGlyph(t *testing.T) {
	missing := func(r rune) (*glyph, error) {
		if r == '馬' {
			return nil, fmt.Errorf("フォントに %q がありません", r)
		}
		return fakeGlyph(), nil
	}
	if _, err := buildGlyphSet(missing, testUPEM); err == nil {
		t.Error("反転元が無いのにエラーにならなかった")
	}
}

// space と "+" だけは入力フォントに無くても代替を合成して続行する。
func TestBuildGlyphSetSynthesizesSpaceAndPlus(t *testing.T) {
	noSpacePlus := func(r rune) (*glyph, error) {
		if r == ' ' || r == '+' {
			return nil, fmt.Errorf("フォントに %q がありません", r)
		}
		return fakeGlyph(), nil
	}
	set, err := buildGlyphSet(noSpacePlus, testUPEM)
	if err != nil {
		t.Fatal(err)
	}
	if plus := set.glyphs[set.plusGID]; len(plus.contours) == 0 {
		t.Error("+ の代替グリフ (十字) が合成されていない")
	}
	if sp := set.glyphs[set.cmap[' ']]; len(sp.contours) != 0 {
		t.Error("空白に輪郭が入っている")
	}
}
