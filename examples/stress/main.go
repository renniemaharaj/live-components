package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed templates/*.html static/*
var assets embed.FS

func main() {
	address := flag.String("addr", "127.0.0.1:9120", "HTTP listen address")
	flag.Parse()

	if err := serve(*address); err != nil {
		log.Fatal(err)
	}
}

func serve(address string) error {
	app := newLab()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	server := &http.Server{
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    16 << 10,
	}
	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	done := make(chan error, 1)
	go func() {
		done <- server.Serve(listener)
	}()

	fmt.Printf("Live Components Lab: http://%s\n", listener.Addr())
	select {
	case err := <-done:
		app.runner.stop()
		return err
	case <-shutdown.Done():
	}

	app.runner.stop()
	app.closeStreams()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		server.Close()
		return err
	}
	if err := app.runner.wait(ctx); err != nil {
		return err
	}
	if err := <-done; !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
