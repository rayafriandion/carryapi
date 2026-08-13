package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"carryapi/internal/settings"
)

const (
	listenHostAll       = "all"
	listenHostIPv4All   = "0.0.0.0"
	listenHostIPv6All   = "::"
	listenHostLoopback4 = "127.0.0.1"
	listenHostLoopback6 = "::1"
)

type listenerConfig struct {
	mode   string
	hosts  []string
	locked bool
	source string
}

func currentListenerConfig(cfgHost, cfgSource string, cfgSet bool, store *settings.Store) listenerConfig {
	if cfgSet {
		mode := normalizeListenerMode(cfgHost)
		hosts := listenerHostsForMode(mode)
		return listenerConfig{mode: mode, hosts: hosts, locked: true, source: cfgSource}
	}

	v, ok, _ := store.Get("listen_host")
	if !ok || strings.TrimSpace(v) == "" {
		v = listenHostAll
	}
	v = normalizeListenerMode(v)
	hosts := listenerHostsForMode(v)
	return listenerConfig{mode: v, hosts: hosts, locked: false, source: "database"}
}

func normalizeListenerMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return listenHostAll
	}
	return mode
}

func listenerHostsForMode(mode string) []string {
	switch mode {
	case "", listenHostAll, listenHostIPv6All:
		return []string{listenHostIPv6All}
	case listenHostIPv4All:
		return []string{listenHostIPv4All}
	case listenHostLoopback4:
		return []string{listenHostLoopback4}
	case listenHostLoopback6:
		return []string{listenHostLoopback6}
	default:
		return nil
	}
}

func (lc listenerConfig) broadcastOn() bool {
	switch lc.mode {
	case listenHostLoopback4, listenHostLoopback6:
		return false
	default:
		return true
	}
}

func (s *Server) listenerConfig() listenerConfig {
	cfgHost := s.cfg.Host
	cfgSource := s.cfg.ListenHostFrom
	if !s.cfg.ListenHostSet {
		cfgSource = "default"
	}
	return currentListenerConfig(cfgHost, cfgSource, s.cfg.ListenHostSet, s.deps.Store)
}

func (s *Server) listenAddrs() ([]string, error) {
	lc := s.listenerConfig()
	addrs := make([]string, 0, len(lc.hosts))
	for _, host := range lc.hosts {
		addrs = append(addrs, net.JoinHostPort(host, strconv.Itoa(s.cfg.Port)))
	}
	return addrs, nil
}

func (s *Server) resolveListeners() ([]net.Listener, error) {
	addrs, err := s.listenAddrs()
	if err != nil {
		return nil, err
	}
	listeners := make([]net.Listener, 0, len(addrs))
	for _, addr := range addrs {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			for _, opened := range listeners {
				_ = opened.Close()
			}
			return nil, fmt.Errorf("listen %s: %w", addr, err)
		}
		listeners = append(listeners, ln)
	}
	return listeners, nil
}

func (s *Server) serveListeners(listeners []net.Listener) error {
	if len(listeners) == 0 {
		return errors.New("no listener")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, len(listeners))
	var wg sync.WaitGroup
	for _, ln := range listeners {
		wg.Add(1)
		go func(ln net.Listener) {
			defer wg.Done()
			err := s.httpServer.Serve(ln)
			if ctx.Err() != nil || errors.Is(err, http.ErrServerClosed) {
				err = nil
			}
			errCh <- err
		}(ln)
	}

	var serveErr error
	go func() {
		wg.Wait()
		close(errCh)
	}()
	for err := range errCh {
		if err != nil && !errors.Is(err, net.ErrClosed) && serveErr == nil {
			serveErr = err
			cancel()
			_ = s.httpServer.Close()
		}
	}
	return serveErr
}
