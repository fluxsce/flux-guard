package setting

import (
	"context"
	"encoding/json"
	"sync"

	"fluxguard/common/buildinfo"
	"fluxguard/common/crypto"
	"fluxguard/common/id"
	"fluxguard/server/utils/secret"
	dbx "github.com/fluxsce/dbx/db"
)

const (
	// GroupIngestToken 是租户共用的上报准入令牌。主机、应用、数据库代理都带这一把。
	GroupIngestToken = "ingestToken"

	// DevIngestPlain 是开发态种子明文，与 configs 里的 agent_token 相同。
	DevIngestPlain = "agt_dev_local"
)

// IngestToken 是上报令牌的落库形态。
// 进来的请求用摘要校验。TokenEnc 是同一把令牌的密文，拨代理时出示，不是另一把密钥。
type IngestToken struct {
	TokenHash  string `json:"tokenHash"`
	TokenEnc   string `json:"tokenEnc,omitempty"`
	StatusFlag string `json:"statusFlag"`
	Hint       string `json:"hint"`
}

type ingestCache struct {
	mu    sync.RWMutex
	items map[string]IngestToken
}

var ingestTokens = &ingestCache{items: map[string]IngestToken{}}

// ParseIngest 解析上报令牌分组。缺摘要时视为未签发。
func ParseIngest(content string) IngestToken {
	var item IngestToken
	if content == "" {
		return item
	}
	_ = json.Unmarshal([]byte(content), &item)
	if item.StatusFlag != "N" {
		item.StatusFlag = "Y"
	}
	return item
}

// EncodeIngest 把上报令牌写成入库 JSON。
func EncodeIngest(item IngestToken) (string, error) {
	if item.StatusFlag != "N" {
		item.StatusFlag = "Y"
	}
	raw, err := json.Marshal(item)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// PutIngest 写入进程缓存，上报校验立即生效。
func PutIngest(tenantID string, item IngestToken) {
	tenantID = tenantKey(tenantID)
	ingestTokens.mu.Lock()
	ingestTokens.items[tenantID] = item
	ingestTokens.mu.Unlock()
}

// GetIngest 返回租户上报令牌。没有签发时 TokenHash 为空。
func GetIngest(tenantID string) IngestToken {
	tenantID = tenantKey(tenantID)
	ingestTokens.mu.RLock()
	defer ingestTokens.mu.RUnlock()
	return ingestTokens.items[tenantID]
}

// IssueIngestToken 生成一把新的上报令牌。明文只在这次返回值里再出现一次，库里同时留下摘要和密文。
func IssueIngestToken() (IngestToken, string) {
	plain := "agt_" + id.New()
	item := IngestToken{StatusFlag: "Y", Hint: hintOf(plain)}
	_ = sealIngest(&item, plain)
	return item, plain
}

// DevIngestToken 是开发种子。摘要和密文都对应 DevIngestPlain。
func DevIngestToken() IngestToken {
	item := IngestToken{StatusFlag: "Y", Hint: hintOf(DevIngestPlain)}
	_ = sealIngest(&item, DevIngestPlain)
	return item
}

// IngestPlain 返回拨号时要带上的上报令牌。没有密文或已停用时为空，服务器不去拨。
func IngestPlain(tenantID string) string {
	item := GetIngest(tenantID)
	if item.StatusFlag != "Y" {
		return ""
	}
	return RevealIngest(item)
}

// RevealIngest 解出落库密文，供控制台查看和复制。没有密文或解不开时为空。
func RevealIngest(item IngestToken) string {
	if item.TokenEnc == "" {
		return ""
	}
	plain, err := crypto.Decrypt("", item.TokenEnc)
	if err != nil {
		return ""
	}
	return plain
}

func sealIngest(item *IngestToken, plain string) error {
	sealed, err := crypto.Encrypt("", plain)
	if err != nil {
		return err
	}
	item.TokenHash = secret.HashToken(plain)
	item.TokenEnc = sealed
	return nil
}

// TenantByIngestToken 用明文令牌换租户。未签发、已停用或不匹配时拒绝。
func TenantByIngestToken(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	ingestTokens.mu.RLock()
	defer ingestTokens.mu.RUnlock()
	for tenantID, item := range ingestTokens.items {
		if item.StatusFlag != "Y" || item.TokenHash == "" {
			continue
		}
		if secret.MatchToken(item.TokenHash, token) {
			return tenantID, true
		}
	}
	return "", false
}

func hintOf(plain string) string {
	if len(plain) <= 4 {
		return plain
	}
	return plain[len(plain)-4:]
}

// installDevIngest 在本地和测试里补上种子令牌。已有开发令牌只补密文，不换摘要。
// 发行构建会写入 Commit，不再安装 agt_dev_local。
func installDevIngest(ctx context.Context, db *dbx.DB) error {
	if buildinfo.Commit != "none" {
		return nil
	}
	var count int
	if err := scanCount(ctx, db, &count); err != nil {
		return err
	}
	if count > 0 {
		return backfillDevPlain(ctx, db)
	}
	item := DevIngestToken()
	content, err := EncodeIngest(item)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		INSERT INTO FG_SYS_SETTING (
		  tenantId, groupCode, content, addWho, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  'default', @groupCode, @content, 'system', 'system', 'INIT', 1, 'Y', '开发态上报令牌'
		)`, dbx.Args{"groupCode": GroupIngestToken, "content": content})
	if err != nil {
		return err
	}
	PutIngest("default", item)
	return nil
}

// backfillDevPlain 给已经落库、只有摘要的开发令牌补上密文。摘要对不上时不动。
func backfillDevPlain(ctx context.Context, db *dbx.DB) error {
	var content string
	row, err := db.QueryRow(ctx, `
		SELECT content FROM FG_SYS_SETTING
		WHERE tenantId = 'default' AND groupCode = @groupCode AND activeFlag = 'Y'`,
		dbx.Args{"groupCode": GroupIngestToken})
	if err != nil {
		return err
	}
	if err := row.Scan(&content); err != nil {
		return err
	}
	item := ParseIngest(content)
	if item.TokenEnc != "" || !secret.MatchToken(item.TokenHash, DevIngestPlain) {
		return nil
	}
	if err := sealIngest(&item, DevIngestPlain); err != nil {
		return err
	}
	encoded, err := EncodeIngest(item)
	if err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `
		UPDATE FG_SYS_SETTING
		SET content = @content, editWho = 'system', currentVersion = currentVersion + 1
		WHERE tenantId = 'default' AND groupCode = @groupCode AND activeFlag = 'Y'`,
		dbx.Args{"content": encoded, "groupCode": GroupIngestToken}); err != nil {
		return err
	}
	PutIngest("default", item)
	return nil
}

func scanCount(ctx context.Context, db *dbx.DB, dest *int) error {
	row, err := db.QueryRow(ctx, `
		SELECT count(*) FROM FG_SYS_SETTING
		WHERE tenantId = 'default' AND groupCode = @groupCode AND activeFlag = 'Y'`,
		dbx.Args{"groupCode": GroupIngestToken})
	if err != nil {
		return err
	}
	return row.Scan(dest)
}
