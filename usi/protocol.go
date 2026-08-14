package usi

import (
	"strconv"
	"strings"
)

// ここは USI の**プロトコルの語彙**（エンジンが返す行の読み取り）。
// 座標と手表記（usi.go）と同じく仕様なので core に置く。
//
// ⚠️ **セッション管理はここに置かない。** どのエンジンをいつ起動し、いつ stop し、
// いつ捨てるかはアプリ（ikkyoku）の仕事。ここが持つのは「1 行を読んで意味に直す」だけ。
//
// ⚠️ **JS 側（core/web）には今のところ対応物を置いていない。** USI を話すのは
// デスクトップアプリの Go 側で、ブラウザは構造化された結果しか見ないため。
// **ブラウザから USI を扱う必要が出たら、そのときに JS 側も揃えること**
// （SFEN のように「同じ仕様を 2 ソースで持つ」対象になる）。

// Info は info 行の中身。**エンジンが書かなかった項目は Has* が false。**
//
// 0 と「書かれていない」を区別する必要がある（`score cp 0` は互角という情報で、
// score が無い info 行とは意味が違う）。
type Info struct {
	Depth    int
	SelDepth int
	// MultiPV は候補手の順位（1 が最善）。宣言が無ければ 0。
	MultiPV int
	// ScoreCP はセンチポーン評価値。**エンジンの手番側から見た値**
	// （先手視点に直すのは受け取った側の仕事）。
	ScoreCP  int
	HasScore bool
	// ScoreMate は詰みまでの手数。正なら手番側が詰ます。
	ScoreMate        int
	HasMate          bool
	LowerBound       bool
	UpperBound       bool
	Nodes            int64
	HasNodes         bool
	TimeMS           int64
	HasTime          bool
	NPS              int64
	HashFullPerMille int
	// PV は読み筋（USI の手文字列）。
	PV []string
	// Text は `info string ...` の中身（エンジンからの任意メッセージ）。
	Text string
	// CurrMove は `info currmove <手>`。
	CurrMove string
}

// ParseInfo は info 行を読む。info 行でなければ ok=false。
//
// **知らないトークンは黙って読み飛ばす。** USI はエンジンごとに独自の項目を足すので、
// 未知の語で行ごと捨てると、そのエンジンの info が全部読めなくなる。
//
// `info string ...` は以降すべてが自由文なので、そこで読み取りを打ち切る。
func ParseInfo(line string) (Info, bool) {
	f := strings.Fields(line)
	if len(f) == 0 || f[0] != "info" {
		return Info{}, false
	}
	var in Info
	for i := 1; i < len(f); i++ {
		switch f[i] {
		case "string":
			// 以降は自由文。ここで終わり。
			in.Text = strings.Join(f[i+1:], " ")
			return in, true
		case "pv":
			// pv は行末まで手が並ぶ。**残り全部**なのでここで終わり。
			in.PV = append(in.PV, f[i+1:]...)
			return in, true
		case "depth":
			in.Depth, i = nextInt(f, i)
		case "seldepth":
			in.SelDepth, i = nextInt(f, i)
		case "multipv":
			in.MultiPV, i = nextInt(f, i)
		case "nodes":
			var v int64
			v, i = nextInt64(f, i)
			in.Nodes, in.HasNodes = v, true
		case "time":
			var v int64
			v, i = nextInt64(f, i)
			in.TimeMS, in.HasTime = v, true
		case "nps":
			in.NPS, i = nextInt64(f, i)
		case "hashfull":
			in.HashFullPerMille, i = nextInt(f, i)
		case "currmove":
			if i+1 < len(f) {
				i++
				in.CurrMove = f[i]
			}
		case "score":
			// score cp <v> / score mate <n> / score mate + / score mate -
			if i+1 >= len(f) {
				continue
			}
			kind := f[i+1]
			i++
			switch kind {
			case "cp":
				in.ScoreCP, i = nextInt(f, i)
				in.HasScore = true
			case "mate":
				// 手数の分からない詰み（"+" / "-"）を返すエンジンがある。
				if i+1 < len(f) && (f[i+1] == "+" || f[i+1] == "-") {
					i++
					in.ScoreMate = 1
					if f[i] == "-" {
						in.ScoreMate = -1
					}
				} else {
					in.ScoreMate, i = nextInt(f, i)
				}
				in.HasMate = true
			}
		case "lowerbound":
			in.LowerBound = true
		case "upperbound":
			in.UpperBound = true
		}
	}
	return in, true
}

// Option は `usi` の応答で宣言される option 行 1 つぶん。
//
// エンジンは `usi` に対して自分が受け付ける設定を並べて返す。将棋 UI は
// **その宣言をもとに `isready` の前に `setoption` を送る**（値を変えていなければ
// 既定値を送る）のが一般的な作法。
type Option struct {
	// Name は option 名。**空白を含みうる**（"Book File" など）。
	Name string
	// Type は "check" / "spin" / "combo" / "button" / "string" / "filename"。
	//
	// ⚠️ **button は値を持たない。** `setoption name X` を送ること自体が
	// 「押した」という動作になる（"Clear Hash" など）ので、**既定値の送信対象に
	// 含めないこと。**
	Type string
	// Default は既定値。**空白を含みうる**（パスなど）。
	//
	// USI/UCI の慣習で `default <empty>` は空文字を意味するので、空文字に直してある。
	Default string
	// HasDefault は default が宣言されていたか（空文字との区別）。
	HasDefault bool

	Min, Max       int
	HasMin, HasMax bool

	// Vars は combo の選択肢（`var` の並び）。
	Vars []string
}

// IsButton は押すだけの option か（値を持たない）。
func (o Option) IsButton() bool { return o.Type == "button" }

// optionKeys は option 行の中で「ここから次の項目」を意味する語。
//
// ⚠️ **名前も既定値も空白を含みうる**ので、位置ではなくこの語で区切って読む。
// 語はすべて小文字なので、"Max Threads" のような**名前の一部とは衝突しない**
// （衝突しうるのは小文字の "max" などをそのまま名前に使ったエンジンだけ）。
var optionKeys = map[string]bool{
	"name": true, "type": true, "default": true,
	"min": true, "max": true, "var": true,
}

// ParseOption は option 行を読む。option 行でなければ ok=false。
//
// 例: `option name USI_Hash type spin default 256 min 1 max 33554432`
func ParseOption(line string) (Option, bool) {
	f := strings.Fields(line)
	if len(f) == 0 || f[0] != "option" {
		return Option{}, false
	}

	var o Option
	key := ""
	var buf []string
	flush := func() {
		v := strings.Join(buf, " ")
		buf = buf[:0]
		switch key {
		case "name":
			o.Name = v
		case "type":
			o.Type = v
		case "default":
			// `default <empty>` は空文字の意味（USI/UCI の慣習）。
			if v == "<empty>" {
				v = ""
			}
			o.Default, o.HasDefault = v, true
		case "min":
			if n, err := strconv.Atoi(v); err == nil {
				o.Min, o.HasMin = n, true
			}
		case "max":
			if n, err := strconv.Atoi(v); err == nil {
				o.Max, o.HasMax = n, true
			}
		case "var":
			if v != "" {
				o.Vars = append(o.Vars, v)
			}
		}
	}

	for _, tok := range f[1:] {
		if optionKeys[tok] {
			flush()
			key = tok
			continue
		}
		buf = append(buf, tok)
	}
	flush()

	if o.Name == "" {
		return Option{}, false
	}
	return o, true
}

// ParseBestmove は bestmove 行を読む。bestmove 行でなければ ok=false。
//
// `bestmove resign` / `bestmove win` もそのまま move に入る（投了・入玉宣言。
// **手として解釈するのは呼び出し側**で、ここは行を分けるだけ）。
func ParseBestmove(line string) (move, ponder string, ok bool) {
	f := strings.Fields(line)
	if len(f) < 2 || f[0] != "bestmove" {
		return "", "", false
	}
	move = f[1]
	if len(f) >= 4 && f[2] == "ponder" {
		ponder = f[3]
	}
	return move, ponder, true
}

// nextInt は f[i+1] を整数として読み、進めた添字を返す。
// 読めなければ値は 0 のまま、添字も進めない（知らない形は読み飛ばす）。
func nextInt(f []string, i int) (int, int) {
	if i+1 >= len(f) {
		return 0, i
	}
	v, err := strconv.Atoi(f[i+1])
	if err != nil {
		return 0, i
	}
	return v, i + 1
}

func nextInt64(f []string, i int) (int64, int) {
	if i+1 >= len(f) {
		return 0, i
	}
	v, err := strconv.ParseInt(f[i+1], 10, 64)
	if err != nil {
		return 0, i
	}
	return v, i + 1
}
