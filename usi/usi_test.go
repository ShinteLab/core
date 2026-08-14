package usi

import "testing"

func TestSquareRoundTrip(t *testing.T) {
	cases := []struct {
		s    string
		x, y int
	}{
		{"7g", 3, 7},
		{"1a", 9, 1},
		{"9i", 1, 9},
		{"5e", 5, 5},
	}
	for _, c := range cases {
		x, y, ok := ParseSquare(c.s)
		if !ok || x != c.x || y != c.y {
			t.Errorf("ParseSquare(%q) = (%d,%d,%v), want (%d,%d,true)", c.s, x, y, ok, c.x, c.y)
		}
		if got := FormatSquare(c.x, c.y); got != c.s {
			t.Errorf("FormatSquare(%d,%d) = %q, want %q", c.x, c.y, got, c.s)
		}
	}
}

func TestParseSquareInvalid(t *testing.T) {
	for _, s := range []string{"", "7", "7gg", "0a", "7j", "aa"} {
		if _, _, ok := ParseSquare(s); ok {
			t.Errorf("ParseSquare(%q) unexpectedly ok", s)
		}
	}
}

func TestMoveRoundTrip(t *testing.T) {
	for _, s := range []string{"7g7f", "7g7f+", "P*5e", "8h2b+", "resign"} {
		m, ok := ParseMove(s)
		if !ok {
			t.Fatalf("ParseMove(%q) not ok", s)
		}
		if got := m.String(); got != s {
			t.Errorf("round-trip %q -> %q", s, got)
		}
	}
}

func TestParseMoveInvalid(t *testing.T) {
	for _, s := range []string{"", "1", "1g#", "zzz"} {
		if _, ok := ParseMove(s); ok {
			t.Errorf("ParseMove(%q) unexpectedly ok", s)
		}
	}
}
