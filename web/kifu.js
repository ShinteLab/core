// KIF 形式(将棋の棋譜テキスト)への変換ヘルパ。
// スクレイピング等の I/O は含まない純粋関数のみ。Node のスクレイパ(kicho プロジェクト)
// からも、将来のブラウザ棋譜ビューアからも共通利用できるようにする。
//
// 注: 現状は読売・竜王戦ペイロード → KIF の変換のみ。CSA/KI2/SFEN との相互変換は
// 未実装で、将来このモジュールへ追加する候補。

// KIF の相対表記修飾(同じ移動先に複数駒が動ける場合の区別)。
// KIF の (筋段) 座標表記を使う場合は不要なので指し手名から除去する。
export const KIF_MOVE_MODIFIERS = ["右", "左", "上", "寄", "引", "直", "不成"];

// KIF ドキュメントの定型行。
export const KIF_HIRATE_HEADER = "手合割：平手";
export const KIF_MOVE_COLUMN_HEADER = "手数----指手---------消費時間--";

// 対局終了を表す指し手名(投了・中断など)。駒の移動ではないので座標を付けない。
// 読売のペイロードは終局を `"投了打"` のように "打" 付きで返すため、
// 末尾の "打" を除いた形でも判定する。
// Go 側の実装は shinte/core/kifu の TerminalMarkers / TerminalMarker。片方だけ直さないこと。
export const KIF_TERMINAL_MARKERS = [
  "投了", "中断", "千日手", "持将棋", "詰み", "切れ負け",
  "反則勝ち", "反則負け", "入玉勝ち", "不戦勝", "不戦敗",
];

/**
 * 指し手名が対局終了を表すならその名称を返す(そうでなければ null)。
 * @param {string} move
 * @returns {string|null}
 */
export function terminalMarker(move) {
  const name = String(move).trim().replace(/打$/, "");
  return KIF_TERMINAL_MARKERS.includes(name) ? name : null;
}

/**
 * 指し手の漢字表記から KIF の相対表記修飾(右/左/上/寄/引/直/不成)を除去する。
 * @param {string} move
 * @returns {string}
 */
export function stripMoveModifiers(move) {
  let name = move;
  for (const m of KIF_MOVE_MODIFIERS) {
    name = name.replace(m, "");
  }
  return name;
}

/**
 * KIF の1手行を組み立てる。
 *   `${num} ${移動先の漢字表記}(${移動元の筋}${移動元の段})`
 * 指し手名に "打" を含む場合は打ちなので移動元座標を 0,0 にする("打" 自体は残す)。
 * 投了・中断などの終局行は座標を付けずに `${num} 投了` の形で出力する。
 * @param {number|string} num  手数
 * @param {string} move        指し手の漢字表記(例 "７六歩", "５五角打", "７八銀右")
 * @param {number|string} fromX 移動元の筋
 * @param {number|string} fromY 移動元の段
 * @returns {string}
 */
export function formatKifuLine(num, move, fromX, fromY) {
  const terminal = terminalMarker(move);
  if (terminal !== null) {
    return `${num} ${terminal}`;
  }
  let x = fromX;
  let y = fromY;
  if (String(move).includes("打")) {
    x = 0;
    y = 0;
  }
  const name = stripMoveModifiers(move);
  return `${num} ${name}(${x}${y})`;
}

/**
 * 消費時間を KIF の "分:秒" 表記にする(例 " 4:00")。
 * 分は 60 を超えても繰り上げない(KIF の1手消費時間欄の慣例)。
 * Go 側の実装は shinte/core/kifu の FormatSpend。片方だけ直さないこと。
 * @param {number} seconds
 * @returns {string}
 */
export function formatSpend(seconds) {
  const s = Math.max(0, Math.floor(Number(seconds) || 0));
  return `${String(Math.floor(s / 60)).padStart(2, " ")}:${String(s % 60).padStart(2, "0")}`;
}

/**
 * 累計消費時間を KIF の "時:分:秒" 表記にする(例 "01:00:00")。
 * Go 側の実装は shinte/core/kifu の FormatTotal。
 * @param {number} seconds
 * @returns {string}
 */
export function formatTotal(seconds) {
  const s = Math.max(0, Math.floor(Number(seconds) || 0));
  const pad = (n) => String(n).padStart(2, "0");
  return `${pad(Math.floor(s / 3600))}:${pad(Math.floor(s / 60) % 60)}:${pad(s % 60)}`;
}

/**
 * 指し手行の末尾に付く消費時間欄を組み立てる。
 *   ( 4:00/01:00:00)   消費時間 / その対局者の累計消費時間
 * Go 側の実装は shinte/core/kifu の FormatTimes。
 * @param {number} spendSeconds
 * @param {number} totalSeconds
 * @returns {string}
 */
export function formatTimes(spendSeconds, totalSeconds) {
  return `(${formatSpend(spendSeconds)}/${formatTotal(totalSeconds)})`;
}

/**
 * 読売・竜王戦ペイロードの1手オブジェクトから KIF 行を作る。
 * @param {{num:(number|string), move:string, fr_x:(number|string), fr_y:(number|string)}} obj
 * @returns {string}
 */
export function yomiuriKifuLine(obj) {
  return formatKifuLine(obj.num, obj.move, obj.fr_x, obj.fr_y);
}

/**
 * 読売・竜王戦ペイロードの手配列(平手初期局面)から KIF ドキュメントの行配列を作る。
 * 先頭要素(arr[0])は指し手ではなくメタ情報のため、列ヘッダに置き換える(元実装踏襲)。
 * @param {Array<{num:(number|string), move:string, fr_x:(number|string), fr_y:(number|string)}>} arr
 * @returns {string[]}
 */
export function buildYomiuriHirateKifu(arr) {
  const lines = [KIF_HIRATE_HEADER];
  arr.forEach((obj, idx) => {
    if (idx === 0) {
      lines.push(KIF_MOVE_COLUMN_HEADER);
    } else {
      lines.push(yomiuriKifuLine(obj));
    }
  });
  return lines;
}
