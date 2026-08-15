// <shogi-hand> : SFEN の持ち駒部分を描画するフレームワーク非依存の Web Component。
//
// 属性:
//   hands  SFEN の持ち駒部分(例 "S2Pb3p" / "-")
//   side   "b"(先手, 既定) か "w"(後手)。その側の持ち駒のみ表示する
//   flip   後手視点(駒を180度回転)にする(値の有無で判定)
//
// 例:
//   <shogi-hand hands="R2Pb" side="b"></shogi-hand>

import { ensureShogiFont, FONT_FAMILY } from "./font.js";
import { parseHands, HAND_ORDER } from "./sfen.js";

// 駒フォントの実体は font.js の ensureShogiFont() が文書側に登録する。
// ここに @font-face を書いてもシャドウ DOM 内では効かない(font.js の注記を参照)。
const STYLE = `
  :host { display: inline-block; }
  .hand {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    font-family: sans-serif;
    font-size: 14px;
    color: #444;
  }
  .piece {
    /* ⚠️ **差し替え口は shogi-board と揃えること**(--shogi-font / --shogi-piece-color)。
       以前ここだけ family を直書きしていて、**盤のフォントを差し替えると
       持ち駒だけ既定の字のまま**になっていた(2026-08-16 に直した)。 */
    font-family: var(--shogi-font, "${FONT_FAMILY}");
    font-size: 28px;
    line-height: 1;
    color: var(--shogi-piece-color, #1a1a1a);
  }
  .piece.gote { transform: rotate(180deg); }
  .count { font-size: 13px; color: #333; margin-left: 1px; }
  .none { color: #999; }
`;

export class ShogiHandElement extends HTMLElement {
  static get observedAttributes() {
    return ["hands", "side", "flip"];
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

  _render() {
    const side = this.getAttribute("side") === "w" ? "white" : "black";
    const isWhite = side === "white";
    const flip = this.hasAttribute("flip");
    const parsed = parseHands(this.getAttribute("hands") || "-");
    const counts = parsed[side];

    const parts = [];
    for (const k of HAND_ORDER) {
      const n = counts[k] || 0;
      if (n <= 0) continue;
      const glyph = isWhite ? k.toLowerCase() : k;
      // 後手側は基本 180 度回転。flip(後手視点)なら反転を打ち消す。
      const rotate = isWhite !== flip;
      parts.push(
        `<span class="piece${rotate ? " gote" : ""}">${glyph}</span>` +
        (n > 1 ? `<span class="count">${n}</span>` : "")
      );
    }
    const body = parts.length ? parts.join("") : `<span class="none">なし</span>`;
    this._root.innerHTML = `<style>${STYLE}</style><div class="hand">${body}</div>`;
  }
}

if (typeof customElements !== "undefined" && !customElements.get("shogi-hand")) {
  customElements.define("shogi-hand", ShogiHandElement);
}
