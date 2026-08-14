// <shogi-board> : SFEN から将棋盤を SVG 描画するフレームワーク非依存の
// Web Component。React(prokishi) からも素の HTML(suteme/core) からも使える。
//
// 属性:
//   sfen       盤面(SFEN の盤面部分 or 完全な SFEN。先頭トークンを盤面として使う)
//   last-move  直前手(USI 表記, 例 "7g7f" / "P*5e")。移動先(と移動元)を強調する
//   flip       後手視点(180度回転) にする(値の有無で判定)
//   coords     "false" 以外なら筋・段の座標を表示(既定: 表示)
//   fluid      置き場所の幅いっぱいに広げる(値の有無で判定)
//   gyoku      王を玉で書く。値で先後を選ぶ(下記)
//   hidari-uma 馬を左馬で書く(値の有無で判定。先後の区別は無い)
//
// 駒の字(玉・左馬)は**駒フォントの stylistic set** (ss01/ss02) で切り替える。
// **SVG に描く文字は SFEN のまま(K / +B)で、字だけが変わる。**
//
//   <shogi-board>                    王・馬 (既定)
//   <shogi-board gyoku>              両方とも玉
//   <shogi-board gyoku="black">      先手だけ玉 (後手は王)
//   <shogi-board gyoku="white">      後手だけ玉 (先手は王)
//   <shogi-board hidari-uma>         馬を左馬に (盤全体。片側だけは無い)
//
// ⚠️ **先後で分けるのに属性名を分けられない。** HTML の属性名は大文字小文字を
// 区別しないので `K-gyoku` と `k-gyoku` が同じ属性になってしまう。だから値で選ぶ。
// 値は SFEN の手番トークンに合わせて black/white(未知の値と値なしは両方)。
//
// ⚠️ **先後で分けるので、CSS を host に当てるだけでは足りない。** K と k は
// フォント上で同じグリフなので、feature を盤全体に当てると両方いっぺんに変わる。
// **駒 1 つ 1 つの <text> に当てる**必要があり、そこは盤しか知らないのでここでやる。
//
// 属性を付けなければ何も宣言しないので、**盤全体でよければ利用側の CSS だけでも切り替わる**
// (font-feature-settings は継承プロパティなのでシャドウ DOM の中まで届く):
//
//   shogi-board { font-feature-settings: "ss01"; }   /* 先後まとめて玉 */
//
// ⚠️ **既定では固有サイズ(560px)より大きくならない。** svg の width 属性が
// `CELL*9 + MARGIN*2` で、CSS は `max-width: 100%` しか当てていないので、
// host を広げても縮まないだけで伸びない。盤を主役に大きく見せたい画面
// (ikkyoku の解析タブ)向けに `fluid` を付けると `width: 100%` になる。
// **既定を変えていない**のは、既に置いている側(prokishi / suteme / core のデモ)で
// 盤が勝手に伸びると、周りの余白の意味が変わってしまうため。
//
// 例:
//   <shogi-board sfen="lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL"></shogi-board>

import { ensureShogiFont, FONT_FAMILY } from "./font.js";
import { toGrid } from "./sfen.js";
import { parseMove } from "./usi.js";

const SVGNS = "http://www.w3.org/2000/svg";
const KANJI = ["一", "二", "三", "四", "五", "六", "七", "八", "九"];

const CELL = 56;
const MARGIN = 28;
const N = 9;
const BOARD = CELL * N;
const FONT_SIZE = 44;

// 駒フォントの実体は font.js の ensureShogiFont() が文書側に登録する。
// ここに @font-face を書いてもシャドウ DOM 内では効かない(font.js の注記を参照)。
const STYLE = `
  :host { display: inline-block; }
  svg { display: block; max-width: 100%; height: auto; }
  /* fluid: 置き場所の幅いっぱいに広げる(既定は固有サイズ止まり)。
     ⚠️ **既定を変えないこと** —— 既存の利用側で盤が勝手に伸びる。 */
  :host([fluid]) { display: block; }
  :host([fluid]) svg { width: 100%; }
  text.piece {
    /* 駒フォントの差し替え口。別の元フォントで焼いたものを使うときは、
       その family を --shogi-font に入れる(カスタムプロパティはシャドウ DOM の
       中まで継承する)。**先に font-gothic.js などを import して文書に登録すること。**
         import "@shinte/web/font-gothic.js";
         shogi-board { --shogi-font: "ShogiSFEN Gothic"; } */
    font-family: var(--shogi-font, "${FONT_FAMILY}");
    text-anchor: middle;
    dominant-baseline: central;
    fill: #1a1a1a;
  }
  /* ⚠️ **駒の字の選択(玉・左馬)にここで何も書かないこと。**
     font-feature-settings は継承プロパティなので、利用側が host か その祖先に
     当てた指定がそのままシャドウ内の text.piece まで届く。ここで既定値を
     書いてしまうと、外から当てた指定が**内側の宣言に負けて効かなくなる。**
       shogi-board { font-feature-settings: "ss01"; }   ← これで玉になる */
  text.coord {
    font-family: sans-serif;
    font-size: 16px;
    text-anchor: middle;
    dominant-baseline: central;
    fill: #555;
  }
`;

function svgEl(name, attrs, text) {
  const e = document.createElementNS(SVGNS, name);
  for (const k in attrs) e.setAttribute(k, attrs[k]);
  if (text !== undefined) e.textContent = text;
  return e;
}

export class ShogiBoardElement extends HTMLElement {
  static get observedAttributes() {
    return ["sfen", "last-move", "flip", "coords", "gyoku", "hidari-uma"];
  }

  constructor() {
    super();
    this._root = this.attachShadow({ mode: "open" });
  }

  connectedCallback() {
    ensureShogiFont();
    this._render();
  }
  attributeChangedCallback() { this._render(); }

  // SFEN 属性の盤面部分(先頭トークン)を返す。未指定なら平手初期局面。
  _boardSFEN() {
    const raw = (this.getAttribute("sfen") || "").trim();
    if (!raw) return "lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL";
    return raw.split(/\s+/)[0];
  }

  // 字を選ぶ属性(gyoku / hidari-uma)を先手・後手それぞれの
  // font-feature-settings の値に畳む。当てるものが無い側は null。
  //
  // ⚠️ **font-feature-settings は個別の値が積み上がらない**(後から当てた宣言が
  // 丸ごと勝つ)ので、玉と左馬を 1 つの値にまとめてから当てる。
  _featureSettings() {
    // gyoku は値で先後を選ぶ。値なし・未知の値は両方。
    let gyoku = { black: false, white: false };
    if (this.hasAttribute("gyoku")) {
      const v = (this.getAttribute("gyoku") || "").trim().toLowerCase();
      if (v === "black") gyoku = { black: true, white: false };
      else if (v === "white") gyoku = { black: false, white: true };
      else gyoku = { black: true, white: true };
    }
    // ⚠️ **左馬に先後の区別は無い。値は見ずに有無だけで判定する。**
    // 玉は王将/玉将という駒そのものの呼び分け(上位者が王)なので片側だけがありうるが、
    // 左馬は盤の見た目の選択なので、使うと決めたら盤全体がそうなる。
    const uma = this.hasAttribute("hidari-uma");
    const build = (side) => {
      const list = [];
      if (gyoku[side]) list.push('"ss01"'); // 王 -> 玉
      if (uma) list.push('"ss02"');         // 馬 -> 左馬
      return list.length ? list.join(", ") : null;
    };
    return { black: build("black"), white: build("white") };
  }

  // 強調するマス集合(SFEN グリッドの "rank,file")を返す。
  _highlights() {
    const set = new Set();
    const mv = this.getAttribute("last-move");
    if (mv) {
      const m = parseMove(mv.trim());
      if (m && !m.resign) {
        if (m.toX) set.add(`${m.toY - 1},${m.toX - 1}`);
        if (m.fromX) set.add(`${m.fromY - 1},${m.fromX - 1}`);
      }
    }
    return set;
  }

  _render() {
    const flip = this.hasAttribute("flip");
    const showCoords = this.getAttribute("coords") !== "false";

    let grid;
    try {
      grid = toGrid(this._boardSFEN());
    } catch (e) {
      this._root.innerHTML = `<style>${STYLE}</style><pre style="color:#c00">SFEN error: ${e.message}</pre>`;
      return;
    }
    const highlights = this._highlights();
    const feats = this._featureSettings();

    const size = BOARD + MARGIN * 2;
    const svg = svgEl("svg", {
      xmlns: SVGNS, width: size, height: size, viewBox: `0 0 ${size} ${size}`,
    });

    // 盤(木目)と枠線
    svg.appendChild(svgEl("rect", {
      x: MARGIN, y: MARGIN, width: BOARD, height: BOARD,
      fill: "#f3c877", stroke: "#5a3b1a", "stroke-width": 3,
    }));

    // 強調マス
    for (const key of highlights) {
      let [r, c] = key.split(",").map(Number);
      if (flip) { r = 8 - r; c = 8 - c; }
      svg.appendChild(svgEl("rect", {
        x: MARGIN + c * CELL, y: MARGIN + r * CELL, width: CELL, height: CELL,
        fill: "#ffe27a", stroke: "none",
      }));
    }

    for (let i = 1; i < N; i++) {
      const p = MARGIN + i * CELL;
      svg.appendChild(svgEl("line", { x1: p, y1: MARGIN, x2: p, y2: MARGIN + BOARD, stroke: "#5a3b1a", "stroke-width": 1 }));
      svg.appendChild(svgEl("line", { x1: MARGIN, y1: p, x2: MARGIN + BOARD, y2: p, stroke: "#5a3b1a", "stroke-width": 1 }));
    }
    // 星
    for (const gx of [3, 6]) for (const gy of [3, 6]) {
      svg.appendChild(svgEl("circle", { cx: MARGIN + gx * CELL, cy: MARGIN + gy * CELL, r: 4, fill: "#5a3b1a" }));
    }

    // 座標(上辺=筋, 右辺=段)。flip 時は反転。
    if (showCoords) {
      for (let c = 0; c < N; c++) {
        const file = flip ? c + 1 : N - c;
        svg.appendChild(svgEl("text", { class: "coord", x: MARGIN + c * CELL + CELL / 2, y: MARGIN / 2 }, String(file)));
      }
      for (let r = 0; r < N; r++) {
        const rank = flip ? N - 1 - r : r;
        svg.appendChild(svgEl("text", { class: "coord", x: MARGIN + BOARD + MARGIN / 2, y: MARGIN + r * CELL + CELL / 2 }, KANJI[rank]));
      }
    }

    // 駒(SFEN 上段=一段目/左=9筋)。後手(小文字)は180度回転。flip 時は盤全体を反転。
    for (let r = 0; r < N; r++) {
      for (let c = 0; c < N; c++) {
        const tok = grid[r][c];
        if (!tok) continue;
        const dr = flip ? 8 - r : r;
        const dc = flip ? 8 - c : c;
        const cx = MARGIN + dc * CELL + CELL / 2;
        const cy = MARGIN + dr * CELL + CELL / 2;
        const isGote = tok[tok.length - 1] === tok[tok.length - 1].toLowerCase();
        const attrs = { class: "piece", x: cx, y: cy, "font-size": FONT_SIZE };
        // 後手は180度回転。flip(後手視点)ならさらに反転して打ち消す。
        const rotate = isGote !== flip;
        if (rotate) attrs.transform = `rotate(180 ${cx} ${cy})`;
        // 字の選択は駒ごとに当てる(先後で分けるため。上の _featureSettings を参照)
        const ff = isGote ? feats.white : feats.black;
        if (ff) attrs.style = `font-feature-settings: ${ff}`;
        svg.appendChild(svgEl("text", attrs, tok));
      }
    }

    this._root.replaceChildren();
    const style = document.createElement("style");
    style.textContent = STYLE;
    this._root.appendChild(style);
    this._root.appendChild(svg);
  }
}

if (typeof customElements !== "undefined" && !customElements.get("shogi-board")) {
  customElements.define("shogi-board", ShogiBoardElement);
}
