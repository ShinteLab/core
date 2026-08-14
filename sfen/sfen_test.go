package sfen

import "testing"

func TestLetter(t *testing.T) {
	cases := map[int]string{
		Pawn: "P", Lance: "L", Knight: "N", Silver: "S",
		Gold: "G", Rook: "R", Bishop: "B", King: "K",
		GrowthPawn: "P", GrowthRook: "R", GrowthBishop: "B",
		NotFound: "None",
	}
	for code, want := range cases {
		if got := Letter(code); got != want {
			t.Errorf("Letter(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestParsePieceLetter(t *testing.T) {
	if b, black := ParsePieceLetter('P'); b != Pawn || !black {
		t.Errorf("'P' -> (%d,%v)", b, black)
	}
	if b, black := ParsePieceLetter('p'); b != Pawn || black {
		t.Errorf("'p' -> (%d,%v)", b, black)
	}
	if b, _ := ParsePieceLetter('z'); b != NotFound {
		t.Errorf("'z' -> %d, want NotFound", b)
	}
}

const startpos = "lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL"

func TestParseBoardStartpos(t *testing.T) {
	count := 0
	err := ParseBoard(startpos, func(rank, file, base int, black, promoted bool) {
		count++
	})
	if err != nil {
		t.Fatalf("ParseBoard error: %v", err)
	}
	if count != 40 {
		t.Errorf("piece count = %d, want 40", count)
	}
}

func TestParseBoardInvalid(t *testing.T) {
	if err := ParseBoard("9/9/9", nil); err == nil {
		t.Error("expected error for 3 ranks")
	}
}

func TestFormatBoardRoundTrip(t *testing.T) {
	// grid[rank][file] holds the SFEN cell mark, "" for empty.
	var grid [9][9]string
	err := ParseBoard(startpos, func(rank, file, base int, black, promoted bool) {
		m := Letter(base)
		if !black {
			m = string(m[0] + 32) // lower-case for white
		}
		if promoted {
			m = "+" + m
		}
		grid[rank][file] = m
	})
	if err != nil {
		t.Fatal(err)
	}
	got := FormatBoard(func(rank, file int) string { return grid[rank][file] })
	if got != startpos {
		t.Errorf("FormatBoard round-trip:\n got %q\nwant %q", got, startpos)
	}
}
