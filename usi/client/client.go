// Package client は USI エンジンと話す**クライアント側**（＝将棋 UI 側）の実装。
//
// 繋ぎ先が「USI を話すプロセス」でありさえすれば中身を問わない。
// やねうら王・水匠のような既存エンジンでも、prokishi のようなプロキシでも、
// 自作エンジンでも、**ここから上のコードは違いを知らない**。
//
// 親パッケージ（core/usi）が持つのは**プロトコルの語彙**（マス座標・手表記・
// info 行の読み取り）で、ここが持つのは**セッション管理**
// （いつ繋ぎ、いつ送り、いつ止め、いつ捨てるか）。
//
// ⚠️ **親パッケージと性格が違うので分けてある。** core/usi は探索のホットパスからも
// 呼ばれるため alloc を最小に保つ必要があるが、こちらはプロセスと I/O を扱う層で
// ホットパスには乗らない。**同じパッケージに混ぜないこと。**
//
// ⚠️ **「best を問い合わせる同期 API」に丸めないこと。** `GetBestMove(sfen) → 指し手`
// のような形まで畳むと、検討用途で必要なものが軒並み取れなくなる:
//
//   - **MultiPV が出せない**（候補手が複数並ぶことが検討ツールの前提）
//   - **info の逐次ストリームを捨てる**（深さ・評価値・読み筋の途中経過）
//   - **stop で途中まで読んだ結果を使う**という基本操作が表現できない
//
// USI がステートフル（position → go → info… → bestmove）なのは面倒に見えるが、
// **その面倒さがそのまま欲しい機能**なので、素直に USI のまま扱う。
package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ShinteLab/core/usi"
)

// HandshakeTimeout は usiok / readyok を待つ上限。
//
// 相手が USI を話さないプロセスだった場合、ここで待ち続けると呼び出し側が黙って固まる。
// **必ず時間で切ること。**
const HandshakeTimeout = 10 * time.Second

// Transport はエンジンとの入出力。
//
// `os/exec` でプロセスを起こす場合は Exec が返すものを使う。同一プロセスの
// エンジンを `io.Pipe` で繋ぐこともできる（**この層はどちらかを知らない**）。
type Transport struct {
	// In はエンジンの標準出力（こちらが読む側）。
	In io.Reader
	// Out はエンジンの標準入力（こちらが書く側）。
	Out io.Writer
	// Close は後始末（プロセスの終了・パイプのクローズ）。nil 可。
	Close func() error
}

// Session は起動済みのエンジン 1 つとの対話。
//
// **1 つの Session に同時に 1 つの探索しか流せない**（USI がそういう作り）。
// 複数の局面を並べて解析したいなら Session を増やす（＝エンジンを増やす）。
type Session struct {
	t Transport

	// ID は `id name` で名乗った名前。
	ID string
	// Author は `id author`。
	Author string
	// Options はエンジンが `usi` 応答で宣言した option（宣言順）。
	//
	// **値の意味づけはしない。** エンジンごとに種類も範囲も違うので、
	// 「どれを画面に出すか」「どう入力させるか」は呼び出し側が決める。
	Options []usi.Option

	// Applied は Open が実際に送った `setoption` 行（送った順）。
	//
	// **何を送ったかは残す。** option は送っても応答が返らないので、
	// 効いているかどうかを後から確かめる手掛かりがこれしかない。
	Applied []string

	// BestmoveGrace は `stop` を送ってから bestmove を待つ上限（既定 HandshakeTimeout）。
	//
	// **ここを 0 にしないこと。** 待たずに返ると、次の `position` が前の探索の
	// bestmove と混ざる。壊れた相手を待ち続けないための上限であって、
	// 打ち切りを速くするための値ではない。
	BestmoveGrace time.Duration

	// lines はエンジンからの行。読み取り goroutine が流す。
	lines chan string
	// readErr は読み取りが終わった理由（EOF・プロセス終了）。
	readErr chan error

	// mu は送信の直列化。**探索中でも stop を送れる**必要があるので、
	// 送信だけを守り、待ちの側は握らない。
	mu     sync.Mutex
	closed bool
}

// Open はエンジンとの対話を始める（`usi` → `usiok` → **setoption** → `isready` → `readyok`）。
//
// ハンドシェイクが終わるまで返らない。**相手が USI を話さなければここで失敗する**ので、
// 以降のコードは「相手は USI を話す」と思ってよい。
//
// ⚠️ **options は `usiok` と `isready` の間で送る。ここでしか渡せない。**
// 置換表のサイズや評価関数の場所のように**エンジンの初期化に効く option は、
// `isready` より後に送っても間に合わない**（`isready` で確保・読み込みが走る）。
// 探索ごとに変えてよい option（MultiPV など）は Analyze / SetOption 側でよい。
//
// **エンジンが `usi` で宣言した option には、options に無くても既定値を送る**
// （将棋 UI の一般的な作法。何を送るかは plannedOptions）。
func Open(ctx context.Context, t Transport, options map[string]string) (*Session, error) {
	s := &Session{
		t:             t,
		lines:         make(chan string, 64),
		readErr:       make(chan error, 1),
		BestmoveGrace: HandshakeTimeout,
	}
	go s.read()

	ctx, cancel := context.WithTimeout(ctx, HandshakeTimeout)
	defer cancel()

	if err := s.sendCtx(ctx, "usi"); err != nil {
		s.Close()
		return nil, fmt.Errorf("usi: エンジンに usi を送れませんでした: %w", err)
	}
	if err := s.await(ctx, "usiok", func(line string) {
		switch {
		case strings.HasPrefix(line, "id name "):
			s.ID = strings.TrimSpace(strings.TrimPrefix(line, "id name "))
		case strings.HasPrefix(line, "id author "):
			s.Author = strings.TrimSpace(strings.TrimPrefix(line, "id author "))
		case strings.HasPrefix(line, "option "):
			if o, ok := usi.ParseOption(line); ok {
				s.Options = append(s.Options, o)
			}
		}
	}); err != nil {
		s.Close()
		return nil, fmt.Errorf("usi: エンジンが usiok を返しませんでした: %w", err)
	}

	for _, kv := range s.plannedOptions(options) {
		if err := s.SetOption(kv.name, kv.value); err != nil {
			s.Close()
			return nil, fmt.Errorf("usi: setoption を送れませんでした（%s）: %w", kv.name, err)
		}
		s.Applied = append(s.Applied, kv.name+"="+kv.value)
	}

	if err := s.sendCtx(ctx, "isready"); err != nil {
		s.Close()
		return nil, fmt.Errorf("usi: エンジンに isready を送れませんでした: %w", err)
	}
	if err := s.await(ctx, "readyok", nil); err != nil {
		s.Close()
		return nil, fmt.Errorf("usi: エンジンが readyok を返しませんでした: %w", err)
	}
	return s, nil
}

// optionValue は送る予定の option 1 つぶん。
type optionValue struct{ name, value string }

// plannedOptions は `isready` の前に送る setoption を並べる。
//
// **エンジンが宣言した option には、変えていなくても既定値を送る。** これが将棋 UI の
// 一般的な作法で、エンジン側は「UI が設定した状態」で `isready` に入れる。
// 送らないと、エンジンの内部既定と UI が思っている値が食い違ったままになりうる。
//
// 順序は**エンジンが宣言した順**。エンジンによっては前の option が後の option の
// 意味を変える（評価関数の種類を決めてからそのパスを渡す等）ので、勝手に並べ替えない。
//
// 送らないものが 2 つある:
//
//   - ⚠️ **button。** 値を持たず、送ること自体が「押した」という動作になる
//     （"Clear Hash" など）。既定値の送信で押してしまうと副作用が出る
//   - **値が空になるもの。** `default` の宣言が無い、または空。送っても意味が無いうえ、
//     SetOption が値なしの行（＝button と同じ形）を送ってしまう
//
// 宣言に無い名前が overrides にあれば、**それも送る**（名前順で最後に）。
// エンジンが宣言していない option を受け付けることがあり、
// **ユーザーが設定ファイルに明示的に書いたものを黙って捨てない**ため。
func (s *Session) plannedOptions(overrides map[string]string) []optionValue {
	out := make([]optionValue, 0, len(s.Options)+len(overrides))
	used := make(map[string]bool, len(overrides))

	for _, o := range s.Options {
		if o.IsButton() {
			continue
		}
		value := o.Default
		if v, ok := overrides[o.Name]; ok {
			value = v
			used[o.Name] = true
		}
		if value == "" {
			continue
		}
		out = append(out, optionValue{name: o.Name, value: value})
	}

	rest := make([]string, 0, len(overrides))
	for name := range overrides {
		if !used[name] {
			rest = append(rest, name)
		}
	}
	for _, name := range slices.Sorted(slices.Values(rest)) {
		out = append(out, optionValue{name: name, value: overrides[name]})
	}
	return out
}

// NewGame は `usinewgame` を送る。
//
// **`readyok` のあと、最初の `position` の前に送る**（将棋 UI の作法。ShogiHome の
// 実機の並びもこの順）。エンジンはこれを合図に局面ごとの状態（置換表・履歴・
// 学習の蓄積）を捨てる。送らないと、**前の局面の探索結果を引きずったまま次を読む**
// エンジンがある。
//
// ⚠️ **1 局につき 1 回。** 同じ対局の指し手を追いかけるあいだは送らない
// （送るたびに置換表が捨てられ、読みの蓄積が無駄になる）。独立した局面を 1 つずつ
// 解析する用途では、局面ごとに 1 回でよい。
//
// **Open では送っていない。** 「繋ぐ」と「新しい対局を始める」は別の判断で、
// 対局を続ける使い方（同じ接続で指し手を進める）を塞がないため。
func (s *Session) NewGame() error { return s.send("usinewgame") }

// SetOption は `setoption name <name> value <value>` を送る。
// value が空なら値なしの option として送る（button を押すのはこの形）。
func (s *Session) SetOption(name, value string) error {
	if value == "" {
		return s.send("setoption name " + name)
	}
	return s.send("setoption name " + name + " value " + value)
}

// GoOptions は 1 回の探索の指定。
type GoOptions struct {
	// Movetime はこの局面を考える時間。0 なら stop まで考えさせる。
	//
	// ⚠️ **`go movetime` としては送らない。** 解釈しないエンジンがあるため、
	// 常に `go infinite` で投げて**この層が期限で stop を送る**（下記 Analyze）。
	// 「時間はこちらが握る」ほうがエンジンによらず揃う。
	Movetime time.Duration
	// MultiPV は候補手をいくつ出させるか。0/1 なら送らない。
	//
	// ⚠️ **対応していないエンジンでは無視される。** 候補が 1 本しか返らないことを
	// 異常扱いしないこと。
	MultiPV int
}

// Result は 1 回の探索の結末。
type Result struct {
	// Bestmove は `bestmove` の手（"7g7f" / "resign" / "win"）。
	Bestmove string
	// Ponder は `bestmove ... ponder <手>`。
	Ponder string
	// Stopped は時間切れ・キャンセルで打ち切ったか。
	//
	// **打ち切りは失敗ではない。** そこまでの info は届いている。
	Stopped bool
}

// Analyze は 1 局面を解析する（`position` → `go` → info… → `bestmove`）。
//
// sfen は**局面全体の SFEN**（盤面・手番・持ち駒・手数）。指し手の履歴を渡したい場合は
// `position sfen <sfen> moves ...` の moves 部分を sfen 引数の末尾に含めること。
//
// info はエンジンが info 行を出すたびに呼ばれる（nil 可）。
// **読み取り goroutine から呼ばれる**ので、UI へ流すならイベント経由にすること。
//
// ctx をキャンセルするか Movetime が過ぎると `stop` を送り、**エンジンが返す
// bestmove を待って**返る（そこまでの結果は使える）。
func (s *Session) Analyze(ctx context.Context, sfen string, opt GoOptions, info func(usi.Info)) (Result, error) {
	if opt.MultiPV > 1 {
		// **対応していないエンジンは黙って無視する**ので、送るだけ送ってよい。
		if err := s.SetOption("MultiPV", fmt.Sprint(opt.MultiPV)); err != nil {
			return Result{}, err
		}
	}
	// ⚠️ **前の探索の残りを捨ててから始める。**
	//
	// エンジンは bestmove を出したあとにも info を吐くことがあり、それを溜めたままに
	// すると 2 つの害がある:
	//
	//   - 前の局面の評価値を、今の局面のものとして呼び出し側に渡してしまう
	//   - **溜まりきると、こちらが読まないせいでエンジンの書き込みがブロックする。**
	//     エンジンの出力が詰まると `stop` にも応答できなくなり、次の探索ごと固まる
	//     （実際にこれで固まった）
	s.drain()

	if err := s.sendCtx(ctx, "position sfen "+sfen); err != nil {
		return Result{}, err
	}

	// ⚠️ **go movetime に頼らない**（GoOptions.Movetime の注記を参照）。
	if opt.Movetime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opt.Movetime)
		defer cancel()
	}
	// **常に infinite で投げ、打ち切りはこちらの stop で行う。**
	// エンジンが自分で読み切って先に bestmove を返すのは正常（固定深さのエンジンなど）。
	if err := s.sendCtx(ctx, "go infinite"); err != nil {
		return Result{}, err
	}

	stopped := false
	for {
		select {
		case <-ctx.Done():
			if stopped {
				// 既に stop 済みなのに bestmove が来ない。**待ち続けない**
				// （相手が壊れている可能性がある）。
				return Result{Stopped: true}, fmt.Errorf("usi: エンジンが bestmove を返しません")
			}
			stopped = true
			if err := s.send("stop"); err != nil {
				return Result{Stopped: true}, err
			}
			// bestmove を待つための猶予。ここで返ってしまうと、次の position が
			// 前の探索の bestmove と混ざる。
			grace := s.BestmoveGrace
			if grace <= 0 {
				grace = HandshakeTimeout
			}
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), grace)
			defer cancel()

		case err := <-s.readErr:
			return Result{Stopped: stopped}, fmt.Errorf("usi: エンジンとの通信が切れました: %w", err)

		case line := <-s.lines:
			if in, ok := usi.ParseInfo(line); ok {
				if info != nil {
					info(in)
				}
				continue
			}
			if move, ponder, ok := usi.ParseBestmove(line); ok {
				return Result{Bestmove: move, Ponder: ponder, Stopped: stopped}, nil
			}
			// 知らない行は読み飛ばす。
		}
	}
}

// MateOptions は詰み探索の条件。
type MateOptions struct {
	// Limit は考えさせる上限。0 なら stop まで（`go mate infinite`）。
	//
	// ⚠️ **こちらは `go mate <ms>` としてエンジンにも伝える**（`go movetime` を
	// 送らない Analyze とは逆）。**詰み探索では「どれだけ考えたか」が答えの意味を
	// 変える**（時間切れは「詰みなし」ではない）ので、**エンジン自身に
	// 「時間切れだった」と言わせる**ほうが正しい。期限が来たら `stop` も送る。
	Limit time.Duration
}

// MateResult は詰み探索の結末。
type MateResult struct {
	// Kind は答えの種類（詰みあり / 詰みなし / 時間切れ / 非対応）。
	//
	// ⚠️ **「詰みなし」と「時間切れ」を同じ扱いにしないこと。** 後者は
	// **分からなかっただけ**で、長い時間で投げ直せば答えが変わる。
	Kind usi.Checkmate
	// Moves は詰み手順（USI 表記。攻方・玉方が交互）。詰みが無ければ空。
	//
	// ⚠️ **1 手しか返さないエンジンもある**（`bestmove` で答えるエンジンを
	// 含む）。**手数を数えて「何手詰」と言い切らないこと。**
	Moves []string
	// Stopped はこちらから打ち切ったか。**打ち切りは失敗ではない。**
	Stopped bool
}

// Mate は詰み探索（`position` → `go mate` → info… → `checkmate`）。
//
// **詰将棋を解かせる口。** `Analyze` と分けてあるのは、**答えの形がそもそも違う**から
// （あちらは「最善手と評価値」、こちらは「詰むか否かと、詰むならその手順」）。
//
// ⚠️ **`bestmove` で答えるエンジンも受ける**（2026-09-12 に実測）。USI の仕様は
// `checkmate` だが、やねうら王系は `go mate` に対して **`bestmove <手>` を返す**。
// **どちらも終わりの合図として扱うこと** —— 片方しか見ていないと、相手によって
// **黙って返ってこない**（一番たちの悪い壊れ方）。
//
// ⚠️ **詰将棋エンジンは通常の `go` に答えないことがある**（KomoringHeights は
// `bestmove resign` を返す）。**同じエンジンを通常解析にも使えると思わないこと。**
//
// info はエンジンが info 行を出すたびに呼ばれる（nil 可）。`score mate` が入る。
func (s *Session) Mate(ctx context.Context, sfen string, opt MateOptions, info func(usi.Info)) (MateResult, error) {
	// ⚠️ **Analyze と同じく、前の探索の残りを捨ててから始める。**
	s.drain()

	if err := s.sendCtx(ctx, "position sfen "+sfen); err != nil {
		return MateResult{}, err
	}

	limit := "infinite"
	if opt.Limit > 0 {
		limit = fmt.Sprint(opt.Limit.Milliseconds())
		var cancel context.CancelFunc
		// **エンジンの自己申告より少し待つ。** 期限ちょうどで打ち切ると、
		// エンジンが「timeout」と言おうとしているところを奪ってしまう。
		ctx, cancel = context.WithTimeout(ctx, opt.Limit+HandshakeTimeout)
		defer cancel()
	}
	if err := s.sendCtx(ctx, "go mate "+limit); err != nil {
		return MateResult{}, err
	}

	stopped := false
	for {
		select {
		case <-ctx.Done():
			if stopped {
				return MateResult{Stopped: true}, fmt.Errorf("usi: エンジンが checkmate を返しません")
			}
			stopped = true
			if err := s.send("stop"); err != nil {
				return MateResult{Stopped: true}, err
			}
			grace := s.BestmoveGrace
			if grace <= 0 {
				grace = HandshakeTimeout
			}
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), grace)
			defer cancel()

		case err := <-s.readErr:
			return MateResult{Stopped: stopped}, fmt.Errorf("usi: エンジンとの通信が切れました: %w", err)

		case line := <-s.lines:
			if in, ok := usi.ParseInfo(line); ok {
				if info != nil {
					info(in)
				}
				continue
			}
			if kind, moves, ok := usi.ParseCheckmate(line); ok {
				return MateResult{Kind: kind, Moves: moves, Stopped: stopped}, nil
			}
			// ⚠️ **`bestmove` でも終わる**（上の注記）。`resign` は「詰みなし」。
			if move, _, ok := usi.ParseBestmove(line); ok {
				if move == "resign" || move == "win" {
					return MateResult{Kind: usi.CheckmateNone, Stopped: stopped}, nil
				}
				return MateResult{Kind: usi.CheckmateFound, Moves: []string{move}, Stopped: stopped}, nil
			}
			// 知らない行は読み飛ばす。
		}
	}
}

// Close はエンジンを終わらせる（`quit` を送ってから後始末）。
func (s *Session) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	// 返事は待たない（既に死んでいることもある）。
	_ = s.write("quit")
	if s.t.Close != nil {
		return s.t.Close()
	}
	return nil
}

// read はエンジンの出力を 1 行ずつ流す。
func (s *Session) read() {
	sc := bufio.NewScanner(s.t.In)
	// 読み筋が長いエンジンがあるので、既定の 64KB より広く取る。
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		s.lines <- strings.TrimSpace(sc.Text())
	}
	err := sc.Err()
	if err == nil {
		err = io.EOF
	}
	s.readErr <- err
}

// drain は溜まっている行を捨てる（前の探索の残り）。
func (s *Session) drain() {
	for {
		select {
		case <-s.lines:
		default:
			return
		}
	}
}

// await は目的の行が来るまで読み進める。each には途中の行が渡る。
func (s *Session) await(ctx context.Context, want string, each func(string)) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-s.readErr:
			return err
		case line := <-s.lines:
			if line == want {
				return nil
			}
			if each != nil {
				each(line)
			}
		}
	}
}

// sendCtx は 1 行送る。**書き込み自体が返らないことがある**ので ctx で切れるようにしてある。
//
// ⚠️ エンジンが生きているのに標準入力を読まなくなると（ハングしたエンジン）、
// パイプのバッファが埋まった時点で書き込みがブロックしたまま返らない。
// **黙って固まるのが一番たちが悪い**ので、期限で諦めて理由を返す。
//
// 諦めた場合、書き込み中の goroutine はパイプが閉じるまで残る。Exec は
// `exec.CommandContext` でプロセスを起こしているので、ctx のキャンセルで
// プロセスごと片付く。
func (s *Session) sendCtx(ctx context.Context, line string) error {
	done := make(chan error, 1)
	go func() { done <- s.send(line) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// **両方すぐ揃っていると select はどちらを選ぶか決まらない。**
		// 送れているならそちらを採る（期限切れの ctx で呼ばれても、届いた事実は変わらない）。
		select {
		case err := <-done:
			return err
		default:
		}
		return fmt.Errorf("usi: エンジンがコマンドを受け取りません（%s）: %w", line, ctx.Err())
	}
}

// send は 1 行送る。**閉じた後は送らない。**
func (s *Session) send(line string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("usi: エンジンとの接続は閉じています")
	}
	return s.write(line)
}

func (s *Session) write(line string) error {
	_, err := io.WriteString(s.t.Out, line+"\n")
	return err
}
