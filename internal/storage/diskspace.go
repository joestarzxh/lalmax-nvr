package storage

// FilesystemSpace exposes filesystem capacity to other packages.
func FilesystemSpace(path string) (total uint64, free uint64, err error) {
	return filesystemSpace(path)
}
