//go:build darwin

package main

import (
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// disableFontSmoothing turns macOS font smoothing off for Code. MyGo draws text
// with Core Text font smoothing whenever the app's AppleFontSmoothing default
// is on — and its absence counts as on — which thickens glyphs until they look
// bold. The Swift app draws them thin, so Code sets the default to 0 in its own
// NSUserDefaults before the first frame. Set CODE_FONT_SMOOTHING to keep the
// system's look.
func disableFontSmoothing() {
	if keepSystemFontSmoothing() {
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
