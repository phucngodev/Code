//go:build darwin

package main

import (
	"os"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// configureFontSmoothing turns macOS font smoothing off for Code, its default,
// so text is drawn thin rather than thickened by Core Text. Set
// CODE_FONT_SMOOTHING to keep the system's own setting.
func configureFontSmoothing() {
	if os.Getenv("CODE_FONT_SMOOTHING") != "" {
		return
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	class := objc.GetClass("NSUserDefaults")
	if class == 0 {
		return
	}
	defaults := objc.ID(class).Send(objc.RegisterName("standardUserDefaults"))
	key := nsString("AppleFontSmoothing")
	if defaults == 0 || key == 0 {
		return
	}
	defaults.Send(objc.RegisterName("setInteger:forKey:"), int64(0), key)
	// Flush now, so the default is written even if this launch is a second
	// instance that exits at the single-instance lock.
	defaults.Send(objc.RegisterName("synchronize"))
}

// nsString makes an NSString from a Go string, or 0.
func nsString(s string) objc.ID {
	class := objc.GetClass("NSString")
	if class == 0 {
		return 0
	}
	b := append([]byte(s), 0)
	id := objc.ID(class).Send(objc.RegisterName("stringWithUTF8String:"), unsafe.Pointer(&b[0]))
	runtime.KeepAlive(b)
	return id
}
