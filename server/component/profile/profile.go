// Package profile 把页面上的部署方式和认证方式收成连接上的 optionText。
package profile

import (
	"fmt"
	"strconv"
	"strings"

	"fluxguard/server/component/catalog"
)

// Spec 是规范化之后的连接参数。ClearSecret 为真时要清掉已存口令。
type Spec struct {
	Option      string
	User        string
	Database    string
	ClearSecret bool
}

// Normalize 按种类检查部署和认证，并写成 deploy:auth。
func Normalize(kind, option, user, secret, database string, creating bool) (Spec, error) {
	deploy, auth := Split(option)
	user = strings.TrimSpace(user)
	database = strings.TrimSpace(database)
	switch kind {
	case catalog.Redis:
		return redisSpec(deploy, auth, user, secret, database, creating)
	case catalog.MySQL:
		if user == "" {
			return Spec{}, fmt.Errorf("请填写 MySQL 账号")
		}
		if creating && strings.TrimSpace(secret) == "" {
			return Spec{}, fmt.Errorf("请填写 MySQL 口令")
		}
		return Spec{Option: "server:password", User: user, Database: database}, nil
	case catalog.Mongo:
		if auth == "" {
			auth = "none"
		}
		if auth != "none" && auth != "password" {
			return Spec{}, fmt.Errorf("不支持的 MongoDB 认证方式")
		}
		if auth == "none" {
			return Spec{Option: "standalone:none", ClearSecret: true}, nil
		}
		if user == "" || (creating && strings.TrimSpace(secret) == "") {
			return Spec{}, fmt.Errorf("MongoDB 用户名口令需要同时填写")
		}
		if database == "" {
			database = "admin"
		}
		return Spec{Option: "standalone:password", User: user, Database: database}, nil
	case catalog.Kafka:
		if auth == "" {
			auth = "none"
		}
		if auth != "none" && auth != "plain" {
			return Spec{}, fmt.Errorf("不支持的 Kafka 认证方式")
		}
		if auth == "none" {
			return Spec{Option: "cluster:none", ClearSecret: true}, nil
		}
		if user == "" || (creating && strings.TrimSpace(secret) == "") {
			return Spec{}, fmt.Errorf("SASL PLAIN 需要用户名和口令")
		}
		return Spec{Option: "cluster:plain", User: user}, nil
	case catalog.ZK:
		return Spec{Option: "standalone:none", ClearSecret: true}, nil
	case catalog.Nacos:
		if auth == "" {
			auth = "none"
		}
		if auth != "none" && auth != "login" {
			return Spec{}, fmt.Errorf("不支持的 Nacos 认证方式")
		}
		if auth == "none" {
			return Spec{Option: "server:none", ClearSecret: true}, nil
		}
		if user == "" || (creating && strings.TrimSpace(secret) == "") {
			return Spec{}, fmt.Errorf("Nacos 登录需要用户名和口令")
		}
		return Spec{Option: "server:login", User: user}, nil
	default:
		return Spec{}, fmt.Errorf("不支持的组件种类")
	}
}

// Split 拆开 deploy:auth。空值留给调用方按旧数据推断。
func Split(option string) (deploy, auth string) {
	deploy, auth, _ = strings.Cut(strings.TrimSpace(option), ":")
	return deploy, auth
}

func redisSpec(deploy, auth, user, secret, database string, creating bool) (Spec, error) {
	if deploy == "" {
		deploy = "standalone"
	}
	if deploy != "standalone" && deploy != "sentinel" && deploy != "cluster" {
		return Spec{}, fmt.Errorf("Redis 部署方式应为单机、哨兵或集群")
	}
	if auth == "" {
		auth = inferRedisAuth(user, secret, creating)
	}
	if auth != "none" && auth != "password" && auth != "acl" {
		return Spec{}, fmt.Errorf("Redis 认证方式应为无、口令或 ACL")
	}
	spec := Spec{Option: deploy + ":" + auth, Database: database}
	switch auth {
	case "none":
		spec.ClearSecret = true
	case "password":
		if creating && strings.TrimSpace(secret) == "" {
			return Spec{}, fmt.Errorf("请填写 Redis 口令")
		}
	case "acl":
		if user == "" || (creating && strings.TrimSpace(secret) == "") {
			return Spec{}, fmt.Errorf("ACL 需要用户名和口令")
		}
		spec.User = user
	}
	if deploy == "sentinel" && database == "" {
		return Spec{}, fmt.Errorf("哨兵需要填写主节点名")
	}
	if deploy == "standalone" && database != "" {
		n, err := strconv.Atoi(database)
		if err != nil || n < 0 || n > 15 {
			return Spec{}, fmt.Errorf("Redis 库号应为 0 到 15")
		}
	}
	if deploy != "standalone" && deploy != "sentinel" {
		spec.Database = ""
	}
	return spec, nil
}

func inferRedisAuth(user, secret string, creating bool) string {
	if user != "" {
		return "acl"
	}
	if strings.TrimSpace(secret) != "" {
		return "password"
	}
	if !creating {
		return "password"
	}
	return "none"
}
