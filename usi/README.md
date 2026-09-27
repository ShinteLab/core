# usi / usi/client パッケージ

USI 仕様（`usi`）と、USI エンジンと話すクライアント側（`usi/client`）。
パスは `core/` からの相対で書いている。

## usi パッケージ

内部座標 (x,y) は 1..9。USI は筋 = `10-x` の数字、段 = `'a'+y-1`。例: (3,7) ↔ `"7g"`。

| API | 内容 |
|---|---|
| `FormatSquare(x,y)` / `ParseSquare(s)` | マス座標の相互変換（範囲外は `ok=false`） |
| `Move` | 手の中立表現（`Resign` / `Drop`（打つ駒の大文字）/ `FromX,FromY` / `ToX,ToY` / `Promote`） |
| `ParseMove(s) (Move, bool)` | `"7g7f"` / `"7g7f+"` / `"P*5e"` / `"resign"` に対応 |
| `Move.String()` | USI 手文字列を生成 |

長さ 2 の文字列は移動元のみとして `From` に格納する（engine の従来挙動の互換）。

`protocol.go` はエンジンが返す行の読み取り。

| API | 内容 |
|---|---|
| `Info` / `ParseInfo(line)` | `info` 行。**書かれなかった項目は `Has*` で区別する**（`score cp 0` は互角という情報で、score が無い行とは意味が違う） |
| `Option` / `ParseOption(line)` | `usi` 応答の `option` 行。**名前も既定値も空白を含みうる**ので、位置ではなく語（`name`/`type`/`default`/`min`/`max`/`var`）で区切って読む。`default <empty>` は空文字に直す |
| `ParseBestmove(line)` | `bestmove` 行（`resign` / `win` もそのまま `move` に入る。解釈は呼び出し側） |
| `Checkmate` / `ParseCheckmate(line)` | `checkmate` 行（`go mate` の答え）。種類は `CheckmateFound` / `None` / `Timeout` / `NotImplemented`。**手順の無い `checkmate` は「詰みあり・手順 0 手」**（下記 usi/client） |

- **知らないトークンは読み飛ばす。** エンジンごとに独自の項目が付くので、
  未知の語で行ごと捨てると**そのエンジンの info が全部読めなくなる**
- ⚠️ **JS 側（`web/`）に対応物は無い。** USI を話すのはデスクトップアプリの Go 側で、
  ブラウザは構造化された結果しか見ないため。`AGENTS.md` の「Go と JS の 2 ソース」の対象外。
  **ブラウザから USI を扱う必要が出たら、そのときに揃えること**

## usi/client パッケージ

**USI エンジンと話すクライアント側**（＝将棋 UI 側）。繋ぎ先が「USI を話すプロセス」で
ありさえすれば中身を問わない（やねうら王・水匠 / prokishi / 自作エンジン）。
利用側は `ikkyoku`（Phase 4）。

| API | 内容 |
|---|---|
| `Transport` | エンジンとの入出力（`In`/`Out`/`Close`）。**プロセスかパイプかをこの層は知らない** |
| `Exec(ctx, path, args...)` | 実行ファイルを起こして `Transport` を返す。**作業ディレクトリは実行ファイルの場所**（評価関数・定跡を相対パスで読むエンジンが多い） |
| `Open(ctx, t, options)` | `usi` → `usiok` → **`setoption`** → `isready` → `readyok`。`ID` / `Author` / `Options`（宣言）/ `Applied`（送った内容）を持つ |
| `Session.Analyze(ctx, sfen, opt, info)` | `position` → `go infinite` → info… → `bestmove` |
| `Session.Mate(ctx, sfen, opt, info)` | **詰み探索**（2026-09-12）。`position` → `go mate` → info… → `checkmate`。答えは 4 通り（詰みあり / なし / **時間切れ** / 非対応） |
| `Session.NewGame()` | `usinewgame`。**`readyok` のあと、最初の `position` の前**（Open では送らない） |
| `Session.SetOption(name, value)` / `Close()` | |

**崩さないこと**:

- ⚠️ **「best を問い合わせる同期 API」に丸めない。** `GetBestMove(sfen) → 指し手` まで
  畳むと、**MultiPV が出せない・info の逐次ストリームを捨てる・stop で途中まで読んだ
  結果を使えない**。USI がステートフルなのは面倒に見えるが、**その面倒さがそのまま
  欲しい機能**
- ⚠️ **エンジンが宣言した option には、変えていなくても既定値を送る**（`plannedOptions`）。
  将棋 UI の一般的な作法で、送らないとエンジンの内部既定と UI が思っている値が
  食い違ったままになる。順序は**宣言された順**（前の option が後の option の意味を
  変えるエンジンがある）。送らないのは 2 つだけ:
  **button**（送ること自体が「押した」動作になる。"Clear Hash" など）と、
  **値が空になるもの**（値なしの行は button と同じ形になってしまう）
- ⚠️ **`usinewgame` を落とさない。** `readyok` → `usinewgame` → `position` → `go` が
  将棋 UI の並び（ShogiHome の実機もこの順）。送らないと**前の局面の探索結果を
  引きずったまま次を読む**エンジンがある。ただし**1 局につき 1 回**で、同じ対局の
  指し手を追いかけるあいだは送らない（送るたびに置換表が捨てられる）
- ⚠️ **`go movetime` を送らない。** 解釈しないエンジンがある（自作 `engine` がそれ）。
  **常に `go infinite` で投げ、期限が来たらこちらが `stop` を送る**
- ⚠️ **詰み探索だけは逆で、`go mate <ms>` と時間を伝える**（`Mate`。2026-09-12）。
  **時間切れは「詰みなし」ではない**ので、**エンジン自身に「timeout」と言わせる**
  ほうが正しい（こちらが stop で切ると、詰みが無いのか分からなかったのかが消える）
- ⚠️ **語が「checkmate」だけの行を捨てないこと**（2026-09-12 に実機で踏んだ）。
  **既に詰んでいる局面**に `go mate` を送ると、KomoringHeights は**手順の無い
  `checkmate`** を返す。2 語以上を要求していたせいで読み飛ばし、**返事を待ち続けて
  時間切れ**になっていた（画面には「エンジンが checkmate を返しません」と出た）
- ⚠️ **答えを待つ余裕は短く**（`MateGrace`。3 秒）。答えないエンジンに当たったときの
  待ち時間がそのままこれで決まる —— **3 秒の詰み探索で 20 秒待たされて「返しません」**
  では、何が起きたのか分からない
- ⚠️ **詰み探索は `bestmove` でも終わる。** USI の仕様は `checkmate` だが、
  **やねうら王系は `go mate` に `bestmove <手>` を返す**（2026-09-12 に実測）。
  **両方を終わりの合図として扱うこと** —— 片方しか見ていないと、相手によっては
  **黙って返ってこない**（`bestmove resign` は「詰みなし」）
- ⚠️ **詰将棋エンジンは通常の `go` に答えないことがある**（KomoringHeights は
  `bestmove resign` を返す。実測）。**同じエンジンを通常解析にも使えると思わないこと**
- ⚠️ **`Analyze` と `Mate` を 1 つに畳まないこと。** 答えの形がそもそも違う
  （最善手と評価値 / 詰むか否かとその手順）
- ⚠️ **探索を始める前に、前の探索の残りを捨てる**（`drain`）。bestmove のあとにも info を
  吐くエンジンがあり、溜めたままだと前の局面の評価値を今の局面のものとして渡すうえ、
  **溜まりきるとエンジンの書き込みが詰まって `stop` にも応答できなくなる**（実際に固まった）
- ⚠️ **`stop` のあとは `bestmove` を待つ**（`BestmoveGrace`）。待たずに返ると
  次の `position` が前の探索の bestmove と混ざる。**0 にしないこと**
- ⚠️ **書き込み自体が返らないことがある**（エンジンが標準入力を読まなくなった場合）ので、
  ハンドシェイクと探索の送信は ctx で切れるようにしてある（`sendCtx`）
- **option 行は解釈せずそのまま持つ。** 種類も既定値もエンジンごとに違う
- テストは**実際のエンジンの実行ファイルを要求しない**（`io.Pipe` 越しの偽エンジン）。
  手元にどのエンジンがあるかでテストが左右されると、壊れたときに切り分けられない
