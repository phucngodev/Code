package editor

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/notes/theme"
)

// TestInlineImageDecodesOnce checks a note's image is read once and stays in
// the cache, so it does not blink as frames go by.
func TestInlineImageDecodesOnce(t *testing.T) {
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 120, 80))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	f, err := os.Create(filepath.Join(dir, "p.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	note := filepath.Join(dir, "n.md")
	text := "# Note\n\nSome text.\n\n![p](p.png)\n"
	if err := os.WriteFile(note, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	calls := 0
	orig := decodeImage
	decodeImage = func(path string) (ui.ImageSource, float32, float32) {
		calls++
		return orig(path)
	}
	t.Cleanup(func() { decodeImage = orig; ClearImageCache() })

	ed := New(text, "markdown", note)
	look := Look{Theme: theme.System(false, ui.RGB(0, 122, 255)), Fonts: theme.Fonts{Size: 15}, LineHeight: 1.6, LineWidth: 700}
	tt := ui.NewTester(func(c *ui.Context) { ed.View(c.Key("ed"), look).Fill() }, 800, 600)
	for i := 0; i < 8; i++ {
		tt.Frame()
		time.Sleep(25 * time.Millisecond)
	}
	if calls != 1 {
		t.Fatalf("the image was decoded %d times, want 1", calls)
	}
}

// TestTwoLargeImagesDoNotThrash checks two big pictures do not evict each
// other frame after frame, which would make them blink. The budget counts
// the decoded (scaled) size, not the file's original size.
func TestTwoLargeImagesDoNotThrash(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.png", "b.png"} {
		img := image.NewRGBA(image.Rect(0, 0, 3000, 2500))
		for i := range img.Pix {
			img.Pix[i] = 180
		}
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	note := filepath.Join(dir, "n.md")
	text := "# Note\n\n![a](a.png)\n\n![b](b.png)\n"
	if err := os.WriteFile(note, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	calls := 0
	orig := decodeImage
	decodeImage = func(path string) (ui.ImageSource, float32, float32) {
		calls++
		return orig(path)
	}
	t.Cleanup(func() { decodeImage = orig; ClearImageCache() })

	ed := New(text, "markdown", note)
	look := Look{Theme: theme.System(false, ui.RGB(0, 122, 255)), Fonts: theme.Fonts{Size: 15}, LineHeight: 1.6, LineWidth: 700}
	tt := ui.NewTester(func(c *ui.Context) { ed.View(c.Key("ed"), look).Fill() }, 800, 900)
	for i := 0; i < 10; i++ {
		tt.Frame()
		time.Sleep(30 * time.Millisecond)
	}
	if calls != 2 {
		t.Fatalf("the two images were decoded %d times, want 2", calls)
	}
}

// TestClearImageCacheDropsPictures checks the cache is emptied, so closing a
// notes view lets its pictures be collected.
func TestClearImageCacheDropsPictures(t *testing.T) {
	images.Lock()
	images.m["x"] = &cachedImage{}
	images.sizes["x"] = [2]float32{1, 1}
	images.pixels = 100
	images.Unlock()

	ClearImageCache()

	images.Lock()
	defer images.Unlock()
	if len(images.m) != 0 || len(images.sizes) != 0 || images.pixels != 0 {
		t.Fatalf("cache not cleared: %d entries, %d sizes, %d pixels", len(images.m), len(images.sizes), images.pixels)
	}
}

// TestDecodeCapsLongSide checks a large picture is decoded no larger than
// maxImagePixels on its long side, while still reporting its real size.
func TestDecodeCapsLongSide(t *testing.T) {
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 3000, 2000))
	for i := range img.Pix {
		img.Pix[i] = 120
	}
	path := filepath.Join(dir, "big.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	src, w, h := decodeImageFile(path)
	if w != 3000 || h != 2000 {
		t.Fatalf("reported size %vx%v, want 3000x2000", w, h)
	}
	b, ok := src.(*ui.Bitmap)
	if !ok {
		t.Fatalf("decoded to %T, want *ui.Bitmap", src)
	}
	bw, bh := b.Size()
	if bw > maxImagePixels || bh > maxImagePixels {
		t.Fatalf("decoded %dx%d, want a long side <= %d", bw, bh, maxImagePixels)
	}
}
