package models

// LoginRequest 登录请求。校验用户 ID，验证码票可空；提交后按票核销。
type LoginRequest struct {
	UserID      string `json:"userId"`
	Password    string `json:"password"`
	CaptchaId   string `json:"captchaId"`
	CaptchaCode string `json:"captchaCode"`
}

// LoginResult 是登录成功后交给前端缓存的用户上下文。
// token 同时写入会话 Cookie。expireAt 是用户账号过期时间。
// timeout 是环境设置里的接口超时，单位毫秒，前端用它覆盖访问超时。
type LoginResult struct {
	Token           string `json:"token"`
	UserID          string `json:"userId"`
	UserName        string `json:"userName"`
	RealName        string `json:"realName"`
	TenantID        string `json:"tenantId"`
	TenantAdminFlag string `json:"tenantAdminFlag"`
	ExpireAt        string `json:"expireAt"`
	LoginTime       string `json:"loginTime"`
	Timeout         int    `json:"timeout"`
	DefaultPageSize int    `json:"defaultPageSize"`
	MaxPageSize     int    `json:"maxPageSize"`
}

// CaptchaRequest 获取验证码。目前只支持 random。
type CaptchaRequest struct {
	Type string `json:"type"`
}

// CaptchaResponse 验证码响应。答案不出网，只返回签名票和图片。
type CaptchaResponse struct {
	CaptchaId string `json:"captchaId"`
	Image     string `json:"image"`
	ExpireAt  int64  `json:"expireAt"`
}

// SelfProfile 是当前用户能看见的本人资料。不含口令、角色和管理员标记。
type SelfProfile struct {
	UserID   string `json:"userId"`
	UserName string `json:"userName"`
	RealName string `json:"realName"`
	Email    string `json:"email"`
	Mobile   string `json:"mobile"`
	Gender   int    `json:"gender"`
}

// SelfProfileInput 只接收本人可改的字段。用户、租户、角色和状态不在这里。
type SelfProfileInput struct {
	RealName        string `json:"realName"`
	Email           string `json:"email"`
	Mobile          string `json:"mobile"`
	Gender          int    `json:"gender"`
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}
