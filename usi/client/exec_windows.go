package client

import (
	"os/exec"
	"syscall"
)

// createNoWindow は CreateProcess の CREATE_NO_WINDOW（syscall には定数が無い）。
const createNoWindow = 0x08000000

// hideConsole はエンジンのコンソール窓を出さずに起動させる。
//
// ⚠️ **GUI アプリ（`-H windowsgui`）から呼ばれると、付けないと窓が出る。**
// USI エンジンはコンソールアプリなので、コンソールを持たない親から起動すると
// Windows が子のために新しいコンソールを割り当てて表示する。親がコンソールを
// 持つとき（`go run`・`wails3 dev`）はそれを引き継ぐので窓が出ず、開発中は気づけない。
// 標準入出力はパイプで繋いでいるので、窓が無くても USI のやり取りには影響しない。
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
