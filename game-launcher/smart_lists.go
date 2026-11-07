package main

import "os"

func launchFileMissing(path string) bool {
	if path == "" {
		return true
	}
	_, err := os.Stat(path)
	// Permission/I/O failures do not prove that a file is absent; .app bundles
	// are directories and are valid launch targets on macOS.
	return os.IsNotExist(err)
}
