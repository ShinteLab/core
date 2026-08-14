// Package sfen は SFEN の駒文字マッピングと盤面文字列の解析・生成を
// 各プロジェクト共通で扱うための仕様パッケージ。
//
// 駒コードの整数値は shinte/engine の PieceType のベース値と一致させてある。
// これにより engine は自身の駒種を変換なしでそのまま渡せる(ホットパスで
// 追加コストを発生させない)。
//
// engine のホットパス(指し手生成・置換表)からも Letter などが呼ばれるため、
// 実装は switch/バイト演算ベースで確保(alloc)を最小に保つこと。
package sfen

import (
	"fmt"
	"strconv"
	"strings"
)

// ベース駒コード(engine.PieceType のベース値と一致)。
const (
	Pawn   = 0
	Lance  = 1
	Knight = 2
	Silver = 3
	Gold   = 4
	Rook   = 5
	Bishop = 6
	King   = 7

	// 成駒コード(engine の Growth* と一致)。
	GrowthPawn   = 8
	GrowthLance  = 9
	GrowthKnight = 10
	GrowthSilver = 11
	GrowthRook   = 12
	GrowthBishop = 13

	NotFound = 99
)

// Letter はベース/成駒コードに対応する SFEN の大文字を返す。
// 成駒はベース駒の文字を返す(engine の PieceType.Mark() と同じ挙動)。
func Letter(code int) string {
	switch code {
	case Pawn, GrowthPawn:
		return "P"
	case Lance, GrowthLance:
		return "L"
	case Knight, GrowthKnight:
		return "N"
	case Silver, GrowthSilver:
		return "S"
	case Gold:
		return "G"
	case Rook, GrowthRook:
		return "R"
	case Bishop, GrowthBishop:
		return "B"
	case King:
		return "K"
	}
	return "None"
}

// ParsePieceLetter は SFEN の1文字(大文字=先手/小文字=後手)を
// ベース駒コードと手番(black=先手)に変換する。未知の文字は NotFound。
func ParsePieceLetter(c byte) (base int, black bool) {
	black = true
	if c >= 'a' && c <= 'z' {
		c -= 32
		black = false
	}
	switch c {
	case 'P':
		base = Pawn
	case 'L':
		base = Lance
	case 'N':
		base = Knight
	case 'S':
		base = Silver
	case 'G':
		base = Gold
	case 'R':
		base = Rook
	case 'B':
		base = Bishop
	case 'K':
		base = King
	default:
		base = NotFound
	}
	return base, black
}

// ParseBoard は SFEN 盤面部分('/' 区切りの9段)を解析し、駒のあるマスごとに
// place を呼ぶ。rank/file は SFEN 記述順の 0..8(rank=0 が先頭段、
// file=0 が各段の先頭文字)。段数が9でない・段内のマス合計が9でない場合は
// エラーを返す(place はそこまでに解析できた分について呼ばれ得る)。
func ParseBoard(board string, place func(rank, file, base int, black, promoted bool)) error {
	ranks := strings.Split(board, "/")
	if len(ranks) != 9 {
		return fmt.Errorf("sfen parse error: rank count %d != 9 [%s]", len(ranks), board)
	}
	for rank := 0; rank < 9; rank++ {
		line := ranks[rank]
		file := 0
		for idx := 0; idx < len(line); idx++ {
			c := line[idx]
			if c >= '0' && c <= '9' {
				file += int(c - '0')
				continue
			}
			promoted := false
			if c == '+' {
				promoted = true
				idx++
				if idx >= len(line) {
					return fmt.Errorf("sfen parse error: dangling '+' [%s]", line)
				}
				c = line[idx]
			}
			base, black := ParsePieceLetter(c)
			place(rank, file, base, black, promoted)
			file++
		}
		if file != 9 {
			return fmt.Errorf("sfen parse error: rank %d has %d files [%s]", rank, file, line)
		}
	}
	return nil
}

// FormatBoard は 9x9 の盤面を SFEN 盤面文字列(9段を '/' 区切り)に変換する。
// cell(rank, file) は SFEN 記述順(rank=0 が先頭段、file=0 が各段の先頭文字)の
// マス表記を返す("" なら空マス)。連続する空マスは数字にまとめる。
func FormatBoard(cell func(rank, file int) string) string {
	var sb strings.Builder
	for rank := 0; rank < 9; rank++ {
		empty := 0
		for file := 0; file < 9; file++ {
			m := cell(rank, file)
			if m == "" {
				empty++
				continue
			}
			if empty != 0 {
				sb.WriteString(strconv.Itoa(empty))
				empty = 0
			}
			sb.WriteString(m)
		}
		if empty != 0 {
			sb.WriteString(strconv.Itoa(empty))
		}
		if rank != 8 {
			sb.WriteByte('/')
		}
	}
	return sb.String()
}
