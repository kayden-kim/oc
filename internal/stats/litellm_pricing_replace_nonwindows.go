//go:build !windows

package stats

func replaceFileAtomically(oldPath, newPath string) error {
	return renameFile(oldPath, newPath)
}
