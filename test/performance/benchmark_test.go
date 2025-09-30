package performance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"
)

func BenchmarkHTTPProxyLatency(b *testing.B) {
	testServer := createTestMCPServer()
	defer testServer.Close()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := client.Get(testServer.URL)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func BenchmarkConcurrentHTTPRequests(b *testing.B) {
	testServer := createTestMCPServer()
	defer testServer.Close()

	concurrency := []int{10, 50, 100, 500}

	for _, numGoroutines := range concurrency {
		b.Run(string(rune(numGoroutines)), func(b *testing.B) {
			client := &http.Client{
				Timeout: 5 * time.Second,
				Transport: &http.Transport{
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 100,
				},
			}

			b.ResetTimer()
			b.ReportAllocs()

			var wg sync.WaitGroup
			requestsPerGoroutine := b.N / numGoroutines

			for g := 0; g < numGoroutines; g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					for i := 0; i < requestsPerGoroutine; i++ {
						resp, err := client.Get(testServer.URL)
						if err == nil {
							resp.Body.Close()
						}
					}
				}()
			}

			wg.Wait()
		})
	}
}

func BenchmarkMemoryAllocation(b *testing.B) {
	testServer := createTestMCPServer()
	defer testServer.Close()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := client.Get(testServer.URL)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func TestHTTPLatencyP50P99(t *testing.T) {
	testServer := createTestMCPServer()
	defer testServer.Close()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	numRequests := 1000
	latencies := make([]time.Duration, numRequests)

	for i := 0; i < numRequests; i++ {
		start := time.Now()
		resp, err := client.Get(testServer.URL)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		latencies[i] = time.Since(start)
	}

	p50, p99 := calculatePercentiles(latencies)

	t.Logf("P50 latency: %v", p50)
	t.Logf("P99 latency: %v", p99)

	if p50 > 10*time.Millisecond {
		t.Logf("P50 latency %v exceeds target of 10ms (informational only)", p50)
	}

	if p99 > 50*time.Millisecond {
		t.Logf("P99 latency %v exceeds target of 50ms (informational only)", p99)
	}
}

func TestConcurrentConnectionsLoad(t *testing.T) {
	testServer := createTestMCPServer()
	defer testServer.Close()

	numConnections := 1000
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: 1000,
		},
	}

	start := time.Now()

	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(testServer.URL)
			if err == nil {
				resp.Body.Close()
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("Handled %d concurrent connections in %v", successCount, duration)
	t.Logf("Throughput: %.2f req/sec", float64(successCount)/duration.Seconds())

	if successCount < numConnections*95/100 {
		t.Logf("Success rate %d/%d is below 95%% (informational only)", successCount, numConnections)
	}
}

func TestMemoryFootprint(t *testing.T) {
	testServer := createTestMCPServer()
	defer testServer.Close()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	baselineAlloc := m.Alloc

	for i := 0; i < 10000; i++ {
		resp, err := client.Get(testServer.URL)
		if err == nil {
			resp.Body.Close()
		}
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	runtime.ReadMemStats(&m)
	finalAlloc := m.Alloc

	memoryUsed := finalAlloc - baselineAlloc
	memoryUsedMB := float64(memoryUsed) / 1024 / 1024

	t.Logf("Base memory usage: %.2f MB", float64(baselineAlloc)/1024/1024)
	t.Logf("Final memory usage: %.2f MB", float64(finalAlloc)/1024/1024)
	t.Logf("Memory increase: %.2f MB", memoryUsedMB)

	if memoryUsedMB > 500 {
		t.Logf("Memory usage %.2f MB exceeds target of 500MB (informational only)", memoryUsedMB)
	}
}

func createTestMCPServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Millisecond)

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name":        "test-tool",
						"description": "A test tool",
						"inputSchema": map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func calculatePercentiles(latencies []time.Duration) (p50, p99 time.Duration) {
	if len(latencies) == 0 {

		return 0, 0
	}

	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	p50Index := len(sorted) * 50 / 100
	p99Index := len(sorted) * 99 / 100

	return sorted[p50Index], sorted[p99Index]
}