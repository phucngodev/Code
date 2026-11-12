package app

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// TestTerminalShowsKittyImage feeds a Kitty-graphics image and checks the view
// paints it, so an image a program sends is not silently dropped.
func TestTerminalShowsKittyImage(t *testing.T) {
	w, s := liveShellWindow(t)
	defer s.close()

	// A 4x4 red image in each format a program sends: raw RGB, raw RGBA and
	// PNG (what most send).
	rgb := make([]byte, 4*4*3)
	rgba := make([]byte, 4*4*4)
	for i := 0; i < 4*4; i++ {
		rgb[i*3], rgba[i*4] = 0xff, 0xff
		rgba[i*4+3] = 0xff
	}
	var pngBuf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for i := range img.Pix {
		if i%4 == 0 {
			img.Pix[i] = 0xff
		} else if i%4 == 3 {
			img.Pix[i] = 0xff
		}
	}
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		format int
		data   []byte
	}{
		{24, rgb},
		{32, rgba},
		{100, pngBuf.Bytes()},
	}
	for _, c := range cases {
		data := base64.StdEncoding.EncodeToString(c.data)
		cmd := fmt.Sprintf("\x1b_Ga=T,f=%d,s=4,v=4,i=1;%s\x1b\\", c.format, data)
		s.term.Feed([]byte(cmd))

		tester := ui.NewTester(w.view, 900, 600)
		if !waitUntil(func() bool {
			tester.Frame()
			return hasRed(tester.Image())
		}, 3*time.Second) {
			t.Fatalf("the Kitty image (format %d) did not appear in the terminal view", c.format)
		}
	}
}

// hasRed reports whether any pixel is strongly red.
func hasRed(img *image.RGBA) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a > 0x8000 && r > 0xc000 && g < 0x2000 && bl < 0x2000 {
				return true
			}
		}
	}
	return false
}

// TestTerminalShowsKittyQueryResponse checks the terminal answers the Kitty
// graphics support query, which programs ask before sending an image.
func TestTerminalShowsKittyQueryResponse(t *testing.T) {
	if err := terminal.Load(); err != nil {
		t.Skipf("libghostty not available: %v", err)
	}
	client, server := net.Pipe()
	term, err := terminal.New(terminal.Options{Conn: client})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()

	// The query kitty and chafa send: "is this a Kitty graphics terminal?".
	query := "\x1b_Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\"
	go func() { _, _ = server.Write([]byte(query)) }()

	_ = server.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 128)
	n, err := server.Read(buf)
	if err != nil {
		t.Fatalf("no response to the Kitty query: %v", err)
	}
	if !strings.Contains(string(buf[:n]), "OK") {
		t.Fatalf("Kitty query response = %q, want an OK", buf[:n])
	}
}

// TestTerminalShowsKittyPlaceholderImage feeds a virtual Kitty image and its
// Unicode placeholder cells, as yazi shows a preview, and checks the view
// paints it.
func TestTerminalShowsKittyPlaceholderImage(t *testing.T) {
	w, s := liveShellWindow(t)
	defer s.close()

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		switch i % 4 {
		case 0:
			img.Pix[i] = 0xff
		case 3:
			img.Pix[i] = 0xff
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatal(err)
	}
	data := base64.StdEncoding.EncodeToString(pngBuf.Bytes())
	// A virtual image (U=1), then a row of placeholders whose foreground color
	// is the image id.
	s.term.Feed([]byte(fmt.Sprintf("\x1b_Ga=T,f=100,U=1,i=1,q=2;%s\x1b\\", data)))
	var cells strings.Builder
	cells.WriteString("\x1b[38;2;0;0;1m")
	for i := 0; i < 6; i++ {
		cells.WriteRune(0x10EEEE)
	}
	s.term.Feed([]byte(cells.String()))

	tester := ui.NewTester(w.view, 900, 600)
	if !waitUntil(func() bool {
		tester.Frame()
		return hasRed(tester.Image())
	}, 3*time.Second) {
		t.Fatal("the Kitty placeholder image did not appear in the terminal view")
	}
}

// TestTerminalShowsChunkedKittyImage feeds a large image split into several
// chunks, as chafa and kitty icat send one.
func TestTerminalShowsChunkedKittyImage(t *testing.T) {
	w, s := liveShellWindow(t)
	defer s.close()

	const n = 64
	rgba := make([]byte, n*n*4)
	for i := 0; i < n*n; i++ {
		rgba[i*4], rgba[i*4+3] = 0xff, 0xff
	}
	data := base64.StdEncoding.EncodeToString(rgba)
	// Chunks split at a multiple of 4 base64 characters, as Kitty requires.
	chunk := 4096
	parts := []string{}
	for i := 0; i < len(data); i += chunk {
		end := i + chunk
		if end > len(data) {
			end = len(data)
		}
		parts = append(parts, data[i:end])
	}
	for i, part := range parts {
		more := 1
		if i == len(parts)-1 {
			more = 0
		}
		var cmd string
		if i == 0 {
			cmd = fmt.Sprintf("\x1b_Ga=T,f=32,s=%d,v=%d,i=1,m=%d;%s\x1b\\", n, n, more, part)
		} else {
			cmd = fmt.Sprintf("\x1b_Gm=%d;%s\x1b\\", more, part)
		}
		s.term.Feed([]byte(cmd))
	}

	tester := ui.NewTester(w.view, 900, 600)
	if !waitUntil(func() bool {
		tester.Frame()
		return hasRed(tester.Image())
	}, 3*time.Second) {
		t.Fatal("the chunked Kitty image did not appear in the terminal view")
	}
}

// TestTerminalAdvertisesGhostty checks the shell Code starts sees the terminal
// identity Pi and PiG use to turn on images.
func TestTerminalAdvertisesGhostty(t *testing.T) {
	_, s := liveShellWindow(t)
	defer s.close()
	if !waitUntil(func() bool { return strings.TrimSpace(s.term.Text()) != "" }, 5*time.Second) {
		t.Fatal("the shell produced no prompt")
	}
	s.term.Send([]byte("echo TERM_PROGRAM=$TERM_PROGRAM\n"))
	if !waitUntil(func() bool { return strings.Contains(s.term.Text(), "TERM_PROGRAM=ghostty") }, 5*time.Second) {
		t.Fatal("the shell should see TERM_PROGRAM=ghostty")
	}
}
