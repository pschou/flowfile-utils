package tempfile

import (
	"math/rand"
	"os"
	"path"
	"sync"
)

// The purpose of this package is to provide temp files and then clean them up
// when a signal terminate is received.

func Cleanup() {
	for file, _ := range tmpFile {
		os.Remove(file)
	}
}

var (
	// Folder in which to create temp files
	Folder = "/tmp"

	// Prefix to add to the temp file names
	Prefix = "tmp-"

	// Toggle debug
	Debug        = false
	tmpFile      = make(map[string]struct{})
	tmpFileMutex sync.Mutex
)

// Remove a file and free up the resource name for future use
func Remove(f string) {
	tmpFileMutex.Lock()
	defer tmpFileMutex.Unlock()
	if _, ok := tmpFile[f]; ok {
		delete(tmpFile, f)
		os.Remove(f)
	}
}

// Create a new file path for use
func New() string {
	tmpFileMutex.Lock()
	defer tmpFileMutex.Unlock()
	for {
		test := path.Join(Folder, Prefix+randStringBytes(8))
		if _, ok := tmpFile[test]; !ok {
			tmpFile[test] = struct{}{}
			return test
		}
	}
}

func randStringBytes(n int) string {
	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
