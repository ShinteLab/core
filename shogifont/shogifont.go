// Package shogifont は日本語フォントから将棋駒の文字だけを抜き出し、
// SFEN 表記 (P,L,N,S,G,B,R,K / 小文字 / +付き成駒) で表示できる
// 軽量 TTF フォントを焼く。
//
// 生成フォントでは <text>P</text> で「歩」が表示される。
// 成駒 (+P など) は GSUB リガチャで「と」等のグリフに置換される。
//
// ⚠️ **生成物は元フォントの字形をそのまま持つ派生物。** 元フォントの権利表記は
// name テーブルに引き継ぐが (readSrcNames)、**引き継いだからといって再配布して
// よいわけではない。** 何を入力にしてよいかは呼び出し側の責任で、例えば ikkyoku は
// 「端末に入っているフォントから、その端末で使うぶんだけ焼く」(再配布しない)
// という前提で Build を呼んでいる。core が同梱するフォント (web/font*.js) の側は、
// 派生物の再配布を認めるライセンスのものだけを焼いている (web/README.md)。
package shogifont

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// DefaultFamily は生成フォントの既定の family 名 (CSS の font-family に出る名前)。
const DefaultFamily = "ShogiSFEN"

// Options は Build の指定。
type Options struct {
	// Family は生成フォントの family 名。空なら DefaultFamily。
	//
	// ⚠️ **元フォントを変えて焼くときは必ず変えること。** 同名で複数登録すると
	// どれが当たるかブラウザ任せになり、切り替えが効かなくなる。
	Family string

	// Index はコレクション (TTC) の中の何番目の書体を使うか。
	// 単独フォント (TTF / OTF) では 0 だけが有効。
	//
	// **書体ごとに別物として扱えること自体が要る** —— msmincho.ttc のように
	// 1 ファイルに MS 明朝と MS P明朝が同居しているものがあり、
	// 先頭決め打ちでは片方を選べない。
	Index int
}

// Face は入力フォントに入っている書体 1 つ分の素性。
//
// **Missing が空かどうかが「焼けるか」の判定そのもの。** 端末に入っている
// フォントの大半は駒の字を持たないので、選ばせる UI はこれを見て印を付ける。
type Face struct {
	Index     int    // コレクション内の位置 (Options.Index にそのまま渡せる)
	Family    string // name ID 1
	SubFamily string // 2 (Regular / Bold など)
	Full      string // 4

	// 元フォントの権利表記。⚠️ **空でも「制約が無い」ではない**
	// (name テーブルに条項を持たないフォントは珍しくない)。
	Copyright  string
	Trademark  string
	License    string
	LicenseURL string

	// Missing は Required のうちこのフォントに無い字。空なら Build できる。
	Missing []rune
}

// Required は駒を描くのに要る字を返す。
//
// **公開してあるのは、選ばせる UI が「何が足りないか」を出せるようにするため。**
// 空白と "+" は無くても合成する (synthPlus) ので入っていない。
func Required() []rune {
	rs := make([]rune, 0, len(basePieces)+len(promotedPieces)+len(altPieces))
	for _, p := range basePieces {
		rs = append(rs, p.kanji)
	}
	for _, p := range promotedPieces {
		rs = append(rs, p.kanji)
	}
	for _, p := range altPieces {
		rs = append(rs, p.kanji)
	}
	// 反転字 (左馬) の元は成駒の「馬」なので、上で既に入っている。
	return rs
}

// Faces は入力フォント (TTF / OTF / TTC) に入っている書体を列挙する。
//
// **Build する前に呼べることが要点。** 焼いてみて初めて「字が無い」と分かるのでは、
// 一覧に印を付けられない。
func Faces(src []byte) ([]Face, error) {
	coll, err := sfnt.ParseCollection(src)
	if err != nil {
		return nil, fmt.Errorf("フォントを読めません: %w", err)
	}
	req := Required()
	faces := make([]Face, 0, coll.NumFonts())
	for i := 0; i < coll.NumFonts(); i++ {
		f, err := coll.Font(i)
		if err != nil {
			// **1 つ読めなくても他は返す。** コレクションの一部だけ扱えないことがある。
			continue
		}
		var buf sfnt.Buffer
		get := func(id sfnt.NameID) string {
			s, err := f.Name(&buf, id)
			if err != nil {
				return ""
			}
			return strings.TrimSpace(s)
		}
		face := Face{
			Index:      i,
			Family:     get(sfnt.NameIDFamily),
			SubFamily:  get(sfnt.NameIDSubfamily),
			Full:       get(sfnt.NameIDFull),
			Copyright:  get(sfnt.NameIDCopyright),
			Trademark:  get(sfnt.NameIDTrademark),
			License:    get(sfnt.NameIDLicense),
			LicenseURL: get(sfnt.NameIDLicenseURL),
		}
		for _, r := range req {
			gi, err := f.GlyphIndex(&buf, r)
			if err != nil || gi == 0 {
				face.Missing = append(face.Missing, r)
			}
		}
		faces = append(faces, face)
	}
	if len(faces) == 0 {
		return nil, fmt.Errorf("読める書体がありません")
	}
	return faces, nil
}

// Build は駒フォントを焼いて TTF のバイト列を返す。
func Build(src []byte, opt Options) ([]byte, error) {
	coll, err := sfnt.ParseCollection(src)
	if err != nil {
		return nil, fmt.Errorf("フォントを読めません: %w", err)
	}
	if opt.Index < 0 || opt.Index >= coll.NumFonts() {
		return nil, fmt.Errorf("書体 %d はこのフォントにありません (%d 書体)", opt.Index, coll.NumFonts())
	}
	f, err := coll.Font(opt.Index)
	if err != nil {
		return nil, fmt.Errorf("書体 %d を読めません: %w", opt.Index, err)
	}
	family := opt.Family
	if family == "" {
		family = DefaultFamily
	}

	var buf sfnt.Buffer
	upem := int(f.UnitsPerEm())
	ppem := fixed.Int26_6(upem << 6)

	// グリフ構成: 0=.notdef, 1=space, 2=plus, 3..10=基本駒, 11..16=成駒,
	// 17=異体字(玉), 18=反転(左馬)
	set, err := buildGlyphSet(func(r rune) (*glyph, error) {
		return extract(f, &buf, r, ppem)
	}, upem)
	if err != nil {
		return nil, err
	}

	met, err := f.Metrics(&buf, ppem, font.HintingNone)
	if err != nil {
		return nil, fmt.Errorf("メトリクスを読めません: %w", err)
	}
	return buildFont(set, fontInfo{
		family:    family,
		names:     readSrcNames(f, &buf),
		upem:      upem,
		ascent:    round26_6(met.Ascent),
		descent:   round26_6(met.Descent), // 正の値 (下方向)
		capHeight: round26_6(met.CapHeight),
		xHeight:   round26_6(met.XHeight),
	}), nil
}

// 駒と SFEN 文字の対応 (小文字=後手も同じグリフを割り当てる)
var basePieces = []struct {
	kanji  rune
	letter rune
}{
	{'歩', 'P'}, {'香', 'L'}, {'桂', 'N'}, {'銀', 'S'},
	{'金', 'G'}, {'角', 'B'}, {'飛', 'R'}, {'王', 'K'},
}

// 成駒: "+" + base のリガチャで表示する
var promotedPieces = []struct {
	kanji rune
	base  rune
}{
	{'と', 'P'}, {'杏', 'L'}, {'圭', 'N'}, {'全', 'S'},
	{'馬', 'B'}, {'龍', 'R'},
}

// 異体字。同じ駒を別の字で書きたいときのためにグリフを同梱し、
// その漢字からだけ引けるようにする (SFEN 文字は base のまま王を指す)。
// 「王を使いたくない」向けに玉を出す用途。呼び出し側が 'K' ではなく
// '玉' の1文字を描けばよく、フォントを作り分けなくて済む。
var altPieces = []struct {
	kanji   rune
	base    rune
	feature string // stylistic set のタグ。base の字をこの字に差し替える
}{
	{'玉', 'K', "ss01"},
}

// 左馬。「馬」を左右反転した縁起物の字で、Unicode に符号位置が無いので
// 私用領域 (PUA) に置き、輪郭は馬から合成する。
const leftHorse rune = 0xE000

var mirroredPieces = []struct {
	code    rune   // 割り当てる符号位置
	source  rune   // 反転元の漢字
	feature string // stylistic set のタグ。source の字をこの字に差し替える
}{
	{leftHorse, '馬', "ss02"},
}

// altFeature は GSUB の stylistic set 1 つ分。
//
// **異体字は「別の符号位置を描く」でも「同じ符号位置のまま feature で切り替える」でも
// 出せるようにしてある。** 前者はどの描画経路でも動くが本文の文字が変わり、
// 後者は本文が SFEN のまま (K / +B) で済むがブラウザの HTML/SVG でしか効かない
// (Canvas 2D と Go の x/image/font は GSUB を解釈しない)。**どちらが要るかは
// 利用側の描画経路次第なので、フォントは両方を積んでおいて選ばせる。**
//
// 玉と左馬で別のタグにしてあるのは、片方だけ切り替えたいことがあるため
// (1 つの feature にまとめると連動してしまう)。
type altFeature struct {
	tag  string      // "ss01" など。FeatureList は昇順に並べる決まりなので後で整列する
	subs [][2]uint16 // {置換元 GID, 置換先 GID}
}

// glyphSource は 1 文字分のグリフを取り出す。テストが入力フォントを用意せずに
// 差し替えられるよう、buildGlyphSet はこれ越しにしか元フォントを触らない。
type glyphSource func(r rune) (*glyph, error)

// fontSet は buildFont に渡す一式。
type fontSet struct {
	glyphs  []*glyph
	cmap    map[rune]uint16 // 符号位置 -> GID
	ligs    [][2]uint16     // {baseGID, ligatureGID}
	alts    []altFeature    // stylistic set (ss01=玉, ss02=左馬)
	plusGID uint16
}

// buildGlyphSet はグリフ表・cmap・リガチャ表を組み立てる。
// グリフの並びは 0=.notdef, 1=space, 2=plus, 基本駒, 成駒, 異体字, 反転字 の順。
func buildGlyphSet(load glyphSource, upem int) (*fontSet, error) {
	glyphs := make([]*glyph, 0, 3+len(basePieces)+len(promotedPieces)+len(altPieces)+len(mirroredPieces))
	glyphs = append(glyphs, &glyph{adv: 0}) // .notdef

	sp, err := load(' ')
	if err != nil {
		sp = &glyph{adv: upem / 4}
	}
	sp.contours = nil // 空白は輪郭不要
	glyphs = append(glyphs, sp)

	plus, err := load('+')
	if err != nil {
		plus = synthPlus(upem)
	}
	glyphs = append(glyphs, plus)

	const gidPlus = 2
	gidBase := map[rune]uint16{}
	for _, p := range basePieces {
		g, err := load(p.kanji)
		if err != nil {
			return nil, fmt.Errorf("駒 %q のグリフを抽出できません: %w", p.kanji, err)
		}
		gidBase[p.letter] = uint16(len(glyphs))
		glyphs = append(glyphs, g)
	}
	gidPromoted := map[rune]uint16{}
	gidKanji := map[rune]uint16{} // 漢字 -> GID (stylistic set の置換元を引くため)
	for _, p := range basePieces {
		gidKanji[p.kanji] = gidBase[p.letter]
	}
	for _, p := range promotedPieces {
		g, err := load(p.kanji)
		if err != nil {
			return nil, fmt.Errorf("成駒 %q のグリフを抽出できません: %w", p.kanji, err)
		}
		gidPromoted[p.base] = uint16(len(glyphs))
		gidKanji[p.kanji] = uint16(len(glyphs))
		glyphs = append(glyphs, g)
	}
	gidAlt := map[rune]uint16{}
	for _, p := range altPieces {
		g, err := load(p.kanji)
		if err != nil {
			return nil, fmt.Errorf("異体字 %q のグリフを抽出できません: %w", p.kanji, err)
		}
		gidAlt[p.kanji] = uint16(len(glyphs))
		glyphs = append(glyphs, g)
	}
	for _, p := range mirroredPieces {
		g, err := load(p.source)
		if err != nil {
			return nil, fmt.Errorf("反転元 %q のグリフを抽出できません: %w", p.source, err)
		}
		gidAlt[p.code] = uint16(len(glyphs))
		glyphs = append(glyphs, g.mirrored())
	}

	// cmap: SFEN 文字 (大文字/小文字とも同じグリフ) と元の漢字も引けるようにする
	cmapMap := map[rune]uint16{' ': 1, '+': gidPlus}
	for _, p := range basePieces {
		gid := gidBase[p.letter]
		cmapMap[p.letter] = gid
		cmapMap[p.letter+32] = gid // 小文字
		cmapMap[p.kanji] = gid
	}
	for _, p := range promotedPieces {
		cmapMap[p.kanji] = gidPromoted[p.base]
	}
	for _, p := range altPieces {
		cmapMap[p.kanji] = gidAlt[p.kanji]
	}
	for _, p := range mirroredPieces {
		cmapMap[p.code] = gidAlt[p.code]
	}

	// GSUB リガチャ: plus + base -> promoted
	var ligs [][2]uint16
	for _, p := range promotedPieces {
		ligs = append(ligs, [2]uint16{gidBase[p.base], gidPromoted[p.base]})
	}

	// GSUB stylistic set: 同じ文字のまま字を差し替える経路
	byTag := map[string]int{}
	var alts []altFeature
	add := func(tag string, from, to uint16) {
		i, ok := byTag[tag]
		if !ok {
			i = len(alts)
			byTag[tag] = i
			alts = append(alts, altFeature{tag: tag})
		}
		alts[i].subs = append(alts[i].subs, [2]uint16{from, to})
	}
	for _, p := range altPieces {
		add(p.feature, gidBase[p.base], gidAlt[p.kanji])
	}
	for _, p := range mirroredPieces {
		// リガチャ (+B -> 馬) は lookup 0 で先に適用されるので、置換元は馬のグリフ。
		add(p.feature, gidKanji[p.source], gidAlt[p.code])
	}
	sort.Slice(alts, func(i, j int) bool { return alts[i].tag < alts[j].tag })

	return &fontSet{glyphs: glyphs, cmap: cmapMap, ligs: ligs, alts: alts, plusGID: gidPlus}, nil
}

// ---- グリフ抽出 ----

type point struct {
	x, y int
	on   bool
}

type glyph struct {
	contours               [][]point
	adv                    int
	xmin, ymin, xmax, ymax int
}

func round26_6(v fixed.Int26_6) int {
	return int(math.Round(float64(v) / 64))
}

func extract(f *sfnt.Font, buf *sfnt.Buffer, r rune, ppem fixed.Int26_6) (*glyph, error) {
	gi, err := f.GlyphIndex(buf, r)
	if err != nil {
		return nil, err
	}
	if gi == 0 {
		return nil, fmt.Errorf("フォントに %q がありません", r)
	}
	segs, err := f.LoadGlyph(buf, gi, ppem, nil)
	if err != nil {
		return nil, err
	}
	adv, err := f.GlyphAdvance(buf, gi, ppem, font.HintingNone)
	if err != nil {
		return nil, err
	}
	g := segmentsToGlyph(segs)
	g.adv = round26_6(adv)
	return g, nil
}

// LoadGlyph は y 軸下向きの座標を返すので、フォント単位 (y 上向き) に変換する
func segmentsToGlyph(segs sfnt.Segments) *glyph {
	g := &glyph{}
	var cur []point
	var cx, cy float64
	flush := func() {
		if len(cur) > 1 {
			f0, l := cur[0], cur[len(cur)-1]
			if f0.x == l.x && f0.y == l.y && f0.on && l.on {
				cur = cur[:len(cur)-1] // 始点と重なる終点は捨てる
			}
		}
		if len(cur) >= 2 {
			g.contours = append(g.contours, cur)
		}
		cur = nil
	}
	pt := func(x, y float64, on bool) {
		cur = append(cur, point{int(math.Round(x)), int(math.Round(y)), on})
		cx, cy = x, y
	}
	cv := func(p fixed.Point26_6) (float64, float64) {
		return float64(p.X) / 64, -float64(p.Y) / 64
	}
	for _, s := range segs {
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			flush()
			x, y := cv(s.Args[0])
			pt(x, y, true)
		case sfnt.SegmentOpLineTo:
			x, y := cv(s.Args[0])
			pt(x, y, true)
		case sfnt.SegmentOpQuadTo:
			qx, qy := cv(s.Args[0])
			x, y := cv(s.Args[1])
			pt(qx, qy, false)
			pt(x, y, true)
		case sfnt.SegmentOpCubeTo:
			// TrueType glyf は 2 次曲線のみなので 3 次曲線を近似変換する
			c1x, c1y := cv(s.Args[0])
			c2x, c2y := cv(s.Args[1])
			p3x, p3y := cv(s.Args[2])
			for _, q := range cubicToQuads(cx, cy, c1x, c1y, c2x, c2y, p3x, p3y, 2) {
				pt(q[0], q[1], false)
				pt(q[2], q[3], true)
			}
		}
	}
	flush()
	g.computeBounds()
	return g
}

// mirrored は左右反転したグリフを返す (左馬用)。
//
// 反転は**送り幅の中心**で行う。字面の中心で折り返すと左右のサイドベアリングが
// 入れ替わらず、元の字と重ねたときに位置がずれる。
//
// 併せて各輪郭の点順を逆にして巻き方向を戻す。TrueType は nonzero 塗りなので
// 全輪郭が一斉に裏返る反転では塗り自体は変わらないが、
// 「外側は時計回り」という TrueType の約束から外れるとフォント検証で引っかかる。
// 先頭の点は on-curve なので、p0 を残して以降を逆順にする
// (単純に slice を逆にすると off-curve 点が輪郭の先頭に来る)。
func (g *glyph) mirrored() *glyph {
	m := &glyph{adv: g.adv, contours: make([][]point, 0, len(g.contours))}
	for _, c := range g.contours {
		rev := make([]point, 0, len(c))
		flip := func(p point) point {
			p.x = g.adv - p.x
			return p
		}
		rev = append(rev, flip(c[0]))
		for i := len(c) - 1; i >= 1; i-- {
			rev = append(rev, flip(c[i]))
		}
		m.contours = append(m.contours, rev)
	}
	m.computeBounds()
	return m
}

func (g *glyph) computeBounds() {
	first := true
	for _, c := range g.contours {
		for _, p := range c {
			if first {
				g.xmin, g.xmax, g.ymin, g.ymax = p.x, p.x, p.y, p.y
				first = false
				continue
			}
			if p.x < g.xmin {
				g.xmin = p.x
			}
			if p.x > g.xmax {
				g.xmax = p.x
			}
			if p.y < g.ymin {
				g.ymin = p.y
			}
			if p.y > g.ymax {
				g.ymax = p.y
			}
		}
	}
}

// 3 次ベジェを分割して 2 次ベジェ列 {cx,cy,x,y} に近似する
func cubicToQuads(p0x, p0y, c1x, c1y, c2x, c2y, p3x, p3y float64, depth int) [][4]float64 {
	if depth == 0 {
		qx := (3*(c1x+c2x) - p0x - p3x) / 4
		qy := (3*(c1y+c2y) - p0y - p3y) / 4
		return [][4]float64{{qx, qy, p3x, p3y}}
	}
	mid := func(ax, ay, bx, by float64) (float64, float64) {
		return (ax + bx) / 2, (ay + by) / 2
	}
	abx, aby := mid(p0x, p0y, c1x, c1y)
	bcx, bcy := mid(c1x, c1y, c2x, c2y)
	cdx, cdy := mid(c2x, c2y, p3x, p3y)
	abcx, abcy := mid(abx, aby, bcx, bcy)
	bcdx, bcdy := mid(bcx, bcy, cdx, cdy)
	mx, my := mid(abcx, abcy, bcdx, bcdy)
	left := cubicToQuads(p0x, p0y, abx, aby, abcx, abcy, mx, my, depth-1)
	right := cubicToQuads(mx, my, bcdx, bcdy, cdx, cdy, p3x, p3y, depth-1)
	return append(left, right...)
}

// 元フォントに "+" が無い場合の代替グリフ (十字)
func synthPlus(upem int) *glyph {
	t := upem / 16                     // 線の半太さ
	l := upem * 30 / 100               // 腕の半長さ
	cxm, cym := upem*28/100, upem*3/10 // 中心
	rect := func(x0, y0, x1, y1 int) []point {
		return []point{{x0, y0, true}, {x0, y1, true}, {x1, y1, true}, {x1, y0, true}}
	}
	g := &glyph{
		contours: [][]point{
			rect(cxm-l, cym-t, cxm+l, cym+t),
			rect(cxm-t, cym-l, cxm+t, cym+l),
		},
		adv: upem * 56 / 100,
	}
	g.computeBounds()
	return g
}

// ---- TTF 出力 ----

// srcNames は元フォントから引き継ぐ権利表記。
//
// ⚠️ **生成物は元フォントの字形をそのまま持つ派生物。** 表記を落とすと、何に由来し
// どういう条件で使えるものかがバイナリから判らなくなる。**空でも構わないが、
// 「元フォントに書いていなかった」のであって「制約が無い」ではない。**
// 実際、name テーブルにライセンス条項を持たないフォントは珍しくない
// (配布元の規約を別に確認すること)。
type srcNames struct {
	copyright   string // name ID 0
	trademark   string // 7
	description string // 10 (何から作ったかをここに書く)
	license     string // 13
	licenseURL  string // 14
}

// readSrcNames は元フォントの権利表記を読む。無い項目は空のまま。
func readSrcNames(f *sfnt.Font, buf *sfnt.Buffer) srcNames {
	get := func(id sfnt.NameID) string {
		s, err := f.Name(buf, id)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(s)
	}
	src := srcNames{
		copyright:  get(sfnt.NameIDCopyright),
		trademark:  get(sfnt.NameIDTrademark),
		license:    get(sfnt.NameIDLicense),
		licenseURL: get(sfnt.NameIDLicenseURL),
	}
	if name := get(sfnt.NameIDFamily); name != "" {
		src.description = "将棋の駒 19 文字だけを抜き出した派生フォント。元フォント: " + name
		if v := get(sfnt.NameIDManufacturer); v != "" {
			src.description += " (" + v + ")"
		}
	}
	return src
}

type fontInfo struct {
	family    string // CSS の font-family に出す名前
	names     srcNames
	upem      int
	ascent    int
	descent   int
	capHeight int
	xHeight   int
}

// ビッグエンディアンのバイト列ビルダ
type w struct{ b []byte }

func (x *w) u8(v int)  { x.b = append(x.b, byte(v)) }
func (x *w) u16(v int) { x.b = append(x.b, byte(v>>8), byte(v)) }
func (x *w) s16(v int) { x.u16(v & 0xFFFF) }
func (x *w) u32(v uint32) {
	x.b = append(x.b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}
func (x *w) tag(s string)   { x.b = append(x.b, s...) }
func (x *w) bytes(b []byte) { x.b = append(x.b, b...) }
func (x *w) pad4() {
	for len(x.b)%4 != 0 {
		x.u8(0)
	}
}

func buildFont(set *fontSet, info fontInfo) []byte {
	glyphs, cmapMap := set.glyphs, set.cmap
	numGlyphs := len(glyphs)

	// glyf / loca
	glyf := &w{}
	loca := &w{}
	maxPoints, maxContours := 0, 0
	for _, g := range glyphs {
		loca.u32(uint32(len(glyf.b)))
		glyf.bytes(encodeGlyph(g))
		np := 0
		for _, c := range g.contours {
			np += len(c)
		}
		if np > maxPoints {
			maxPoints = np
		}
		if len(g.contours) > maxContours {
			maxContours = len(g.contours)
		}
	}
	loca.u32(uint32(len(glyf.b)))
	glyf.pad4()

	// 全体のバウンディングボックスと水平メトリクス
	xmin, ymin, xmax, ymax := 0, 0, 0, 0
	advMax, minLSB, minRSB, xMaxExt := 0, 0, 0, 0
	firstBox := true
	advSum := 0
	for _, g := range glyphs {
		advSum += g.adv
		if g.adv > advMax {
			advMax = g.adv
		}
		if len(g.contours) == 0 {
			continue
		}
		if firstBox {
			xmin, ymin, xmax, ymax = g.xmin, g.ymin, g.xmax, g.ymax
			minLSB, minRSB, xMaxExt = g.xmin, g.adv-g.xmax, g.xmax
			firstBox = false
		}
		if g.xmin < xmin {
			xmin = g.xmin
		}
		if g.ymin < ymin {
			ymin = g.ymin
		}
		if g.xmax > xmax {
			xmax = g.xmax
		}
		if g.ymax > ymax {
			ymax = g.ymax
		}
		if g.xmin < minLSB {
			minLSB = g.xmin
		}
		if g.adv-g.xmax < minRSB {
			minRSB = g.adv - g.xmax
		}
		if g.xmax > xMaxExt {
			xMaxExt = g.xmax
		}
	}

	hmtx := &w{}
	for _, g := range glyphs {
		hmtx.u16(g.adv)
		if len(g.contours) == 0 {
			hmtx.s16(0)
		} else {
			hmtx.s16(g.xmin)
		}
	}

	// head
	head := &w{}
	head.u32(0x00010000)
	head.u32(0x00010000) // fontRevision 1.0
	head.u32(0)          // checkSumAdjustment (後で設定)
	head.u32(0x5F0F3CF5) // magic
	head.u16(0x0003)     // flags: baseline y=0, lsb=xMin
	head.u16(info.upem)
	now := uint64(time.Now().Unix() + 2082844800) // 1904 年起点
	head.u32(uint32(now >> 32))
	head.u32(uint32(now))
	head.u32(uint32(now >> 32))
	head.u32(uint32(now))
	head.s16(xmin)
	head.s16(ymin)
	head.s16(xmax)
	head.s16(ymax)
	head.u16(0) // macStyle
	head.u16(8) // lowestRecPPEM
	head.s16(2) // fontDirectionHint
	head.s16(1) // indexToLocFormat: long
	head.s16(0) // glyphDataFormat

	// hhea
	hhea := &w{}
	hhea.u32(0x00010000)
	hhea.s16(info.ascent)
	hhea.s16(-info.descent)
	hhea.s16(0) // lineGap
	hhea.u16(advMax)
	hhea.s16(minLSB)
	hhea.s16(minRSB)
	hhea.s16(xMaxExt)
	hhea.s16(1) // caretSlopeRise
	hhea.s16(0) // caretSlopeRun
	hhea.s16(0) // caretOffset
	hhea.s16(0)
	hhea.s16(0)
	hhea.s16(0)
	hhea.s16(0)
	hhea.s16(0) // metricDataFormat
	hhea.u16(numGlyphs)

	// maxp
	maxp := &w{}
	maxp.u32(0x00010000)
	maxp.u16(numGlyphs)
	maxp.u16(maxPoints)
	maxp.u16(maxContours)
	maxp.u16(0) // maxCompositePoints
	maxp.u16(0) // maxCompositeContours
	maxp.u16(2) // maxZones
	maxp.u16(0) // maxTwilightPoints
	maxp.u16(0) // maxStorage
	maxp.u16(0) // maxFunctionDefs
	maxp.u16(0) // maxInstructionDefs
	maxp.u16(0) // maxStackElements
	maxp.u16(0) // maxSizeOfInstructions
	maxp.u16(0) // maxComponentElements
	maxp.u16(0) // maxComponentDepth

	// OS/2 (version 4)
	firstChar, lastChar := 0x10FFFF, 0
	for r := range cmapMap {
		if int(r) < firstChar {
			firstChar = int(r)
		}
		if int(r) > lastChar {
			lastChar = int(r)
		}
	}
	os2 := &w{}
	os2.u16(4)
	os2.s16(advSum / numGlyphs) // xAvgCharWidth
	os2.u16(400)                // usWeightClass
	os2.u16(5)                  // usWidthClass
	os2.u16(0)                  // fsType
	sub := info.upem * 65 / 100
	os2.s16(sub)
	os2.s16(sub)
	os2.s16(0)
	os2.s16(info.upem * 14 / 100)
	os2.s16(sub)
	os2.s16(sub)
	os2.s16(0)
	os2.s16(info.upem * 48 / 100)
	os2.s16(info.upem * 5 / 100)  // yStrikeoutSize
	os2.s16(info.upem * 26 / 100) // yStrikeoutPosition
	os2.s16(0)                    // sFamilyClass
	for i := 0; i < 10; i++ {     // panose
		os2.u8(0)
	}
	os2.u32(1)                     // ulUnicodeRange1: Basic Latin
	os2.u32((1 << 27) | (1 << 17)) // CJK 統合漢字 / ひらがな
	os2.u32(0)
	os2.u32(0)
	os2.tag("SHGI") // achVendID
	os2.u16(0x0040) // fsSelection: REGULAR
	os2.u16(firstChar)
	os2.u16(lastChar)
	os2.s16(info.ascent)
	os2.s16(-info.descent)
	os2.s16(0) // sTypoLineGap
	os2.u16(info.ascent)
	os2.u16(info.descent)
	os2.u32(1 | (1 << 17)) // ulCodePageRange1: Latin1 + Shift-JIS
	os2.u32(0)
	os2.s16(info.xHeight)
	os2.s16(info.capHeight)
	os2.u16(0)    // usDefaultChar
	os2.u16(0x20) // usBreakChar
	os2.u16(2)    // usMaxContext (リガチャ 2 文字)

	// post (version 3: グリフ名なし)
	post := &w{}
	post.u32(0x00030000)
	post.u32(0)                     // italicAngle
	post.s16(-info.upem * 10 / 100) // underlinePosition
	post.s16(info.upem * 5 / 100)   // underlineThickness
	post.u32(0)                     // isFixedPitch
	post.u32(0)
	post.u32(0)
	post.u32(0)
	post.u32(0)

	tables := []table{
		{"cmap", buildCmap(cmapMap)},
		{"glyf", glyf.b},
		{"head", head.b},
		{"hhea", hhea.b},
		{"hmtx", hmtx.b},
		{"loca", loca.b},
		{"maxp", maxp.b},
		{"name", buildName(info.family, info.names)},
		{"post", post.b},
		{"OS/2", os2.b},
		{"GSUB", buildGSUB(set.plusGID, set.ligs, set.alts)},
	}
	return assemble(tables)
}

// 15 バイトのヘッダを持つ単純グリフとしてエンコードする (空グリフは長さ 0)
func encodeGlyph(g *glyph) []byte {
	if len(g.contours) == 0 {
		return nil
	}
	b := &w{}
	b.s16(len(g.contours))
	b.s16(g.xmin)
	b.s16(g.ymin)
	b.s16(g.xmax)
	b.s16(g.ymax)
	total := 0
	for _, c := range g.contours {
		total += len(c)
		b.u16(total - 1)
	}
	b.u16(0) // instructionLength
	for _, c := range g.contours {
		for _, p := range c {
			if p.on {
				b.u8(1)
			} else {
				b.u8(0)
			}
		}
	}
	prev := 0
	for _, c := range g.contours {
		for _, p := range c {
			b.s16(p.x - prev)
			prev = p.x
		}
	}
	prev = 0
	for _, c := range g.contours {
		for _, p := range c {
			b.s16(p.y - prev)
			prev = p.y
		}
	}
	b.pad4()
	return b.b
}

// cmap format 4 (BMP のみ)
func buildCmap(m map[rune]uint16) []byte {
	var chars []int
	for r := range m {
		chars = append(chars, int(r))
	}
	sort.Ints(chars)

	type seg struct{ start, end, gid int }
	var segs []seg
	for _, c := range chars {
		g := int(m[rune(c)])
		if n := len(segs); n > 0 && c == segs[n-1].end+1 && g == segs[n-1].gid+(c-segs[n-1].start) {
			segs[n-1].end = c
			continue
		}
		segs = append(segs, seg{c, c, g})
	}
	segs = append(segs, seg{0xFFFF, 0xFFFF, 0}) // 終端 (idDelta=1)

	segCount := len(segs)
	p := 1
	es := 0
	for p*2 <= segCount {
		p *= 2
		es++
	}
	sub := &w{}
	sub.u16(4)
	sub.u16(16 + segCount*8) // length
	sub.u16(0)               // language
	sub.u16(segCount * 2)
	sub.u16(p * 2)
	sub.u16(es)
	sub.u16(segCount*2 - p*2)
	for _, s := range segs {
		sub.u16(s.end)
	}
	sub.u16(0) // reservedPad
	for _, s := range segs {
		sub.u16(s.start)
	}
	for i, s := range segs {
		if i == segCount-1 {
			sub.u16(1)
		} else {
			sub.u16((s.gid - s.start) & 0xFFFF)
		}
	}
	for range segs {
		sub.u16(0) // idRangeOffset
	}

	// ヘッダ: Unicode (0,3) と Windows (3,1) で同じサブテーブルを共有
	c := &w{}
	c.u16(0)
	c.u16(2)
	c.u16(0)
	c.u16(3)
	c.u32(20)
	c.u16(3)
	c.u16(1)
	c.u32(20)
	c.bytes(sub.b)
	return c.b
}

// family は CSS の font-family に出す名前 (既定 "ShogiSFEN")。
// 元フォントを変えて焼いたものは**別の名前にすること。** 同名で複数登録すると
// どれが当たるかブラウザ任せになり、切り替えが効かなくなる。
//
// ⚠️ **元フォントの権利表記 (copyright / trademark / license / licenseURL) を
// そのまま引き継ぐ。** 生成物は元フォントの字形をそのまま持つ派生物なので、
// 表記を落とすと**何に由来するどういう条件のものか判らないバイナリ**になる。
// notice には「何から作ったか」を description(10) に入れる。
func buildName(family string, src srcNames) []byte {
	ps := strings.ReplaceAll(family, " ", "") + "-Regular"
	entries := []struct {
		id int
		s  string
	}{
		{1, family},
		{2, "Regular"},
		{3, ps},
		{4, family},
		{5, "Version 1.0"},
		{6, ps},
	}
	// 元フォント由来の表記。空のものは書かない。
	for _, e := range []struct {
		id int
		s  string
	}{
		{0, src.copyright},
		{7, src.trademark},
		{10, src.description},
		{13, src.license},
		{14, src.licenseURL},
	} {
		if e.s != "" {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	n := &w{}
	n.u16(0)
	n.u16(len(entries))
	n.u16(6 + len(entries)*12)
	str := &w{}
	for _, e := range entries {
		// ⚠️ **バイト数ではなく UTF-16 の符号単位で数えること。** 権利表記には
		// "©" や日本語が入るので、len(string) で数えると長さが合わず name が壊れる。
		u := utf16.Encode([]rune(e.s))
		n.u16(3)      // Windows
		n.u16(1)      // Unicode BMP
		n.u16(0x0409) // en-US
		n.u16(e.id)
		n.u16(len(u) * 2)
		n.u16(len(str.b))
		for _, c := range u {
			str.u16(int(c))
		}
	}
	n.bytes(str.b)
	return n.b
}

// GSUB を組み立てる。
//
//	lookup 0        liga/rlig     plus + base -> 成駒 (リガチャ置換)
//	lookup 1..n     ss01/ss02…    王 -> 玉 / 馬 -> 左馬 (単一置換)
//
// ⚠️ **リガチャの lookup を先頭に置くこと。** 適用は lookup の並び順なので、
// 先に `+B` を馬にしてからでないと ss02 (馬 -> 左馬) の置換元が現れない。
func buildGSUB(plusGID uint16, ligs [][2]uint16, alts []altFeature) []byte {
	// Feature: liga と rlig は同じ Feature テーブル (lookup 0) を共有する。
	// FeatureList は tag の昇順に並べる決まり。liga < rlig < ss01 < ss02 なので
	// リガチャ 2 つのあとに alts (呼び出し側で整列済み) を並べれば順序は満たす。
	type featureRec struct {
		tag    string
		lookup int
	}
	feats := []featureRec{{"liga", 0}, {"rlig", 0}}
	for i, a := range alts {
		feats = append(feats, featureRec{a.tag, 1 + i})
	}

	// ScriptList: DFLT / latn が同じ Script テーブルを共有
	sl := &w{}
	sl.u16(2)
	sl.tag("DFLT")
	sl.u16(14)
	sl.tag("latn")
	sl.u16(14)
	sl.u16(4) // defaultLangSys (Script 先頭からのオフセット)
	sl.u16(0) // langSysCount
	sl.u16(0) // lookupOrder
	sl.u16(0xFFFF)
	sl.u16(len(feats)) // featureIndexCount
	for i := range feats {
		sl.u16(i)
	}

	// FeatureList。Feature テーブル本体は FeatureRecord の後ろに並べる。
	// liga と rlig は同じテーブルを指すので、実体は len(feats)-1 個。
	fl := &w{}
	fl.u16(len(feats))
	recEnd := 2 + len(feats)*6
	tableOff := func(i int) int {
		if i == 0 || i == 1 { // liga/rlig は共有
			return recEnd
		}
		return recEnd + (i-1)*6
	}
	for i, f := range feats {
		fl.tag(f.tag)
		fl.u16(tableOff(i))
	}
	for i, f := range feats {
		if i == 1 {
			continue // rlig は liga のテーブルを共有
		}
		fl.u16(0)        // featureParams
		fl.u16(1)        // lookupIndexCount
		fl.u16(f.lookup) // lookupListIndices[0]
	}

	// LigatureSubst subtable (lookup 0)
	n := len(ligs)
	ligSub := &w{}
	ligSub.u16(1)  // format
	ligSub.u16(8)  // coverageOffset
	ligSub.u16(1)  // ligSetCount
	ligSub.u16(14) // ligatureSetOffsets[0]
	ligSub.u16(1)  // Coverage format 1
	ligSub.u16(1)
	ligSub.u16(int(plusGID))
	ligSub.u16(n) // LigatureSet
	for i := 0; i < n; i++ {
		ligSub.u16(2 + n*2 + i*6)
	}
	for _, lg := range ligs {
		ligSub.u16(int(lg[1])) // ligatureGlyph
		ligSub.u16(2)          // componentCount
		ligSub.u16(int(lg[0])) // 2 文字目
	}

	// Lookup 1 つ分 (subtable は 1 つ) を組む
	lookup := func(typ int, sub []byte) []byte {
		x := &w{}
		x.u16(typ)
		x.u16(0) // lookupFlag
		x.u16(1) // subTableCount
		x.u16(8) // subtableOffset
		x.bytes(sub)
		return x.b
	}

	lookups := [][]byte{lookup(4, ligSub.b)}
	for _, a := range alts {
		// SingleSubst format 2 (置換元と置換先を対で持つ。差分を計算しなくてよい)。
		// Coverage の GID は昇順でなければならず、substituteGlyphIDs はその並びに対応する。
		sort.Slice(a.subs, func(i, j int) bool { return a.subs[i][0] < a.subs[j][0] })
		m := len(a.subs)
		sub := &w{}
		sub.u16(2)       // substFormat
		sub.u16(6 + m*2) // coverageOffset
		sub.u16(m)       // glyphCount
		for _, s := range a.subs {
			sub.u16(int(s[1])) // substituteGlyphIDs
		}
		sub.u16(1) // Coverage format 1
		sub.u16(m)
		for _, s := range a.subs {
			sub.u16(int(s[0]))
		}
		lookups = append(lookups, lookup(1, sub.b))
	}

	// LookupList
	ll := &w{}
	ll.u16(len(lookups))
	off := 2 + len(lookups)*2
	for _, lk := range lookups {
		ll.u16(off)
		off += len(lk)
	}
	for _, lk := range lookups {
		ll.bytes(lk)
	}

	g := &w{}
	g.u32(0x00010000)
	o := 10
	g.u16(o)
	o += len(sl.b)
	g.u16(o)
	o += len(fl.b)
	g.u16(o)
	g.bytes(sl.b)
	g.bytes(fl.b)
	g.bytes(ll.b)
	return g.b
}

// ---- フォントファイル組み立て ----

type table struct {
	tag  string
	data []byte
}

func checksum(b []byte) uint32 {
	var s uint32
	for i := 0; i < len(b); i += 4 {
		var v uint32
		for j := 0; j < 4; j++ {
			v <<= 8
			if i+j < len(b) {
				v |= uint32(b[i+j])
			}
		}
		s += v
	}
	return s
}

func assemble(tables []table) []byte {
	sort.Slice(tables, func(i, j int) bool { return tables[i].tag < tables[j].tag })
	n := len(tables)
	p := 1
	es := 0
	for p*2 <= n {
		p *= 2
		es++
	}
	out := &w{}
	out.u32(0x00010000) // sfnt version (TrueType)
	out.u16(n)
	out.u16(p * 16)
	out.u16(es)
	out.u16(n*16 - p*16)

	offset := 12 + n*16
	headOffset := -1
	for _, t := range tables {
		if t.tag == "head" {
			headOffset = offset
		}
		out.tag(t.tag)
		out.u32(checksum(t.data))
		out.u32(uint32(offset))
		out.u32(uint32(len(t.data)))
		offset += (len(t.data) + 3) &^ 3
	}
	for _, t := range tables {
		out.bytes(t.data)
		out.pad4()
	}

	// head.checkSumAdjustment
	adj := 0xB1B0AFBA - checksum(out.b)
	a := headOffset + 8
	out.b[a] = byte(adj >> 24)
	out.b[a+1] = byte(adj >> 16)
	out.b[a+2] = byte(adj >> 8)
	out.b[a+3] = byte(adj)
	return out.b
}
