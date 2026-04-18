package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ohjann/ralphplusplus/internal/viewer"
)

// runViewer implements `ralph viewer [--port N]`. Follows the install-skill
// dispatch pattern: it runs before config.Parse so it needs no prd.json and
// works in any cwd.
func runViewer(args []string) error {
	fs := flag.NewFlagSet("viewer", flag.ContinueOnError)
	port := fs.Int("port", 0, "TCP port to bind on 127.0.0.1 (0 = OS-chosen)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	lockF, existing, err := viewer.Acquire()
	if err != nil {
		return fmt.Errorf("viewer lock: %w", err)
	}
	if existing != nil {
		token, tokErr := viewer.LoadOrCreateToken()
		if tokErr != nil {
			return fmt.Errorf("read token: %w", tokErr)
		}
		fmt.Printf("http://127.0.0.1:%d/?token=%s\n", existing.Port, token)
		return nil
	}
	defer viewer.Release(lockF)

	token, err := viewer.LoadOrCreateToken()
	if err != nil {
		return fmt.Errorf("viewer token: %w", err)
	}

	// Server lifetime controls the projects.Index fsnotify watcher.
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()

	vs, err := viewer.NewServer(serverCtx, token, Version)
	if err != nil {
		return fmt.Errorf("viewer server: %w", err)
	}
	handler := vs.Handler()

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		return fmt.Errorf("listener returned non-TCP addr %T", ln.Addr())
	}
	actualPort := tcpAddr.Port

	if err := viewer.Write(lockF, viewer.LockInfo{
		PID:       os.Getpid(),
		Port:      actualPort,
		StartedAt: time.Now(),
	}); err != nil {
		_ = ln.Close()
		return fmt.Errorf("write lockfile: %w", err)
	}

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Printf("http://127.0.0.1:%d/?token=%s\n", actualPort, token)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	return nil
}
