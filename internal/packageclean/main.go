// Command packageclean removes MyGo's pinned libghostty-vt from a Linux app
// directory built by `mygo build`, and from the .deb and .tar.gz archives that
// mygo build made from that directory before this tool can run. Code ships
// only the library scripts/build-libghostty.sh built.
//
//	go run ./internal/packageclean build/linux-amd64
//	go run ./internal/packageclean -check build/linux-amd64
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// bundledNames are the files of MyGo's pinned libghostty-vt. Code's own
// library is always named libghostty-vt-local-<goos>-<goarch>.<ext>.
var bundledNames = map[string]bool{
	"libghostty-vt.dylib": true,
	"libghostty-vt.so":    true,
	"ghostty-vt.dll":      true,
}

func isBundled(name string) bool { return bundledNames[path.Base(strings.TrimSuffix(name, "/"))] }
func isLocal(name string) bool   { return strings.HasPrefix(path.Base(name), "libghostty-vt-local-") }

func main() {
	check := flag.Bool("check", false, "only verify that no MyGo library ships")
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatal("usage: packageclean [-check] <build-dir>...")
	}
	for _, dir := range flag.Args() {
		var err error
		if *check {
			err = verify(dir)
		} else {
			err = clean(dir)
		}
		if err != nil {
			log.Fatalf("packageclean %s: %v", dir, err)
		}
	}
}

// clean removes the bundled library everywhere it was copied, then verifies.
func clean(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	local := false
	var archives []string
	for _, entry := range entries {
		name := entry.Name()
		switch {
		case isBundled(name) && entry.Type().IsRegular():
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				return err
			}
		case isLocal(name):
			local = true
		case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".deb"):
			archives = append(archives, name)
		}
	}
	if !local {
		return fmt.Errorf("no libghostty-vt-local-* in %s: run ./scripts/build-libghostty.sh before packaging", dir)
	}
	for _, name := range archives {
		p := filepath.Join(dir, name)
		if strings.HasSuffix(name, ".deb") {
			if err := rewriteDeb(p); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		} else {
			if err := rewriteArchive(p); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	if err := rewriteFileList(dir); err != nil {
		return err
	}
	if err := rewritePKGBUILD(dir); err != nil {
		return err
	}
	return verify(dir)
}

// verify fails when any artifact still holds MyGo's library or none holds
// Code's. It is also the -check mode used after a build.
func verify(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	local := false
	for _, entry := range entries {
		name := entry.Name()
		switch {
		case isBundled(name):
			return fmt.Errorf("%s still ships MyGo's %s", dir, name)
		case isLocal(name):
			local = true
		}
	}
	if !local {
		return fmt.Errorf("%s ships no libghostty-vt-local-*", dir)
	}
	for _, entry := range entries {
		name := entry.Name()
		p := filepath.Join(dir, name)
		switch {
		case strings.HasSuffix(name, ".tar.gz"):
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if err := checkTarGz(data, p); err != nil {
				return err
			}
		case strings.HasSuffix(name, ".deb"):
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			members, err := readAr(data)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			for _, m := range members {
				if m.name == "data.tar.gz" {
					if err := checkTarGz(m.data, name+":"+m.name); err != nil {
						return err
					}
				}
			}
		case name == ".mygo-files":
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for line := range strings.Lines(string(data)) {
				if isBundled(strings.TrimSpace(line)) {
					return fmt.Errorf("%s still lists MyGo's library", name)
				}
			}
		}
	}
	return nil
}

func checkTarGz(data []byte, label string) error {
	_, err := walkTarGz(data, func(hdr *tar.Header, r io.Reader) ([]byte, bool, error) {
		if isBundled(hdr.Name) {
			return nil, false, fmt.Errorf("%s still holds MyGo's %s", label, hdr.Name)
		}
		return nil, true, nil
	})
	return err
}

// rewriteArchive drops the bundled library from a .tar.gz without touching
// any other entry.
func rewriteArchive(p string) error {
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	out, err := filterTarGz(data)
	if err != nil {
		return err
	}
	if bytes.Equal(data, out) {
		return nil
	}
	return os.WriteFile(p, out, 0o644)
}

// walkTarGz reads a gzip tar. transform returns replacement content for a
// regular file, or false to drop the entry.
func walkTarGz(data []byte, transform func(*tar.Header, io.Reader) ([]byte, bool, error)) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var out bytes.Buffer
	zw, err := gzip.NewWriterLevel(&out, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	tw := tar.NewWriter(zw)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		var content []byte
		if hdr.Typeflag == tar.TypeReg && transform != nil {
			content, err = io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
		}
		replacement, keep, err := transform(hdr, bytes.NewReader(content))
		if err != nil {
			return nil, err
		}
		if !keep {
			continue
		}
		h := *hdr
		if hdr.Typeflag == tar.TypeReg && replacement != nil {
			h.Size = int64(len(replacement))
		}
		if err := tw.WriteHeader(&h); err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg {
			if replacement != nil {
				if _, err := tw.Write(replacement); err != nil {
					return nil, err
				}
			} else if content != nil {
				if _, err := tw.Write(content); err != nil {
					return nil, err
				}
			} else if _, err := io.Copy(tw, tr); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// filterTarGz drops MyGo's library entries and returns the size of the
// regular files left, which the Debian control file reports.
func filterTarGz(data []byte) ([]byte, error) {
	var total int64
	out, err := walkTarGz(data, func(hdr *tar.Header, r io.Reader) ([]byte, bool, error) {
		if isBundled(hdr.Name) {
			return nil, false, nil
		}
		if hdr.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(r)
			if err != nil {
				return nil, false, err
			}
			total += int64(len(content))
			return content, true, nil
		}
		return nil, true, nil
	})
	return out, err
}

func installedSize(data []byte) (int64, error) {
	var total int64
	_, err := walkTarGz(data, func(hdr *tar.Header, r io.Reader) ([]byte, bool, error) {
		if hdr.Typeflag == tar.TypeReg {
			total += hdr.Size
		}
		return nil, true, nil
	})
	if err != nil {
		return 0, err
	}
	return (total + 1023) / 1024, nil
}

var installedSizeLine = regexp.MustCompile(`(?m)^Installed-Size: *[0-9]+$`)

func replaceInstalledSize(control []byte, size int64) []byte {
	line := []byte(fmt.Sprintf("Installed-Size: %d", size))
	if loc := installedSizeLine.FindIndex(control); loc != nil {
		out := append([]byte(nil), control[:loc[0]]...)
		out = append(out, line...)
		return append(out, control[loc[1]:]...)
	}
	return control
}

// rewriteDeb filters data.tar.gz and updates the control tarball's md5sums
// and Installed-Size, so dpkg --verify does not report a missing file.
func rewriteDeb(p string) error {
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	members, err := readAr(data)
	if err != nil {
		return err
	}
	var filtered []byte
	for i := range members {
		if members[i].name == "data.tar.gz" {
			if filtered, err = filterTarGz(members[i].data); err != nil {
				return err
			}
			members[i].data = filtered
		}
	}
	if filtered == nil {
		return fmt.Errorf("no data.tar.gz in %s", p)
	}
	size, err := installedSize(filtered)
	if err != nil {
		return err
	}
	for i := range members {
		if members[i].name != "control.tar.gz" {
			continue
		}
		out, err := walkTarGz(members[i].data, func(hdr *tar.Header, r io.Reader) ([]byte, bool, error) {
			content, err := io.ReadAll(r)
			if err != nil {
				return nil, false, err
			}
			switch path.Base(hdr.Name) {
			case "md5sums":
				return filterMD5Sums(content), true, nil
			case "control":
				return replaceInstalledSize(content, size), true, nil
			}
			return content, true, nil
		})
		if err != nil {
			return err
		}
		members[i].data = out
	}
	return os.WriteFile(p, writeAr(members), 0o644)
}

// filterMD5Sums drops the lines naming files that are no longer installed.
func filterMD5Sums(data []byte) []byte {
	out := make([]byte, 0, len(data))
	for line := range strings.Lines(string(data)) {
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 && isBundled(fields[len(fields)-1]) {
			continue
		}
		out = append(out, []byte(trimmed+"\n")...)
	}
	return out
}

// arMember is one member of a Debian package's ar archive. The header is kept
// byte for byte so the rewritten package keeps its metadata.
type arMember struct {
	header [60]byte
	name   string
	data   []byte
}

func readAr(data []byte) ([]arMember, error) {
	if !bytes.HasPrefix(data, []byte("!<arch>\n")) {
		return nil, fmt.Errorf("not an ar archive")
	}
	pos := len("!<arch>\n")
	var members []arMember
	for pos+60 <= len(data) {
		var h [60]byte
		copy(h[:], data[pos:pos+60])
		if string(h[58:60]) != "`\n" {
			return nil, fmt.Errorf("bad ar member header at %d", pos)
		}
		size, err := strconv.ParseInt(strings.TrimSpace(string(h[48:58])), 10, 64)
		if err != nil || size < 0 {
			return nil, fmt.Errorf("bad ar member size at %d", pos)
		}
		start := pos + 60
		end := start + int(size)
		if end > len(data) {
			return nil, fmt.Errorf("truncated ar member at %d", pos)
		}
		members = append(members, arMember{
			header: h,
			name:   strings.TrimSpace(string(h[0:16])),
			data:   append([]byte(nil), data[start:end]...),
		})
		pos = end
		if size%2 == 1 {
			pos++
		}
	}
	return members, nil
}

func writeAr(members []arMember) []byte {
	var out bytes.Buffer
	out.WriteString("!<arch>\n")
	for _, m := range members {
		h := m.header
		copy(h[48:58], []byte(fmt.Sprintf("%-10d", len(m.data))))
		out.Write(h[:])
		out.Write(m.data)
		if len(m.data)%2 == 1 {
			out.WriteByte('\n')
		}
	}
	return out.Bytes()
}

// rewriteFileList drops the removed library from .mygo-files, which the
// updater uses to know what an older version installed.
func rewriteFileList(dir string) error {
	p := filepath.Join(dir, ".mygo-files")
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var out strings.Builder
	for line := range strings.Lines(string(data)) {
		name := strings.TrimRight(line, "\r\n")
		if name == "" || isBundled(name) {
			continue
		}
		out.WriteString(name + "\n")
	}
	if out.String() == string(data) {
		return nil
	}
	return os.WriteFile(p, []byte(out.String()), 0o644)
}

var (
	sourceLine = regexp.MustCompile(`(?m)^source=\(([^)]*)\)`)
	sumsLine   = regexp.MustCompile(`(?m)^sha256sums=\(([^)]*)\)`)
	quoted     = regexp.MustCompile(`'[^']*'`)
)

// rewritePKGBUILD refreshes the archive's checksum, since cleaning the
// .tar.gz changes its bytes.
func rewritePKGBUILD(dir string) error {
	p := filepath.Join(dir, "PKGBUILD")
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	source := sourceLine.FindSubmatch(data)
	sums := sumsLine.FindSubmatch(data)
	if source == nil || sums == nil {
		return nil
	}
	name := quoted.FindString(string(source[1]))
	if name == "" {
		return nil
	}
	name = strings.Trim(name, "'")
	archive := filepath.Join(dir, name)
	contents, err := os.ReadFile(archive)
	if err != nil {
		return fmt.Errorf("PKGBUILD names %s: %w", name, err)
	}
	sum := sha256.Sum256(contents)
	hexSum := hex.EncodeToString(sum[:])
	loc := sumsLine.FindIndex(data)
	if loc == nil {
		return nil
	}
	line := string(data[loc[0]:loc[1]])
	first := quoted.FindStringIndex(line)
	if first == nil {
		return nil
	}
	newLine := line[:first[0]] + "'" + hexSum + "'" + line[first[1]:]
	out := append([]byte(nil), data[:loc[0]]...)
	out = append(out, []byte(newLine)...)
	out = append(out, data[loc[1]:]...)
	return os.WriteFile(p, out, 0o644)
}
