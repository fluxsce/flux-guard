package id

import (
	"sync"
	"testing"
)

func TestNewUnique(t *testing.T) {
	const n = 64
	const each = 200
	seen := make(map[string]struct{}, n*each)
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			local := make([]string, 0, each)
			for j := 0; j < each; j++ {
				value := New()
				if len(value) != 32 {
					t.Errorf("len=%d", len(value))
					return
				}
				local = append(local, value)
			}
			mu.Lock()
			defer mu.Unlock()
			for _, value := range local {
				if _, ok := seen[value]; ok {
					t.Errorf("duplicate %s", value)
					return
				}
				seen[value] = struct{}{}
			}
		}()
	}
	wg.Wait()
	if len(seen) != n*each {
		t.Fatalf("got %d ids", len(seen))
	}
}
