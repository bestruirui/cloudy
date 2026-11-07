package app

import (
	"fmt"
	"syscall"
	"unsafe"
)

// osVersion names the system as feedback tells it, from what the kernel
// says, not what a compatibility shim would: Windows 11 reports itself as
// 10, with a build of 22000 or later.
func osVersion() string {
	var v struct {
		size                     uint32
		major, minor, build, plf uint32
		csd                      [128]uint16
	}
	v.size = uint32(unsafe.Sizeof(v))
	proc := syscall.NewLazyDLL("ntdll.dll").NewProc("RtlGetVersion")
	if r, _, _ := proc.Call(uintptr(unsafe.Pointer(&v))); r != 0 {
		return "Windows"
	}
	name := "Windows 10"
	if v.major == 10 && v.build >= 22000 {
		name = "Windows 11"
	} else if v.major != 10 {
		name = fmt.Sprintf("Windows %d.%d", v.major, v.minor)
	}
	return fmt.Sprintf("%s（版本号 %d）", name, v.build)
}
