package sfen

import (
	"errors"
	"strings"
	"testing"
)

const startBoard = "lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL"

func TestInspectStartpos(t *testing.T) {
	info := Inspect(startBoard, CheckAll)
	if !info.OK() {
		t.Fatalf("平手初期局面で違反: %v", info.Messages())
	}
	if len(info.Hands) != 0 {
		t.Errorf("初期局面の駒台は空のはず: %v", info.Hands)
	}
	if info.Black[Pawn] != 9 || info.White[Pawn] != 9 {
		t.Errorf("歩の枚数が違う: 先手 %d 後手 %d", info.Black[Pawn], info.White[Pawn])
	}
}

// 盤上に無い駒は駒台の枚数として逆算される
func TestInspectHands(t *testing.T) {
	// 玉2枚だけの盤面 → 玉以外は全部駒台
	board := "4k4/9/9/9/9/9/9/9/4K4"
	info := Inspect(board, CheckAll)
	if !info.OK() {
		t.Fatalf("違反が出た: %v", info.Messages())
	}
	want := map[int]int{Pawn: 18, Lance: 4, Knight: 4, Silver: 4, Gold: 4, Bishop: 2, Rook: 2}
	for base, n := range want {
		if info.Hands[base] != n {
			t.Errorf("%s の駒台 = %d, want %d", Name(base), info.Hands[base], n)
		}
	}
}

func TestInspectTooManyPieces(t *testing.T) {
	// 飛が先後合わせて3枚
	board := "4k4/9/9/9/4R4/9/9/1R5R1/4K4"
	info := Inspect(board, CheckAll)
	vs := info.Filter(CheckPieceCount)
	if len(vs) != 1 {
		t.Fatalf("駒数違反 = %d件 (%v), want 1", len(vs), info.Messages())
	}
	if vs[0].Piece != Rook || vs[0].Count != 3 || vs[0].Limit != 2 {
		t.Errorf("違反の内容が違う: %+v", vs[0])
	}
	if info.Hands[Rook] != 0 {
		t.Errorf("超過した駒種を駒台に入れてはいけない: %v", info.Hands)
	}
	// チェックを外せば報告されない
	if q := Inspect(board, CheckKing); len(q.Filter(CheckPieceCount)) != 0 {
		t.Errorf("CheckPieceCount を外しても報告された: %v", q.Messages())
	}
}

func TestInspectKing(t *testing.T) {
	board := "9/9/9/9/9/9/9/9/4K4" // 後手の玉が無い
	info := Inspect(board, CheckAll)
	vs := info.Filter(CheckKing)
	if len(vs) != 1 {
		t.Fatalf("玉の違反 = %d件 (%v), want 1", len(vs), info.Messages())
	}
	if vs[0].Black || vs[0].Count != 0 {
		t.Errorf("後手の玉なしとして報告されるはず: %+v", vs[0])
	}
	if !strings.Contains(vs[0].Detail, "後手") {
		t.Errorf("メッセージに手番が入らない: %q", vs[0].Detail)
	}
}

func TestInspectDoubledPawn(t *testing.T) {
	board := "4k4/9/9/9/9/9/P8/P8/4K4" // 先手の歩が9筋に2枚
	info := Inspect(board, CheckAll)
	vs := info.Filter(CheckDoubledPawn)
	if len(vs) != 1 {
		t.Fatalf("二歩 = %d件 (%v), want 1", len(vs), info.Messages())
	}
	if !vs[0].Black || vs[0].Count != 2 {
		t.Errorf("違反の内容が違う: %+v", vs[0])
	}
	// と金は二歩に数えない
	ok := Inspect("4k4/9/9/9/9/9/P8/+P8/4K4", CheckAll)
	if len(ok.Filter(CheckDoubledPawn)) != 0 {
		t.Errorf("と金を二歩に数えた: %v", ok.Messages())
	}
}

func TestInspectDeadPiece(t *testing.T) {
	// 先手の歩が1段目、後手の桂が8段目
	board := "P8/9/4k4/9/9/9/9/8n/4K4"
	info := Inspect(board, CheckAll)
	vs := info.Filter(CheckDeadPiece)
	if len(vs) != 2 {
		t.Fatalf("行き所のない駒 = %d件 (%v), want 2", len(vs), info.Messages())
	}
	if vs[0].Piece != Pawn || !vs[0].Black {
		t.Errorf("1件目が違う: %+v", vs[0])
	}
	if vs[1].Piece != Knight || vs[1].Black {
		t.Errorf("2件目が違う: %+v", vs[1])
	}
}

func TestInspectSyntax(t *testing.T) {
	info := Inspect("lnsgkgsnl/1r5b1", CheckAll)
	if len(info.Filter(CheckSyntax)) == 0 {
		t.Fatalf("段数不足が報告されない: %v", info.Messages())
	}
}

func TestBoardInfoErr(t *testing.T) {
	board := "9/9/9/9/9/9/9/9/4K4"
	info := Inspect(board, CheckAll)
	if err := info.Err(CheckPieceCount); err != nil {
		t.Errorf("駒数の違反は無いはず: %v", err)
	}
	err := info.Err(CheckKing)
	if err == nil {
		t.Fatal("玉の違反が error にならない")
	}
	var v Violation
	if !errors.As(err, &v) || v.Check != CheckKing {
		t.Errorf("Violation を取り出せない: %v", err)
	}
}

func TestFormatHands(t *testing.T) {
	if got := FormatHands(nil, nil); got != "-" {
		t.Errorf("FormatHands(nil, nil) = %q, want %q", got, "-")
	}
	black := map[int]int{Pawn: 2, Rook: 1}
	white := map[int]int{Silver: 3}
	if got, want := FormatHands(black, white), "R2P3s"; got != want {
		t.Errorf("FormatHands = %q, want %q", got, want)
	}
}
