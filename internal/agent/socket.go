package agent

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// maximumLineLength bounds a line so a client cannot make the listener buffer
// without limit. Longer lines are not events cde would send.
const maximumLineLength = 64 * 1024

// SocketName is the file name of a running Code's agent socket.
func SocketName(pid int) string { return fmt.Sprintf("agent.%d.sock", pid) }

// SocketPath returns the socket path for a pid in a directory.
func SocketPath(dir string, pid int) string { return filepath.Join(dir, SocketName(pid)) }

// NewestSocket returns the socket of the newest running Code, for cde run
// outside Code.
func NewestSocket(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	best := ""
	var bestMod int64
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "agent.") || !strings.HasSuffix(name, ".sock") {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "agent."), ".sock"))
		if err != nil || !processRunning(pid) {
			continue
		}
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if mod := info.ModTime().UnixNano(); mod > bestMod {
			best, bestMod = path, mod
		}
	}
	return best
}

// RemoveStaleSockets deletes sockets whose Code is no longer running.
func RemoveStaleSockets(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "agent.") || !strings.HasSuffix(name, ".sock") {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "agent."), ".sock"))
		if err == nil && !processRunning(pid) {
			os.Remove(filepath.Join(dir, name))
		}
	}
}

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, os.ErrPermission)
}

// Listener accepts connections from cde and hands every event it reads to
// Deliver. Each client is read on its own goroutine, so a slow client never
// holds up the others.
type Listener struct {
	path    string
	Deliver func(Event)

	mu       sync.Mutex
	listener net.Listener
	clients  map[net.Conn]struct{}
	closed   bool
}

// NewListener makes a listener for a socket path.
func NewListener(path string, deliver func(Event)) *Listener {
	return &Listener{path: path, Deliver: deliver, clients: map[net.Conn]struct{}{}}
}

// Path returns the socket path.
func (l *Listener) Path() string { return l.path }

// Start binds the socket and begins accepting.
func (l *Listener) Start() error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	os.Remove(l.path)
	ln, err := net.Listen("unix", l.path)
	if err != nil {
		return err
	}
	// Only the user may connect, as the events name their terminals.
	os.Chmod(l.path, 0o600)
	l.mu.Lock()
	l.listener = ln
	l.mu.Unlock()
	go l.accept(ln)
	return nil
}

// Stop closes the listener and every client, and removes the socket.
func (l *Listener) Stop() {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	l.closed = true
	ln := l.listener
	clients := make([]net.Conn, 0, len(l.clients))
	for conn := range l.clients {
		clients = append(clients, conn)
	}
	l.clients = map[net.Conn]struct{}{}
	l.mu.Unlock()

	if ln != nil {
		ln.Close()
	}
	for _, conn := range clients {
		conn.Close()
	}
	os.Remove(l.path)
}

func (l *Listener) accept(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		l.mu.Lock()
		if l.closed {
			l.mu.Unlock()
			conn.Close()
			return
		}
		l.clients[conn] = struct{}{}
		l.mu.Unlock()
		go l.read(conn)
	}
}

func (l *Listener) read(conn net.Conn) {
	defer func() {
		l.mu.Lock()
		delete(l.clients, conn)
		l.mu.Unlock()
		conn.Close()
	}()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 4096), maximumLineLength)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		event, err := ParseLine(line)
		if err != nil {
			continue
		}
		if l.Deliver != nil {
			l.Deliver(event)
		}
	}
}
