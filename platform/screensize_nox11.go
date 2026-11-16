//go:build linux && (!cgo || nox11)

package platform

func screenSize() (int, int) { return 0, 0 }
