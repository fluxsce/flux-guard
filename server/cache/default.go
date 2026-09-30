package cache

import "fluxguard/server/cache/memory"

func init() {
	mem, err := memory.NewMemoryCache(nil)
	if err != nil {
		panic(err)
	}
	if err := AddCache("default", mem); err != nil {
		panic(err)
	}
}
