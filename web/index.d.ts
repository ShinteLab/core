// @shinte/web の型定義(consumer 向け。実体は素の ESM JavaScript)。

export interface Cell {
  rank: number;
  file: number;
  mark: string;
  base: number;
  black: boolean;
  promoted: boolean;
}

export interface Move {
  resign: boolean;
  drop: string;
  fromX: number;
  fromY: number;
  toX: number;
  toY: number;
  promote: boolean;
}

export interface Hands {
  black: Record<string, number>;
  white: Record<string, number>;
}

export interface ResolvedPosition {
  board: string;
  hands: string;
  handsObj: { b: Record<string, number>; w: Record<string, number> };
  turn: "b" | "w";
  lastMove: string;
  moveCount: number;
}

export namespace sfen {
  const Pawn: number, Lance: number, Knight: number, Silver: number,
    Gold: number, Rook: number, Bishop: number, King: number,
    GrowthPawn: number, GrowthLance: number, GrowthKnight: number,
    GrowthSilver: number, GrowthRook: number, GrowthBishop: number, NotFound: number;
  const HAND_ORDER: string[];
  function letter(code: number): string;
  function parsePieceLetter(c: string): { base: number; black: boolean };
  function parseBoard(board: string): Cell[];
  function formatBoard(cell: (rank: number, file: number) => string | null): string;
  function toGrid(board: string): string[][];
  function parseHands(str: string): Hands;
  function formatHands(hands: Hands): string;
}

export namespace position {
  const STARTPOS_BOARD: string;
  function applyUsiMove(
    grid: string[][],
    hands: { b: Record<string, number>; w: Record<string, number> },
    turn: "b" | "w",
    mv: Move,
  ): "b" | "w";
  function resolvePosition(cmd: string): ResolvedPosition | null;
}

export namespace csa {
  function usiToCsa(grid: string[][], turn: "b" | "w", usiMove: string): string | null;
  function csaToUsi(grid: string[][], csaMove: string): string | null;
}

export namespace usi {
  const Resign: string;
  function formatSquare(x: number, y: number): string;
  function parseSquare(s: string): { x: number; y: number } | null;
  function parseMove(s: string): Move | null;
  function formatMove(m: Move): string;
}

export interface YomiuriMove {
  num: number | string;
  move: string;
  fr_x: number | string;
  fr_y: number | string;
}

export namespace kifu {
  const KIF_MOVE_MODIFIERS: string[];
  const KIF_HIRATE_HEADER: string;
  const KIF_MOVE_COLUMN_HEADER: string;
  function stripMoveModifiers(move: string): string;
  function formatKifuLine(num: number | string, move: string, fromX: number | string, fromY: number | string): string;
  function yomiuriKifuLine(obj: YomiuriMove): string;
  function buildYomiuriHirateKifu(arr: YomiuriMove[]): string[];
}

export function toGrid(board: string): string[][];
export function parseBoard(board: string): Cell[];
export function formatBoard(cell: (rank: number, file: number) => string | null): string;
export function letter(code: number): string;
export function parsePieceLetter(c: string): { base: number; black: boolean };
export function parseSquare(s: string): { x: number; y: number } | null;
export function formatSquare(x: number, y: number): string;
export function parseMove(s: string): Move | null;
export function formatMove(m: Move): string;
export function parseHands(str: string): Hands;
export function formatHands(hands: Hands): string;
export function resolvePosition(cmd: string): ResolvedPosition | null;
export function applyUsiMove(
  grid: string[][],
  hands: { b: Record<string, number>; w: Record<string, number> },
  turn: "b" | "w",
  mv: Move,
): "b" | "w";

/** <shogi-board> のカスタム要素クラス。import 時に自動登録される。 */
export class ShogiBoardElement extends HTMLElement {}
/** <shogi-hand> のカスタム要素クラス。import 時に自動登録される。 */
export class ShogiHandElement extends HTMLElement {}

declare global {
  interface HTMLElementTagNameMap {
    "shogi-board": ShogiBoardElement;
    "shogi-hand": ShogiHandElement;
  }
}
