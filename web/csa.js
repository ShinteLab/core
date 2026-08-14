// CSA 棋譜形式の指し手と USI 表記の相互変換。
//
// CSA の指し手: 手番(+/-) + 移動元(筋段2桁, 打ちは "00") + 移動先(筋段2桁) + 駒2文字。
//   例: "+7776FU" = 先手 7七 → 7六 の歩 = USI "7g7f"
// 成りは移動後の駒名で表す(歩が成れば "TO")。USI(+付き)への変換には移動元の駒が
// 元々成っていたかの判定が要るため、盤(grid)を参照する。
//
// 注: KI2(漢字の相対表記, 移動元省略)や 漢字表記 → USI は、どの駒が動けるかの
// 曖昧性解決に合法手生成が必要で、この JS では実装しない(Go の engine 側を使う)。

import { formatSquare } from "./usi.js";
import { parseMove } from "./usi.js";

// SFEN mark(大文字ベース + 任意の '+') → CSA 駒コード
const CSA_BY_KEY = {
  P: "FU", L: "KY", N: "KE", S: "GI", G: "KI", B: "KA", R: "HI", K: "OU",
  "+P": "TO", "+L": "NY", "+N": "NK", "+S": "NG", "+B": "UM", "+R": "RY",
};
const KEY_BY_CSA = Object.fromEntries(
  Object.entries(CSA_BY_KEY).map(([k, v]) => [v, k]),
);

// SFEN mark(例 'P','+p','r') → CSA 駒コード(手番は含まない)。
function markToCsa(mark) {
  const promoted = mark[0] === "+";
  const letter = (promoted ? mark[1] : mark[0]).toUpperCase();
  return CSA_BY_KEY[(promoted ? "+" : "") + letter];
}

// USI 内部座標(x,y) → CSA の "筋段"(筋 = 10-x, 段 = y)
function csaSquare(x, y) {
  return `${10 - x}${y}`;
}
// CSA の "筋段" → USI 内部座標(x,y)
function fromCsaSquare(fr) {
  const file = Number(fr[0]);
  const rank = Number(fr[1]);
  return { x: 10 - file, y: rank };
}

/**
 * USI 手 → CSA 手。移動する駒種の判定に盤(grid)と手番が要る。
 * @param {string[][]} grid  SFEN グリッド([rank][file] の mark, "" は空)
 * @param {'b'|'w'} turn
 * @param {string} usiMove
 * @returns {string|null}
 */
export function usiToCsa(grid, turn, usiMove) {
  const mv = parseMove(usiMove);
  if (!mv || mv.resign) return null;
  const side = turn === "b" ? "+" : "-";
  const to = csaSquare(mv.toX, mv.toY);

  if (mv.drop) {
    const csa = CSA_BY_KEY[mv.drop.toUpperCase()];
    if (!csa) return null;
    return `${side}00${to}${csa}`;
  }

  const from = csaSquare(mv.fromX, mv.fromY);
  const mark = grid[mv.fromY - 1][mv.fromX - 1];
  if (!mark) return null;
  const destMark = mv.promote && mark[0] !== "+" ? "+" + mark : mark;
  const csa = markToCsa(destMark);
  if (!csa) return null;
  return `${side}${from}${to}${csa}`;
}

/**
 * CSA 手 → USI 手。成り判定(移動元の駒が既に成っているか)に盤(grid)が要る。
 * @param {string[][]} grid
 * @param {string} csaMove  例 "+7776FU"
 * @returns {string|null}
 */
export function csaToUsi(grid, csaMove) {
  if (!csaMove || csaMove.length < 7) return null;
  const from = csaMove.slice(1, 3);
  const to = csaMove.slice(3, 5);
  const piece = csaMove.slice(5, 7);
  const key = KEY_BY_CSA[piece];
  if (!key) return null;

  const toSq = fromCsaSquare(to);
  const toStr = formatSquare(toSq.x, toSq.y);

  if (from === "00") {
    // 打ち: 打てるのは成っていない駒のみ
    const letter = key.replace("+", "");
    return `${letter}*${toStr}`;
  }

  const fromSq = fromCsaSquare(from);
  const fromStr = formatSquare(fromSq.x, fromSq.y);
  const mark = grid[fromSq.y - 1][fromSq.x - 1];
  const wasPromoted = mark && mark[0] === "+";
  const csaIsPromoted = key[0] === "+";
  const promote = csaIsPromoted && !wasPromoted;
  return `${fromStr}${toStr}${promote ? "+" : ""}`;
}
