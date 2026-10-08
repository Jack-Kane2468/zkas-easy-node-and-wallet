package rotatinglog

import (
	"os"
	"sync"
	"time"
)

// Writer keeps the child process's output pipe healthy when a log viewer holds
// a Windows sharing lock. Rotation failure must not permanently close the log.
type Writer struct {
	mu          sync.Mutex
	path        string
	file        *os.File
	size, limit int64
	retryAt     time.Time
	rename      func(string, string) error
}

func New(path string, limit int64) (*Writer, error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	st, e := f.Stat()
	if e != nil {
		f.Close()
		return nil, e
	}
	return &Writer{path: path, file: f, size: st.Size(), limit: limit, rename: os.Rename}, nil
}
func (l *Writer) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.size+int64(len(b)) > l.limit && time.Now().After(l.retryAt) {
		l.file.Close()
		os.Remove(l.path + ".1")
		if e := l.rename(l.path, l.path+".1"); e != nil {
			// Another reader can temporarily prohibit rename on Windows. Reopen the
			// current file and keep streaming; retry rotation later without spinning.
			l.retryAt = time.Now().Add(5 * time.Second)
		} else {
			l.size = 0
			l.retryAt = time.Time{}
		}
		f, e := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			return 0, e
		}
		l.file = f
	}
	n, e := l.file.Write(b)
	l.size += int64(n)
	return n, e
}
func (l *Writer) Close() error { l.mu.Lock(); defer l.mu.Unlock(); return l.file.Close() }
