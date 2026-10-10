package plugin

import (
	"context"
	"encoding/json"

	"fluxguard/protocol"
)

// Call 把一次调用交给已加载的插件，并把 result 解进 dest。
// 方法必须写在该插件的 yaml 清单里。这里不解释指标，也不写库。
func Call(ctx context.Context, id, method string, params any, secret string, dest any) error {
	activeMu.Lock()
	item := active[id]
	activeMu.Unlock()
	if item == nil || item.svc == nil {
		return &protocol.ExtFailure{Code: protocol.ExtCodeUnavailable, Message: "没有这个插件"}
	}
	if _, ok := item.methods[method]; !ok {
		return &protocol.ExtFailure{Code: protocol.ExtCodeMethodNotFound, Message: "这种组件没有这项操作"}
	}
	// 清单没有把这个方法列入凭据注入时，参数里的口令不会送出。
	if _, ok := item.credentials[method]; !ok {
		secret = ""
		params = dropSecret(params)
	}
	return item.svc.roundTrip(ctx, method, params, secret, dest)
}

func dropSecret(params any) any {
	raw, err := json.Marshal(params)
	if err != nil {
		return params
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return params
	}
	delete(obj, "secret")
	return obj
}

// MaintainArgs 返回清单里这项维护操作允许的参数名。操作未登记时 ok 为 false。
func MaintainArgs(id, op string) (map[string]struct{}, bool) {
	activeMu.Lock()
	item := active[id]
	activeMu.Unlock()
	if item == nil {
		return nil, false
	}
	args, ok := item.ops[op]
	if !ok {
		return nil, false
	}
	out := make(map[string]struct{}, len(args))
	for key := range args {
		out[key] = struct{}{}
	}
	return out, true
}
