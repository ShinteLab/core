package kifu

// CSA 形式の棋譜ファイル全体を Document に読む（2026-11-15）。
//
// **読んだ結果は KIF と同じ Document にする。** 開始局面は手合割か盤面図（`Start`）、
// 指し手は KIF の指し手名（"７六歩"）と移動元で持つ。こうしておくと、使う側は
// **KIF と同じ 1 本の経路**（`StartSFEN` → `DecodeMoves`）で扱え、`String()` で
// KIF として書き出せる（CSA → KIF の変換にもなる）。
//
// 指し手の解釈は `DecodeCSA`（csa.go）、指し手名を付けるのは `Notation`（notation.go）。
// ここがやるのは**ファイルの構造を読むこと**（対局情報・開始局面・手番・
// 消費時間・コメント・終局）だけ。
//
//	V2.2
//	N+先手の名前
//	N-後手の名前
//	$EVENT:棋戦
//	$START_TIME:2026/11/15 10:00:00
//	PI                       （平手。PI82HI22KA のように続けると、その駒を落とす）
//	P1-KY-KE-GI-KI-OU-KI-GI-KE-KY   （または P1〜P9 で 1 段ずつ）
//	P+00FU                   （駒の追加。00 は駒台、00AL は残り全部）
//	+                        （手番）
//	+7776FU,T12              （指し手。カンマで 1 行に並べてもよい）
//	'*コメント               （直前の手のコメント）
//	%TORYO
//
// ⚠️ **合法性は見ない**（csa.go と同じ）。
// ⚠️ **開始局面が読めなければエラーにする**（平手へ黙って倒さない。BOD と同じ理由 ——
// 別の局面として読めてしまう）。
// ⚠️ **最初の 1 局だけを読む**（`/` で区切った 2 局目以降は読まない）。

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ShinteLab/core/sfen"
)

// ErrCSAMoves は CSA の指し手が途中で読めなくなったことを表す。
//
// `ParseCSA` は**そこまでの手を持った Document と一緒に**返す（`DecodeCSA` と同じ流儀）。
// `errors.Is(err, ErrCSAMoves)` なら Document は使える。
var ErrCSAMoves = errors.New("kifu: CSA の指し手が途中で読めなくなりました")

// csaMovesError は指し手の途中で止まった理由。**文言は止まった理由そのまま**にして、
// 種類だけ `ErrCSAMoves` として見分けられるようにする。
type csaMovesError struct{ err error }

func (e csaMovesError) Error() string        { return e.err.Error() }
func (e csaMovesError) Unwrap() error        { return e.err }
func (e csaMovesError) Is(target error) bool { return target == ErrCSAMoves }

// csaFullSet は 1 組の駒の枚数（`00AL` で残りを配るのに使う）。玉は配らない。
var csaFullSet = map[int]int{
	sfen.Pawn: 18, sfen.Lance: 4, sfen.Knight: 4, sfen.Silver: 4,
	sfen.Gold: 4, sfen.Bishop: 2, sfen.Rook: 2,
}

// IsCSA は text が CSA 形式の棋譜に見えるかを返す。
//
// KIF の行（"手合割：平手"・"   1 ７六歩(77)"・盤面図の "+---"）とは
// 先頭の形で見分けられる。**CSA にしか無い行が 1 つでもあれば CSA** とする。
func IsCSA(text string) bool {
	for _, raw := range strings.Split(normalizeNewlines(stripBOM(text)), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || line[0] == '\'' {
			continue
		}
		for _, s := range strings.Split(line, ",") {
			if isCSALine(strings.TrimSpace(s)) {
				return true
			}
		}
	}
	return false
}

// isCSALine は CSA にしか現れない形の 1 文かを返す。
func isCSALine(s string) bool {
	switch {
	case s == "+" || s == "-":
		return true
	case strings.HasPrefix(s, "V2") || strings.HasPrefix(s, "V3"):
		return true
	case strings.HasPrefix(s, "N+") || strings.HasPrefix(s, "N-"):
		return true
	case strings.HasPrefix(s, "PI"):
		return true
	case len(s) >= 2 && s[0] == 'P' && s[1] >= '1' && s[1] <= '9':
		return true
	case strings.HasPrefix(s, "$START_TIME:") || strings.HasPrefix(s, "$EVENT:"):
		return true
	}
	return isCSAMove(s)
}

// isCSAMove は "+7776FU" の形かを返す（盤は見ない）。
func isCSAMove(s string) bool {
	if len(s) != 7 || (s[0] != '+' && s[0] != '-') {
		return false
	}
	for i := 1; i <= 4; i++ {
		if digit(s[i]) < 0 {
			return false
		}
	}
	_, ok := csaPieces[s[5:7]]
	return ok
}

// csaStart は開始局面を組み立てている途中の状態。
type csaStart struct {
	cells [81]square // (y-1)*9 + (x-1)。x は 1..9 で 9 筋が 1（BOD と同じ並び）
	hands map[bool]map[int]int
	// all は駒台に `00AL`（残り全部）を置く側。0 なら無し、+1 先手、-1 後手。
	all  int
	seen bool // 盤の行（PI / P1〜P9 / P+ / P-）を 1 つでも読んだか
	turn byte // '+' / '-'。0 なら手番の行が無い
}

// ParseCSA は CSA 形式の棋譜を Document に読む。
//
//   - 対局者（N+ / N-）・棋戦（$EVENT）・場所（$SITE）・開始日時（$START_TIME）・
//     持ち時間（$TIME_LIMIT / $TIME）を読む
//   - 開始局面が手合割の初期局面と同じなら `Handicap` に名前を入れ、違えば `Start` に SFEN を入れる
//   - 消費時間（T 行）があれば `ShowTime` を立てる。コメントは `'*` 行だけを拾う
//     （`'**` は shogi-server の読み筋なので拾わない）
//   - 終局（%TORYO など）は KIF の終局の名前（"投了"）の手として最後に足す
//
// ⚠️ **開始局面が読めなければ Document はゼロ値でエラー。**
// ⚠️ **指し手の途中で読めなくなったら、そこまでの手を持った Document と
// `ErrCSAMoves` を返す**（`errors.Is` で見分ける）。
func ParseCSA(text string) (Document, error) {
	var d Document
	st := csaStart{hands: map[bool]map[int]int{true: {}, false: {}}}
	var moves []string         // 指し手と終局表記（DecodeCSA に渡す並び）
	var spends []time.Duration // moves と同じ並び
	var comments []string      // moves と同じ並び
	sawTime := false

	lines := strings.Split(normalizeNewlines(stripBOM(text)), "\n")
lines:
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if line[0] == '\'' {
			// コメント。`'*` だけが棋譜のコメント（Kifu for Windows などが書く）。
			if strings.HasPrefix(line, "'*") && !strings.HasPrefix(line, "'**") {
				c := strings.TrimPrefix(line, "'*")
				if n := len(comments); n > 0 {
					comments[n-1] = joinComment(comments[n-1], c)
				} else {
					d.Comment = joinComment(d.Comment, c)
				}
			}
			continue
		}
		if line == "/" {
			break // 2 局目以降は読まない
		}
		// ⚠️ **カンマで区切るのは指し手と時間の行だけ**（名前や棋戦名にカンマが入りうる）。
		parts := []string{line}
		if strings.ContainsRune("+-T%", rune(line[0])) {
			parts = strings.Split(line, ",")
		}
		for _, part := range parts {
			s := strings.TrimSpace(part)
			if s == "" {
				continue
			}
			switch {
			case s[0] == 'V':
				// 版（V2.2 など）。読み方は変えない。
			case strings.HasPrefix(s, "N+"):
				d.Black = strings.TrimSpace(s[2:])
			case strings.HasPrefix(s, "N-"):
				d.White = strings.TrimSpace(s[2:])
			case s[0] == '$':
				applyCSAInfo(&d, s)
			case s[0] == 'P':
				if len(moves) > 0 {
					return Document{}, fmt.Errorf("kifu: 指し手のあとに局面の行があります: %q", s)
				}
				if err := st.apply(s); err != nil {
					return Document{}, err
				}
			case (s == "+" || s == "-") && len(moves) == 0:
				st.turn = s[0]
			case s[0] == 'T':
				if n := len(spends); n > 0 {
					if v, err := strconv.ParseFloat(strings.TrimSpace(s[1:]), 64); err == nil && v >= 0 {
						spends[n-1] = time.Duration(v) * time.Second
						sawTime = true
					}
				}
			case s[0] == '+' || s[0] == '-' || s[0] == '%':
				moves = append(moves, s)
				spends = append(spends, 0)
				comments = append(comments, "")
				if s[0] == '%' {
					break lines // 終局のあとは読まない
				}
			default:
				return Document{}, fmt.Errorf("kifu: CSA の行が読めません: %q", s)
			}
		}
	}

	// 手番の行が無ければ最初の手の符号から決める（手が無ければ先手番）。
	if st.turn == 0 {
		st.turn = '+'
		for _, m := range moves {
			if m[0] == '+' || m[0] == '-' {
				st.turn = m[0]
				break
			}
		}
	}
	if !st.seen {
		// 局面の行が 1 つも無いのは「CSA ではない」か「壊れている」。平手に倒さない。
		if len(moves) == 0 && d.Black == "" && d.White == "" && d.Event == "" {
			return Document{}, nil // 何も読めなかった（Empty() が true）
		}
		return Document{}, fmt.Errorf("kifu: CSA の開始局面（PI か P1〜P9）がありません")
	}
	start, err := st.sfen()
	if err != nil {
		return Document{}, err
	}
	d.Start = start
	for name, board := range handicapBoards {
		if board == start {
			d.Handicap, d.Start = name, ""
			break
		}
	}
	d.ShowTime = sawTime

	usiMoves, end, decodeErr := DecodeCSA(start, moves)
	note, err := NewNotation(start)
	if err != nil {
		return Document{}, err
	}
	for i, mv := range usiMoves {
		t, err := note.Next(mv)
		if err != nil || !t.OK {
			// DecodeCSA が通した手なので起きないはず。起きたらそこで止める。
			if err == nil {
				err = fmt.Errorf("指し手名を付けられません: %q", mv)
			}
			return d, csaMovesError{fmt.Errorf("kifu: %d手目: %w", i+1, err)}
		}
		d.Moves = append(d.Moves, Move{
			Num: i + 1, Name: t.Name, FromX: t.FromX, FromY: t.FromY,
			Spend: spends[i], Comment: comments[i],
		})
	}
	if decodeErr != nil {
		return d, csaMovesError{decodeErr}
	}
	if end != "" {
		i := len(usiMoves)
		d.Moves = append(d.Moves, Move{
			Num: i + 1, Name: end, Spend: spends[i], Comment: comments[i],
		})
	}
	return d, nil
}

// joinComment は複数行のコメントを "\n" で繋ぐ（KIF の `*` 行と同じ）。
func joinComment(dst, add string) string {
	if dst == "" {
		return add
	}
	return dst + "\n" + add
}

// applyCSAInfo は `$` で始まる対局情報を読む。知らない項目は読み飛ばす。
func applyCSAInfo(d *Document, s string) {
	key, value, ok := strings.Cut(s[1:], ":")
	if !ok {
		return
	}
	value = strings.TrimSpace(value)
	switch key {
	case "EVENT":
		d.Event = value
	case "SITE":
		d.Place = value
	case "START_TIME":
		if t, ok := parseKifTime(value); ok {
			d.StartedAt = t
		}
	case "TIME_LIMIT":
		// "HH:MM+SS"（持ち時間 + 秒読み）。V2.2 の書き方。
		limit, byoyomi, _ := strings.Cut(value, "+")
		if h, m, ok := strings.Cut(limit, ":"); ok {
			hh, e1 := strconv.Atoi(h)
			mm, e2 := strconv.Atoi(m)
			if e1 == nil && e2 == nil {
				d.TimeLimit = time.Duration(hh)*time.Hour + time.Duration(mm)*time.Minute
			}
		}
		if n, err := strconv.Atoi(byoyomi); err == nil {
			d.Countdown = time.Duration(n) * time.Second
		}
	case "TIME":
		// "持ち時間秒+秒読み秒+加算秒"。V3.0 の書き方。加算（フィッシャー）は KIF に欄が無いので落とす。
		parts := strings.Split(value, "+")
		if n, err := strconv.ParseFloat(parts[0], 64); err == nil {
			d.TimeLimit = time.Duration(n) * time.Second
		}
		if len(parts) >= 2 {
			if n, err := strconv.ParseFloat(parts[1], 64); err == nil {
				d.Countdown = time.Duration(n) * time.Second
			}
		}
	}
}

// apply は局面の行（PI / P1〜P9 / P+ / P-）を 1 つ読む。
func (st *csaStart) apply(s string) error {
	st.seen = true
	switch {
	case strings.HasPrefix(s, "PI"):
		return st.applyPI(s[2:])
	case s[1] >= '1' && s[1] <= '9':
		return st.applyRank(int(s[1]-'0'), s[2:])
	case s[1] == '+' || s[1] == '-':
		return st.applyPieces(s[1] == '+', s[2:])
	}
	return fmt.Errorf("kifu: CSA の局面の行が読めません: %q", s)
}

// applyPI は平手の初期配置を置き、続く "82HI" などの駒を取り除く（駒落ち）。
func (st *csaStart) applyPI(rest string) error {
	b, err := parseBoard(handicapBoards[HirateHandicap])
	if err != nil {
		return err
	}
	st.cells = b.cells
	if len(rest)%4 != 0 {
		return fmt.Errorf("kifu: PI の行が読めません: %q", "PI"+rest)
	}
	for i := 0; i < len(rest); i += 4 {
		f, r := digit(rest[i]), digit(rest[i+1])
		p, ok := csaPieces[rest[i+2:i+4]]
		if f < 1 || r < 1 || !ok {
			return fmt.Errorf("kifu: PI の行が読めません: %q", "PI"+rest)
		}
		x, y := 10-f, r
		c := st.cells[index(x, y)]
		if !c.occupied || c.base != p.base || c.promoted != p.promoted {
			return fmt.Errorf("kifu: PI で取り除く駒がありません: %q", rest[i:i+4])
		}
		st.cells[index(x, y)] = square{}
	}
	return nil
}

// applyRank は 1 段（"-KY-KE-GI-KI-OU-KI-GI-KE-KY" / " * "）を読む。9 筋から並ぶ。
func (st *csaStart) applyRank(rank int, rest string) error {
	// 末尾の空白が削られていることがある（最後のマスが " * " のとき）。
	for len(rest) < 27 {
		rest += " "
	}
	if len(rest) != 27 {
		return fmt.Errorf("kifu: CSA の %d 段目が読めません: %q", rank, rest)
	}
	for i := 0; i < 9; i++ {
		cell := rest[i*3 : i*3+3]
		if strings.TrimSpace(cell) == "*" || strings.TrimSpace(cell) == "" {
			st.cells[index(i+1, rank)] = square{}
			continue
		}
		p, ok := csaPieces[cell[1:3]]
		if !ok || (cell[0] != '+' && cell[0] != '-') {
			return fmt.Errorf("kifu: CSA の %d 段目が読めません: %q", rank, cell)
		}
		st.cells[index(i+1, rank)] = square{
			base: p.base, promoted: p.promoted, black: cell[0] == '+', occupied: true,
		}
	}
	return nil
}

// applyPieces は "P+00FU59OU" のような駒の追加を読む（00 は駒台、00AL は残り全部）。
func (st *csaStart) applyPieces(black bool, rest string) error {
	if len(rest)%4 != 0 {
		return fmt.Errorf("kifu: CSA の駒の行が読めません: %q", rest)
	}
	for i := 0; i < len(rest); i += 4 {
		f, r := digit(rest[i]), digit(rest[i+1])
		kind := rest[i+2 : i+4]
		if f == 0 && r == 0 && kind == "AL" {
			st.all = 1
			if !black {
				st.all = -1
			}
			continue
		}
		p, ok := csaPieces[kind]
		if f < 0 || r < 0 || !ok {
			return fmt.Errorf("kifu: CSA の駒の行が読めません: %q", rest)
		}
		if f == 0 && r == 0 {
			if p.promoted || p.base == sfen.King {
				return fmt.Errorf("kifu: その駒は駒台に置けません: %q", rest[i:i+4])
			}
			st.hands[black][p.base]++
			continue
		}
		if f == 0 || r == 0 {
			return fmt.Errorf("kifu: CSA の駒の行が読めません: %q", rest)
		}
		st.cells[index(10-f, r)] = square{base: p.base, promoted: p.promoted, black: black, occupied: true}
	}
	return nil
}

// sfen は組み立てた開始局面を完全形の SFEN にする（手数は 1）。
func (st *csaStart) sfen() (string, error) {
	if st.all != 0 {
		// 盤と駒台に無い駒を、玉を除いて全部その側の駒台に置く（詰将棋の玉方の持駒）。
		left := map[int]int{}
		for base, n := range csaFullSet {
			left[base] = n
		}
		for _, c := range st.cells {
			if c.occupied {
				left[c.base]--
			}
		}
		for _, hand := range st.hands {
			for base, n := range hand {
				left[base] -= n
			}
		}
		for base, n := range left {
			if base == sfen.King {
				continue // 玉は配らない
			}
			if n < 0 {
				return "", fmt.Errorf("kifu: CSA の局面の駒が 1 組より多くあります")
			}
			st.hands[st.all > 0][base] += n
		}
	}
	board := sfen.FormatBoard(func(r, f int) string {
		// SFEN の記述順（rank=0 が一段目、file=0 が 9 筋）は x=file+1 / y=rank+1。
		s := st.cells[r*9+f]
		if !s.occupied {
			return ""
		}
		letter := sfen.Letter(s.base)
		if !s.black {
			letter = strings.ToLower(letter)
		}
		if s.promoted {
			letter = "+" + letter
		}
		return letter
	})
	turn := "b"
	if st.turn == '-' {
		turn = "w"
	}
	return fmt.Sprintf("%s %s %s 1", board, turn, formatSFENHands(st.hands)), nil
}
