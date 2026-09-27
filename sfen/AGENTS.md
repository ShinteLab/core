# sfen パッケージ

SFEN 仕様（駒文字マッピング・盤面の解析/生成・盤面の検証）。
駒コードの値と alloc の制約はルートの `AGENTS.md`（最重要の設計方針）を先に読むこと。
**理由や経緯はコードのコメントに書いてある。** ここには要点と置き場所だけを置く。

| ファイル | 中身 |
|---|---|
| `sfen.go` | 駒コード定数・`Letter` / `ParsePieceLetter`・`ParseBoard` / `FormatBoard`（**ホットパス寄り。alloc を増やさない**） |
| `validate.go` | 盤面の検証 `Inspect` → `BoardInfo`（駒数・駒台の逆算・違反一覧）、`PieceLimit` / `HandOrder` / `Name` / `FormatHands`（**I/O 境界用。map と fmt を使ってよい**） |

## 崩さないこと

- **`rank`/`file` は SFEN 記述順の 0..8** で、engine・`kifu` の内部座標 (x,y in 1..9) とは別物
- **駒数上限・二歩・行き所のない駒はここにだけ書く。** suteme や ikkyoku に枚数表を持たせない
- **検証結果をエラーにするかは呼び出し側が決める**（`Check` のビットマスクで選ぶ）。
  画像認識のように「おかしい盤面」が普通に来る用途があるため
- 壊れた盤面でも読めた分は集計する。**上限を超えた駒種は `Hands` に入れない**
- `Inspect` をホットパスから呼ばない
- **JS 側（`web/sfen.js`）に検証は無い。** フロントで要るようになったら足して `web/test.mjs` で揃える
