// shogi_font: 日本語フォントから将棋駒の文字だけを抜き出し、
// SFEN 表記 (P,L,N,S,G,B,R,K / 小文字 / +付き成駒) で表示できる
// 軽量 TTF フォントを生成するツール。
//
//	go run . [-name FAMILY] [-index N] {inputfont} [output.ttf]
//	go run . -list {inputfont}
//
// -name は生成フォントの family 名。元フォントを変えて焼き分けるときに使う
// (Noto Serif JP→ShogiSFEN / Noto Sans JP→ShogiSFEN Gothic)。
// -index は TTC の中の何番目の書体か (-list で確かめる)。
//
// ⚠️ **中身は `shogifont` パッケージ。ここはその薄い CLI でしかない。**
// ライブラリとして焼きたい側 (ikkyoku の「端末のフォントから駒の字を作る」)
// が import できるように分けてある。**ここにロジックを戻さないこと。**
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ShinteLab/core/shogifont"
)

func main() {
	// -name は生成フォントの family 名。**元フォントを変えて焼くときは必ず変えること。**
	// 同名で複数登録するとどれが当たるかブラウザ任せになり、切り替えが効かなくなる。
	family := flag.String("name", shogifont.DefaultFamily, "生成フォントの family 名")
	index := flag.Int("index", 0, "コレクション (TTC) の中の書体番号")
	list := flag.Bool("list", false, "書体を一覧するだけ (焼かない)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: shogi_font [-name FAMILY] [-index N] {inputfont} [output.ttf]")
		fmt.Fprintln(os.Stderr, "       shogi_font -list {inputfont}")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	inPath := flag.Arg(0)
	outPath := "shogi.ttf"
	if flag.NArg() >= 2 {
		outPath = flag.Arg(1)
	}

	data, err := os.ReadFile(inPath)
	check(err)

	if *list {
		faces, err := shogifont.Faces(data)
		check(err)
		for _, f := range faces {
			state := "焼ける"
			if len(f.Missing) > 0 {
				state = "字が足りない: " + string(f.Missing)
			}
			fmt.Printf("%d\t%s %s\t%s\n", f.Index, f.Family, f.SubFamily, state)
		}
		return
	}

	out, err := shogifont.Build(data, shogifont.Options{Family: *family, Index: *index})
	check(err)
	check(os.WriteFile(outPath, out, 0o644))
	fmt.Printf("%s を出力しました (family=%q, %d バイト)\n", outPath, *family, len(out))
}

func check(err error) {
	if err != nil {
		msg := err.Error()
		// 字が足りないときは -list を促す (どの書体なら焼けるか分かる)。
		if strings.Contains(msg, "グリフを抽出できません") {
			msg += "\n-list で書体を確かめてください"
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}
