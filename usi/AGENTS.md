# usi / usi/client パッケージ

USI 仕様（`usi`）と、USI エンジンと話すクライアント側（`usi/client`。＝将棋 UI 側。利用側は ikkyoku）。
**理由・経緯・実機で踏んだ不具合はコードのコメントに書いてある。** ここには要点と置き場所だけを置く。

| ファイル | 中身 |
|---|---|
| `usi.go` | マス座標（内部 (x,y) は 1..9、USI の筋 = `10-x`）と手（`Move` / `ParseMove`）。**ホットパス寄り** |
| `protocol.go` | エンジンが返す行の読み取り（`ParseInfo` / `ParseOption` / `ParseBestmove` / `ParseCheckmate`） |
| `client/client.go` | セッション（`Open` / `NewGame` / `Analyze` / `Mate` / `SetOption` / `Close`） |
| `client/exec.go` | 実行ファイルの起動（`Exec`） |
| `client/exec_windows.go` / `exec_other.go` | 起動時にコンソール窓を出さない（`hideConsole`。Windows 以外は何もしない） |

## usi で崩さないこと

- **知らないトークンは読み飛ばす**（未知の語で行ごと捨てると、そのエンジンの info が全部読めなくなる）
- **書かれなかった項目は `Has*` で区別する**（`score cp 0` と score が無い行は別）
- **option 行は位置ではなく語で区切る**（名前も既定値も空白を含みうる）
- **語が `checkmate` だけの行も答え**（既に詰んでいる局面。`ParseCheckmate` のコメント）
- **JS 側（`web/`）に対応物は無い**（USI を話すのは Go 側だけ）

## usi/client で崩さないこと

どれも `client/client.go` の該当箇所のコメントに理由がある。

- **「best を問い合わせる同期 API」に丸めない**（MultiPV・info の逐次・stop 途中の結果が消える。パッケージ冒頭）
- **宣言された option は、変えていなくても既定値を宣言順に送る。** button と値が空のものは送らない（`plannedOptions`）
- **option は `usiok` と `isready` の間で送る**（`Open`）
- **`usinewgame` は `readyok` のあと・最初の `position` の前に、1 局 1 回**（`NewGame`。Open では送らない）
- **通常の解析は `go movetime` を使わず `go infinite` ＋ こちらの `stop`**（`GoOptions.Movetime`）
- **詰み探索は逆に `go mate <ms>` で時間を伝える。** 時間切れは「詰みなし」ではない（`MateOptions.Limit`）
- **詰み探索は `checkmate` と `bestmove` のどちらでも終わる**（やねうら王系は `bestmove`。`Mate`）
- **詰み探索の余裕 `MateGrace` は短く**（答えないエンジンでの待ち時間がそのまま決まる）
- **`Analyze` と `Mate` を 1 つに畳まない**（答えの形が違う）
- **探索の前に前の残りを捨てる**（`drain`。溜めるとエンジンが詰まって stop も効かなくなる）
- **`stop` のあとは `bestmove` を待つ。`BestmoveGrace` を 0 にしない**
- **送信は ctx で切れるようにする**（`sendCtx`。標準入力を読まないエンジンで固まらない）
- **標準エラーは捨てる**（`Exec`。読まずに繋ぐとエンジンが止まる）。作業ディレクトリは実行ファイルの場所
- **Windows ではエンジンのコンソール窓を出さない**（`hideConsole`。GUI アプリから起こすと窓が出る。`wails3 dev` では出ないので気づけない）
- **テストは実エンジンを要求しない**（`io.Pipe` 越しの偽エンジン）
