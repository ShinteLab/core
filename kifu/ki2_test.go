package kifu

import (
	"errors"
	"strings"
	"testing"
)

const hirateSFEN = "lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL b - 1"

func TestDecodeKI2Sequence(t *testing.T) {
	// 中継の書き方そのまま（算用数字・"同" の後に空白なし・打ちの「打」なし）。
	names := []string{"▲26歩", "△84歩", "▲25歩", "△85歩", "▲24歩", "△同歩", "▲同飛", "△23歩"}
	got, end, err := DecodeKI2(hirateSFEN, names)
	if err != nil {
		t.Fatal(err)
	}
	if end != "" {
		t.Fatalf("end = %q", end)
	}
	wantUSI := []string{"2g2f", "8c8d", "2f2e", "8d8e", "2e2d", "2c2d", "2h2d", "P*2c"}
	wantName := []string{"２六歩", "８四歩", "２五歩", "８五歩", "２四歩", "同　歩", "同　飛", "２三歩打"}
	if len(got) != len(wantUSI) {
		t.Fatalf("len = %d", len(got))
	}
	for i, m := range got {
		if m.USI != wantUSI[i] || m.Name != wantName[i] {
			t.Errorf("%d: got %s %s, want %s %s", i+1, m.USI, m.Name, wantUSI[i], wantName[i])
		}
	}
	// KIF の移動元（筋・段）も入る。
	if got[0].FromX != 2 || got[0].FromY != 7 {
		t.Errorf("from = %d%d, want 27", got[0].FromX, got[0].FromY)
	}
}

func TestKI2Decoder(t *testing.T) {
	tests := []struct {
		name, sfen, move, usi string
	}{
		{"修飾で絞る（右）", "4k4/9/9/9/9/9/9/9/3GKG3 b - 1", "▲５八金右", "4i5h"},
		{"修飾で絞る（左）", "4k4/9/9/9/9/9/9/9/3GKG3 b - 1", "58金左", "6i5h"},
		{"ピンされた駒は数えない", "4r4/9/9/9/9/9/9/4G4/3GK4 b - 1", "68金", "6i6h"},
		{"成", "k8/9/9/4S4/9/9/9/9/K8 b - 1", "４三銀成", "5d4c+"},
		{"不成", "k8/9/9/4S4/9/9/9/9/K8 b - 1", "４三銀不成", "5d4c"},
		{"成も不成も無ければ成らない", "k8/9/9/4S4/9/9/9/9/K8 b - 1", "４三銀", "5d4c"},
		{"打があれば打ち", "4k4/9/9/9/9/9/9/4G4/4K4 b G 1", "５七金打", "G*5g"},
		{"打が無ければ盤上の駒", "4k4/9/9/9/9/9/9/4G4/4K4 b G 1", "５七金", "5h5g"},
		{"竜と王の表記ゆれ", "4k4/9/9/9/9/9/9/9/+R3K4 b - 1", "９一竜", "9i9a"},
		{"成駒の略記", "4k4/9/9/9/9/9/9/+S8/4K4 b - 1", "９七全", "9h9g"},
		{"後手は向きを逆に見る", "3gkg3/9/9/9/9/9/9/9/4K4 w - 1", "△５二金右", "6a5b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := NewKI2Decoder(tt.sfen)
			if err != nil {
				t.Fatal(err)
			}
			got, err := d.Next(tt.move)
			if err != nil {
				t.Fatal(err)
			}
			if got.USI != tt.usi {
				t.Errorf("USI = %s, want %s", got.USI, tt.usi)
			}
		})
	}
}

func TestKI2DecoderRefuses(t *testing.T) {
	tests := []struct {
		name, sfen, move string
	}{
		{"絞れない", "4k4/9/9/9/9/9/9/9/3GKG3 b - 1", "58金"},
		{"行ける駒が無く打てもしない（駒がある）", hirateSFEN, "89飛"},
		{"修飾が合わない", "4k4/9/9/9/9/9/9/9/3GKG3 b - 1", "58金直"},
		{"最初の手は同にできない", hirateSFEN, "同歩"},
		{"手番記号が合わない", hirateSFEN, "△76歩"},
		{"移動先が読めない", hirateSFEN, "歩"},
		{"駒の名前が読めない", hirateSFEN, "76象"},
		{"成駒は打てない", "4k4/9/9/9/9/9/9/9/4K4 b - 1", "55馬"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := NewKI2Decoder(tt.sfen)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.Next(tt.move); !errors.Is(err, ErrKI2Move) {
				t.Fatalf("err = %v, want ErrKI2Move", err)
			}
		})
	}
}

// 中継の誤記で止まっても、そこまでの手は返す。
func TestDecodeKI2KeepsPrefix(t *testing.T) {
	got, _, err := DecodeKI2(hirateSFEN, []string{"▲26歩", "△84歩", "▲89飛"})
	if !errors.Is(err, ErrKI2Move) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "3手目") {
		t.Errorf("何手目かが入っていない: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestDecodeKI2Terminal(t *testing.T) {
	got, end, err := DecodeKI2(hirateSFEN, []string{"▲76歩", "△投了", "▲26歩"})
	if err != nil {
		t.Fatal(err)
	}
	if end != "投了" || len(got) != 1 {
		t.Fatalf("end = %q, len = %d", end, len(got))
	}
}
