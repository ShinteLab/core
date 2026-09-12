package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/ShinteLab/core/usi"
)

// fakeEngine は USI を話すふりをするテスト用の相手。
//
// **実際のエンジンの実行ファイルを要求しないこと。** 手元にどのエンジンがあるかに
// テストが左右されると、壊れたときに原因が切り分けられない。
type fakeEngine struct {
	out io.Writer
	// Cmds はクライアントから届いたコマンド（検証用）。
	Cmds chan string
}

// say はエンジンから行を送る。
func (e *fakeEngine) say(lines ...string) {
	for _, l := range lines {
		_, _ = io.WriteString(e.out, l+"\n")
	}
}

// newFake は on でコマンドに応じる相手を立てる。
// on は読み取り goroutine から呼ばれるので、**待つ処理は goroutine に逃がすこと。**
func newFake(on func(e *fakeEngine, cmd string)) (Transport, *fakeEngine) {
	cmdR, cmdW := io.Pipe() // クライアント → エンジン
	outR, outW := io.Pipe() // エンジン → クライアント

	e := &fakeEngine{out: outW, Cmds: make(chan string, 64)}
	go func() {
		defer outW.Close()
		sc := bufio.NewScanner(cmdR)
		for sc.Scan() {
			line := sc.Text()
			e.Cmds <- line
			if line == "quit" {
				return
			}
			on(e, line)
		}
	}()

	return Transport{
		In:  outR,
		Out: cmdW,
		Close: func() error {
			_ = cmdW.Close()
			return outR.Close()
		},
	}, e
}

// handshake は usi/isready にだけ答える既定の応答。
func handshake(e *fakeEngine, cmd string) bool {
	switch cmd {
	case "usi":
		// **option は宣言しない。** 宣言すると既定値が送られるので、
		// option を主題にしないテストの期待値が揺れる（宣言する側は個別に立てる）。
		e.say("id name Fake Engine", "id author test", "usiok")
		return true
	case "isready":
		e.say("readyok")
		return true
	}
	return false
}

func openFake(t *testing.T, on func(e *fakeEngine, cmd string)) (*Session, *fakeEngine) {
	t.Helper()
	tr, e := newFake(func(e *fakeEngine, cmd string) {
		if handshake(e, cmd) {
			return
		}
		if on != nil {
			on(e, cmd)
		}
	})
	s, err := Open(context.Background(), tr, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, e
}

// ハンドシェイクで素性を拾えること。
func TestOpenReadsIdentity(t *testing.T) {
	tr, _ := newFake(func(e *fakeEngine, cmd string) {
		switch cmd {
		case "usi":
			e.say("id name Fake Engine", "id author test",
				"option name MultiPV type spin default 1 min 1 max 5", "usiok")
		case "isready":
			e.say("readyok")
		}
	})
	s, err := Open(context.Background(), tr, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if s.ID != "Fake Engine" {
		t.Errorf("ID = %q, want %q", s.ID, "Fake Engine")
	}
	if s.Author != "test" {
		t.Errorf("Author = %q, want %q", s.Author, "test")
	}
	// 宣言された option は構造化して持つ（**値の意味づけはしない**）。
	if len(s.Options) != 1 || s.Options[0].Name != "MultiPV" || s.Options[0].Default != "1" {
		t.Errorf("Options = %+v", s.Options)
	}
}

// ⚠️ **エンジンが宣言した option には、変えていなくても既定値を送る。**
// これが将棋 UI の一般的な作法で、送らないとエンジンの内部既定と
// UI が思っている値が食い違ったままになる。
func TestOpenSendsDeclaredDefaults(t *testing.T) {
	tr, e := newFake(func(e *fakeEngine, cmd string) {
		switch cmd {
		case "usi":
			e.say(
				"id name Fake Engine",
				"option name USI_Hash type spin default 256 min 1 max 1024",
				"option name USI_Ponder type check default false",
				// ⚠️ button は押すだけなので**送らない**（送ると "Clear Hash" が走る）。
				"option name Clear Hash type button",
				// 既定値が空のものも送らない（値なしの行は button と同じ形になる）。
				"option name BookDir type string default <empty>",
				"option name NoDefault type string",
				"usiok",
			)
		case "isready":
			e.say("readyok")
		}
	})
	s, err := Open(context.Background(), tr, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// **宣言された順**に送る（前の option が後の option の意味を変えるエンジンがある）。
	want := []string{
		"usi",
		"setoption name USI_Hash value 256",
		"setoption name USI_Ponder value false",
		"isready",
	}
	for _, w := range want {
		select {
		case got := <-e.Cmds:
			if got != w {
				t.Fatalf("送った行 = %q, want %q", got, w)
			}
		case <-time.After(time.Second):
			t.Fatalf("%q が送られてきません", w)
		}
	}
}

// 設定した値は既定値に優先し、**宣言された位置で**送られること。
func TestOpenOverridesDeclaredDefaults(t *testing.T) {
	tr, e := newFake(func(e *fakeEngine, cmd string) {
		switch cmd {
		case "usi":
			e.say("id name Fake Engine",
				"option name USI_Hash type spin default 256 min 1 max 4096",
				"option name Threads type spin default 1 min 1 max 64",
				"usiok")
		case "isready":
			e.say("readyok")
		}
	})
	s, err := Open(context.Background(), tr, map[string]string{
		"USI_Hash": "1024",
		// 宣言に無い名前も**捨てずに送る**（明示的に書いたものを黙って落とさない）。
		"EvalDir": "eval",
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	want := []string{
		"usi",
		"setoption name USI_Hash value 1024",
		"setoption name Threads value 1",
		"setoption name EvalDir value eval",
		"isready",
	}
	for _, w := range want {
		select {
		case got := <-e.Cmds:
			if got != w {
				t.Fatalf("送った行 = %q, want %q", got, w)
			}
		case <-time.After(time.Second):
			t.Fatalf("%q が送られてきません", w)
		}
	}
	// 何を送ったかは残す（option には応答が無いので、確かめる手掛かりがこれだけ）。
	if len(s.Applied) != 3 {
		t.Errorf("Applied = %v", s.Applied)
	}
}

// ⚠️ **option は usiok と isready の間に送ること。**
//
// 置換表のサイズや評価関数の場所は `isready` で確保・読み込みが走るので、
// **後から送っても間に合わない**。しかもエンジンは黙って既定値で動くので、
// 順序を間違えても画面では気づけない。
func TestOpenSendsOptionsBetweenUsiokAndIsready(t *testing.T) {
	tr, e := newFake(func(e *fakeEngine, cmd string) { handshake(e, cmd) })
	s, err := Open(context.Background(), tr, map[string]string{
		"USI_Hash": "1024",
		"EvalDir":  "eval",
		"Threads":  "4",
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// 名前順で送る（map の走査順のままだと、エンジンによって結果が変わりうる）。
	want := []string{
		"usi",
		"setoption name EvalDir value eval",
		"setoption name Threads value 4",
		"setoption name USI_Hash value 1024",
		"isready",
	}
	for _, w := range want {
		select {
		case got := <-e.Cmds:
			if got != w {
				t.Fatalf("送った行 = %q, want %q", got, w)
			}
		case <-time.After(time.Second):
			t.Fatalf("%q が送られてきません", w)
		}
	}
}

// ⚠️ **USI を話さない相手で固まらないこと。** 待ち続けると呼び出し側が黙って止まる。
//
// 起動直後に落ちたプロセスを模す（標準入力への書き込みは OS のバッファに入るので
// 通るが、標準出力は即 EOF）。
func TestOpenFailsWhenEngineDies(t *testing.T) {
	outR, outW := io.Pipe()
	outW.Close()

	done := make(chan error, 1)
	go func() {
		_, err := Open(context.Background(), Transport{In: outR, Out: io.Discard}, nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("応答が無いのに Open が成功しました")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Open が返りません（応答の無い相手で固まっている）")
	}
}

// position → go → info… → bestmove が通ること。
func TestAnalyzeStreamsInfoThenBestmove(t *testing.T) {
	s, _ := openFake(t, func(e *fakeEngine, cmd string) {
		if strings.HasPrefix(cmd, "go") {
			go e.say(
				"info depth 1 score cp 10 nodes 100 pv 7g7f",
				"info depth 2 score cp 20 nodes 400 pv 2g2f",
				"bestmove 2g2f ponder 3c3d",
			)
		}
	})

	var got []usi.Info
	r, err := s.Analyze(context.Background(), "startsfen b - 1", GoOptions{}, func(in usi.Info) {
		got = append(got, in)
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.Bestmove != "2g2f" || r.Ponder != "3c3d" {
		t.Errorf("Result = %+v", r)
	}
	if r.Stopped {
		t.Error("自分から返ってきたのに Stopped が立っています")
	}
	if len(got) != 2 || got[1].Depth != 2 || got[1].ScoreCP != 20 {
		t.Errorf("info の受け取りが違います: %+v", got)
	}
}

// 送っている行が USI として正しいこと。
// ⚠️ **go movetime には頼らない**（解釈しないエンジンがある）。常に go infinite。
func TestAnalyzeSendsExpectedCommands(t *testing.T) {
	s, e := openFake(t, func(e *fakeEngine, cmd string) {
		if strings.HasPrefix(cmd, "go") {
			go e.say("bestmove 7g7f")
		}
	})
	if _, err := s.Analyze(context.Background(), "SFEN b - 1", GoOptions{MultiPV: 3}, nil); err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	want := []string{
		"usi", "isready",
		"setoption name MultiPV value 3",
		"position sfen SFEN b - 1",
		"go infinite",
	}
	for _, w := range want {
		select {
		case got := <-e.Cmds:
			if got != w {
				t.Fatalf("送った行 = %q, want %q", got, w)
			}
		case <-time.After(time.Second):
			t.Fatalf("%q が送られてきません", w)
		}
	}
}

// MultiPV が 1 以下なら setoption を送らないこと（既定を勝手に触らない）。
func TestAnalyzeSkipsMultiPVWhenNotAsked(t *testing.T) {
	s, e := openFake(t, func(e *fakeEngine, cmd string) {
		if strings.HasPrefix(cmd, "go") {
			go e.say("bestmove 7g7f")
		}
	})
	if _, err := s.Analyze(context.Background(), "SFEN b - 1", GoOptions{}, nil); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	<-e.Cmds // usi
	<-e.Cmds // isready
	if got := <-e.Cmds; got != "position sfen SFEN b - 1" {
		t.Errorf("setoption を送っています: %q", got)
	}
}

// ⚠️ **打ち切りは stop を送って bestmove を待つ。** ここで待たずに返ると、
// 次の position が前の探索の bestmove と混ざる。
func TestAnalyzeStopsOnDeadlineAndWaitsForBestmove(t *testing.T) {
	s, _ := openFake(t, func(e *fakeEngine, cmd string) {
		switch {
		case strings.HasPrefix(cmd, "go"):
			// 自分からは終わらない相手（stop されるまで読み続ける）。
			go e.say("info depth 1 score cp 5 nodes 10 pv 7g7f")
		case cmd == "stop":
			go e.say("bestmove 7g7f")
		}
	})

	r, err := s.Analyze(context.Background(), "SFEN b - 1", GoOptions{Movetime: 150 * time.Millisecond}, nil)
	if err != nil {
		t.Fatalf("打ち切りをエラーにしないこと: %v", err)
	}
	if r.Bestmove != "7g7f" {
		t.Errorf("bestmove = %q", r.Bestmove)
	}
	if !r.Stopped {
		t.Error("打ち切ったのに Stopped が false です")
	}
}

// ctx のキャンセルでも同じこと。
func TestAnalyzeStopsOnContextCancel(t *testing.T) {
	s, _ := openFake(t, func(e *fakeEngine, cmd string) {
		if cmd == "stop" {
			go e.say("bestmove 5i5h")
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	r, err := s.Analyze(ctx, "SFEN b - 1", GoOptions{}, nil)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.Bestmove != "5i5h" || !r.Stopped {
		t.Errorf("Result = %+v", r)
	}
}

// stop したのに bestmove を返さない相手で永久に待たないこと。
//
// **stop まで到達させてから諦めさせる**（ctx を最初から切ると position の送信で
// 落ちてしまい、この経路を通らない）。
func TestAnalyzeGivesUpWhenBestmoveNeverComes(t *testing.T) {
	s, _ := openFake(t, nil) // go にも stop にも答えない
	s.BestmoveGrace = 200 * time.Millisecond

	done := make(chan error, 1)
	go func() {
		_, err := s.Analyze(context.Background(), "SFEN b - 1",
			GoOptions{Movetime: 50 * time.Millisecond}, nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("bestmove が来ないのにエラーになりませんでした")
		}
		if !strings.Contains(err.Error(), "bestmove") {
			t.Errorf("理由が伝わりません: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Analyze が返りません")
	}
}

// 探索中にエンジンが落ちたら、待ち続けずにエラーで返ること。
func TestAnalyzeFailsWhenEngineDiesMidSearch(t *testing.T) {
	s, _ := openFake(t, func(e *fakeEngine, cmd string) {
		if strings.HasPrefix(cmd, "go") {
			// 何も返さずに口が閉じる（= プロセスが落ちた）。
			go func() {
				time.Sleep(30 * time.Millisecond)
				if c, ok := e.out.(io.Closer); ok {
					_ = c.Close()
				}
			}()
		}
	})
	done := make(chan error, 1)
	go func() {
		_, err := s.Analyze(context.Background(), "SFEN b - 1", GoOptions{}, nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("エンジンが落ちたのにエラーになりませんでした")
		}
		if !strings.Contains(err.Error(), "通信") {
			t.Errorf("理由が伝わりません: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Analyze が返りません")
	}
}

// ⚠️ **前の探索の残りを次に持ち越さないこと。**
//
// bestmove のあとに info を吐くエンジンがある。溜めたままにすると前の局面の評価値を
// 今の局面のものとして渡してしまううえ、**溜まりきるとエンジンの書き込みが詰まって
// stop にも応答できなくなる**（実際にこれで固まった）。
func TestAnalyzeDropsLeftoverInfoFromPreviousSearch(t *testing.T) {
	s, _ := openFake(t, func(e *fakeEngine, cmd string) {
		if strings.HasPrefix(cmd, "go") {
			go func() {
				e.say("info depth 9 score cp 999 nodes 1 pv 1a1b", "bestmove 1a1b")
				// bestmove のあとに漏れてくる残り。
				e.say("info depth 9 score cp 888 nodes 2 pv 9i9h")
			}()
		}
	})

	if _, err := s.Analyze(context.Background(), "SFEN b - 1", GoOptions{}, nil); err != nil {
		t.Fatalf("1 回目: %v", err)
	}
	// 残りが届くのを待ってから 2 回目を始める。
	time.Sleep(50 * time.Millisecond)

	var got []usi.Info
	if _, err := s.Analyze(context.Background(), "SFEN2 b - 1", GoOptions{}, func(in usi.Info) {
		got = append(got, in)
	}); err != nil {
		t.Fatalf("2 回目: %v", err)
	}
	for _, in := range got {
		if in.ScoreCP == 888 {
			t.Fatal("前の探索の info が 2 回目に混ざっています")
		}
	}
}

// 何本も続けて解析しても詰まらないこと（上の溜まりが起きると数回で固まる）。
func TestAnalyzeRepeatedSearchesDoNotStall(t *testing.T) {
	s, _ := openFake(t, func(e *fakeEngine, cmd string) {
		if strings.HasPrefix(cmd, "go") {
			go func() {
				for i := range 40 {
					e.say("info depth 1 score cp " + fmt.Sprint(i) + " nodes 1 pv 7g7f")
				}
				e.say("bestmove 7g7f")
				for i := range 40 {
					e.say("info depth 2 score cp " + fmt.Sprint(i) + " nodes 2 pv 7g7f")
				}
			}()
		}
	})
	for i := range 5 {
		done := make(chan error, 1)
		go func() {
			_, err := s.Analyze(context.Background(), "SFEN b - 1", GoOptions{}, nil)
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("%d 回目: %v", i+1, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%d 回目で固まりました", i+1)
		}
	}
}

// ⚠️ **`usinewgame` は `readyok` のあと、最初の `position` の前。**
// 落とすと、前の局面の探索結果を引きずったまま次を読むエンジンがある。
//
// **Open では送らない**（「繋ぐ」と「新しい対局を始める」は別の判断で、
// 同じ接続で指し手を進める使い方を塞がないため）。
func TestNewGameIsSeparateFromOpen(t *testing.T) {
	s, e := openFake(t, func(e *fakeEngine, cmd string) {
		if strings.HasPrefix(cmd, "go") {
			go e.say("bestmove 7g7f")
		}
	})
	if err := s.NewGame(); err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if _, err := s.Analyze(context.Background(), "SFEN b - 1", GoOptions{}, nil); err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	want := []string{"usi", "isready", "usinewgame", "position sfen SFEN b - 1", "go infinite"}
	for _, w := range want {
		select {
		case got := <-e.Cmds:
			if got != w {
				t.Fatalf("送った行 = %q, want %q", got, w)
			}
		case <-time.After(time.Second):
			t.Fatalf("%q が送られてきません", w)
		}
	}
}

// 閉じたあとに送らないこと。
func TestSendAfterCloseFails(t *testing.T) {
	s, _ := openFake(t, nil)
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := s.SetOption("Threads", "4"); err == nil {
		t.Error("閉じたあとに送れてしまいました")
	}
	// 二重の Close は何もしない。
	if err := s.Close(); err != nil {
		t.Errorf("2 回目の Close: %v", err)
	}
}

// 起動できない実行ファイルは、その場で理由付きで失敗すること。
func TestExecFailsForMissingBinary(t *testing.T) {
	_, err := Exec(context.Background(), "no-such-engine-binary.exe")
	if err == nil {
		t.Fatal("存在しない実行ファイルで成功しました")
	}
	if !strings.Contains(err.Error(), "エンジンを起動できませんでした") {
		t.Errorf("理由が伝わりません: %v", err)
	}
}

// 詰み探索（`go mate` → `checkmate`）。**KomoringHeights で実測した形をそのまま偽物に言わせる。**
func TestMateFound(t *testing.T) {
	s, e := openFake(t, func(e *fakeEngine, cmd string) {
		if strings.HasPrefix(cmd, "go mate") {
			e.say("info time 1 nodes 3 score mate 1 pv G*5b",
				"checkmate B*4d 1a1b R*2b 1b1a R*1b")
		}
	})

	var infos int
	r, err := s.Mate(context.Background(), "4k4/9/4P4/9/9/9/9/9/9 b G17p 1",
		MateOptions{Limit: time.Second}, func(usi.Info) { infos++ })
	if err != nil {
		t.Fatalf("Mate: %v", err)
	}
	if r.Kind != usi.CheckmateFound {
		t.Fatalf("Kind = %v, want 詰みあり", r.Kind)
	}
	if len(r.Moves) != 5 || r.Moves[0] != "B*4d" || r.Moves[4] != "R*1b" {
		t.Errorf("Moves = %v（手順が全部入っていない）", r.Moves)
	}
	if infos == 0 {
		t.Error("info が届いていません")
	}
	// ⚠️ **`go mate <ms>` として送ること**（時間切れをエンジン自身に言わせる）。
	var sawGo bool
	for len(e.Cmds) > 0 {
		if cmd := <-e.Cmds; strings.HasPrefix(cmd, "go mate ") && cmd != "go mate infinite" {
			sawGo = true
		}
	}
	if !sawGo {
		t.Error("go mate <ms> が送られていません")
	}
}

// ⚠️ **「詰みなし」と「時間切れ」と「非対応」は別物**（時間切れは投げ直せば変わる）。
func TestMateAnswers(t *testing.T) {
	for _, tt := range []struct {
		line string
		want usi.Checkmate
	}{
		{"checkmate nomate", usi.CheckmateNone},
		{"checkmate timeout", usi.CheckmateTimeout},
		{"checkmate notimplemented", usi.CheckmateNotImplemented},
	} {
		t.Run(tt.line, func(t *testing.T) {
			s, _ := openFake(t, func(e *fakeEngine, cmd string) {
				if strings.HasPrefix(cmd, "go mate") {
					e.say(tt.line)
				}
			})
			r, err := s.Mate(context.Background(), "4k4/9/9/9/9/9/9/9/9 b 18p 1",
				MateOptions{Limit: time.Second}, nil)
			if err != nil {
				t.Fatalf("Mate: %v", err)
			}
			if r.Kind != tt.want {
				t.Errorf("Kind = %v, want %v", r.Kind, tt.want)
			}
			if len(r.Moves) != 0 {
				t.Errorf("Moves = %v（手順が無い答えなのに入っている）", r.Moves)
			}
		})
	}
}

// ⚠️ **`bestmove` で答えるエンジンも受けること**（やねうら王系は `go mate` に
// bestmove を返す。実測）。片方しか見ていないと**黙って返ってこない**。
func TestMateAcceptsBestmove(t *testing.T) {
	t.Run("手を返す", func(t *testing.T) {
		s, _ := openFake(t, func(e *fakeEngine, cmd string) {
			if strings.HasPrefix(cmd, "go mate") {
				e.say("info score mate 1 pv G*5b", "bestmove G*5b")
			}
		})
		r, err := s.Mate(context.Background(), "sfen", MateOptions{Limit: time.Second}, nil)
		if err != nil {
			t.Fatalf("Mate: %v", err)
		}
		if r.Kind != usi.CheckmateFound || len(r.Moves) != 1 || r.Moves[0] != "G*5b" {
			t.Errorf("Kind=%v Moves=%v", r.Kind, r.Moves)
		}
	})
	t.Run("投了は詰みなし", func(t *testing.T) {
		s, _ := openFake(t, func(e *fakeEngine, cmd string) {
			if strings.HasPrefix(cmd, "go mate") {
				e.say("bestmove resign")
			}
		})
		r, err := s.Mate(context.Background(), "sfen", MateOptions{Limit: time.Second}, nil)
		if err != nil {
			t.Fatalf("Mate: %v", err)
		}
		if r.Kind != usi.CheckmateNone {
			t.Errorf("Kind = %v, want 詰みなし", r.Kind)
		}
	})
}

// 答えないエンジンは打ち切れること（`stop` を送って諦める）。
func TestMateStops(t *testing.T) {
	s, e := openFake(t, func(e *fakeEngine, cmd string) {
		// go mate に何も答えない（黙って返ってこないエンジンの再現）。
		if cmd == "stop" {
			e.say("checkmate timeout")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	r, err := s.Mate(ctx, "sfen", MateOptions{}, nil)
	if err != nil {
		t.Fatalf("Mate: %v", err)
	}
	if !r.Stopped {
		t.Error("打ち切ったことが分かりません")
	}
	if r.Kind != usi.CheckmateTimeout {
		t.Errorf("Kind = %v, want 時間切れ", r.Kind)
	}
	var sawStop bool
	for len(e.Cmds) > 0 {
		if <-e.Cmds == "stop" {
			sawStop = true
		}
	}
	if !sawStop {
		t.Error("stop が送られていません")
	}
}
