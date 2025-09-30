package server

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"runtime"
	rpprof "runtime/pprof"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

// ProfilingServer manages debug profiling endpoints
type ProfilingServer struct {
	enabled      bool
	apiKeyHash   string
	logger       *logging.Logger
	startTime    time.Time
	authRequired bool
}

// NewProfilingServer creates a new profiling server
func NewProfilingServer(enabled bool, apiKey string, logger *logging.Logger) *ProfilingServer {
	return &ProfilingServer{
		enabled:      enabled,
		apiKeyHash:   apiKey,
		logger:       logger,
		startTime:    time.Now(),
		authRequired: apiKey != "",
	}
}

// RegisterHandlers registers profiling endpoints on an HTTP mux
func (ps *ProfilingServer) RegisterHandlers(mux *http.ServeMux) {
	if !ps.enabled {
		ps.logger.Info("Profiling endpoints are disabled")

		return
	}

	ps.logger.Info("Registering profiling endpoints (authentication: %v)", ps.authRequired)

	mux.HandleFunc("/debug/pprof/", ps.authMiddleware(pprof.Index))
	mux.HandleFunc("/debug/pprof/cmdline", ps.authMiddleware(pprof.Cmdline))
	mux.HandleFunc("/debug/pprof/profile", ps.authMiddleware(pprof.Profile))
	mux.HandleFunc("/debug/pprof/symbol", ps.authMiddleware(pprof.Symbol))
	mux.HandleFunc("/debug/pprof/trace", ps.authMiddleware(pprof.Trace))
	mux.HandleFunc("/debug/pprof/heap", ps.authMiddleware(ps.handleHeap))
	mux.HandleFunc("/debug/pprof/goroutine", ps.authMiddleware(ps.handleGoroutine))
	mux.HandleFunc("/debug/pprof/threadcreate", ps.authMiddleware(ps.handleThreadCreate))
	mux.HandleFunc("/debug/pprof/block", ps.authMiddleware(ps.handleBlock))
	mux.HandleFunc("/debug/pprof/mutex", ps.authMiddleware(ps.handleMutex))
	mux.HandleFunc("/debug/pprof/allocs", ps.authMiddleware(ps.handleAllocs))

	mux.HandleFunc("/debug/stats", ps.authMiddleware(ps.handleStats))
	mux.HandleFunc("/debug/gc", ps.authMiddleware(ps.handleGC))
}

// authMiddleware adds authentication to profiling endpoints
func (ps *ProfilingServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ps.enabled {
			http.Error(w, "Profiling is disabled", http.StatusForbidden)

			return
		}

		if ps.authRequired {
			apiKey := r.Header.Get("Authorization")
			if apiKey == "" {
				apiKey = r.URL.Query().Get("api_key")
			}

			if apiKey != ps.apiKeyHash && "Bearer "+apiKey != ps.apiKeyHash {
				ps.logger.Warning("Unauthorized profiling access attempt from %s", r.RemoteAddr)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)

				return
			}
		}

		next(w, r)
	}
}

// handleHeap handles heap profiling
func (ps *ProfilingServer) handleHeap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=heap-%d.prof", time.Now().Unix()))

	if err := rpprof.WriteHeapProfile(w); err != nil {
		ps.logger.Error("Failed to write heap profile: %v", err)
		http.Error(w, "Failed to generate heap profile", http.StatusInternalServerError)
	}
}

// handleGoroutine handles goroutine profiling
func (ps *ProfilingServer) handleGoroutine(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	profile := rpprof.Lookup("goroutine")
	if profile == nil {
		http.Error(w, "Goroutine profile not available", http.StatusInternalServerError)

		return
	}

	if err := profile.WriteTo(w, 2); err != nil {
		ps.logger.Error("Failed to write goroutine profile: %v", err)
	}
}

// handleThreadCreate handles thread creation profiling
func (ps *ProfilingServer) handleThreadCreate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	profile := rpprof.Lookup("threadcreate")
	if profile == nil {
		http.Error(w, "Thread creation profile not available", http.StatusInternalServerError)

		return
	}

	if err := profile.WriteTo(w, 2); err != nil {
		ps.logger.Error("Failed to write threadcreate profile: %v", err)
	}
}

// handleBlock handles blocking profiling
func (ps *ProfilingServer) handleBlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	profile := rpprof.Lookup("block")
	if profile == nil {
		http.Error(w, "Block profile not available", http.StatusInternalServerError)

		return
	}

	if err := profile.WriteTo(w, 2); err != nil {
		ps.logger.Error("Failed to write block profile: %v", err)
	}
}

// handleMutex handles mutex profiling
func (ps *ProfilingServer) handleMutex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	profile := rpprof.Lookup("mutex")
	if profile == nil {
		http.Error(w, "Mutex profile not available", http.StatusInternalServerError)

		return
	}

	if err := profile.WriteTo(w, 2); err != nil {
		ps.logger.Error("Failed to write mutex profile: %v", err)
	}
}

// handleAllocs handles allocation profiling
func (ps *ProfilingServer) handleAllocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	profile := rpprof.Lookup("allocs")
	if profile == nil {
		http.Error(w, "Allocs profile not available", http.StatusInternalServerError)

		return
	}

	if err := profile.WriteTo(w, 2); err != nil {
		ps.logger.Error("Failed to write allocs profile: %v", err)
	}
}

// handleStats returns runtime statistics
func (ps *ProfilingServer) handleStats(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
  "uptime_seconds": %d,
  "goroutines": %d,
  "memory": {
    "alloc_bytes": %d,
    "total_alloc_bytes": %d,
    "sys_bytes": %d,
    "num_gc": %d,
    "heap_alloc_bytes": %d,
    "heap_sys_bytes": %d,
    "heap_idle_bytes": %d,
    "heap_in_use_bytes": %d,
    "heap_released_bytes": %d,
    "heap_objects": %d,
    "stack_in_use_bytes": %d,
    "stack_sys_bytes": %d,
    "mspan_in_use_bytes": %d,
    "mspan_sys_bytes": %d,
    "mcache_in_use_bytes": %d,
    "mcache_sys_bytes": %d,
    "buck_hash_sys_bytes": %d,
    "gc_sys_bytes": %d,
    "other_sys_bytes": %d,
    "next_gc_bytes": %d,
    "last_gc_time": "%s",
    "gc_cpu_fraction": %.4f
  },
  "gc": {
    "num_gc": %d,
    "num_forced_gc": %d,
    "pause_total_ns": %d,
    "pause_ns": %v,
    "pause_end": %v
  },
  "runtime": {
    "version": "%s",
    "num_cpu": %d,
    "gomaxprocs": %d,
    "num_cgo_call": %d
  }
}`,
		int(time.Since(ps.startTime).Seconds()),
		runtime.NumGoroutine(),
		m.Alloc,
		m.TotalAlloc,
		m.Sys,
		m.NumGC,
		m.HeapAlloc,
		m.HeapSys,
		m.HeapIdle,
		m.HeapInuse,
		m.HeapReleased,
		m.HeapObjects,
		m.StackInuse,
		m.StackSys,
		m.MSpanInuse,
		m.MSpanSys,
		m.MCacheInuse,
		m.MCacheSys,
		m.BuckHashSys,
		m.GCSys,
		m.OtherSys,
		m.NextGC,
		time.Unix(0, int64(m.LastGC)).Format(time.RFC3339),
		m.GCCPUFraction,
		m.NumGC,
		m.NumForcedGC,
		m.PauseTotalNs,
		m.PauseNs,
		m.PauseEnd,
		runtime.Version(),
		runtime.NumCPU(),
		runtime.GOMAXPROCS(0),
		runtime.NumCgoCall(),
	)
}

// handleGC triggers garbage collection
func (ps *ProfilingServer) handleGC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	ps.logger.Info("Manual GC triggered from %s", r.RemoteAddr)

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	start := time.Now()
	runtime.GC()
	duration := time.Since(start)

	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	freed := int64(before.HeapAlloc) - int64(after.HeapAlloc)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
  "success": true,
  "duration_ms": %.2f,
  "memory_freed_bytes": %d,
  "heap_before_bytes": %d,
  "heap_after_bytes": %d,
  "num_gc": %d
}`,
		duration.Seconds()*1000,
		freed,
		before.HeapAlloc,
		after.HeapAlloc,
		after.NumGC,
	)
}

// EnableBlockProfiling enables block profiling
func (ps *ProfilingServer) EnableBlockProfiling(rate int) {
	runtime.SetBlockProfileRate(rate)
	ps.logger.Info("Block profiling enabled with rate %d", rate)
}

// EnableMutexProfiling enables mutex profiling
func (ps *ProfilingServer) EnableMutexProfiling(rate int) {
	runtime.SetMutexProfileFraction(rate)
	ps.logger.Info("Mutex profiling enabled with rate %d", rate)
}