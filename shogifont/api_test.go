package shogifont

import (
	"slices"
	"strings"
	"testing"

	"golang.org/x/image/font/sfnt"
)

// ⚠️ **ここでも実在の日本語フォントは読まない**（shogifont_test.go の冒頭と同じ理由）。
// 代わりに、自分で焼いた TTF を入力として食わせる。生成物は駒の字を cmap に
// 全部持っているので、`Faces` / `Build` の入力として成立する。
func bakedFont(t *testing.T) []byte {
	t.Helper()
	set, err := buildGlyphSet(fakeSource, testUPEM)
	if err != nil {
		t.Fatal(err)
	}
	return buildFont(set, fontInfo{
		family: "Baked Test",
		names:  srcNames{copyright: "(c) test", license: "test license"},
		upem:   testUPEM, ascent: 880, descent: 120, capHeight: 700, xHeight: 500,
	})
}

// Required は Build が要求する字と一致していること。
// **食い違うと、一覧では「焼ける」と出ているのに Build が失敗する**
// （UI からは理由の分からない失敗になる）。
func TestRequiredMatchesBuild(t *testing.T) {
	req := Required()
	for _, p := range basePieces {
		if !slices.Contains(req, p.kanji) {
			t.Errorf("基本駒 %q が Required に無い", p.kanji)
		}
	}
	for _, p := range promotedPieces {
		if !slices.Contains(req, p.kanji) {
			t.Errorf("成駒 %q が Required に無い", p.kanji)
		}
	}
	for _, p := range altPieces {
		if !slices.Contains(req, p.kanji) {
			t.Errorf("異体字 %q が Required に無い", p.kanji)
		}
	}
	// 反転字の元は成駒の馬。**別枠で数えないこと**（同じ字を 2 回出すことになる）。
	for _, p := range mirroredPieces {
		if !slices.Contains(req, p.source) {
			t.Errorf("反転元 %q が Required に無い", p.source)
		}
	}
	// 空白と "+" は合成できるので要求しない。
	for _, r := range []rune{' ', '+'} {
		if slices.Contains(req, r) {
			t.Errorf("%q は合成できるので Required に入れない", r)
		}
	}
}

// Faces は書体の素性と権利表記を返し、字が揃っていれば Missing が空になること。
func TestFaces(t *testing.T) {
	faces, err := Faces(bakedFont(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(faces) != 1 {
		t.Fatalf("書体数 = %d, want 1", len(faces))
	}
	f := faces[0]
	if f.Index != 0 {
		t.Errorf("Index = %d, want 0", f.Index)
	}
	if f.Family != "Baked Test" {
		t.Errorf("Family = %q, want %q", f.Family, "Baked Test")
	}
	if len(f.Missing) != 0 {
		t.Errorf("駒の字が揃っているのに Missing = %q", string(f.Missing))
	}
	// 権利表記は引き継いだものがそのまま読めること
	// （**何に由来するか**を UI に出す唯一の材料）。
	if f.Copyright != "(c) test" {
		t.Errorf("Copyright = %q", f.Copyright)
	}
	if f.License != "test license" {
		t.Errorf("License = %q", f.License)
	}
}

// 駒の字を持たないフォントは、Missing に**足りない字を全部**並べること。
// **「焼けない」だけでは、なぜ選べないのか画面から分からない。**
func TestFacesMissing(t *testing.T) {
	// 駒を 1 つも持たないフォントの代わりに、cmap から駒を落としたものを焼く。
	set, err := buildGlyphSet(fakeSource, testUPEM)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range Required() {
		delete(set.cmap, r)
	}
	faces, err := Faces(buildFont(set, fontInfo{
		family: "No Pieces", upem: testUPEM, ascent: 880, descent: 120,
		capHeight: 700, xHeight: 500,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(faces[0].Missing), string(Required()); got != want {
		t.Errorf("Missing = %q, want %q", got, want)
	}
}

// Build は family 名を name テーブルに書き、再パースできる TTF を返すこと。
func TestBuild(t *testing.T) {
	out, err := Build(bakedFont(t), Options{Family: "Rebaked"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := sfnt.Parse(out)
	if err != nil {
		t.Fatalf("焼いた TTF を再パースできない: %v", err)
	}
	var buf sfnt.Buffer
	name, err := f.Name(&buf, sfnt.NameIDFamily)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Rebaked" {
		t.Errorf("family = %q, want %q", name, "Rebaked")
	}
	// 元フォントの権利表記は派生物にも残ること（落とすと由来が判らなくなる）。
	if c, _ := f.Name(&buf, sfnt.NameIDCopyright); c != "(c) test" {
		t.Errorf("copyright が引き継がれていない: %q", c)
	}
	// 駒が引けること（ここが崩れると盤にラテン文字が出る）。
	for _, r := range []rune{'P', 'p', '歩', 'と', '玉', leftHorse} {
		gi, err := f.GlyphIndex(&buf, r)
		if err != nil || gi == 0 {
			t.Errorf("%q を引けない (gi=%d, err=%v)", r, gi, err)
		}
	}
}

// Family を省いたら既定の family 名になること
// （**呼び出し側に既定値を書かせない**）。
func TestBuildDefaultFamily(t *testing.T) {
	out, err := Build(bakedFont(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	f, err := sfnt.Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	var buf sfnt.Buffer
	if name, _ := f.Name(&buf, sfnt.NameIDFamily); name != DefaultFamily {
		t.Errorf("family = %q, want %q", name, DefaultFamily)
	}
}

// 無い書体番号は、何番まであるかが分かる文言で断ること。
func TestBuildBadIndex(t *testing.T) {
	_, err := Build(bakedFont(t), Options{Index: 3})
	if err == nil {
		t.Fatal("無い書体番号なのにエラーにならなかった")
	}
	if !strings.Contains(err.Error(), "1 書体") {
		t.Errorf("何書体あるか分からない文言: %v", err)
	}
}

// フォントでないものを渡したら、読めないと言うこと（panic しない）。
func TestFacesNotAFont(t *testing.T) {
	if _, err := Faces([]byte("これはフォントではない")); err == nil {
		t.Error("フォントでないのにエラーにならなかった")
	}
	if _, err := Build([]byte("これはフォントではない"), Options{}); err == nil {
		t.Error("フォントでないのにエラーにならなかった")
	}
}
