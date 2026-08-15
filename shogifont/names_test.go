package shogifont

import (
	"testing"
	"unicode/utf16"
)

// nameRecord はテスト用の name テーブルの 1 行。
type nameRecord struct {
	pid, esid, lang, id uint16
	s                   string
}

// buildTestNameTable は name テーブルのバイト列を組む。
// **name テーブルを自前で読むのが目的の機能なので、入力も自前で組む**
// （実在フォントを読まない方針は shogifont_test.go と同じ）。
func buildTestNameTable(recs []nameRecord) []byte {
	hdr := &w{}
	hdr.u16(0)
	hdr.u16(len(recs))
	hdr.u16(6 + len(recs)*12)
	str := &w{}
	for _, r := range recs {
		u := utf16.Encode([]rune(r.s))
		hdr.u16(int(r.pid))
		hdr.u16(int(r.esid))
		hdr.u16(int(r.lang))
		hdr.u16(int(r.id))
		hdr.u16(len(u) * 2)
		hdr.u16(len(str.b))
		for _, c := range u {
			str.u16(int(c))
		}
	}
	hdr.bytes(str.b)
	return hdr.b
}

// 言語で引き分けられること。**ここが効かないと、選ばせる一覧が
// 英語名になったり日本語名になったりして揃わない。**
func TestParseNameTableByLanguage(t *testing.T) {
	tbl := buildTestNameTable([]nameRecord{
		// ⚠️ **日本語を先に置いてある。** x/image は並び順で最初のものを返すので、
		// この並びだと英語名が取れない（実在の日本語フォントで実際に起きる）。
		{pidWindows, psidWindowsUCS2, langWindowsJaJP, nameIDFamily, "游明朝"},
		{pidWindows, psidWindowsUCS2, langWindowsEnUS, nameIDFamily, "Yu Mincho"},
		{pidWindows, psidWindowsUCS2, langWindowsJaJP, nameIDSubFamily, "標準"},
		{pidWindows, psidWindowsUCS2, langWindowsEnUS, nameIDSubFamily, "Regular"},
		// Macintosh Roman は日本語を表せないので読まない。
		{1, 0, 0, nameIDFamily, "Mac Name"},
	})
	n := parseNameTable(tbl)
	if n == nil {
		t.Fatal("name テーブルを読めなかった")
	}
	if got := n.pick(nameIDFamily, langWindowsJaJP); got != "游明朝" {
		t.Errorf("日本語名 = %q, want 游明朝", got)
	}
	if got := n.pick(nameIDFamily, langWindowsEnUS); got != "Yu Mincho" {
		t.Errorf("英語名 = %q, want Yu Mincho", got)
	}
	if got := n.pick(nameIDSubFamily, langWindowsJaJP, langWindowsEnUS); got != "標準" {
		t.Errorf("優先順が効いていない: %q", got)
	}
	// 無い言語は空。**英語に勝手に倒さないこと**（倒すと呼び出し側が
	// 「日本語名がある」と誤解する）。
	if got := n.pick(nameIDFamily, 0x0412); got != "" {
		t.Errorf("無い言語で %q が返った", got)
	}
	// 無い nameID も空。
	if got := n.pick(nameIDLicense, langWindowsEnUS, langWindowsJaJP); got != "" {
		t.Errorf("無い nameID で %q が返った", got)
	}
	// Macintosh のレコードは拾わない。
	if got := n.any(nameIDFamily); got != "游明朝" && got != "Yu Mincho" {
		t.Errorf("Windows/UCS-2 以外を拾った: %q", got)
	}
}

// 壊れた入力で panic しないこと（端末には壊れたフォントも入っている）。
func TestParseNameTableBroken(t *testing.T) {
	tbl := buildTestNameTable([]nameRecord{
		{pidWindows, psidWindowsUCS2, langWindowsJaJP, nameIDFamily, "游明朝"},
	})
	for _, n := range []int{0, 1, 5, 6, 10, len(tbl) - 1} {
		if n < 0 || n > len(tbl) {
			continue
		}
		parseNameTable(tbl[:n]) // panic しなければよい
	}
	if parseNameTable(nil) != nil {
		t.Error("空の入力で nil 以外が返った")
	}
}

// TTC のヘッダから書体ごとのテーブルディレクトリを引けること。
func TestFontOffset(t *testing.T) {
	// 単独フォント: 先頭が "ttcf" でなければ 0 番だけが有効。
	single := make([]byte, 64)
	copy(single, []byte{0, 1, 0, 0})
	if off, ok := fontOffset(single, 0); !ok || off != 0 {
		t.Errorf("単独フォント: off=%d ok=%v", off, ok)
	}
	if _, ok := fontOffset(single, 1); ok {
		t.Error("単独フォントで 1 番目が引けた")
	}

	ttc := &w{}
	ttc.tag("ttcf")
	ttc.u32(0x00020000)
	ttc.u32(2)
	ttc.u32(100)
	ttc.u32(200)
	for _, tc := range []struct {
		index int
		want  uint32
		ok    bool
	}{{0, 100, true}, {1, 200, true}, {2, 0, false}, {-1, 0, false}} {
		off, ok := fontOffset(ttc.b, tc.index)
		if ok != tc.ok || (ok && off != tc.want) {
			t.Errorf("index %d: off=%d ok=%v, want off=%d ok=%v", tc.index, off, ok, tc.want, tc.ok)
		}
	}
	if _, ok := fontOffset([]byte("ttcf"), 0); ok {
		t.Error("短すぎる TTC ヘッダを読めてしまった")
	}
}

// 焼いた TTF から日本語名を読もうとしても、無いものは空で返ること
// （生成器は en-US しか書かない）。
func TestReadNamesOnBakedFont(t *testing.T) {
	n := readNames(bakedFont(t), 0)
	if n == nil {
		t.Fatal("焼いた TTF の name テーブルを読めなかった")
	}
	if got := n.pick(nameIDFamily, langWindowsEnUS); got != "Baked Test" {
		t.Errorf("family = %q, want Baked Test", got)
	}
	if got := n.pick(nameIDFamily, langWindowsJaJP); got != "" {
		t.Errorf("日本語名が無いのに %q が返った", got)
	}
}
