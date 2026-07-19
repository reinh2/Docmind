package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// Registry emits Prometheus text format without a client-library dependency.
type Registry struct {
	mu       sync.RWMutex
	counters map[string]float64
	gauges   map[string]float64
	sums     map[string]float64
	counts   map[string]uint64
}

func New() *Registry {
	return &Registry{counters: map[string]float64{}, gauges: map[string]float64{}, sums: map[string]float64{}, counts: map[string]uint64{}}
}
func (m *Registry) Inc(name string) { m.mu.Lock(); defer m.mu.Unlock(); m.counters[name]++ }
func (m *Registry) Set(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
}
func (m *Registry) Observe(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sums[name] += value
	m.counts[name]++
}
func (m *Registry) Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	m.mu.RLock()
	defer m.mu.RUnlock()
	var lines []string
	for k, v := range m.counters {
		lines = append(lines, fmt.Sprintf("%s %.0f", k, v))
	}
	for k, v := range m.gauges {
		lines = append(lines, fmt.Sprintf("%s %g", k, v))
	}
	for k, sum := range m.sums {
		lines = append(lines, fmt.Sprintf("%s_sum %g", k, sum), fmt.Sprintf("%s_count %d", k, m.counts[k]))
	}
	sort.Strings(lines)
	_, _ = w.Write([]byte(strings.Join(lines, "\n") + "\n"))
}
