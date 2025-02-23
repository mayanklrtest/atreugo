//go:build !windows
// +build !windows

package atreugo

import (
	"net"
	"os"
	"os/signal"
	"runtime"

	"github.com/valyala/fasthttp/prefork"
)

// IsPreforkChild checks if the current thread/process is a child.
func IsPreforkChild() bool {
	return prefork.IsChild()
}

func (s *Atreugo) newPreforkServer() *prefork.Prefork {
	p := &prefork.Prefork{
		Network:          s.cfg.Network,
		Reuseport:        s.cfg.Reuseport,
		RecoverThreshold: runtime.GOMAXPROCS(0) / 2,
		Logger:           s.cfg.Logger,
		ServeFunc:        s.Serve,
	}

	if s.cfg.GracefulShutdown {
		p.ServeFunc = s.ServeGracefully
	}

	return p
}

// ServeGracefully serves incoming connections from the given listener with graceful shutdown
//
// It's blocked until the given listener returns permanent error.
//
// WARNING: Windows is not supportted.
func (s *Atreugo) ServeGracefully(ln net.Listener) error {
	s.cfg.GracefulShutdown = true

	listenErr := make(chan error, 1)

	go func() {
		listenErr <- s.Serve(ln)
	}()

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, s.cfg.GracefulShutdownSignals...)

	select {
	case err := <-listenErr:
		return err
	case <-osSignals:
		s.cfg.Logger.Print("Shutdown signal received")

		if err := s.engine.Shutdown(); err != nil {
			return wrapError(err, "failed to shutdown")
		}

		s.cfg.Logger.Print("Server gracefully stopped")
	}

	return nil
}

// ListenAndServe serves requests from the given network and address in the atreugo configuration.
//
// Pass custom listener to Serve/ServeGracefully if you want to use it.
func (s *Atreugo) ListenAndServe() error {
	if s.cfg.Prefork {
		return s.newPreforkServer().ListenAndServe(s.cfg.Addr) // nolint:wrapcheck
	}

	ln, err := s.getListener()
	if err != nil {
		return err
	}

	if s.cfg.GracefulShutdown {
		return s.ServeGracefully(ln)
	}

	return s.Serve(ln)
}
