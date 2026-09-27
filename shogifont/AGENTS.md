# shogifont パッケージ（と、その CLI である root）

日本語フォントから駒の字だけを抜き出し、SFEN 表記（`P` → 歩、`+P` → と）で表示できる
軽量 TTF を焼く。利用側は root の CLI（`web/font*.js` の元を焼く）と ikkyoku（端末のフォントから焼く）。
**理由・経緯はコードのコメントに、配るフォントの判断と焼き直しの手順は `web/README.md` に書いてある。**
ここには要点と置き場所だけを置く。

| ファイル | 中身 |
|---|---|
| `shogifont.go` | `Faces`（焼く前の書体一覧と足りない字）・`Build`・グリフ表（`basePieces` / `promotedPieces` / `altPieces` / `mirroredPieces`）・GSUB・TTF の組み立て |
| `names.go` | name テーブルの自前読み取り（言語を選ぶため） |
| `main.go`（root） | 薄い CLI。`-name` / `-index` / `-list` |

## 崩さないこと

- **ロジックを root に戻さない。** ikkyoku が import する入口が消える。`Build` は `os.Exit` しない
- **`Faces` は `Build` の前に呼べること自体が要点**（焼く前に足りない字を出すため）
- **日本語名が無いときに `LocalFamily` を英語名で埋めない**（`names.go`）
- **元フォントを変えたら `-name`（family 名）も必ず変える**（同名だとブラウザがどれを使うか決まらない）
- **リガチャの lookup を先頭に置く**（`buildGSUB`。先に `+B` を馬にしないと左馬の置換元が現れない）
- **反転は送り幅の中心で折り返す**（`mirrored`）
- **フォントを作り分けない**（玉・左馬は stylistic set `ss01` / `ss02` と、予備として cmap の `玉` / U+E000 の両方で出す）。
  **左馬の U+E000 はこのフォント専用の位置**
- **テストは実在の日本語フォントを読まない**（`glyphSource` 越しに作り物の輪郭を渡す）

## 配るフォントと端末で焼くフォントを混同しない

- **`web/font*.js` に入れてよいのは、派生物の作成と再配布を認めるライセンスのものだけ**（今は Noto の OFL 1.1 の 2 つ）。
  外したフォントとその理由、`fsType` を根拠にしないこと、OFL.txt の同梱は `web/README.md` の「ライセンス表記」「フォントを足す前に確認すること」
- **ikkyoku が端末で焼くのは再配布を伴わないので別の話。** 上の基準を持ち込まない（手元の游明朝で表示するのは正当）。
  逆に端末で焼いたものを配ってよいわけでもない。**`shogifont` はどちらも判断しない**（パッケージ冒頭）

## 焼き直しと確認

- **Noto は可変フォントで既定が細い。** `fonttools varLib.instancer` で wght=400 にしてから渡す。
  2 つとも焼き直して `node web/gen-fonts.mjs` を通す（手順は `web/README.md`）
- 確認は `go test ./shogifont` のあと、`test.html` / `board.html` で基本駒・成駒・玉・左馬を目で見る。
  GSUB を深く見たいときは fontTools で開き、リガチャ 6 組と feature `liga` / `rlig` / `ss01` / `ss02` があること
- **ikkyoku が端末で焼くフォントは焼き直しの手順に関係しない**
