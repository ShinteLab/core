// shinte 共有フロントエンドパッケージのエントリ。
// 盤表示 Web Component と、SFEN/USI の共通ロジックを再エクスポートする。
//
// 使い方(素の HTML / suteme・core):
//   <script type="module">
//     import "./core/web/index.js"; // <shogi-board> を登録
//   </script>
//   <shogi-board sfen="..."></shogi-board>
//
// 使い方(React / prokishi):
//   import "@shinte/web";                 // 副作用で <shogi-board> を登録
//   import { parseMove, toGrid } from "@shinte/web";

export * as sfen from "./sfen.js";
export * as usi from "./usi.js";
export * as kifu from "./kifu.js";
export * as csa from "./csa.js";
export * as position from "./position.js";
export { ShogiBoardElement } from "./shogi-board.js";
export { ShogiHandElement } from "./shogi-hand.js";

// 名前空間なしでもよく使う関数を直接エクスポートする。
export { toGrid, parseBoard, formatBoard, letter, parsePieceLetter, parseHands, formatHands } from "./sfen.js";
export { parseSquare, formatSquare, parseMove, formatMove } from "./usi.js";
export { resolvePosition, applyUsiMove } from "./position.js";
