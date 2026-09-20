// Package web は共有フロントエンド資産(将棋盤 Web Component と SFEN/USI/KIF の
// JS ロジック)を Go の embed で公開する。バンドラを持たず Go embed で静的配信する
// 消費側(suteme の training サーバ等)が、ファイルをコピーせずに配信できるようにする。
//
// 例(net/http):
//
//	mux.Handle("/shinte-web/", http.StripPrefix("/shinte-web/", http.FileServer(http.FS(web.Assets))))
//
// 以後、ブラウザから /shinte-web/index.js や /shinte-web/shogi-board.js を読み込める。
package web

import "embed"

// ⚠️ **OFL.txt を外さないこと。** font.js / font-gothic.js には SIL OFL 1.1 の
// フォントから作った派生フォントが埋まっており、OFL はライセンス文の同梱を求める。
// これを配信する側が条件を満たせるよう、資産と一緒に持たせてある。
//
//go:embed *.js OFL.txt
var Assets embed.FS
