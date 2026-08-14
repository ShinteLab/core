// SFEN の駒文字マッピングと盤面文字列の解析・生成。
// Go 側の shinte/core/sfen と挙動を一致させること(共通ゴールデンで担保)。

// ベース駒コード(engine.PieceType / Go core/sfen と一致)。
export const Pawn = 0;
export const Lance = 1;
export const Knight = 2;
export const Silver = 3;
export const Gold = 4;
export const Rook = 5;
export const Bishop = 6;
export const King = 7;
export const GrowthPawn = 8;
export const GrowthLance = 9;
export const GrowthKnight = 10;
export const GrowthSilver = 11;
export const GrowthRook = 12;
export const GrowthBishop = 13;
export const NotFound = 99;

/**
 * ベース/成駒コードに対応する SFEN の大文字を返す(成駒はベース駒の文字)。
 * @param {number} code
 * @returns {string}
 */
export function letter(code) {
  switch (code) {
    case Pawn: case GrowthPawn: return "P";
    case Lance: case GrowthLance: return "L";
    case Knight: case GrowthKnight: return "N";
    case Silver: case GrowthSilver: return "S";
    case Gold: return "G";
    case Rook: case GrowthRook: return "R";
    case Bishop: case GrowthBishop: return "B";
    case King: return "K";
  }
  return "None";
}

/**
 * SFEN の1文字(大文字=先手/小文字=後手)をベース駒コードと手番に変換する。
 * @param {string} c 単一文字
 * @returns {{base: number, black: boolean}}
 */
export function parsePieceLetter(c) {
  let black = true;
  let code = c.charCodeAt(0);
  if (code >= 97 && code <= 122) { // a-z
    code -= 32;
    black = false;
  }
  let base;
  switch (String.fromCharCode(code)) {
    case "P": base = Pawn; break;
    case "L": base = Lance; break;
    case "N": base = Knight; break;
    case "S": base = Silver; break;
    case "G": base = Gold; break;
    case "R": base = Rook; break;
    case "B": base = Bishop; break;
    case "K": base = King; break;
    default: base = NotFound;
  }
  return { base, black };
}

/** @typedef {{ mark: string, base: number, black: boolean, promoted: boolean }} Cell */

/**
 * SFEN 盤面部分('/' 区切りの9段)を解析し、駒のあるマスの配列を返す。
 * rank/file は SFEN 記述順の 0..8。mark はそのマスの SFEN 表記("+p" 等)。
 * 段数が9でない・段内のマス合計が9でない場合は例外を投げる。
 * @param {string} board
 * @returns {Array<{rank:number, file:number} & Cell>}
 */
export function parseBoard(board) {
  const ranks = board.split("/");
  if (ranks.length !== 9) {
    throw new Error(`sfen parse error: rank count ${ranks.length} != 9 [${board}]`);
  }
  const out = [];
  for (let rank = 0; rank < 9; rank++) {
    const line = ranks[rank];
    let file = 0;
    for (let idx = 0; idx < line.length; idx++) {
      let ch = line[idx];
      if (ch >= "0" && ch <= "9") {
        file += Number(ch);
        continue;
      }
      let promoted = false;
      let mark = ch;
      if (ch === "+") {
        promoted = true;
        idx++;
        if (idx >= line.length) throw new Error(`sfen parse error: dangling '+' [${line}]`);
        ch = line[idx];
        mark = "+" + ch;
      }
      const { base, black } = parsePieceLetter(ch);
      out.push({ rank, file, mark, base, black, promoted });
      file++;
    }
    if (file !== 9) {
      throw new Error(`sfen parse error: rank ${rank} has ${file} files [${line}]`);
    }
  }
  return out;
}

/**
 * 9x9 の盤面を SFEN 盤面文字列(9段を '/' 区切り)に変換する。
 * cell(rank, file) は SFEN 記述順のマス表記を返す(空マスは "" または null)。
 * @param {(rank:number, file:number) => (string|null)} cell
 * @returns {string}
 */
export function formatBoard(cell) {
  const parts = [];
  for (let rank = 0; rank < 9; rank++) {
    let line = "";
    let empty = 0;
    for (let file = 0; file < 9; file++) {
      const m = cell(rank, file);
      if (!m) { empty++; continue; }
      if (empty !== 0) { line += String(empty); empty = 0; }
      line += m;
    }
    if (empty !== 0) line += String(empty);
    parts.push(line);
  }
  return parts.join("/");
}

// 持ち駒の SFEN 並び順(飛角金銀桂香歩)。
export const HAND_ORDER = ["R", "B", "G", "S", "N", "L", "P"];

/**
 * SFEN の持ち駒部分を先後の枚数(大文字キー)に解析する。"-" や空は全0。
 * 例: "S2Pb3p" -> { black: {S:1, P:2}, white: {B:1, P:3} }
 * @param {string} str
 * @returns {{black: Object<string,number>, white: Object<string,number>}}
 */
export function parseHands(str) {
  const black = {};
  const white = {};
  if (!str || str === "-") return { black, white };
  let i = 0;
  while (i < str.length) {
    let num = 0;
    let hasNum = false;
    while (i < str.length && str[i] >= "0" && str[i] <= "9") {
      num = num * 10 + (str.charCodeAt(i) - 48);
      hasNum = true;
      i++;
    }
    if (i >= str.length) break;
    const c = str[i++];
    const { base, black: isBlack } = parsePieceLetter(c);
    if (base === NotFound) continue;
    const key = letter(base);
    const count = hasNum ? num : 1;
    const target = isBlack ? black : white;
    target[key] = (target[key] || 0) + count;
  }
  return { black, white };
}

/**
 * 先後の持ち駒(大文字キー)を SFEN の持ち駒文字列に変換する。空なら "-"。
 * @param {{black: Object<string,number>, white: Object<string,number>}} hands
 * @returns {string}
 */
export function formatHands(hands) {
  let out = "";
  const emit = (counts, lower) => {
    for (const k of HAND_ORDER) {
      const n = counts[k] || 0;
      if (n <= 0) continue;
      out += (n > 1 ? String(n) : "") + (lower ? k.toLowerCase() : k);
    }
  };
  emit(hands.black || {}, false);
  emit(hands.white || {}, true);
  return out || "-";
}

/**
 * SFEN 盤面部分を [9][9] の mark 配列(空は "")に展開する。表示部品向け。
 * @param {string} board
 * @returns {string[][]}
 */
export function toGrid(board) {
  const grid = Array.from({ length: 9 }, () => Array(9).fill(""));
  for (const p of parseBoard(board)) {
    grid[p.rank][p.file] = p.black ? p.mark.toUpperCase() : p.mark.toLowerCase();
  }
  return grid;
}
