package stats

import (
	"os"
	"path/filepath"
)

var atomicWriteCreateTempFile = os.CreateTemp
var atomicWriteRenameFile = os.Rename
var atomicReplaceFile = atomicReplaceFileImpl

func writeFileAtomically(path string, data []byte, perm os.FileMode) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp, err := atomicWriteCreateTempFile(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := atomicWriteRenameFile(tmpPath, path); err != nil {
		if err := atomicReplaceFile(tmpPath, path); err != nil {
			return err
		}
	}
	return nil
}
