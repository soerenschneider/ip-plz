package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"time"
)

type NamedClientStore struct {
	mu       sync.RWMutex
	filePath string
	cache    map[string]*NamedClient // always a non-nil, authoritative copy
}

func NewNamedClientStore(filePath string) *NamedClientStore {
	return &NamedClientStore{
		filePath: filePath,
		cache:    make(map[string]*NamedClient),
	}
}

func (s *NamedClientStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	clients, err := s.readFromDisk()
	if err != nil {
		return err
	}
	s.cache = clients
	return nil
}

func (s *NamedClientStore) Read() map[string]*NamedClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.copyCache()
}

func (s *NamedClientStore) Write(clients map[string]*NamedClient) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if clients == nil {
		clients = make(map[string]*NamedClient)
	}
	if err := s.writeToDisk(clients); err != nil {
		return err
	}
	s.cache = clients
	return nil
}

func (s *NamedClientStore) UpdateIp(key string, ip string) error {
	if key == "" {
		return errors.New("upsert key must not be empty")
	}
	if ip == "" {
		return errors.New("cannot upsert empty ip")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.copyCache()
	next[key].IpAddress = ip
	next[key].TimeSeen = time.Now()
	if err := s.writeToDisk(next); err != nil {
		return err
	}
	s.cache = next
	return nil
}

func (s *NamedClientStore) Upsert(key string, client *NamedClient) error {
	if key == "" {
		return errors.New("upsert key must not be empty")
	}
	if client == nil {
		return errors.New("cannot upsert nil client")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.copyCache()
	next[key] = client
	if err := s.writeToDisk(next); err != nil {
		return err
	}
	s.cache = next
	return nil
}

func (s *NamedClientStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.cache[key]; !exists {
		return nil
	}

	next := s.copyCache()
	delete(next, key)
	if err := s.writeToDisk(next); err != nil {
		return err
	}
	s.cache = next
	return nil
}

func (s *NamedClientStore) Get(key string) (*NamedClient, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.cache[key]
	if !ok {
		return nil, false
	}
	cp := *c // return a copy so callers cannot mutate cached state
	return &cp, true
}

func (s *NamedClientStore) Each(fn func(key string, client NamedClient)) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for k, v := range s.cache {
		fn(k, *v) // pass a copy so fn cannot mutate cached state
	}
}

func (s *NamedClientStore) Has(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.cache[key]
	return ok
}

func (s *NamedClientStore) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.cache))
	for k := range s.cache {
		keys = append(keys, k)
	}
	return keys
}

func (s *NamedClientStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.cache)
}

// copyCache returns a shallow copy of the current cache map.
// Pointer values are not deep-copied; callers that need independence must copy
// the structs themselves (Get and Each already do this for their return values).
func (s *NamedClientStore) copyCache() map[string]*NamedClient {
	cp := make(map[string]*NamedClient, len(s.cache))
	for k, v := range s.cache {
		cp[k] = v
	}
	return cp
}

func (s *NamedClientStore) readFromDisk() (map[string]*NamedClient, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return make(map[string]*NamedClient), nil
		}
		return nil, fmt.Errorf("read file %q: %w", s.filePath, err)
	}

	clients := make(map[string]*NamedClient)
	if err := json.Unmarshal(data, &clients); err != nil {
		return nil, fmt.Errorf("unmarshal %q: %w", s.filePath, err)
	}
	return clients, nil
}

// writeToDisk serialises clients to a sibling temp file and renames it into
// place atomically. It does NOT update the cache — the caller is responsible.
func (s *NamedClientStore) writeToDisk(clients map[string]*NamedClient) error {
	data, err := json.MarshalIndent(clients, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	tmpFile, err := os.CreateTemp(tempDir(s.filePath), ".named_client_*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return fmt.Errorf("rename %q → %q: %w", tmpPath, s.filePath, err)
	}

	success = true
	return nil
}

// tempDir returns the directory portion of path, defaulting to "." so that
// os.CreateTemp always targets the same filesystem as the destination file.
func tempDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if os.IsPathSeparator(path[i]) {
			return path[:i]
		}
	}
	return "."
}
