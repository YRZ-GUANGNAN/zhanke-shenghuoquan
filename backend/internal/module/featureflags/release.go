package featureflags

import "errors"

const CampusInfoScope = "campus_info"

var ErrReleaseRestricted = errors.New("当前发行版本仅开放校园指南与基础账号服务；新增业务需发布新版本并重新审核")

// RestrictToCampusInfo is called by the composition root, never by an API or an
// environment switch. Stored policy remains intact for audit and rollback.
func RestrictToCampusInfo() {
	state.Lock()
	defer state.Unlock()
	state.releaseScope = CampusInfoScope
}

// Caller holds the state lock. Always clone: the release must not overwrite
// durable flags or accidentally re-enable features when policy storage is off.
func releasePolicy(value Policy) Policy {
	value = clonePolicy(value)
	if state.releaseScope == CampusInfoScope {
		for _, key := range AllKeys {
			value.Flags[key] = false
		}
		value.PostingMode = "admin_only"
		value.Enabled = true
		value.ReleaseScope = CampusInfoScope
	}
	return value
}
