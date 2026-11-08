// Command beer-menu-api runs the Beer Menu REST API. It reads BEER_MENU_DATA_DIR
// (default /data) and BEER_MENU_PORT (default 8080), prepares and migrates the database,
// then serves HTTP until SIGTERM or SIGINT. "beer-menu-api healthcheck" probes the
// running API's health endpoint and exits 0 when it returns 200, else 1.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/helmnet/beer-menu/api/internal/httpapi"
	"github.com/helmnet/beer-menu/api/internal/store"
)

// version is the release version, set with -ldflags "-X main.version=X.Y.Z".
var version = "dev"

func main() {
	// Database, -wal, -shm and backup files hold secrets in later features; create
	// them 0600 and folders 0700 from the first byte.
	syscall.Umask(0o077)
	groups, err := os.Getgroups()
	if err == nil {
		err = checkNotRoot(os.Geteuid(), os.Getegid(), groups)
	}
	if err != nil {
		slog.Error("startup refused", "err", err)
		os.Exit(1)
	}
	cfg := config{
		DataDir: envOr("BEER_MENU_DATA_DIR", "/data"),
		Port:    envOr("BEER_MENU_PORT", "8080"),
	}
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck(cfg.Port))
	}
	if err := run(cfg); err != nil {
		slog.Error("beer-menu-api stopped", "err", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	slog.Info("starting beer-menu-api", "version", version, "data_dir", cfg.DataDir)
	db, err := startup(ctx, cfg, store.Migrations, time.Now)
	if err != nil {
		return err
	}
	defer db.Close()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewMux(httpapi.Deps{DB: db, Version: version}),
		ReadHeaderTimeout: 10 * time.Second,
		// Without these an idle keep-alive or a slow request body holds a connection forever.
		ReadTimeout: 30 * time.Second,
		IdleTimeout: 2 * time.Minute,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	slog.Info("listening", "addr", srv.Addr)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// checkNotRoot returns an error naming the UID/GID when the UID, the GID or any
// supplementary group is 0. The deployer picks the UID/GID (and can add groups with
// group_add), so root must be refused at runtime.
func checkNotRoot(uid, gid int, groups []int) error {
	if uid == 0 || gid == 0 || slices.Contains(groups, 0) {
		return fmt.Errorf("refusing to run as root: running as UID %d, GID %d, groups %v; set a non-root UID and GID and no root group", uid, gid, groups)
	}
	return nil
}

// healthcheck is the container health probe: the scratch image has no shell or curl.
func healthcheck(port string) int {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/api/health")
	if err != nil {
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
