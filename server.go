package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

func serveHTML(ctx context.Context, pr ParseResult) {
	loaded := make(chan struct{})

	charts := generateCharts(pr)

	http.HandleFunc("GET /", func(writer http.ResponseWriter, request *http.Request) {
		rendered, err := render(pr, charts, true)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte(fmt.Sprintf("Error rendering HTML: %s", err)))
			slog.Error("Error rendering HTML", "err", err)
			return
		}

		_, _ = writer.Write([]byte(rendered))
	})
	http.HandleFunc("GET /loaded", func(writer http.ResponseWriter, request *http.Request) {
		loadedHandler(writer, request, loaded)
	})

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		slog.Error("Error creating listener", "err", err)
		return
	}
	port := listener.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	server := &http.Server{}
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Error serving", "err", err)
		}
	}()

	url := fmt.Sprintf("http://localhost:%d", port)
	slog.Debug("Opening %s in your browser", "url", url)

	err = openBrowser(url)
	if err != nil {
		slog.Error("Error opening browser", "err", err)
		return
	}

	if !keepRunning {
		select {
		case <-loaded:
			slog.Debug("Browser successfully loaded the page.")
		case <-time.After(10 * time.Second):
			slog.Error("Timeout: Browser did not load the page within 10 seconds.")
		case <-ctx.Done():
			slog.Debug("Context was canceled.")
		}
	} else {
		select {
		case <-ctx.Done():
			slog.Debug("Context was canceled.")
		}
	}

	slog.Debug("Shutting down the server...")
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Error shutting down server", "err", err)
	}
}

func loadedHandler(w http.ResponseWriter, r *http.Request, loaded chan struct{}) {
	// Signal that the page has been loaded
	select {
	case loaded <- struct{}{}:
	default:
		// Channel already signaled, do nothing
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
