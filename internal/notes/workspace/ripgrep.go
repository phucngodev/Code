package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// rgBinary is the ripgrep binary, or "" when it is not installed. It is looked
// up once, since which is not cheap and the answer does not change.
var rgBinary = sync.OnceValue(func() string {
	p, err := exec.LookPath("rg")
	if err != nil {
		return ""
	}
	return p
})

// rgMessage is one line of ripgrep's --json output.
type rgMessage struct {
	Type string `json:"type"`
	Data struct {
		Path struct {
			Text string `json:"text"`
		} `json:"path"`
		Lines struct {
			Text string `json:"text"`
		} `json:"lines"`
		LineNumber     int `json:"line_number"`
		AbsoluteOffset int `json:"absolute_offset"`
		Submatches     []struct {
			Start int `json:"start"`
			End   int `json:"end"`
		} `json:"submatches"`
	} `json:"data"`
}

// searchRipgrep runs ripgrep over the workspace. It returns the results and
// true, or false when ripgrep failed and the caller should search in Go.
func (w *Workspace) searchRipgrep(rg, query string, current func() bool) (SearchResult, bool) {
	if strings.TrimSpace(query) == "" {
		return SearchResult{}, true
	}
	w.mu.RLock()
	root := w.root
	titles := make(map[string]string, len(w.idx.notes))
	mods := make(map[string]time.Time, len(w.idx.notes))
	for p, n := range w.idx.notes {
		if isEncrypted(p) {
			continue
		}
		titles[p] = n.Title
		mods[p] = n.Modified
	}
	w.mu.RUnlock()

	// -S is smart case, -F fixed strings: the same matching the Go search does.
	// Encrypted .gpg.md notes are left alone, since their text is unreadable.
	args := []string{"--json", "--no-config", "--no-messages", "--color=never", "-S", "-F",
		"--glob", "!*.gpg.md", "--type", "markdown", "--max-count", "100", "--", query, root}
	cmd := exec.Command(rg, args...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, io.Discard
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		// Exit 1 means no match, which is not a failure.
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return SearchResult{}, true
		}
		return SearchResult{}, false
	}

	files := map[string]*SearchFile{}
	var order []string
	dec := json.NewDecoder(&out)
	for {
		var m rgMessage
		if err := dec.Decode(&m); err != nil {
			break
		}
		if m.Type != "match" {
			continue
		}
		path := m.Data.Path.Text
		if _, ok := titles[path]; !ok {
			continue
		}
		f := files[path]
		if f == nil {
			title := titles[path]
			if title == "" {
				title = baseName(path)
			}
			f = &SearchFile{Path: path, Title: title}
			files[path] = f
			order = append(order, path)
		}
		f.Count += len(m.Data.Submatches)
		if len(f.Matches) >= 5 || len(m.Data.Submatches) == 0 {
			continue
		}
		// The match's place in the note, in UTF-16 units, as the editor's
		// selection counts it.
		sm := m.Data.Submatches[0]
		units, length := w.unitsAt(path, m.Data.AbsoluteOffset+sm.Start, sm.End-sm.Start)
		f.Matches = append(f.Matches, SearchMatch{
			Line:   m.Data.LineNumber,
			Text:   strings.TrimRight(m.Data.Lines.Text, "\n"),
			Column: sm.Start,
			Length: sm.End - sm.Start,
			Offset: units,
			Units:  length,
		})
	}
	if !current() {
		return SearchResult{}, true
	}
	// Newest notes first, as the Go search orders them.
	slices.SortFunc(order, func(a, b string) int {
		if c := mods[b].Compare(mods[a]); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	res := SearchResult{Total: len(order)}
	for _, p := range order[:min(300, len(order))] {
		res.Files = append(res.Files, *files[p])
	}
	return res, true
}

// unitsAt is the UTF-16 offset of the n bytes at byte offset off in the file
// at path, and the UTF-16 length of those bytes.
func (w *Workspace) unitsAt(path string, off, n int) (int, int) {
	data, err := os.ReadFile(path)
	if err != nil || off > len(data) {
		return off, n
	}
	end := min(off+n, len(data))
	return utf16Units(data[:off]), utf16Units(data[off:end])
}

// utf16Units counts the UTF-16 code units of b, as the editor's selection
// counts a note.
func utf16Units(b []byte) int {
	n := 0
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
		i += size
	}
	return n
}

// baseName is the last part of a path, without a directory.
func baseName(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}
