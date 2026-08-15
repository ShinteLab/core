package shogifont

import (
	"unicode/utf16"
)

// name テーブルから言語を選んで名前を読む。
//
// ⚠️ **`x/image/font/sfnt` の `Font.Name` では言語を選べない。** あれは name テーブルに
// 並んでいる順で最初に当たったものを返すので、**英語とは限らないし日本語とも限らない**
// (同じ日本語フォントでも、游明朝は "Yu Mincho"、玉ねぎ楷書は
// "Tamanegi Kaisho Geki FreeVer 7" が返る、という具合に並び順次第)。
//
// **選ばせる UI は日本語の名前で並べたい**ので、ここだけ自前で読む。
// 読むのは名前の文字列だけで、グリフには一切触らない。

const (
	pidWindows       = 3
	psidWindowsUCS2  = 1
	langWindowsEnUS  = 0x0409
	langWindowsJaJP  = 0x0411
	nameIDFamily     = 1
	nameIDSubFamily  = 2
	nameIDFullName   = 4
	nameIDCopyright  = 0
	nameIDTrademark  = 7
	nameIDLicense    = 13
	nameIDLicenseURL = 14
)

// localizedNames は 1 書体分の name テーブルを、言語ごとに引ける形で持つ。
type localizedNames struct {
	// byLang[languageID][nameID] = 文字列
	byLang map[uint16]map[uint16]string
}

// pick は nameID を「ja-JP → en-US → その他の順」で引く。
func (n *localizedNames) pick(id uint16, langs ...uint16) string {
	if n == nil {
		return ""
	}
	for _, lang := range langs {
		if m := n.byLang[lang]; m != nil {
			if s := m[id]; s != "" {
				return s
			}
		}
	}
	return ""
}

// any は言語を問わず最初に見つかったものを返す (最後の逃げ道)。
func (n *localizedNames) any(id uint16) string {
	if n == nil {
		return ""
	}
	for _, m := range n.byLang {
		if s := m[id]; s != "" {
			return s
		}
	}
	return ""
}

// readNames は src の index 番目の書体の name テーブルを読む。
//
// **読めなければ nil を返すだけでエラーにしない。** 名前が読めないことは
// フォントが使えないことを意味しない (呼び出し側は x/image 側の答えに落ちる)。
func readNames(src []byte, index int) *localizedNames {
	off, ok := fontOffset(src, index)
	if !ok {
		return nil
	}
	tbl, ok := findTable(src, off, "name")
	if !ok {
		return nil
	}
	return parseNameTable(tbl)
}

// fontOffset は index 番目の書体のテーブルディレクトリの位置を返す。
// TTC (先頭 4 バイトが "ttcf") ならヘッダの表から引く。
func fontOffset(src []byte, index int) (uint32, bool) {
	if len(src) < 12 || index < 0 {
		return 0, false
	}
	if string(src[:4]) != "ttcf" {
		if index != 0 {
			return 0, false
		}
		return 0, true
	}
	n := int(be32(src[8:]))
	if index >= n {
		return 0, false
	}
	at := 12 + index*4
	if len(src) < at+4 {
		return 0, false
	}
	return be32(src[at:]), true
}

// findTable はテーブルディレクトリから tag のテーブルの中身を切り出す。
func findTable(src []byte, dirOffset uint32, tag string) ([]byte, bool) {
	d := int(dirOffset)
	if d < 0 || len(src) < d+12 {
		return nil, false
	}
	num := int(be16(src[d+4:]))
	for i := 0; i < num; i++ {
		e := d + 12 + i*16
		if len(src) < e+16 {
			return nil, false
		}
		if string(src[e:e+4]) != tag {
			continue
		}
		off, length := int(be32(src[e+8:])), int(be32(src[e+12:]))
		if off < 0 || length < 0 || len(src) < off+length {
			return nil, false
		}
		return src[off : off+length], true
	}
	return nil, false
}

// parseNameTable は name テーブル (format 0/1 共通の部分) を読む。
//
// ⚠️ **Windows / UCS-2 のレコードだけを読む。** Macintosh Roman は
// 日本語を表せないので、選ばせる名前の材料にならない。
func parseNameTable(tbl []byte) *localizedNames {
	const headerSize, entrySize = 6, 12
	if len(tbl) < headerSize {
		return nil
	}
	count := int(be16(tbl[2:]))
	strOff := int(be16(tbl[4:]))
	if len(tbl) < headerSize+entrySize*count {
		return nil
	}
	out := &localizedNames{byLang: map[uint16]map[uint16]string{}}
	for i := 0; i < count; i++ {
		e := headerSize + entrySize*i
		pid, esid := be16(tbl[e:]), be16(tbl[e+2:])
		if pid != pidWindows || esid != psidWindowsUCS2 {
			continue
		}
		lang, id := be16(tbl[e+4:]), be16(tbl[e+6:])
		length, off := int(be16(tbl[e+8:])), int(be16(tbl[e+10:]))
		at := strOff + off
		if length <= 0 || at < 0 || len(tbl) < at+length || length%2 != 0 {
			continue
		}
		u := make([]uint16, 0, length/2)
		for j := 0; j < length; j += 2 {
			u = append(u, be16(tbl[at+j:]))
		}
		s := trimSpace(string(utf16.Decode(u)))
		if s == "" {
			continue
		}
		m := out.byLang[lang]
		if m == nil {
			m = map[uint16]string{}
			out.byLang[lang] = m
		}
		// **先勝ち。** 同じ言語に同じ nameID が 2 つあることは普通は無いが、
		// あったときに後ろで上書きする理由も無い。
		if _, dup := m[id]; !dup {
			m[id] = s
		}
	}
	if len(out.byLang) == 0 {
		return nil
	}
	return out
}

func be16(b []byte) uint16 { return uint16(b[0])<<8 | uint16(b[1]) }

func be32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// trimSpace は前後の空白と、名前によく紛れ込む NUL を落とす。
func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r' || s[i] == 0) {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n' || s[j-1] == '\r' || s[j-1] == 0) {
		j--
	}
	return s[i:j]
}
