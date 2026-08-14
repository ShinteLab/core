// USI のマス座標と手表記。Go 側の shinte/core/usi と挙動を一致させること。
//
// 内部座標 (x,y) は 1..9。USI のマス表記は筋(file)が 10-x の数字、
// 段(rank)が 'a'+y-1。例: (x=3,y=7) <-> "7g"。

export const Resign = "resign";

/**
 * 内部座標 (x,y in 1..9) を USI マス文字列に変換する。例: (3,7) -> "7g"。
 * @param {number} x @param {number} y @returns {string}
 */
export function formatSquare(x, y) {
  const f = String.fromCharCode((10 - x) + 48);
  const r = String.fromCharCode((y - 1) + 97);
  return f + r;
}

/**
 * USI マス文字列を内部座標に変換する。範囲外・長さ不正は null。
 * @param {string} s @returns {{x:number, y:number}|null}
 */
export function parseSquare(s) {
  if (!s || s.length !== 2) return null;
  const f = s.charCodeAt(0);
  const r = s.charCodeAt(1);
  if (f < 49 || f > 57 || r < 97 || r > 105) return null; // '1'..'9', 'a'..'i'
  return { x: 10 - (f - 48), y: (r - 96) };
}

/**
 * @typedef {Object} Move
 * @property {boolean} resign
 * @property {string} drop  打つ駒の大文字。打ちでなければ ""
 * @property {number} fromX @property {number} fromY
 * @property {number} toX   @property {number} toY
 * @property {boolean} promote
 */

/**
 * USI 手文字列を Move に変換する。"7g7f" / "7g7f+" / "P*5e" / "resign"。
 * パース不能なら null。
 * @param {string} s @returns {Move|null}
 */
export function parseMove(s) {
  const m = { resign: false, drop: "", fromX: 0, fromY: 0, toX: 0, toY: 0, promote: false };
  if (s === Resign) { m.resign = true; return m; }
  const n = s.length;
  if (n === 2) {
    const p = parseSquare(s);
    if (!p) return null;
    m.fromX = p.x; m.fromY = p.y;
    return m;
  }
  if (n >= 4) {
    if (s[1] === "*") {
      const p = parseSquare(s.slice(2, 4));
      if (!p) return null;
      m.drop = s[0];
      m.toX = p.x; m.toY = p.y;
      return m;
    }
    const from = parseSquare(s.slice(0, 2));
    const to = parseSquare(s.slice(2, 4));
    if (!from || !to) return null;
    m.fromX = from.x; m.fromY = from.y;
    m.toX = to.x; m.toY = to.y;
    if (n === 5) m.promote = true;
    return m;
  }
  return null;
}

/**
 * Move を USI 手文字列に変換する。
 * @param {Move} m @returns {string}
 */
export function formatMove(m) {
  if (m.resign) return Resign;
  if (m.drop) return m.drop + "*" + formatSquare(m.toX, m.toY);
  return formatSquare(m.fromX, m.fromY) + formatSquare(m.toX, m.toY) + (m.promote ? "+" : "");
}
