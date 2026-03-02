/*
Copyright 2024 DevOps Click.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package main is the entrypoint for the NGINX Config Reloader sidecar.
// It watches a directory for configuration changes and performs safe,
// validated NGINX reloads. Runs as a sidecar container alongside NGINX.
//
// Usage:
//
//	./reloader --watch-dir=/etc/nginx --nginx-binary=/usr/sbin/nginx
//	./reloader --debug --reload-timeout=30s
package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/devops-click/nginx-operator/internal/version"
)

// reloader watches a directory for NGINX config changes and performs safe reloads.
type reloader struct {
	watchDir      string
	nginxBinary   string
	reloadTimeout time.Duration
	pollInterval  time.Duration
	debug         bool

	mu             sync.Mutex
	lastConfigHash string
	lastReloadTime time.Time
	reloadCount    int64
	lastError      string
	healthy        bool
}

func main() {
	var (
		watchDir      string
		nginxBinary   string
		reloadTimeout time.Duration
		pollInterval  time.Duration
		healthAddr    string
		debug         bool
		showVersion   bool
	)

	flag.StringVar(&watchDir, "watch-dir", "/etc/nginx", "Directory to watch for config changes.")
	flag.StringVar(&nginxBinary, "nginx-binary", "/usr/sbin/nginx", "Path to the nginx binary.")
	flag.DurationVar(&reloadTimeout, "reload-timeout", 30*time.Second, "Timeout for nginx reload operations.")
	flag.DurationVar(&pollInterval, "poll-interval", 2*time.Second, "Interval between config change polls.")
	flag.StringVar(&healthAddr, "health-address", ":8082", "Address for health endpoint.")
	flag.BoolVar(&debug, "debug", false, "Enable debug logging.")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit.")
	flag.Parse()

	if showVersion {
		fmt.Println(version.Get().String())
		os.Exit(0)
	}

	r := &reloader{
		watchDir:      watchDir,
		nginxBinary:   nginxBinary,
		reloadTimeout: reloadTimeout,
		pollInterval:  pollInterval,
		debug:         debug,
		healthy:       true,
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Start health endpoint
	go r.startHealthServer(healthAddr)

	// Compute initial config hash
	hash, err := r.computeConfigHash()
	if err != nil {
		logError("failed to compute initial config hash: %v", err)
		os.Exit(1)
	}
	r.lastConfigHash = hash
	logInfo("reloader started, watching %s (initial hash: %s)", watchDir, hash[:12])

	// Main watch loop
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logInfo("shutting down reloader")
			return
		case sig := <-sigCh:
			logInfo("received signal %s, shutting down", sig)
			cancel()
			return
		case <-ticker.C:
			if err := r.checkAndReload(ctx); err != nil {
				logError("reload check failed: %v", err)
			}
		}
	}
}

// checkAndReload compares the current config hash with the last known hash.
// If they differ, it validates the new config and performs a reload.
func (r *reloader) checkAndReload(ctx context.Context) error {
	hash, err := r.computeConfigHash()
	if err != nil {
		return fmt.Errorf("failed to compute config hash: %w", err)
	}

	r.mu.Lock()
	if hash == r.lastConfigHash {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	logInfo("config change detected (old: %s, new: %s)", r.lastConfigHash[:12], hash[:12])

	// Validate config before reload
	if err := r.validateConfig(ctx); err != nil {
		r.mu.Lock()
		r.lastError = err.Error()
		r.healthy = false
		r.mu.Unlock()
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Perform reload
	if err := r.reloadNginx(ctx); err != nil {
		r.mu.Lock()
		r.lastError = err.Error()
		r.healthy = false
		r.mu.Unlock()
		return fmt.Errorf("reload failed: %w", err)
	}

	// Update state
	r.mu.Lock()
	r.lastConfigHash = hash
	r.lastReloadTime = time.Now()
	r.reloadCount++
	r.lastError = ""
	r.healthy = true
	r.mu.Unlock()

	logInfo("nginx reloaded successfully (hash: %s, total reloads: %d)", hash[:12], r.reloadCount)
	return nil
}

// validateConfig runs nginx -t to validate the current configuration.
func (r *reloader) validateConfig(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.reloadTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.nginxBinary, "-t")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx -t failed: %s: %w", string(output), err)
	}

	if r.debug {
		logDebug("nginx -t output: %s", string(output))
	}

	return nil
}

// reloadNginx sends a reload signal to the NGINX master process.
func (r *reloader) reloadNginx(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.reloadTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.nginxBinary, "-s", "reload")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx -s reload failed: %s: %w", string(output), err)
	}

	return nil
}

// computeConfigHash walks the watch directory and computes a combined SHA-256 hash
// of all .conf files. This detects any config change including additions, deletions,
// and modifications.
func (r *reloader) computeConfigHash() (string, error) {
	h := sha256.New()

	err := filepath.WalkDir(r.watchDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Only hash .conf files and nginx.conf
		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		name := filepath.Base(path)
		if ext != ".conf" && name != "nginx.conf" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		// Include path in hash to detect file renames/moves
		h.Write([]byte(path))
		h.Write(data)

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk config directory: %w", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// startHealthServer starts a simple HTTP health endpoint.
func (r *reloader) startHealthServer(addr string) {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		healthy := r.healthy
		r.mu.Unlock()

		if healthy {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "ok")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "unhealthy")
		}
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		defer r.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"healthy":%t,"lastReload":"%s","reloadCount":%d,"configHash":"%s","lastError":"%s"}`,
			r.healthy,
			r.lastReloadTime.Format(time.RFC3339),
			r.reloadCount,
			r.lastConfigHash,
			r.lastError,
		)
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logInfo("health server listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logError("health server failed: %v", err)
	}
}

// --- Logging helpers (writing to stderr as per user rules) ---

func logInfo(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[INFO] %s %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

func logError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[ERROR] %s %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

func logDebug(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[DEBUG] %s %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
}
