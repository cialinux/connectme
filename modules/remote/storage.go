package remote

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"unicode"
)

const maxFileBytes int64 = 32 << 20
const sessionFileQuota int64 = 128 << 20
const maxClipboardBytes = 64 << 10

var sessionIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var errTransferLimit = errors.New("transfer limit")

type fileStore struct{ base string }
type fileSpace struct {
	store  *fileStore
	id     string
	root   *os.Root
	mu     sync.Mutex
	once   sync.Once
	done   chan struct{}
	closed bool
}
type fileEntry struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func newFileStore(base string) (*fileStore, error) {
	if base == "" {
		return nil, nil
	}
	if !filepath.IsAbs(base) || filepath.Clean(base) == "/" {
		return nil, errors.New("invalid transfer root")
	}
	base = filepath.Clean(base)
	info, err := os.Lstat(base)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("transfer root must be an existing directory")
	}
	pending := filepath.Join(base, ".pending")
	if err = os.Mkdir(pending, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	info, err = os.Lstat(pending)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("invalid pending directory")
	}
	store := &fileStore{base: base}
	// These are temporary, server-generated session directories, never user paths.
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if sessionIDPattern.MatchString(entry.Name()) {
			if err = os.RemoveAll(filepath.Join(base, entry.Name())); err != nil {
				return nil, err
			}
		}
	}
	entries, err = os.ReadDir(pending)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "upload-") && !entry.IsDir() {
			if err = os.Remove(filepath.Join(pending, entry.Name())); err != nil {
				return nil, err
			}
		}
	}
	return store, nil
}
func (s *fileStore) create(id string) (*fileSpace, error) {
	if s == nil || !sessionIDPattern.MatchString(id) {
		return nil, errors.New("file transfer unavailable")
	}
	dir := filepath.Join(s.base, id)
	if err := os.Mkdir(dir, 0700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		os.Remove(dir)
		return nil, err
	}
	return &fileSpace{store: s, id: id, root: root, done: make(chan struct{})}, nil
}
func (s *fileSpace) close() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		close(s.done)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.closed = true
		s.root.Close()
		if sessionIDPattern.MatchString(s.id) {
			_ = os.RemoveAll(filepath.Join(s.store.base, s.id))
		}
	})
}
func validFilename(name string) bool {
	if name == "" || len(name) > 160 || strings.HasPrefix(name, ".") || strings.TrimSpace(name) != name || strings.HasSuffix(name, ".") || strings.ContainsAny(name, `/\:*?"<>|`) {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	stem := strings.ToUpper(strings.Split(name, ".")[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" {
		return false
	}
	if len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9' {
		return false
	}
	return true
}
func (s *fileSpace) list() ([]fileEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, fs.ErrClosed
	}
	f, err := s.root.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(256)
	if err != nil && err != io.EOF {
		return nil, err
	}
	result := []fileEntry{}
	for _, entry := range entries {
		if !validFilename(entry.Name()) || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, e := entry.Info()
		if e == nil && info.Mode().IsRegular() {
			result = append(result, fileEntry{entry.Name(), info.Size()})
		}
	}
	return result, nil
}
func (s *fileSpace) upload(name string, reader io.Reader) (int64, error) {
	if !validFilename(name) {
		return 0, fs.ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, fs.ErrClosed
	}
	if _, err := s.root.Lstat(name); err == nil {
		return 0, fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}
	tmp, err := os.CreateTemp(filepath.Join(s.store.base, ".pending"), "upload-")
	if err != nil {
		return 0, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	n, err := io.Copy(tmp, io.LimitReader(reader, maxFileBytes+1))
	if err != nil {
		return n, err
	}
	if n > maxFileBytes {
		return n, errTransferLimit
	}
	select {
	case <-s.done:
		return n, fs.ErrClosed
	default:
	}
	var used int64
	var count int
	err = fs.WalkDir(s.root.FS(), ".", func(path string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		count++
		if count > 4096 {
			return errTransferLimit
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !entry.IsDir() {
			info, e := entry.Info()
			if e != nil {
				return e
			}
			used += info.Size()
		}
		if used+n > sessionFileQuota {
			return errTransferLimit
		}
		return nil
	})
	if err != nil {
		return n, err
	}
	if err = tmp.Sync(); err != nil {
		return n, err
	}
	if err = tmp.Close(); err != nil {
		return n, err
	}
	// Hard-link publication is atomic and never overwrites an existing target.
	// Both paths are beneath the fixed transfer mount; the session root is generated.
	err = os.Link(tmp.Name(), filepath.Join(s.store.base, s.id, name))
	return n, err
}
func (s *fileSpace) download(name string) (*os.File, int64, error) {
	if !validFilename(name) {
		return nil, 0, fs.ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, 0, fs.ErrClosed
	}
	f, err := s.root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, 0, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, 0, fs.ErrInvalid
	}
	if info.Size() > maxFileBytes {
		f.Close()
		return nil, 0, errTransferLimit
	}
	return f, info.Size(), nil
}
