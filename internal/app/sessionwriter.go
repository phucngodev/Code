package app

import (
	"log"
	"sync"
	"time"

	"github.com/phucngodev/code/internal/model"
)

// sessionWriter writes session snapshots on its own goroutine, so saving never
// stalls the frame that requested it. Only the newest snapshot is kept while a
// write is in flight, since the file wants the latest state, not every step.
type sessionWriter struct {
	path string

	mu      sync.Mutex
	pending *model.Session
	writing bool
	wake    chan struct{}
}

func newSessionWriter(path string) *sessionWriter {
	w := &sessionWriter{path: path, wake: make(chan struct{}, 1)}
	go w.loop()
	return w
}

// save queues a snapshot, replacing one not yet written.
func (w *sessionWriter) save(session *model.Session) {
	w.mu.Lock()
	w.pending = session
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *sessionWriter) loop() {
	for range w.wake {
		for {
			w.mu.Lock()
			session := w.pending
			w.pending = nil
			if session == nil {
				w.mu.Unlock()
				break
			}
			w.writing = true
			w.mu.Unlock()

			if err := session.Save(w.path); err != nil {
				log.Printf("codeterm: could not save the session: %v", err)
			}

			w.mu.Lock()
			w.writing = false
			w.mu.Unlock()
		}
	}
}

// flush writes everything queued before it and waits, bounded, so quitting
// cannot hang on a stalled disk.
func (w *sessionWriter) flush(session *model.Session) {
	w.save(session)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		w.mu.Lock()
		idle := w.pending == nil && !w.writing
		w.mu.Unlock()
		if idle {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}
