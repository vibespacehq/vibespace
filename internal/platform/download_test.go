package platform

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifySHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	content := []byte("hello vibespace")

	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Compute correct hash
	h := sha256.Sum256(content)
	correctHash := hex.EncodeToString(h[:])

	// Valid hash should pass
	if err := verifySHA256(path, correctHash); err != nil {
		t.Errorf("verifySHA256 with correct hash returned error: %v", err)
	}

	// Wrong hash should fail
	err := verifySHA256(path, "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Error("verifySHA256 with wrong hash should return error")
	}
}

func TestVerifySHA256MissingFile(t *testing.T) {
	err := verifySHA256("/nonexistent/file", "abc")
	if err == nil {
		t.Error("verifySHA256 on missing file should return error")
	}
}

func TestParseSHA256SUMS(t *testing.T) {
	content := `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  empty.txt
a591a6d40bf420404a011733cfb7b190d62c65bf0bcda32b57b277d9ad9f146e  hello.txt
b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c  other.bin`

	t.Run("existing asset", func(t *testing.T) {
		hash, err := parseSHA256SUMS(content, "hello.txt")
		if err != nil {
			t.Fatalf("parseSHA256SUMS error: %v", err)
		}
		if hash != "a591a6d40bf420404a011733cfb7b190d62c65bf0bcda32b57b277d9ad9f146e" {
			t.Errorf("hash = %q, unexpected", hash)
		}
	})

	t.Run("missing asset", func(t *testing.T) {
		_, err := parseSHA256SUMS(content, "notfound.txt")
		if err == nil {
			t.Error("parseSHA256SUMS for missing asset should return error")
		}
	})
}

// createTarGz creates a tar.gz in memory with the given file entries.
func createTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     0644,
			Size:     int64(len(e.content)),
			Typeflag: e.typeflag,
			Linkname: e.linkname,
		}
		if e.typeflag == tar.TypeDir {
			hdr.Mode = 0755
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader error: %v", err)
		}
		if e.typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(e.content)); err != nil {
				t.Fatalf("Write error: %v", err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tar close error: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close error: %v", err)
	}
	return buf.Bytes()
}

type tarEntry struct {
	name     string
	content  string
	typeflag byte
	linkname string
}

func TestExtractTarGz(t *testing.T) {
	data := createTarGz(t, []tarEntry{
		{name: "dir/", typeflag: tar.TypeDir},
		{name: "dir/hello.txt", content: "hello world", typeflag: tar.TypeReg},
		{name: "script.sh", content: "#!/bin/sh\necho hi", typeflag: tar.TypeReg},
	})

	dir := t.TempDir()
	if err := extractTarGz(bytes.NewReader(data), dir); err != nil {
		t.Fatalf("extractTarGz error: %v", err)
	}

	// Check file exists and has correct content
	content, err := os.ReadFile(filepath.Join(dir, "dir", "hello.txt"))
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("file content = %q, want %q", string(content), "hello world")
	}

	// Check script exists
	if _, err := os.Stat(filepath.Join(dir, "script.sh")); err != nil {
		t.Errorf("script.sh should exist: %v", err)
	}
}

func TestExtractTarGzTraversal(t *testing.T) {
	data := createTarGz(t, []tarEntry{
		{name: "../escape.txt", content: "evil", typeflag: tar.TypeReg},
	})

	dir := t.TempDir()
	err := extractTarGz(bytes.NewReader(data), dir)
	if err == nil {
		t.Error("extractTarGz with path traversal should return error")
	}
}

func TestExtractTarGzSymlink(t *testing.T) {
	data := createTarGz(t, []tarEntry{
		{name: "real.txt", content: "data", typeflag: tar.TypeReg},
		{name: "link.txt", typeflag: tar.TypeSymlink, linkname: "real.txt"},
	})

	dir := t.TempDir()
	if err := extractTarGz(bytes.NewReader(data), dir); err != nil {
		t.Fatalf("extractTarGz with symlink error: %v", err)
	}

	// Verify symlink exists
	info, err := os.Lstat(filepath.Join(dir, "link.txt"))
	if err != nil {
		t.Fatalf("Lstat error: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("link.txt should be a symlink")
	}
}
