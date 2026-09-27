# sfen パッケージ

SFEN 仕様（駒文字マッピング・盤面の解析/生成・盤面の検証）。
駒コードの値と alloc の制約は `AGENTS.md` の「最重要の設計方針」を先に読むこと。
パスは `core/` からの相対で書いている。

| API | 内容 |
|---|---|
| `Letter(code) string` | 駒コード → SFEN 大文字。成駒はベース駒の文字を返す（engine の `PieceType.Mark()` と同挙動） |
| `ParsePieceLetter(c) (base, black)` | SFEN 1文字 → 駒コード + 手番。未知の文字は `NotFound`(99) |
| `ParseBoard(board, place) error` | 盤面部分（`/` 区切り9段）を走査し、駒のあるマスごとに `place(rank, file, base, black, promoted)` を呼ぶ |
| `FormatBoard(cell) string` | `cell(rank, file)` が返すマス表記から盤面文字列を生成。空マスを数字に圧縮 |

`rank`/`file` は **SFEN 記述順の 0..8**（`rank=0` が先頭段、`file=0` が各段の先頭文字）で、
engine の内部座標 (x,y in 1..9) とは別物。混同しないこと。
段数が 9 でない / 段内のマス合計が 9 でない場合は `ParseBoard` がエラーを返す。

## 盤面の検証（`validate.go`）

盤面が将棋の局面として辻褄が合っているかを調べる。**画像認識（suteme）のように
盤面を外から推測する用途では「おかしい盤面」が普通に出てくる**ので、
どこがおかしいかを構造化して返すところまでが core の役目で、
**それをエラーとして扱うかは呼び出し側が決める**（`Check` のビットマスクで選ぶ）。

| API | 内容 |
|---|---|
| `Check` | チェック種別のビットマスク。`CheckSyntax` / `CheckPieceCount` / `CheckKing` / `CheckDoubledPawn` / `CheckDeadPiece`、まとめて `CheckAll`・駒数だけの `CheckCounts` |
| `Inspect(board, checks) *BoardInfo` | 盤面を調べ、盤上の駒数・駒台の逆算・違反一覧を返す |
| `BoardInfo` | `Black`/`White`（盤上枚数）・`Hands`（駒台の逆算、**先後不明**）・`Violations`。`OK()` / `Filter(check)` / `Err(check)` / `Messages()` |
| `Violation` | 違反1件。`Check`・駒・手番・場所・日本語の `Detail`。**`error` を満たす**（`Err` は `errors.Join` で束ねるので `errors.As` で取り出せる） |
| `PieceLimit(base) int` / `HandOrder` | 先後合計の駒数上限（歩18・香桂銀金4・角飛玉2）と持ち駒の表記順（飛角金銀桂香歩） |
| `Name(code) string` | 駒コード → 日本語名（歩/香/…/龍/馬） |
| `FormatHands(black, white) string` | 持ち駒欄の文字列（`2R2B4G4S4N4L18P` 形式、無ければ `-`） |

- 駒数の上限・二歩・行き所のない駒といった**将棋の仕様はここにだけ書く。**
  suteme や ikkyoku が自前の枚数表を持たないこと。
- 壊れた盤面でも解析できた分は集計する（認識途中の結果を扱うため）。
  上限を超えた駒種は `Hands` に入れない（駒台の枚数に嘘を入れないため）。
- **`Inspect` は map と `fmt` を使う。I/O 境界用であってホットパスからは呼ばない。**
  `Letter` / `ParsePieceLetter` の alloc 制約はこちらには掛けていない。
- JS 側（`web/sfen.js`）には対応物が**まだ無い**。盤面検証はサーバ側でしか
  使っていないため。フロントで同じ判定が要るようになったら JS にも足して
  `web/test.mjs` でゴールデンを揃えること。
