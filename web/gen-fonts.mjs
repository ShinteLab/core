// core/*.ttf から web/font*.js(base64 data URL)を焼き直す。
//
//   cd core
//   go run . C:\Windows\Fonts\yumin.ttf shogi.ttf
//   go run . -name "ShogiSFEN Gothic" $env:TEMP\NotoSansJP-Regular.ttf shogi-gothic.ttf
//   node web/gen-fonts.mjs
//
// ⚠️ **-name の family と下の family を揃えること。** 食い違うと CSS から選べない。
// ⚠️ **.mjs にしてあるのは `assets.go` の `//go:embed *.js` に拾わせないため。**
// これは生成器であって配信物ではない。
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const webDir = path.dirname(fileURLToPath(import.meta.url));
const coreDir = path.join(webDir, "..");

// 既定フォントだけは font.js に埋める(FONT_BASE64 を差し替えるだけ)。
const DEFAULT_TTF = "shogi.ttf";

// ⚠️ **ここに足してよいのは、派生物を作って再配布できるライセンスのフォントだけ。**
// 手元にあるからという理由で足さないこと。過去に桜鯰(利用規約が存在しない)と
// どへた(改造禁止)を入れかけて外している。判断材料は web/README.md の「ライセンス表記」。
const extras = [
  {
    ttf: "shogi-gothic.ttf", mod: "font-gothic.js", key: "GOTHIC", slug: "gothic",
    family: "ShogiSFEN Gothic", fn: "ensureGothicFont",
    src: "Noto Sans JP (SIL OFL 1.1)",
  },
];

const b64of = (ttf) => fs.readFileSync(path.join(coreDir, ttf)).toString("base64");

// font.js は手書きのコードを含むので、埋め込み行だけ差し替える。
const fontJs = path.join(webDir, "font.js");
const b64 = b64of(DEFAULT_TTF);
let src = fs.readFileSync(fontJs, "utf8");
if (!/FONT_BASE64 = '[A-Za-z0-9+/=]+'/.test(src)) {
  throw new Error("font.js の FONT_BASE64 が見つからない");
}
src = src.replace(/FONT_BASE64 = '[A-Za-z0-9+/=]+'/, `FONT_BASE64 = '${b64}'`);
fs.writeFileSync(fontJs, src);
console.log(`font.js (${DEFAULT_TTF}) ${Math.round(b64.length / 1024)}KB`);

// 追加フォントは丸ごと生成する。
for (const d of extras) {
  const data = b64of(d.ttf);
  const kb = Math.round(data.length / 1024);
  const js = `// このファイルは core/${d.ttf} から自動生成された data URL です。
// 元フォント: ${d.src}
// 再生成: web/gen-fonts.mjs (手で書き換えない)
//
// **import した時点で文書に登録される**(index.js が <shogi-board> を登録するのと同じ扱い)。
// 既定の駒フォント(font.js)とは別 family なので、CSS の --shogi-font で選ぶ:
//
//   import "@shinte/web/font-${d.slug}.js";
//   shogi-board { --shogi-font: "${d.family}"; }
//
// ⚠️ **font.js からも index.js からも import しない。** ここは ${kb}KB あるので、
// **使う画面だけが読み込む**ように分けてある。
import { ensureShogiFont } from './font.js';

export const FONT_FAMILY_${d.key} = '${d.family}';
export const FONT_MIME = 'font/ttf';
export const FONT_BASE64_${d.key} = '${data}';
export const FONT_DATA_URL_${d.key} = 'data:' + FONT_MIME + ';base64,' + FONT_BASE64_${d.key};

export function ${d.fn}() {
  return ensureShogiFont(FONT_FAMILY_${d.key}, FONT_DATA_URL_${d.key});
}

${d.fn}();
`;
  fs.writeFileSync(path.join(webDir, d.mod), js);
  console.log(`${d.mod} (${d.ttf}) ${kb}KB  family=${d.family}`);
}
