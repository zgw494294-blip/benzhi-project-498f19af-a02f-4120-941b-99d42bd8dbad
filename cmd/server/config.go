package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type config struct {
	addr      string
	database  string
	selfcheck bool
}

func parseConfig(args []string) (config, error) {
	set := flag.NewFlagSet("mural-release-server", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	var cfg config
	set.StringVar(&cfg.addr, "addr", "", "HTTP 监听地址，必须使用回环地址")
	set.StringVar(&cfg.database, "db", "mural-release.db", "SQLite 数据库路径")
	set.BoolVar(&cfg.selfcheck, "selfcheck", false, "执行真实 HTTP 全流程自检并退出")
	if err := set.Parse(args); err != nil {
		return cfg, err
	}
	if set.NArg() != 0 {
		return cfg, fmt.Errorf("不支持位置参数: %s", strings.Join(set.Args(), " "))
	}
	addrExplicit := false
	set.Visit(func(item *flag.Flag) {
		if item.Name == "addr" {
			addrExplicit = true
		}
	})
	if !addrExplicit {
		port := strings.TrimSpace(os.Getenv("PORT"))
		if port == "" {
			cfg.addr = "127.0.0.1:19081"
		} else {
			number, err := strconv.Atoi(port)
			if err != nil || number < 1 || number > 65535 {
				return cfg, errors.New("PORT 必须为 1 到 65535 的端口号")
			}
			cfg.addr = net.JoinHostPort("127.0.0.1", strconv.Itoa(number))
		}
	}
	if err := validateAddress(cfg.addr); err != nil {
		return cfg, err
	}
	if strings.TrimSpace(cfg.database) == "" {
		return cfg, errors.New("数据库路径不能为空")
	}
	return cfg, nil
}

func validateAddress(addr string) error {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("监听地址必须为 host:port: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("监听端口必须为 1 到 65535")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("监听地址必须使用 127.0.0.1 或其他回环 IP")
	}
	return nil
}
