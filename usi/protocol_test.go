package usi

import (
	"reflect"
	"testing"
)

func TestParseInfo(t *testing.T) {
	tests := []struct {
		name string
		line string
		want Info
	}{
		{
			name: "engine の今の出力",
			line: "info depth 4 score cp 123 nodes 5000 pv 7g7f",
			want: Info{Depth: 4, ScoreCP: 123, HasScore: true, Nodes: 5000, HasNodes: true, PV: []string{"7g7f"}},
		},
		{
			name: "一般的なエンジンの出力",
			line: "info depth 12 seldepth 20 multipv 2 score cp -45 nodes 123456 nps 999 time 250 hashfull 100 pv 2g2f 8c8d 2f2e",
			want: Info{
				Depth: 12, SelDepth: 20, MultiPV: 2, ScoreCP: -45, HasScore: true,
				Nodes: 123456, HasNodes: true, NPS: 999, TimeMS: 250, HasTime: true,
				HashFullPerMille: 100, PV: []string{"2g2f", "8c8d", "2f2e"},
			},
		},
		{
			name: "詰み",
			line: "info depth 5 score mate 3 pv 5e5d 4c4b 5d5c",
			want: Info{Depth: 5, ScoreMate: 3, HasMate: true, PV: []string{"5e5d", "4c4b", "5d5c"}},
		},
		{
			name: "手数の分からない詰み",
			line: "info score mate -",
			want: Info{ScoreMate: -1, HasMate: true},
		},
		// **知らないトークンで行ごと捨てないこと。** エンジンごとに独自の項目が付く。
		{
			name: "知らない項目が混ざる",
			line: "info depth 3 someflag whatever 9 score cp 10 nodes 7",
			want: Info{Depth: 3, ScoreCP: 10, HasScore: true, Nodes: 7, HasNodes: true},
		},
		{
			name: "info string は以降が自由文",
			line: "info string hello score cp 999",
			want: Info{Text: "hello score cp 999"},
		},
		{
			name: "境界(depth の値が無い)",
			line: "info depth",
			want: Info{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseInfo(tt.line)
			if !ok {
				t.Fatalf("ParseInfo(%q) が info 行として読めませんでした", tt.line)
			}
			if !equalInfo(got, tt.want) {
				t.Errorf("ParseInfo(%q)\n got = %+v\nwant = %+v", tt.line, got, tt.want)
			}
		})
	}
}

// **score cp 0 と「score が無い」を混同しないこと。** 前者は互角という情報。
func TestParseInfoScoreZeroIsNotAbsent(t *testing.T) {
	got, _ := ParseInfo("info depth 1 score cp 0")
	if !got.HasScore {
		t.Error("score cp 0 が「score 無し」になっています")
	}
	if none, _ := ParseInfo("info depth 1"); none.HasScore {
		t.Error("score の無い行に HasScore が立っています")
	}
}

func TestParseInfoRejectsOtherLines(t *testing.T) {
	for _, line := range []string{"", "bestmove 7g7f", "readyok", "usiok"} {
		if _, ok := ParseInfo(line); ok {
			t.Errorf("ParseInfo(%q) が info 行として通りました", line)
		}
	}
}

func TestParseOption(t *testing.T) {
	tests := []struct {
		name string
		line string
		want Option
	}{
		{
			name: "spin",
			line: "option name USI_Hash type spin default 256 min 1 max 33554432",
			want: Option{
				Name: "USI_Hash", Type: "spin", Default: "256", HasDefault: true,
				Min: 1, HasMin: true, Max: 33554432, HasMax: true,
			},
		},
		{
			name: "check",
			line: "option name USI_Ponder type check default false",
			want: Option{Name: "USI_Ponder", Type: "check", Default: "false", HasDefault: true},
		},
		// **button は値を持たない**（送ること自体が動作になる）。
		{
			name: "button",
			line: "option name Clear Hash type button",
			want: Option{Name: "Clear Hash", Type: "button"},
		},
		{
			name: "combo",
			line: "option name BookFile type combo default standard_book.db var no_book var standard_book.db",
			want: Option{
				Name: "BookFile", Type: "combo",
				Default: "standard_book.db", HasDefault: true,
				Vars: []string{"no_book", "standard_book.db"},
			},
		},
		// ⚠️ **名前も既定値も空白を含みうる。** 位置で読むと壊れる。
		{
			name: "名前と既定値に空白",
			line: "option name Eval Dir type string default C:\\shogi\\my eval",
			want: Option{
				Name: "Eval Dir", Type: "string",
				Default: "C:\\shogi\\my eval", HasDefault: true,
			},
		},
		// `default <empty>` は空文字の意味。**「宣言されていない」とは区別する。**
		{
			name: "空の既定値",
			line: "option name BookDir type string default <empty>",
			want: Option{Name: "BookDir", Type: "string", Default: "", HasDefault: true},
		},
		{
			name: "既定値の宣言が無い",
			line: "option name SomeName type string",
			want: Option{Name: "SomeName", Type: "string"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseOption(tt.line)
			if !ok {
				t.Fatalf("ParseOption(%q) が option 行として読めませんでした", tt.line)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseOption(%q)\n got = %+v\nwant = %+v", tt.line, got, tt.want)
			}
		})
	}
}

func TestParseOptionRejectsOtherLines(t *testing.T) {
	for _, line := range []string{"", "usiok", "info depth 1", "option type spin"} {
		if _, ok := ParseOption(line); ok {
			t.Errorf("ParseOption(%q) が option 行として通りました", line)
		}
	}
}

func TestParseBestmove(t *testing.T) {
	tests := []struct {
		line   string
		move   string
		ponder string
		ok     bool
	}{
		{"bestmove 7g7f", "7g7f", "", true},
		{"bestmove 7g7f ponder 3c3d", "7g7f", "3c3d", true},
		// 投了・入玉宣言も**そのまま move に入れる**（解釈は呼び出し側）。
		{"bestmove resign", "resign", "", true},
		{"bestmove win", "win", "", true},
		{"info depth 1", "", "", false},
		{"bestmove", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			move, ponder, ok := ParseBestmove(tt.line)
			if ok != tt.ok || move != tt.move || ponder != tt.ponder {
				t.Errorf("ParseBestmove(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.line, move, ponder, ok, tt.move, tt.ponder, tt.ok)
			}
		})
	}
}

// equalInfo は Info を比べる。**PV（スライス）があるので == では比べられない。**
// 空スライスと nil は同じ扱いにする（`pv` の有無で分かれても意味は変わらない）。
func equalInfo(a, b Info) bool {
	if len(a.PV) == 0 && len(b.PV) == 0 {
		a.PV, b.PV = nil, nil
	}
	return reflect.DeepEqual(a, b)
}
