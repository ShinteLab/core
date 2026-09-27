# AGENTS.md

shinte の**共通基盤**。全プロジェクトが参照する将棋の仕様（SFEN / USI / KIF）と、
盤表示のためのフォント・フロントエンド資産を集約する。独立した Go モジュール
`github.com/ShinteLab/core`（依存は `golang.org/x/image` のみ。他プロジェクトを参照しない）。
プロジェクト横断の方針は親ディレクトリの `AGENTS.md` を参照。

## 構成

| パッケージ / ディレクトリ | import パス | 役割 |
|---|---|---|
| `sfen/` | `github.com/ShinteLab/core/sfen` | SFEN 仕様（駒文字マッピング・盤面の解析/生成・盤面の検証） |
| `usi/` | `github.com/ShinteLab/core/usi` | USI 仕様（マス座標・手表記・`info`/`bestmove`/`checkmate` 行の読み取り） |
| `usi/client/` | `github.com/ShinteLab/core/usi/client` | **USI エンジンと話すクライアント側**（＝将棋 UI 側）。セッション管理と `os/exec` での起動。**ホットパスに乗らない層なので親と分けてある** |
| `kifu/` | `github.com/ShinteLab/core/kifu` | KIF 仕様（指し手行・終局判定・KIF ドキュメント組み立て・盤面図・KIF/CSA → USI）と**USI の手 → 日本語表記**（盤が要るのでここ。`usi` ではない） |
| `web/` | `@shinte/web`（JS）/ `github.com/ShinteLab/core/web`（Go embed） | 共有フロント。`<shogi-board>` と SFEN/USI/KIF/CSA の JS ロジック |
| `shogifont/` | `github.com/ShinteLab/core/shogifont` | **将棋駒表示用の軽量フォントを焼くライブラリ**（`Faces` / `Build`） |
| root (`package main`) | — | 上の**薄い CLI**（旧 `board` / 旧モジュール名 `shogifont`） |

`*.html`（`test.html` / `board.html` / `move.html`）はフォントと盤表示の動作確認用。

## 作業の前に読む資料

パッケージごとの詳細（API 表・「崩さないこと」・過去に踏んだ不具合）は各 README にある。
**そのパッケージを触る前に必ず読むこと。** ここには全体に効く方針だけを書く。

| 触るもの | 読む資料 | 主な中身 |
|---|---|---|
| `sfen/` | `sfen/README.md` | 座標の取り方（rank/file は 0..8）、盤面の検証（`Inspect`・駒数上限・二歩） |
| `usi/`・`usi/client/` | `usi/README.md` | 行の読み取りの流儀、**エンジンとのやり取りで崩してはいけない手順**（option・`usinewgame`・`go infinite`/`go mate`・`drain`・`stop`） |
| `kifu/` | `kifu/README.md` | KIF の読み書き、盤面図（BOD）、KIF/CSA → USI、USI → 日本語表記（修飾の判定） |
| `shogifont/`・root の CLI | `shogifont/README.md` | **配ってよいフォントの判断基準**、異体字（玉・左馬）、GSUB、検証手順、焼き直し |
| `web/` | `web/README.md` | `<shogi-board>` の属性と CSS、フォントの焼き直し手順 |

## 最重要の設計方針

**将棋の仕様はここに一本化する。** engine・suteme・prokishi・kicho は自前で
重複実装を持たない。新しい表記変換が必要になったら core に足す。

### 駒コードは engine の PieceType と一致させてある

`core/sfen` のベース駒コード（`Pawn=0` … `King=7`、成駒 `GrowthPawn=8` …）の
**整数値は engine の `PieceType` のベース値と一致**している。おかげで engine は
自身の駒種を変換なしでそのまま渡せる。

- **ホットパス（指し手生成・探索・置換表）に変換を持ち込まない**のが設計意図。
  SFEN/USI 変換は「position 受信」「bestmove 出力」等の **I/O 境界でのみ**呼ぶ。
- そのため `sfen` / `usi` の実装は **switch とバイト演算ベースで、確保(alloc)を最小に保つ**こと。
  `map` やフォーマット関数（`fmt.Sprintf` 等）を安易に持ち込まない。
- **定数値を変えると engine が壊れる。** 値の変更は engine 側と同時に行う。

### Go と JS の 2 ソース

同じ仕様を Go (`sfen`/`usi`/`kifu`) と JS (`web/sfen.js`/`web/usi.js`/`web/kifu.js`/`web/csa.js`) の
2 つで持っている（バックエンドは Go、フロントは TS/JS のため）。**片方だけ直さない。**
挙動は `web/test.mjs`（`node test.mjs`）と Go 側テストのゴールデンで揃える。

- 片側にしか無い機能もある（盤面検証・USI のプロトコル・盤面図・日本語表記は Go だけ）。
  どれが対象外かは各パッケージの README に書いてある
- ブラウザで要るようになったら、そのときに JS へ足してゴールデンを揃える

### 段階的に劣化させる／黙って別物に倒さない

core の読み取り系に共通する流儀（詳細は各 README）。

- **読めない手が出ても、そこまでの結果は返す**（KIF/CSA → USI、読み筋の日本語表記）
- **知らないトークン・方言は読み飛ばす**（USI の `info`、KIF の未知の行）
- ただし**別の局面として読めてしまう誤り**（知らない手合割・読めない盤面図・
  知らない CSA の `%` 表記）は**エラーにする**。棋譜としては通るので画面では気づけない

### テストは手元の環境に依存させない

実エンジン（`usi/client`）も実フォント（`shogifont`）も**テストでは要求しない**。
手元に何があるかで結果が変わると、壊れたときに切り分けられない。

## コマンド

```powershell
go test ./sfen ./usi/... ./kifu   # 仕様パッケージのテスト
go test ./shogifont           # フォント生成ライブラリ（入力フォント不要）
node web/test.mjs             # JS 側ロジックのテスト
go run . {inputfont} [output.ttf]        # フォント生成（core/ から）
go run . -list {inputfont}               # 書体の一覧（TTC の中身・字が足りるか）
```

HTML の確認は ES module を読むため **HTTP 配信が必要**（`test.html` のみ `file://` 可）:

```powershell
npx --yes serve .             # core/ から。→ /board.html, /move.html, /web/demo.html
```

## HTML デモ

- `test.html` — フォント字形の確認のみ（SFEN ロジック無し。`file://` 直開き可）
- `board.html` — 盤面表示デモ。共有 Web Component `<shogi-board>`（`web/`）を使う
  （自前の SFEN パース / SVG 描画は撤去済み）。**駒フォント（明朝 / ゴシック）と
  玉・左馬の切り替えはここで確認する**
  （チェックボックス 2 つ。やっているのは盤にクラスを付け外しするだけ。
  左馬は成った角が要るのでプリセットのボタンを置いてある）
- `move.html` — ドラッグ操作対応の盤面エディタ。SFEN 解析/生成は `web/`（`toGrid`/`formatBoard`）に
  委譲済み（自前 SVG 描画・操作は維持）
