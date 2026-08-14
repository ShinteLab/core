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

//go:embed *.js
var Assets embed.FS
