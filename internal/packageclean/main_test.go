package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type tarFile struct {
	name string
	data []byte
}

func tarGz(t *testing.T, files []tarFile) []byte {
	t.Helper()
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	tw := tar.NewWriter(zw)
	for _, f := range files {
		hdr := &tar.Header{Name: f.name, Mode: 0o644, Size: int64(len(f.data)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(f.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func arMemberData(name string, data []byte) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%-16s%-12d%-6d%-6d%-8s%-10d`\n", name, 0, 0, 0, "100644", len(data))
	b.Write(data)
	if len(data)%2 == 1 {
		b.WriteByte('\n')
	}
	return b.Bytes()
}

func debData(t *testing.T, files []tarFile) []byte {
	t.Helper()
	var sums strings.Builder
	for _, f := range files {
		fmt.Fprintf(&sums, "%x  opt/code/%s\n", sha256.Sum256(f.data), f.name)
	}
	control := []byte("Package: code\nVersion: 0.1.0\nInstalled-Size: 99999\nArchitecture: amd64\n")
	ctl := tarGz(t, []tarFile{{"control", control}, {"md5sums", []byte(sums.String())}})
	var data []tarFile
	for _, f := range files {
		data = append(data, tarFile{name: "opt/code/" + f.name, data: f.data})
	}
	var out bytes.Buffer
	out.WriteString("!<arch>\n")
	out.Write(arMemberData("debian-binary", []byte("2.0\n")))
	out.Write(arMemberData("control.tar.gz", ctl))
	out.Write(arMemberData("data.tar.gz", tarGz(t, data)))
	return out.Bytes()
}

func write(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func listTar(t *testing.T, data []byte) []string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		names = append(names, hdr.Name)
	}
	return names
}

// TestCleanRemovesMygoLibraryFromEveryArtifact mirrors what `mygo build`
// writes for a Linux target: MyGo's library next to the executable, in the
// update archive, in the Debian package, and in the installed file list.
func TestCleanRemovesMygoLibraryFromEveryArtifact(t *testing.T) {
	dir := t.TempDir()
	local := []byte("code's libghostty-vt")
	bundled := []byte("mygo's libghostty-vt")
	files := []tarFile{{"libghostty-vt.so", bundled}, {"libghostty-vt-local-linux-amd64.so", local}}

	write(t, dir, "libghostty-vt.so", bundled)
	write(t, dir, "libghostty-vt-local-linux-amd64.so", local)
	write(t, dir, "code-0.1.0-linux-amd64.tar.gz", tarGz(t, files))
	write(t, dir, "code_0.1.0_amd64.deb", debData(t, files))
	write(t, dir, ".mygo-files", []byte("libghostty-vt.so\nlibghostty-vt-local-linux-amd64.so\n"))
	write(t, dir, "PKGBUILD", []byte("source=('code-0.1.0-linux-amd64.tar.gz' 'code.png')\nsha256sums=('old' 'icon')\n"))

	if err := clean(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "libghostty-vt.so")); !os.IsNotExist(err) {
		t.Fatal("MyGo's library is still next to the executable")
	}
	if err := verify(dir); err != nil {
		t.Fatal(err)
	}

	archive, err := os.ReadFile(filepath.Join(dir, "code-0.1.0-linux-amd64.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range listTar(t, archive) {
		if isBundled(name) {
			t.Fatalf("archive still holds %s", name)
		}
	}
	sum := sha256.Sum256(archive)
	pkgbuild, err := os.ReadFile(filepath.Join(dir, "PKGBUILD"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pkgbuild), "'"+hex.EncodeToString(sum[:])+"'") {
		t.Fatalf("PKGBUILD checksum was not refreshed: %s", pkgbuild)
	}
	list, err := os.ReadFile(filepath.Join(dir, ".mygo-files"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(list), "libghostty-vt.so") {
		t.Fatalf(".mygo-files still lists the removed library: %s", list)
	}
}

// TestCleanFixesDebianControl checks dpkg --verify will not report the file
// we removed as missing, and that Installed-Size shrinks with it.
func TestCleanFixesDebianControl(t *testing.T) {
	dir := t.TempDir()
	files := []tarFile{{"libghostty-vt.so", bytes.Repeat([]byte("x"), 4096)}, {"libghostty-vt-local-linux-amd64.so", []byte("code")}}
	write(t, dir, "libghostty-vt-local-linux-amd64.so", files[1].data)
	write(t, dir, "code_0.1.0_amd64.deb", debData(t, files))
	if err := clean(dir); err != nil {
		t.Fatal(err)
	}
	deb, err := os.ReadFile(filepath.Join(dir, "code_0.1.0_amd64.deb"))
	if err != nil {
		t.Fatal(err)
	}
	members, err := readAr(deb)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		switch m.name {
		case "data.tar.gz":
			for _, name := range listTar(t, m.data) {
				if isBundled(name) {
					t.Fatalf("deb still installs %s", name)
				}
			}
		case "control.tar.gz":
			gz, err := gzip.NewReader(bytes.NewReader(m.data))
			if err != nil {
				t.Fatal(err)
			}
			tr := tar.NewReader(gz)
			for {
				hdr, err := tr.Next()
				if err != nil {
					break
				}
				content, err := readAll(tr)
				if err != nil {
					t.Fatal(err)
				}
				switch filepath.Base(hdr.Name) {
				case "md5sums":
					if strings.Contains(string(content), "libghostty-vt.so") {
						t.Fatalf("md5sums still lists the removed file: %s", content)
					}
				case "control":
					if !strings.Contains(string(content), "Installed-Size: 1\n") {
						t.Fatalf("Installed-Size was not recalculated: %s", content)
					}
				}
			}
			gz.Close()
		}
	}
}

func readAll(r *tar.Reader) ([]byte, error) { return io.ReadAll(r) }

func TestCleanFailsWithoutLocalLibrary(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "libghostty-vt.so", []byte("mygo"))
	if err := clean(dir); err == nil {
		t.Fatal("packaging without Code's own library must fail")
	}
}

func TestVerifyDetectsBundledLibrary(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "libghostty-vt-local-linux-amd64.so", []byte("code"))
	write(t, dir, "libghostty-vt.so", []byte("mygo"))
	if err := verify(dir); err == nil {
		t.Fatal("verify should reject a bundle that still holds MyGo's library")
	}
}

func TestReadWriteArRoundTrip(t *testing.T) {
	members, err := readAr(debData(t, []tarFile{{"libghostty-vt.so", []byte("mygo")}}))
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 3 {
		t.Fatalf("ar members = %d, want 3", len(members))
	}
	again, err := readAr(writeAr(members))
	if err != nil {
		t.Fatal(err)
	}
	for i := range members {
		if again[i].name != members[i].name || !bytes.Equal(again[i].data, members[i].data) {
			t.Fatalf("member %d did not round trip", i)
		}
	}
}
