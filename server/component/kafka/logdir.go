package kafka

import (
	"context"
	"sync"
	"time"
)

func fillLog(ctx context.Context, sample *Sample, opt link, apis apiSet) {
	for i := range sample.Brokers {
		sample.Brokers[i].LogBytes = -1
	}
	sample.LogBytes = -1
	if ctx.Err() != nil {
		return
	}
	ver, ok := apis.use(35, 0)
	if !ok {
		return
	}
	var total int64
	var known bool
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i := range sample.Brokers {
		if !sample.Brokers[i].Up || sample.Brokers[i].Host == "" || sample.Brokers[i].Port <= 0 {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()
			n, got := logBytesAt(ctx, brokerAddr(sample.Brokers[i]), opt, ver)
			if !got {
				return
			}
			mu.Lock()
			sample.Brokers[i].LogBytes = n
			total += n
			known = true
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if known {
		sample.LogBytes = total
	}
}

func logBytesAt(ctx context.Context, addr string, opt link, ver int16) (int64, bool) {
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	raw, _, err := connect(probeCtx, addr, opt)
	if err != nil {
		return 0, false
	}
	defer raw.Close()
	body, err := roundTrip(raw, 35, ver, 8, appendInt32(nil, -1))
	if err != nil {
		return 0, false
	}
	return ParseLogBytes(body)
}

// ParseLogBytes 合计 DescribeLogDirs v0 里各日志目录的分区大小。
func ParseLogBytes(body []byte) (int64, bool) {
	r := reader{b: body}
	if _, err := r.i32(); err != nil {
		return 0, false
	}
	n, err := r.i32()
	if err != nil || n < 0 {
		return 0, false
	}
	var total int64
	var known bool
	for i := int32(0); i < n; i++ {
		code, err := r.i16()
		if err != nil {
			return 0, false
		}
		if _, err := r.str(); err != nil {
			return 0, false
		}
		topics, err := r.i32()
		if err != nil || topics < 0 {
			return 0, false
		}
		for t := int32(0); t < topics; t++ {
			if _, err := r.str(); err != nil {
				return 0, false
			}
			parts, err := r.i32()
			if err != nil || parts < 0 {
				return 0, false
			}
			for p := int32(0); p < parts; p++ {
				if _, err := r.i32(); err != nil {
					return 0, false
				}
				size, err := r.i64()
				if err != nil {
					return 0, false
				}
				if _, err := r.i64(); err != nil {
					return 0, false
				}
				if _, err := r.i8(); err != nil {
					return 0, false
				}
				if code == 0 && size >= 0 {
					total += size
				}
			}
		}
		if code == 0 {
			known = true
		}
	}
	if !known {
		return 0, false
	}
	return total, true
}
