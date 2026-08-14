# @shinte/web

shinte 共有フロントエンド。将棋盤の表示 Web Component `<shogi-board>` と、
SFEN/USI の共通ロジックを提供する。**ビルド不要の素の ESM JavaScript** で書かれており、
バンドラのある環境(prokishi の Vite/React)からも、バンドラの無い環境(suteme・core の
素の HTML)からも同じコードを使える。

## 構成

| ファイル | 役割 |
|---|---|
| `shogi-board.js` | `<shogi-board>` Web Component(Shadow DOM + SVG 描画。フォントは data URL で内蔵) |
| `shogi-hand.js` | `<shogi-hand>` Web Component(持ち駒表示。`side`/`flip` 対応) |
| `sfen.js` | SFEN の駒文字マッピング・盤面 `parseBoard`/`formatBoard`/`toGrid`・持駒 `parseHands`/`formatHands` |
| `usi.js` | USI マス座標 `formatSquare`/`parseSquare`・手表記 `parseMove`/`formatMove` |
| `position.js` | 手適用エンジン `applyUsiMove`/`resolvePosition`(position コマンド → moves 適用後の現在局面。捕獲・成り・打ちを反映。合法手生成はしない) |
| `csa.js` | CSA ⇔ USI 変換 `usiToCsa`/`csaToUsi`(盤参照。KI2/漢字→USI は要 engine で未実装) |
| `kifu.js` | KIF 変換(`formatKifuLine`/`stripMoveModifiers`/`terminalMarker`・読売竜王戦 `buildYomiuriHirateKifu`)。Go 側の対応実装は `github.com/ShinteLab/core/kifu`(そちらがヘッダ付きドキュメント組み立ても持つ)。指し手行の書式は両者で揃えること |
| `font.js` | 既定の駒フォント(Noto Serif JP ベース)を base64 data URL 化したもの(自動生成)と `ensureShogiFont()` |
| `font-gothic.js` | 別の元フォントで焼いた駒フォント(Noto Sans JP)。**import した画面だけが読み込む**(下記) |
| `gen-fonts.mjs` | 上の base64 を `core/*.ttf` から焼き直す生成器。`.mjs` なのは `assets.go` の `//go:embed *.js` に拾わせないため |
| `OFL.txt` | 埋め込んだ駒フォントの由来・権利表記と **SIL OFL 1.1 の全文**。⚠️ 配布物から外さないこと |
| `index.js` | 再エクスポート(これを import すれば `<shogi-board>` も登録される) |
| `index.d.ts` | TypeScript 型定義(prokishi など TS 消費側向け) |
| `assets.go` | `package web`。`//go:embed *.js` で JS 資産を `embed.FS`(`web.Assets`)として公開。Go サーバから配信するため |
| `demo.html` | 単体デモ(`core/board.html` の重複ロジックを置き換える位置づけ) |
| `test.mjs` | ロジックの単体テスト(`node test.mjs`) |

Go 側の `github.com/ShinteLab/core/sfen`・`github.com/ShinteLab/core/usi`・`github.com/ShinteLab/core/kifu` と同じ仕様を JS で持つ
(バックエンドは Go、フロントは TS/JS の2ソース)。
挙動は `test.mjs` と Go 側テストのゴールデンで揃える。

## 使い方

### 素の HTML(suteme / core)

```html
<script type="module" src="/path/to/core/web/index.js"></script>
<shogi-board sfen="lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL"></shogi-board>
```

### Go の embed 配信(suteme training サーバ / バンドラ無しの Go アプリ)

`core/web` は `assets.go` で JS 資産を `embed.FS` として公開している。Go サーバは
これを配信するだけで、ファイルをコピーせず(ドリフト無しで)共有資産を使える。

```go
import shinteweb "github.com/ShinteLab/core/web"

mux.Handle("/shinte-web/", http.StripPrefix("/shinte-web/", http.FileServer(http.FS(shinteweb.Assets))))
```

```html
<script type="module">
  import * as sfen from "/shinte-web/sfen.js";
  import "/shinte-web/shogi-board.js"; // <shogi-board> を登録
  window.ShinteWeb = { sfen };
</script>
```

### React(prokishi / Wails3)

```tsx
import "@shinte/web"; // 副作用で <shogi-board> を登録
// import { parseMove, toGrid } from "@shinte/web"; // ロジックだけ使う場合

export function Board({ sfen, lastMove }: { sfen: string; lastMove?: string }) {
  return <shogi-board sfen={sfen} last-move={lastMove} />;
}
```

JSX でカスタム要素の型を効かせるには `index.d.ts` の `HTMLElementTagNameMap` 拡張を
tsconfig の `types`/`include` に含める(または `/// <reference types="@shinte/web" />`)。

## 属性

| 属性 | 説明 |
|---|---|
| `sfen` | 盤面(SFEN の盤面部分、または完全な SFEN。先頭トークンを盤面として使う)。未指定なら平手初期局面 |
| `last-move` | 直前手の USI 表記(例 `7g7f` / `P*5e`)。移動元・移動先のマスを強調する |
| `flip` | 後手視点(盤全体を 180 度)にする(属性の有無で判定) |
| `coords` | `"false"` 以外なら筋・段の座標を表示(既定: 表示) |
| `fluid` | 置き場所の幅いっぱいに広げる(属性の有無で判定)。**既定では固有サイズ 560px より大きくならない**(svg の `width` 属性が `CELL*9 + MARGIN*2` で、CSS は `max-width: 100%` しか当てていない)。盤を主役に大きく見せたい画面(ikkyoku の解析タブ)向け。⚠️ **既定を変えていないのは、既存の利用側で盤が勝手に伸びると周りの余白の意味が変わるため** |
| `gyoku` | 王を玉で書く。**値で先後を選ぶ**(`black` = 先手だけ / `white` = 後手だけ / 値なし = 両方)。下記 |
| `hidari-uma` | 馬を左馬(馬の左右反転。縁起物の飾り駒の字)で書く(属性の有無で判定)。**先後の区別は無い** |

### 駒フォントを差し替える

既定は Noto Serif JP（明朝）ベースの `ShogiSFEN`。**別の元フォントで焼いたものを使うには、
そのモジュールを import してから CSS で選ぶ。**

```js
import "@shinte/web/font-gothic.js";   // import した時点で文書に登録される
```

```css
shogi-board { --shogi-font: "ShogiSFEN Gothic"; }
```

| モジュール | family | 元フォント | ライセンス | base64 |
|---|---|---|---|---|
| `font.js`（既定・自動で入る） | `ShogiSFEN` | Noto Serif JP（明朝） | **SIL OFL 1.1** | 18KB |
| `font-gothic.js` | `ShogiSFEN Gothic` | Noto Sans JP（ゴシック） | **SIL OFL 1.1** | 13KB |

- ⚠️ **足してよいのは、派生物を作って再配布できるライセンスのフォントだけ**（「ライセンス表記」参照）
- ⚠️ **`index.js` からは import していない。** 全部入りにすると使わない画面まで払うことになる。
  **使う画面が明示的に import する**
- **`--shogi-font` はカスタムプロパティなのでシャドウ DOM の中まで継承する。**
  盤の内側を名指しする必要はない。値は CSS の font-family なので**引用符ごと**渡すこと
  （`'"ShogiSFEN Gothic"'`）
- ⚠️ **family 名を揃えないこと。** 同名で複数登録するとどれが当たるかブラウザ任せになり、
  切り替えが効かなくなる。生成時の `-name` がこの名前
- ⚠️ **`assets.go` の `//go:embed *.js` はこれらも拾う。** Go サーバから配信する側は
  バイナリが増える。要らなければ embed の対象を絞ること
- 玉・左馬（下記）は**どのフォントでも効く**。同じ生成ツールで焼いているので
  `ss01` / `ss02` は 3 つとも入っている

### 駒の字を玉・左馬にする

`gyoku` / `hidari-uma` 属性で切り替える。

| 書き方 | 表示 |
|---|---|
| （なし） | 先手も後手も王 |
| `gyoku` | 両方とも玉 |
| `gyoku="black"` | **先手だけ玉**（後手は王） |
| `gyoku="white"` | **後手だけ玉**（先手は王） |
| `hidari-uma` | 馬を左馬に（**盤全体**） |

値は SFEN の手番トークンに合わせてある。**値なし・知らない値は両方**。

```html
<shogi-board sfen="..." gyoku="white" hidari-uma></shogi-board>
```

- ⚠️ **`gyoku` だけが先後を選べる。`hidari-uma` は有無だけで、値を書いても見ない。**
  玉は王将/玉将という**駒そのものの呼び分け**（上位者が王）なので片側だけがありうるが、
  左馬は**盤の見た目の選択**なので、使うと決めたら盤全体がそうなる
- ⚠️ **属性名を先後で分けられない。** HTML の属性名は大文字小文字を区別しないので、
  `K-gyoku` と `k-gyoku` は同じ属性になる。だから値で選ぶ
- 中身は**駒フォントの stylistic set**（`ss01` = 王→玉 / `ss02` = 馬→左馬）を
  駒 1 つ 1 つの `<text>` に当てている。**`K` と `k` はフォント上で同じグリフ**なので、
  盤全体にまとめて当てると先後を分けられない
- **SVG に描く文字は SFEN のまま**(`K` / `+B`)。SFEN も駒の扱いも変わらない
- ⚠️ **`font-feature-settings` は個別の値が積み上がらない**(後から当てた宣言が丸ごと勝つ)ので、
  玉と左馬は 1 つの値にまとめてから当てている(`_featureSettings`)

#### 先後まとめてでよければ CSS だけでも切り替わる

属性を付けなければ盤側は何も宣言しないので、利用側のスタイルがそのまま効く。
**`<shogi-board>` を既に置いている画面は、CSS を足すだけでよい。**

```css
shogi-board { font-feature-settings: "ss01"; }   /* 先後まとめて玉 */
```

- **なぜシャドウ DOM の外から効くのか**: `font-feature-settings` は**継承プロパティ**なので、
  host(や その祖先)に当てた指定がシャドウ内の `text.piece` まで届く。
  盤の中の要素を `::part` などで名指しする必要はない
- ⚠️ **`shogi-board.js` の `STYLE` にこの既定値を書かないこと。** シャドウ内の宣言が
  当たると継承が止まり、**外から当てた指定が効かなくなる**

#### ブラウザ以外で描くなら

⚠️ **stylistic set が効くのはブラウザの HTML/SVG だけ。** Canvas 2D には feature を渡す口が無く、
Go 側の `x/image/font` は GSUB を解釈しない。**そこで玉・左馬が要るようになったら**、
フォントには字も同梱してあるので `玉` (U+7389) / **U+E000**(左馬。私用領域)を直接描く
(`core/CLAUDE.md` の「異体字と反転字」を参照)。JS 側に対応表は置いていない。

## デモの確認

このディレクトリを静的配信して `demo.html` を開く(`file://` はブラウザ拡張やモジュール解決の
制約で不可のことがあるため HTTP 推奨)。例:

```
npx --yes serve core/web    # もしくは任意の静的サーバ
```

## フォントの再生成

### 駒フォントの登録について

**`@font-face` をシャドウ DOM 内の `<style>` に書いても効かない。** フォントは
シャドウルートではなく文書のフォントセットに対して解決されるため、シャドウ内で
`font-family` を指定しても実体が無く、SFEN の文字(`l` や `P`)がそのまま描画される。
`font.js` の `ensureShogiFont()` が `FontFace` API で文書側に登録するので、
Web Component 側は `connectedCallback` でこれを呼ぶだけでよい。

`shogi_font.go` でフォントを作り直したら、`core/` で 3 つとも焼き直して
`node web/gen-fonts.mjs` を通す:

```powershell
# 既定 (Noto Serif JP)。可変フォントなのでまず wght=400 を実体化する
python -m fontTools.varLib.instancer C:\Windows\Fonts\NotoSerifJP-VF.ttf wght=400 -o $env:TEMP\NotoSerifJP-Regular.ttf
go run . $env:TEMP\NotoSerifJP-Regular.ttf shogi.ttf

# ゴシック (Noto Sans JP)。これも可変フォント (既定は Thin)
python -m fontTools.varLib.instancer C:\Windows\Fonts\NotoSansJP-VF.ttf wght=400 -o $env:TEMP\NotoSansJP-Regular.ttf
go run . -name "ShogiSFEN Gothic" $env:TEMP\NotoSansJP-Regular.ttf shogi-gothic.ttf

node web\gen-fonts.mjs
```

- ⚠️ **実体化を飛ばさないこと。** `NotoSerifJP-VF.ttf` の既定インスタンスは
  **ExtraLight (wght=200)** で、`x/image/font/sfnt` は gvar を解釈しない。
  そのまま食わせると**細い駒ができる**（並べて見ないと気づきにくい）
- ⚠️ **元フォントは生成物の family には残らない**（`description` には入る）。
  別のフォントで焼き直すと駒の字形が黙って全部変わるので、ここに書いてあるものを使うこと
- ⚠️ **`-name` の family と `gen-fonts.mjs` の family を揃えること。** 食い違うと CSS から選べない
- `*.ttf` は `.gitignore` 対象。**リポジトリに入るのは焼いた JS だけ**

### ライセンス表記

生成フォントは元フォントの字形をそのまま持つ**派生物**なので、`name` テーブルの
copyright(0) / trademark(7) / license(13) / licenseURL(14) を**引き継いである**
（`description(10)` に何から作ったかも書く）。

既定フォントの由来:

```
© 2017-2023 Adobe (http://www.adobe.com/).
Noto is a trademark of Google Inc.
SIL Open Font License, Version 1.1  —  http://scripts.sil.org/OFL
```

#### ⚠️ フォントを足す前に確認すること

**「手元に入っているから使える」ではない。** 生成物は元フォントの字形をそのまま持つ
派生物を再配布する行為なので、**元フォントのライセンスが派生物の作成と再配布を
認めていなければ足せない。** 実際に 3 つ外している:

| フォント | 理由 |
|---|---|
| 游明朝 | name 13 が「Microsoft supplied font … **Any other use is prohibited**」。許されるのはコンテンツへの埋め込みと印刷のための一時ダウンロードだけ |
| どへた | **改造禁止** |
| 桜鯰毛筆 | **利用規約が存在しない**（許可が確認できない ＝ 使えない） |

- ⚠️ **`fsType` を根拠にしないこと。** 3 つとも `fsType=8`（編集可能な埋め込み許可）だった。
  これは**文書への埋め込み**の話であって、派生フォントを作って再配布してよいという意味ではない
- ⚠️ **name テーブルにライセンス条項が無いのは「制約が無い」ではない。**
  桜鯰・どへたはどちらも空だった（どへたの name 13 は TTEdit のシリアルだった）
- 現状はどちらも **SIL OFL 1.1** のもの（Noto Serif JP / Noto Sans JP）だけを使っている。
  OFL は派生物の作成・再配布を認めるが、**ライセンス文の同梱**と
  **Reserved Font Name を使わないこと**が条件。family を `ShogiSFEN*` にしてあるのは後者のため
  （Noto Sans JP は `Source` が Reserved Font Name）
- **OFL 1.1 の全文は `web/OFL.txt`。** 埋め込んだフォントの由来と権利表記も先頭に書いてある。
  ⚠️ **配布物から外さないこと**（OFL の条件）。`package.json` の `files` と
  `assets.go` の `//go:embed` にも入れてあるので、npm 配布でも Go 配信でも一緒に付いて回る

## 今後

- suteme の `applyFromSFEN`/`buildSFEN`、`core/board.html` の `parseRow` などフロントに散在する
  SFEN 処理を `sfen.js`/`usi.js` へ寄せて重複を解消する。
- kicho の変換部は `kifu.js` として抽出済み(読売竜王戦ペイロード → KIF)。CSA ⇔ USI は
  `csa.js` に実装済み。**KI2 / 漢字表記 → USI は合法手生成(曖昧性解決)が必要で未実装**
  (この将棋ロジックは Go の engine 側にある。必要なら engine 経由で解決する)。
- 持ち駒・対局状態の表示コンポーネント(`<shogi-hand>` 等)を追加する。
