package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/store"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/web"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("服务退出", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	database := cfg.database
	cleanup := func() {}
	if cfg.selfcheck && database == "mural-release.db" {
		file, err := os.CreateTemp("", "mural-release-selfcheck-*.db")
		if err != nil {
			return fmt.Errorf("创建自检数据库: %w", err)
		}
		database = file.Name()
		if err := file.Close(); err != nil {
			return err
		}
		cleanup = func() { _ = os.Remove(database) }
	}
	defer cleanup()
	repository, err := store.Open(database)
	if err != nil {
		return err
	}
	service := application.NewService(repository)
	defer service.Close()
	httpServer := &http.Server{
		Handler:           web.New(service).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       12 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.addr, err)
	}
	slog.Info("壁画微生物处置放行台已启动", "addr", listener.Addr().String(), "database", database, "selfcheck", cfg.selfcheck)
	serveError := make(chan error, 1)
	go func() { serveError <- httpServer.Serve(listener) }()
	if cfg.selfcheck {
		err := runSelfcheck(listener.Addr().String())
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr := httpServer.Shutdown(shutdownContext)
		serverErr := <-serveError
		if serverErr != nil && !errors.Is(serverErr, http.ErrServerClosed) {
			return serverErr
		}
		if err != nil {
			return fmt.Errorf("自检失败: %w", err)
		}
		if shutdownErr != nil {
			return fmt.Errorf("自检关闭服务: %w", shutdownErr)
		}
		slog.Info("自检通过", "workflow", "建档→证据→评估→方案→试验→复核→冻结→签发→验真")
		return nil
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	select {
	case sig := <-stop:
		slog.Info("收到关闭信号", "signal", sig.String())
	case err := <-serveError:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("关闭 HTTP 服务: %w", err)
	}
	err = <-serveError
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
