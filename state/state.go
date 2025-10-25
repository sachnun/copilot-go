package state

import "sync"

// State mirrors the TypeScript runtime state object for the proxy.
type State struct {
	GitHubToken       string
	CopilotToken      string
	AccountType       string
	Models            any
	VSCodeVersion     string
	ManualApprove     bool
	RateLimitWait     bool
	ShowToken         bool
	RateLimitSeconds  *int
	LastRequestUnixMs *int64
	ServerStartUnixMs *int64
	mutex             sync.RWMutex
}

// Shared is the singleton application state used across the application.
var Shared = &State{
	AccountType:   "individual",
	ManualApprove: false,
	RateLimitWait: false,
	ShowToken:     false,
}

// Update safely updates state using the provided function.
func (s *State) Update(fn func(*State)) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	fn(s)
}

// Read executes fn while holding a read lock.
func (s *State) Read(fn func(*State)) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	fn(s)
}
