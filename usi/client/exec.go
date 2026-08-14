package client

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
)

// Exec は USI エンジンの実行ファイルを起こして Transport を返す。
//
// **作業ディレクトリは実行ファイルの場所にする。** 将棋エンジンは評価関数・定跡を
// 相対パスで読むものが多く、呼び出し側のカレントディレクトリのままだと
// `isready` で黙って失敗する（エンジン側は「ファイルが無い」としか言わない）。
//
// 返る Transport の Close はプロセスの終了まで待つ。**呼ばないと残る。**
// 通常は Session.Close 経由で呼ばれる（`quit` を送ったあとに後始末される）。
func Exec(ctx context.Context, path string, args ...string) (Transport, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = filepath.Dir(path)

	out, err := cmd.StdoutPipe()
	if err != nil {
		return Transport{}, fmt.Errorf("usi: エンジンの標準出力を繋げませんでした: %w", err)
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		return Transport{}, fmt.Errorf("usi: エンジンの標準入力を繋げませんでした: %w", err)
	}
	// ⚠️ **標準エラーは捨てる。** 読まずに繋ぐとバッファが詰まってエンジンが止まる。
	// 内容が要るなら呼び出し側が cmd を組み立てる形に変えること。
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return Transport{}, fmt.Errorf("usi: エンジンを起動できませんでした（%s）: %w", path, err)
	}

	var once sync.Once
	var waitErr error
	return Transport{
		In:  out,
		Out: in,
		Close: func() error {
			once.Do(func() {
				// 標準入力を閉じると、行儀のよいエンジンは自分で終わる
				// （`quit` は Session.Close が先に送っている）。
				_ = in.Close()
				waitErr = cmd.Wait()
			})
			return waitErr
		},
	}, nil
}
