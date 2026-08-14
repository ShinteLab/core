// 依存ゼロの簡易テスト: node test.mjs
// sfen.js / usi.js の挙動を検証する(Go 側 core とゴールデンを揃える)。
import assert from "node:assert/strict";
import * as sfen from "./sfen.js";
import * as usi from "./usi.js";
import * as kifu from "./kifu.js";
import * as position from "./position.js";
import * as csa from "./csa.js";

const START = "lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL";

let passed = 0;
function ok(name, fn) { fn(); passed++; console.log("  ok", name); }

ok("usi square round-trip", () => {
  for (const [s, x, y] of [["7g", 3, 7], ["1a", 9, 1], ["9i", 1, 9], ["5e", 5, 5]]) {
    const p = usi.parseSquare(s);
    assert.deepEqual(p, { x, y });
    assert.equal(usi.formatSquare(x, y), s);
  }
});

ok("usi square invalid", () => {
  for (const s of ["", "7", "7gg", "0a", "7j"]) assert.equal(usi.parseSquare(s), null);
});

ok("usi move round-trip", () => {
  for (const s of ["7g7f", "7g7f+", "P*5e", "8h2b+", "resign"]) {
    const m = usi.parseMove(s);
    assert.ok(m, s);
    assert.equal(usi.formatMove(m), s);
  }
});

ok("usi move invalid", () => {
  for (const s of ["", "1", "1g#", "zzz"]) assert.equal(usi.parseMove(s), null);
});

ok("sfen letter", () => {
  assert.equal(sfen.letter(sfen.Pawn), "P");
  assert.equal(sfen.letter(sfen.GrowthRook), "R");
  assert.equal(sfen.letter(sfen.NotFound), "None");
});

ok("sfen parseBoard startpos", () => {
  const cells = sfen.parseBoard(START);
  assert.equal(cells.length, 40);
});

ok("sfen parseBoard invalid", () => {
  assert.throws(() => sfen.parseBoard("9/9/9"));
});

ok("sfen toGrid + formatBoard round-trip", () => {
  const grid = sfen.toGrid(START);
  const out = sfen.formatBoard((r, f) => grid[r][f]);
  assert.equal(out, START);
});

ok("kifu formatKifuLine normal", () => {
  assert.equal(kifu.formatKifuLine(1, "７六歩", 7, 7), "1 ７六歩(77)");
});

ok("kifu formatKifuLine strips modifier", () => {
  assert.equal(kifu.formatKifuLine(3, "７八銀右", 6, 9), "3 ７八銀(69)");
});

ok("kifu formatKifuLine drop -> (00), keeps 打", () => {
  assert.equal(kifu.formatKifuLine(5, "５五角打", 8, 8), "5 ５五角打(00)");
});

// 読売のペイロードは終局を "投了打" として返す。座標なしの終局行にする。
// Go 側 (shinte/core/kifu) の TestFormatLine と同じ期待値。片方だけ直さないこと。
ok("kifu formatKifuLine terminal move has no coordinates", () => {
  assert.equal(kifu.formatKifuLine(104, "投了打", 0, 0), "104 投了");
  assert.equal(kifu.formatKifuLine(104, "投了", 0, 0), "104 投了");
  assert.equal(kifu.formatKifuLine(51, "中断", 0, 0), "51 中断");
});

ok("kifu terminalMarker does not misfire on drops", () => {
  assert.equal(kifu.terminalMarker("投了打"), "投了");
  assert.equal(kifu.terminalMarker("５三銀打"), null);
  assert.equal(kifu.terminalMarker("７六歩"), null);
});

// Go 側 (shinte/core/kifu) の TestFormatSpendAndTotal / TestFormatTimes と同じ期待値。
ok("kifu formatSpend / formatTotal / formatTimes", () => {
  assert.equal(kifu.formatSpend(0), " 0:00");
  assert.equal(kifu.formatSpend(16), " 0:16");
  assert.equal(kifu.formatSpend(240), " 4:00");
  assert.equal(kifu.formatSpend(4500), "75:00"); // 分は繰り上げない
  assert.equal(kifu.formatSpend(-1), " 0:00");

  assert.equal(kifu.formatTotal(0), "00:00:00");
  assert.equal(kifu.formatTotal(16), "00:00:16");
  assert.equal(kifu.formatTotal(27720), "07:42:00");

  assert.equal(kifu.formatTimes(240, 3600), "( 4:00/01:00:00)");
});

ok("kifu yomiuriKifuLine maps fields", () => {
  assert.equal(kifu.yomiuriKifuLine({ num: 2, move: "３四歩", fr_x: 3, fr_y: 3 }), "2 ３四歩(33)");
});

ok("kifu buildYomiuriHirateKifu structure", () => {
  const arr = [{ meta: true }, { num: 1, move: "７六歩", fr_x: 7, fr_y: 7 }];
  const lines = kifu.buildYomiuriHirateKifu(arr);
  assert.deepEqual(lines, ["手合割：平手", "手数----指手---------消費時間--", "1 ７六歩(77)"]);
});

ok("sfen parseHands", () => {
  const h = sfen.parseHands("S2Pb3p");
  assert.equal(h.black.S, 1);
  assert.equal(h.black.P, 2);
  assert.equal(h.white.B, 1);
  assert.equal(h.white.P, 3);
});

ok("sfen formatHands round-trip", () => {
  const h = sfen.parseHands("R2Pb");
  assert.equal(sfen.formatHands(h), "R2Pb");
  assert.equal(sfen.formatHands({ black: {}, white: {} }), "-");
});

ok("position apply 7g7f", () => {
  const p = position.resolvePosition("position startpos moves 7g7f");
  assert.equal(p.board, "lnsgkgsnl/1r5b1/ppppppppp/9/9/2P6/PP1PPPPPP/1B5R1/LNSGKGSNL");
  assert.equal(p.turn, "w");
  assert.equal(p.lastMove, "7g7f");
  assert.equal(p.moveCount, 1);
});

ok("position capture + promotion + hand", () => {
  const p = position.resolvePosition("position startpos moves 7g7f 3c3d 8h2b+");
  assert.equal(p.hands, "B"); // 先手が角を捕獲して持駒に
  const grid = sfen.toGrid(p.board);
  // 2b (USI) -> grid[1][7] に成った角(+B)
  assert.equal(grid[1][7], "+B");
  // 8h (USI) -> grid[7][1] は空
  assert.equal(grid[7][1], "");
  assert.equal(p.turn, "w");
});

ok("position drop from hand", () => {
  const p = position.resolvePosition(
    "position sfen lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL b P 1 moves P*5e",
  );
  const grid = sfen.toGrid(p.board);
  assert.equal(grid[4][4], "P"); // 5e に歩
  assert.equal(p.hands, "-"); // 持駒の歩を使い切り
});

ok("csa <-> usi normal", () => {
  const grid = sfen.toGrid("lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL");
  assert.equal(csa.usiToCsa(grid, "b", "7g7f"), "+7776FU");
  assert.equal(csa.csaToUsi(grid, "+7776FU"), "7g7f");
});

ok("csa <-> usi promotion", () => {
  const grid = sfen.toGrid("lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL");
  assert.equal(csa.usiToCsa(grid, "b", "8h2b+"), "+8822UM");
  assert.equal(csa.csaToUsi(grid, "+8822UM"), "8h2b+");
});

ok("csa <-> usi drop", () => {
  const grid = sfen.toGrid("lnsgkgsnl/1r5b1/ppppppppp/9/9/9/PPPPPPPPP/1B5R1/LNSGKGSNL");
  assert.equal(csa.usiToCsa(grid, "b", "P*5e"), "+0055FU");
  assert.equal(csa.csaToUsi(grid, "+0055FU"), "P*5e");
});


