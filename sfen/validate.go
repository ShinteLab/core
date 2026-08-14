package sfen

// 盤面(SFEN の盤面部分)が将棋の局面として辻褄が合っているかを調べる仕組み。
//
// 画像認識(suteme)のように「盤面を外から推測する」用途では、出てきた盤面が
// おかしいこと自体は珍しくない。**どこがおかしいかを構造化して返すのが役目**で、
// それをエラーとして扱うかどうかは呼び出し側が決める(Check のビットマスクで選ぶ)。
//
// ここは仕様(将棋のルールと駒数)を持つ場所であって、画像認識やアプリ固有の
// 判断は入れないこと。
//
// **この API は I/O 境界用。** sfen.go の Letter などと違い map と fmt を使うので、
// engine のホットパス(指し手生成・探索)から呼ばないこと。

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Check は盤面チェックの種類。どのチェックを行うか / どの違反をエラーとして
// 扱うかをビットマスクで指定できるようにしてある。
type Check uint

const (
	// CheckSyntax は SFEN 盤面文字列として解析できるか(段数・筋数・駒文字)。
	CheckSyntax Check = 1 << iota
	// CheckPieceCount は駒種ごとの総数が規定枚数を超えていないか。
	CheckPieceCount
	// CheckKing は玉が先後に1枚ずつあるか。
	CheckKing
	// CheckDoubledPawn は二歩(同じ筋に同じ手番の生歩が2枚以上)。
	CheckDoubledPawn
	// CheckDeadPiece は行き所のない駒(歩・香が最終段、桂が最終2段)。
	CheckDeadPiece
)

const (
	// CheckAll は全てのチェック。
	CheckAll = CheckSyntax | CheckPieceCount | CheckKing | CheckDoubledPawn | CheckDeadPiece
	// CheckCounts は駒数の辻褄だけを見る最小構成。駒台の枚数を逆算するには
	// これで足りる(配置のルール違反は見ない)。
	CheckCounts = CheckSyntax | CheckPieceCount | CheckKing
)

// Has は c が x のいずれかを含むかを返す。
func (c Check) Has(x Check) bool { return c&x != 0 }

// String はチェック種別の日本語名を返す(複数ビットなら '|' 区切り)。
func (c Check) String() string {
	if c == 0 {
		return "なし"
	}
	names := []struct {
		bit  Check
		name string
	}{
		{CheckSyntax, "書式"},
		{CheckPieceCount, "駒数"},
		{CheckKing, "玉"},
		{CheckDoubledPawn, "二歩"},
		{CheckDeadPiece, "行き所のない駒"},
	}
	var parts []string
	for _, n := range names {
		if c&n.bit != 0 {
			parts = append(parts, n.name)
		}
	}
	if len(parts) == 0 {
		return "不明"
	}
	return strings.Join(parts, "|")
}

// pieceLimits は先後合計の駒数上限(ベース駒コード → 枚数)。成駒はベース駒として数える。
var pieceLimits = map[int]int{
	Pawn: 18, Lance: 4, Knight: 4, Silver: 4,
	Gold: 4, Bishop: 2, Rook: 2, King: 2,
}

// PieceLimit はベース駒コードの先後合計の上限枚数を返す。未知のコードは 0。
func PieceLimit(base int) int { return pieceLimits[base] }

// HandOrder は持ち駒表記の駒種順(飛・角・金・銀・桂・香・歩)。
// SFEN の持ち駒はこの順で書くのが慣例。玉は含まない。
var HandOrder = []int{Rook, Bishop, Gold, Silver, Knight, Lance, Pawn}

// pieceNames はベース/成駒コードの日本語名。
var pieceNames = map[int]string{
	Pawn: "歩", Lance: "香", Knight: "桂", Silver: "銀",
	Gold: "金", Rook: "飛", Bishop: "角", King: "玉",
	GrowthPawn: "と", GrowthLance: "成香", GrowthKnight: "成桂",
	GrowthSilver: "成銀", GrowthRook: "龍", GrowthBishop: "馬",
}

// Name はベース/成駒コードの日本語名を返す。未知のコードは "?"。
func Name(code int) string {
	if n, ok := pieceNames[code]; ok {
		return n
	}
	return "?"
}

// Violation は盤面の違反1件。Check ごとに埋まるフィールドが違う。
// error を満たすので、そのまま返しても errors.Is/As で扱える。
type Violation struct {
	Check Check `json:"check"`
	// Piece は対象のベース駒コード。該当しなければ NotFound。
	Piece int `json:"piece"`
	// Sided が true のとき Black に意味がある(先手側の違反か)。
	Sided bool `json:"sided"`
	Black bool `json:"black"`
	// Count / Limit は駒数系の違反で埋まる。
	Count int `json:"count"`
	Limit int `json:"limit"`
	// Rank / File は場所の分かる違反(二歩・行き所のない駒)で埋まる。
	// SFEN 記述順の 0..8。該当しなければ -1。
	Rank int `json:"rank"`
	File int `json:"file"`
	// Detail は人に見せる日本語メッセージ。
	Detail string `json:"detail"`
}

// Error は Detail を返す(Violation は error を満たす)。
func (v Violation) Error() string { return v.Detail }

// Side は手番の日本語表記を返す(Sided でなければ "")。
func (v Violation) Side() string {
	if !v.Sided {
		return ""
	}
	if v.Black {
		return "先手"
	}
	return "後手"
}

// BoardInfo は盤面を調べた結果。
type BoardInfo struct {
	// Black / White は盤上の駒数(ベース駒コード → 枚数)。成駒はベース駒に合算する。
	Black map[int]int `json:"black"`
	White map[int]int `json:"white"`
	// Hands は駒数の逆算で求めた駒台の枚数(先後の区別は付かない)。
	// **どちらの持ち駒かは盤面からは決まらない**ので、割り振りは呼び出し側の仕事。
	// 上限を超えている駒種は 0 のまま(Violations に載る)。
	Hands map[int]int `json:"hands"`
	// Violations は見つかった違反。行った Check の分しか入らない。
	Violations []Violation `json:"violations"`
}

// OK は違反が無いことを返す。
func (b *BoardInfo) OK() bool { return len(b.Violations) == 0 }

// Filter は指定した Check に該当する違反だけを返す。
func (b *BoardInfo) Filter(c Check) []Violation {
	var out []Violation
	for _, v := range b.Violations {
		if c.Has(v.Check) {
			out = append(out, v)
		}
	}
	return out
}

// Err は指定した Check に該当する違反をまとめた error を返す(無ければ nil)。
// 個々の Violation は errors.As で取り出せる。
func (b *BoardInfo) Err(c Check) error {
	vs := b.Filter(c)
	if len(vs) == 0 {
		return nil
	}
	errs := make([]error, len(vs))
	for i, v := range vs {
		errs[i] = v
	}
	return errors.Join(errs...)
}

// Messages は違反の日本語メッセージだけを取り出す(UI 表示用)。
func (b *BoardInfo) Messages() []string {
	out := make([]string, 0, len(b.Violations))
	for _, v := range b.Violations {
		out = append(out, v.Detail)
	}
	return out
}

// square は SFEN 記述順の rank/file を「n段m筋」の日本語にする。
func square(rank, file int) string {
	return fmt.Sprintf("%d筋%d段", 9-file, rank+1)
}

// Inspect は SFEN の盤面部分を調べ、駒数・駒台枚数・違反を返す。
// checks に含めたチェックだけを行う(CheckAll で全部)。
//
// 壊れた盤面でも解析できた分は集計する(画像認識の途中結果を扱うため)。
func Inspect(board string, checks Check) *BoardInfo {
	info := &BoardInfo{
		Black: map[int]int{},
		White: map[int]int{},
		Hands: map[int]int{},
	}

	// 生歩の筋ごとの枚数(二歩の判定用)。[手番][筋]
	var pawnFiles [2][9]int
	// 二歩・行き所のない駒は最初の1件だけ場所を報告する(同じ違反で溢れさせない)
	type spot struct{ rank, file int }
	firstPawnDup := map[int]spot{}

	perr := ParseBoard(board, func(rank, file, base int, black, promoted bool) {
		if base == NotFound {
			if checks.Has(CheckSyntax) {
				info.add(Violation{
					Check: CheckSyntax, Piece: NotFound, Rank: rank, File: file,
					Detail: fmt.Sprintf("%s に読めない駒があります", square(rank, file)),
				})
			}
			return
		}
		if black {
			info.Black[base]++
		} else {
			info.White[base]++
		}

		if !promoted && base == Pawn {
			side := 0
			if !black {
				side = 1
			}
			if file >= 0 && file < 9 {
				pawnFiles[side][file]++
				if pawnFiles[side][file] == 2 {
					firstPawnDup[side*9+file] = spot{rank, file}
				}
			}
		}

		if checks.Has(CheckDeadPiece) && !promoted {
			if dead(base, black, rank) {
				info.add(Violation{
					Check: CheckDeadPiece, Piece: base, Sided: true, Black: black,
					Rank: rank, File: file,
					Detail: fmt.Sprintf("行き所のない駒: %s%s が %s にあります",
						sideName(black), Name(base), square(rank, file)),
				})
			}
		}
	})
	if perr != nil && checks.Has(CheckSyntax) {
		info.add(Violation{
			Check: CheckSyntax, Piece: NotFound, Rank: -1, File: -1,
			Detail: fmt.Sprintf("盤面を解析できません: %v", perr),
		})
	}

	if checks.Has(CheckDoubledPawn) {
		keys := make([]int, 0, len(firstPawnDup))
		for k := range firstPawnDup {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		for _, k := range keys {
			black := k < 9
			at := firstPawnDup[k]
			info.add(Violation{
				Check: CheckDoubledPawn, Piece: Pawn, Sided: true, Black: black,
				Rank: at.rank, File: at.file, Count: pawnFiles[k/9][k%9], Limit: 1,
				Detail: fmt.Sprintf("二歩: %sの歩が %d筋に %d枚あります",
					sideName(black), 9-at.file, pawnFiles[k/9][k%9]),
			})
		}
	}

	// 駒数と駒台の逆算。上限超過はチェック対象外でも Hands に嘘を入れないため
	// 常に判定し、報告だけを checks で制御する。玉は駒台に乗らないので
	// ここでは扱わず CheckKing に任せる。
	for _, base := range HandOrder {
		limit := pieceLimits[base]
		total := info.Black[base] + info.White[base]
		if total > limit {
			if checks.Has(CheckPieceCount) {
				info.add(Violation{
					Check: CheckPieceCount, Piece: base, Rank: -1, File: -1,
					Count: total, Limit: limit,
					Detail: fmt.Sprintf("%sが %d枚あります(上限 %d枚)", Name(base), total, limit),
				})
			}
			continue
		}
		if total < limit {
			info.Hands[base] = limit - total
		}
	}

	if checks.Has(CheckKing) {
		for _, black := range []bool{true, false} {
			n := info.Black[King]
			if !black {
				n = info.White[King]
			}
			if n == 1 {
				continue
			}
			detail := fmt.Sprintf("%sの玉が %d枚あります(1枚のはず)", sideName(black), n)
			if n == 0 {
				detail = fmt.Sprintf("%sの玉がありません", sideName(black))
			}
			info.add(Violation{
				Check: CheckKing, Piece: King, Sided: true, Black: black,
				Rank: -1, File: -1, Count: n, Limit: 1, Detail: detail,
			})
		}
	}

	return info
}

func (b *BoardInfo) add(v Violation) { b.Violations = append(b.Violations, v) }

func sideName(black bool) string {
	if black {
		return "先手"
	}
	return "後手"
}

// dead は生駒が行き所のない位置(以後どこにも動けない段)にあるかを返す。
// rank は SFEN 記述順(0 が先手から見て最奥＝1段目)。
func dead(base int, black bool, rank int) bool {
	switch base {
	case Pawn, Lance:
		if black {
			return rank == 0
		}
		return rank == 8
	case Knight:
		if black {
			return rank <= 1
		}
		return rank >= 7
	}
	return false
}

// FormatHands は先後の持ち駒を SFEN の持ち駒表記にする。
// 枚数はベース駒コードをキーにする(HandOrder の順で並べ、2枚以上は枚数を前置)。
// 両者とも持ち駒が無ければ "-"。
func FormatHands(black, white map[int]int) string {
	var sb strings.Builder
	write := func(m map[int]int, upper bool) {
		for _, base := range HandOrder {
			n := m[base]
			if n <= 0 {
				continue
			}
			if n > 1 {
				sb.WriteString(strconv.Itoa(n))
			}
			l := Letter(base)
			if !upper {
				l = strings.ToLower(l)
			}
			sb.WriteString(l)
		}
	}
	write(black, true)
	write(white, false)
	if sb.Len() == 0 {
		return "-"
	}
	return sb.String()
}
