//go:build !windows

package stats

func atomicReplaceFileImpl(oldPath, newPath string) error {
	return atomicWriteRenameFile(oldPath, newPath)
}
