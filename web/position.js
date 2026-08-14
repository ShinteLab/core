// USI の指し手を盤に適用して現在局面を求めるモジュール。
// 合法手生成は行わない(記録済みの手を実行するだけ)。合法性判定や候補手生成が
// 必要な用途は Go の engine 側を使うこと(この JS は表示用の手適用に限定)。

import { toGrid, formatBoard, parseHands, formatHands } from "./sfen.js";
import { parseMove } from "./usi.js";

export const STARTPOS_BOARD =
  "lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL";

// 内部座標(x,y in 1..9) を SFEN グリッド添字 [rank][file] に変換する。
function gridRC(x, y) {
  return [y - 1, x - 1];
}

/**
 * 1手を盤に適用する。grid([9][9] の SFEN mark, "" は空)と hands({b,w} の
 * 大文字キー枚数)を破壊的に更新し、次の手番('b'/'w')を返す。
 * @param {string[][]} grid
 * @param {{b:Object<string,number>, w:Object<string,number>}} hands
 * @param {'b'|'w'} turn
 * @param {import('./usi.js').Move} mv
 * @returns {'b'|'w'}
 */
export function applyUsiMove(grid, hands, turn, mv) {
  const next = turn === "b" ? "w" : "b";
  if (!mv || mv.resign) return turn;

  const [toR, toF] = gridRC(mv.toX, mv.toY);

  if (mv.drop) {
    const key = mv.drop.toUpperCase();
    grid[toR][toF] = turn === "b" ? key : key.toLowerCase();
    if (hands[turn]) hands[turn][key] = (hands[turn][key] || 0) - 1;
    return next;
  }

  const [fromR, fromF] = gridRC(mv.fromX, mv.fromY);
  const piece = grid[fromR][fromF];

  // 移動先に駒があれば捕獲: 成りを解除し大文字(手番側の持駒)として加える
  const captured = grid[toR][toF];
  if (captured) {
    const baseLetter = captured.replace("+", "").toUpperCase();
    hands[turn][baseLetter] = (hands[turn][baseLetter] || 0) + 1;
  }

  let moved = piece;
  if (mv.promote && piece && piece[0] !== "+") {
    moved = "+" + piece;
  }
  grid[toR][toF] = moved;
  grid[fromR][fromF] = "";
  return next;
}

/**
 * USI の position コマンドを解決して現在局面を返す。
 * "position startpos moves ..." / "position sfen <board> <turn> <hands> <num> moves ..."
 * @param {string} cmd
 * @returns {{board:string, hands:string, handsObj:object, turn:('b'|'w'), lastMove:string, moveCount:number}|null}
 */
export function resolvePosition(cmd) {
  const t = cmd.trim().split(/\s+/);
  if (t[0] !== "position") return null;

  let board;
  let turn = "b";
  let handsStr = "-";
  let idx;
  if (t[1] === "startpos") {
    board = STARTPOS_BOARD;
    idx = 2;
  } else if (t[1] === "sfen") {
    board = t[2] || "";
    turn = t[3] || "b";
    handsStr = t[4] || "-";
    // t[5] = 手数
    idx = 6;
  } else {
    return null;
  }

  let grid;
  try {
    grid = toGrid(board);
  } catch (e) {
    return null;
  }
  const ph = parseHands(handsStr);
  const hands = { b: { ...ph.black }, w: { ...ph.white } };

  let cur = turn === "w" ? "w" : "b";
  let lastMove = "";
  let moveCount = 0;

  if (t[idx] === "moves") {
    for (const m of t.slice(idx + 1).filter(Boolean)) {
      const mv = parseMove(m);
      if (!mv || mv.resign) continue;
      cur = applyUsiMove(grid, hands, cur, mv);
      lastMove = m;
      moveCount++;
    }
  }

  return {
    board: formatBoard((r, c) => grid[r][c]),
    hands: formatHands({ black: hands.b, white: hands.w }),
    handsObj: hands,
    turn: cur,
    lastMove,
    moveCount,
  };
}
