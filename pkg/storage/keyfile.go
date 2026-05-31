package storage

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// KeyFile is a file-backed wallet key store: it gob-encodes a map of
// address -> 32-byte seed to a single file. It structurally satisfies
// wallet.KeyStore (the storage_test asserts this) without storage importing the
// wallet package.
//
// Learning project: seeds are stored UNENCRYPTED. The file must be kept separate
// from the chain database and never committed.
type KeyFile struct {
	path string
}

// NewKeyFile returns a KeyFile backed by the given path.
func NewKeyFile(path string) *KeyFile {
	return &KeyFile{path: path}
}

// Load returns all stored address→seed pairs, or an empty map if the file does
// not exist yet.
func (f *KeyFile) Load() (map[string][]byte, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return make(map[string][]byte), nil
	}
	if err != nil {
		return nil, err
	}
	seeds := make(map[string][]byte)
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&seeds); err != nil {
		return nil, err
	}
	return seeds, nil
}

// Save persists the seed for an address. It reads the current map, updates the
// entry, and writes the whole map back atomically (temp file + rename) so a crash
// mid-write cannot corrupt the only copy of the keys.
func (f *KeyFile) Save(address string, seed []byte) error {
	seeds, err := f.Load()
	if err != nil {
		return err
	}
	seeds[address] = append([]byte(nil), seed...)

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(seeds); err != nil {
		return err
	}
	return atomicWrite(f.path, buf.Bytes())
}

// atomicWrite writes data to a temp file in the same directory, then renames it
// over path. Seeds are sensitive, so the file is created with 0600 perms.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".keyfile-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if the rename succeeded

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
