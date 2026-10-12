package store

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"fluxguard/server/config"

	"github.com/go-sql-driver/mysql"
)

// Resolve 把 db 配置变成驱动名和连接串。dsn 优先；否则按主机、端口、账号和库名生成。
func Resolve(db config.DB) (driver, dsn string, err error) {
	driver = strings.ToLower(strings.TrimSpace(db.Driver))
	if driver == "" {
		driver = "sqlite"
	}
	dsn = strings.TrimSpace(db.DSN)
	if dsn != "" {
		return driver, dsn, nil
	}
	switch driver {
	case "sqlite":
		if strings.TrimSpace(db.Host) != "" {
			return "", "", fmt.Errorf("sqlite 使用文件，请把路径写在 db.dsn 或 db.database。要连 MySQL 请把 db.driver 改成 mysql")
		}
		file := strings.TrimSpace(db.Database)
		if file == "" {
			file = "data/flux_guard.db"
		}
		return driver, file, nil
	case "mysql":
		built, err := mysqlDSN(db)
		if err != nil {
			return "", "", err
		}
		return driver, built, nil
	default:
		return "", "", fmt.Errorf("不认识的数据库类型 %s，请用 sqlite 或 mysql", driver)
	}
}

// Describe 是启动日志用的一行，不含口令。
func Describe(driver, dsn string) string {
	if strings.EqualFold(driver, "mysql") {
		parsed, err := mysql.ParseDSN(dsn)
		if err == nil && parsed.Addr != "" {
			name := parsed.DBName
			if name == "" {
				name = "(未指定库名)"
			}
			return driver + " " + parsed.Addr + "/" + name
		}
	}
	return driver + " " + dsn
}

func mysqlDSN(db config.DB) (string, error) {
	host := strings.TrimSpace(db.Host)
	if host == "" {
		return "", fmt.Errorf("mysql 请填写 db.host，或直接填写 db.dsn")
	}
	name := strings.TrimSpace(db.Database)
	if name == "" {
		return "", fmt.Errorf("mysql 请填写 db.database，或直接填写 db.dsn")
	}
	port := db.Port
	if port <= 0 {
		port = 3306
	}
	cfg := mysql.NewConfig()
	cfg.User = db.Username
	cfg.Passwd = db.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(host, strconv.Itoa(port))
	cfg.DBName = name
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	return cfg.FormatDSN(), nil
}
