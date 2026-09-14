package remote

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureSession = "0123456789abcdef0123456789abcdef"

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

type failedReader struct{}

func (failedReader) Read(p []byte) (int, error) { copy(p, "partial"); return 7, io.ErrUnexpectedEOF }

func TestTransferNames(t *testing.T) {
	for _, name := range []string{"", "../secret", "a/b", "a\\b", "CON", "nul.txt", "LPT1.log", "file:stream", ".hidden", "name.", " leading", "x\n.txt"} {
		if validFilename(name) {
			t.Errorf("accepted %q", name)
		}
	}
	for _, name := range []string{"teste.txt", "relatório 2026.pdf", "backup.tar.gz"} {
		if !validFilename(name) {
			t.Errorf("rejected %q", name)
		}
	}
}
func TestTransferStoreIsolationAndCleanup(t *testing.T) {
	base := t.TempDir()
	store, err := newFileStore(base)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.create(fixtureSession)
	if err != nil {
		t.Fatal(err)
	}
	defer first.close()
	second, err := store.create(strings.Repeat("a", 32))
	if err != nil {
		t.Fatal(err)
	}
	defer second.close()
	data := []byte{0, 1, 2, 255, 10}
	if n, err := first.upload("binary.dat", bytes.NewReader(data)); err != nil || n != int64(len(data)) {
		t.Fatalf("upload %d %v", n, err)
	}
	if _, err = first.upload("binary.dat", strings.NewReader("replacement")); !errors.Is(err, fs.ErrExist) {
		t.Fatal("overwrite allowed")
	}
	f, n, err := first.download("binary.dat")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(f)
	f.Close()
	if !bytes.Equal(got, data) || n != int64(len(data)) {
		t.Fatal("binary changed")
	}
	if _, _, err = second.download("binary.dat"); err == nil {
		t.Fatal("cross-session leak")
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err = os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(outside, filepath.Join(base, fixtureSession, "link.txt")); err != nil {
		t.Fatal(err)
	}
	if _, _, err = first.download("link.txt"); err == nil {
		t.Fatal("symlink escaped root")
	}
	first.close()
	if _, err = os.Stat(filepath.Join(base, fixtureSession)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("session not cleaned")
	}
	if _, err = os.Stat(outside); err != nil {
		t.Fatal("cleanup followed symlink")
	}
}
func TestTransferLimitsAndPartialCleanup(t *testing.T) {
	base := t.TempDir()
	store, _ := newFileStore(base)
	space, err := store.create(fixtureSession)
	if err != nil {
		t.Fatal(err)
	}
	defer space.close()
	if _, err = space.upload("large.bin", io.LimitReader(zeroReader{}, maxFileBytes+1)); !errors.Is(err, errTransferLimit) {
		t.Fatalf("large file accepted: %v", err)
	}
	if _, err = space.upload("partial.txt", failedReader{}); err == nil {
		t.Fatal("partial upload accepted")
	}
	items, _ := space.list()
	if len(items) != 0 {
		t.Fatal("failed upload published")
	}
	pending, _ := os.ReadDir(filepath.Join(base, ".pending"))
	if len(pending) != 0 {
		t.Fatal("partial files left behind")
	}
	quota, err := space.root.OpenFile("quota.bin", os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = quota.Truncate(sessionFileQuota)
	quota.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = space.upload("extra.txt", strings.NewReader("x")); !errors.Is(err, errTransferLimit) {
		t.Fatal("quota exceeded")
	}
}
