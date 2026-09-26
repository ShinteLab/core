package kifu

// CSA 形式の指し手（"+7776FU"）を USI の手文字列（"7g7f"）にする変換。
//
// **decode.go（KIF → USI）と違って盤が要る。** CSA は**移動後の駒種**を書く
// （"-2277UM" は「角が 7七 へ動いて馬になった」）ので、**成ったのか、元から
// 成駒だったのか**は移動元の駒を見ないと分からない。
//
// ⚠️ **合法性は見ない**（decode.go と同じ）。
// ⚠️ **JS 側（web/）に対応物は無い**（KIF ドキュメントと同じ扱い）。

import (
	"fmt"
	"strings"

	"github.com/ShinteLab/core/sfen"
	"github.com/ShinteLab/core/usi"
)

// csaPiece は CSA の駒の 2 文字が表す駒種。
type csaPiece struct {
	base     int // sfen.Pawn..King（成駒でもベース駒）
	promoted bool
}

var csaPieces = map[string]csaPiece{
	"FU": {sfen.Pawn, false}, "KY": {sfen.Lance, false}, "KE": {sfen.Knight, false},
	"GI": {sfen.Silver, false}, "KI": {sfen.Gold, false}, "KA": {sfen.Bishop, false},
	"HI": {sfen.Rook, false}, "OU": {sfen.King, false},
	"TO": {sfen.Pawn, true}, "NY": {sfen.Lance, true}, "NK": {sfen.Knight, true},
	"NG": {sfen.Silver, true}, "UM": {sfen.Bishop, true}, "RY": {sfen.Rook, true},
}

// csaSpecials は CSA の終局表記 → KIF の終局の名前（TerminalMarkers にあるものだけ）。
//
// ⚠️ %+ILLEGAL_ACTION / %-ILLEGAL_ACTION は符号＝反則した側なので、手番を見て
// 書き分けないと勝敗が逆になる。必要になるまで入れない。
var csaSpecials = map[string]string{
	"%TORYO":        "投了",
	"%CHUDAN":       "中断",
	"%SENNICHITE":   "千日手",
	"%JISHOGI":      "持将棋",
	"%TSUMI":        "詰み",
	"%TIME_UP":      "切れ負け",
	"%ILLEGAL_MOVE": "反則負け",
	"%KACHI":        "入玉勝ち",
}

// DecodeCSA は開始局面 start（SFEN。StartSFEN の結果など）から、CSA の指し手を
// 順に USI の手文字列にする。
//
//   - 終局表記（%TORYO など）でそこで止め、KIF の終局の名前（"投了"）を end に返す。
//     ⚠️ **知らない終局表記はエラー**。黙って読み飛ばすと、終局した棋譜が
//     対局中に見える
//   - ⚠️ **符号（+/-）と手番が合わなければエラー**。1 手抜けた棋譜を黙って通すと、
//     以後の手が全部反対側の手として読まれる
//   - 時間の行（"T12"）と空行は読み飛ばす
//   - ⚠️ **エラーでも、そこまでに読めた手は返す**（DecodeMoves と同じ流儀）
func DecodeCSA(start string, moves []string) (out []string, end string, err error) {
	b, err := parseBoard(start)
	if err != nil {
		return nil, "", err
	}
	out = make([]string, 0, len(moves))
	for _, raw := range moves {
		s := strings.TrimSpace(raw)
		if s == "" || s[0] == 'T' {
			continue
		}
		num := len(out) + 1
		if s[0] == '%' {
			name, ok := csaSpecials[s]
			if !ok {
				return out, "", fmt.Errorf("kifu: %d手目: 未対応の終局表記です: %q", num, s)
			}
			return out, name, nil
		}
		m, err := b.csaMove(s)
		if err != nil {
			return out, "", fmt.Errorf("kifu: %d手目: %w", num, err)
		}
		if err := b.apply(m); err != nil {
			return out, "", fmt.Errorf("kifu: %d手目: %q: %w", num, s, err)
		}
		out = append(out, m.String())
	}
	return out, "", nil
}

// csaMove は CSA の指し手 1 つを、今の盤（手番・移動元の駒）を見て usi.Move にする。
// 盤は進めない。
func (b *board) csaMove(s string) (usi.Move, error) {
	if len(s) != 7 || (s[0] != '+' && s[0] != '-') {
		return usi.Move{}, fmt.Errorf("CSA の指し手が読めません: %q", s)
	}
	if black := s[0] == '+'; black != b.black {
		return usi.Move{}, fmt.Errorf("手番が合いません: %q", s)
	}
	ff, fr, tf, tr := digit(s[1]), digit(s[2]), digit(s[3]), digit(s[4])
	p, ok := csaPieces[s[5:7]]
	if ff < 0 || fr < 0 || tf < 0 || tr < 0 || !ok {
		return usi.Move{}, fmt.Errorf("CSA の指し手が読めません: %q", s)
	}
	if tf == 0 || tr == 0 {
		return usi.Move{}, fmt.Errorf("移動先が盤の外です: %q", s)
	}
	// core/usi の内部 x は 10-筋（筋の数字は右から数えるため）。
	m := usi.Move{ToX: 10 - tf, ToY: tr}
	if ff == 0 && fr == 0 {
		if p.promoted || p.base == sfen.King {
			return usi.Move{}, fmt.Errorf("その駒は打てません: %q", s)
		}
		m.Drop = sfen.Letter(p.base)[0]
		return m, nil
	}
	if ff == 0 || fr == 0 {
		return usi.Move{}, fmt.Errorf("移動元が盤の外です: %q", s)
	}
	m.FromX, m.FromY = 10-ff, fr
	from := b.at(m.FromX, m.FromY)
	if !from.occupied || from.black != b.black {
		return usi.Move{}, fmt.Errorf("移動元に手番側の駒がありません: %q", s)
	}
	if from.base != p.base {
		return usi.Move{}, fmt.Errorf("移動元の駒と合いません: %q", s)
	}
	// CSA は移動後の駒種を書く。元が生駒で CSA が成駒なら、この手で成った。
	switch {
	case p.promoted && !from.promoted:
		m.Promote = true
	case !p.promoted && from.promoted:
		return usi.Move{}, fmt.Errorf("成駒は元に戻れません: %q", s)
	}
	return m, nil
}

// digit は数字 1 文字を返す。数字でなければ -1。
func digit(c byte) int {
	if c < '0' || c > '9' {
		return -1
	}
	return int(c - '0')
}
