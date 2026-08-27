package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// 日志上报地址
const logReportURL = "http://login.tlcode.com/api/log/report"

// 日志级别常量
const (
	LogLevelError = "ERROR"
	LogLevelWarn  = "WARN"
	LogLevelInfo  = "INFO"
)

// 软件标识
const softKey = "rokGo"

// 卡密（后续从登录接口或配置中赋值）
var cardKey = ""

// 机器标识（默认空,可调用 设置机器标识 设置）
var machineID = ""

// LogReportReq 日志上报请求结构
type LogReportReq struct {
	SoftKey  string `json:"soft_key"`
	Level    string `json:"level"`
	Category string `json:"category"`
	CardKey  string `json:"card_key"`
	Machine  string `json:"machine"`
	Content  string `json:"content"`
}

// 设置卡密 给日志上报用的 card_key 留的口子
func 设置卡密(key string) {
	cardKey = key
}

// 设置机器标识 设置上报日志里的 machine 字段
func 设置机器标识(id string) {
	machineID = id
}

// 上报日志 异步上报一条日志到服务端
//
// 参数:
//   - level: 日志级别,建议用 LogLevelError / LogLevelWarn / LogLevelInfo
//   - category: 日志分类,如 "login"、"run"、"error"
//   - content: 日志具体内容
func 上报日志(level, category, content string) {
	// 如果还没设置机器标识,自动获取一次
	m := machineID
	if m == "" {
		m = 获取机器码()
	}

	req := LogReportReq{
		SoftKey:  softKey,
		Level:    level,
		Category: category,
		CardKey:  cardKey,
		Machine:  m,
		Content:  content,
	}

	// 异步上报,避免卡住游戏主逻辑
	go func() {
		fmt.Printf("[上报日志] level=%s content=%s\n", level, content)

		body, err := json.Marshal(req)
		if err != nil {
			fmt.Printf("[上报日志失败] JSON 编码失败: %v | content=%s\n", err, content)
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Post(logReportURL, "application/json", bytes.NewReader(body))
		if err != nil {
			fmt.Printf("[上报日志失败] 请求失败: %v | url=%s | content=%s\n", err, logReportURL, content)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("[上报日志失败] 状态码: %d | content=%s\n", resp.StatusCode, content)
		} else {
			fmt.Printf("[上报日志成功] level=%s\n", level)
		}
	}()
}

// 上报错误 上报 ERROR 级别日志
func 上报错误(category, content string) {
	上报日志(LogLevelError, category, content)
}

// 上报警告 上报 WARN 级别日志
func 上报警告(category, content string) {
	上报日志(LogLevelWarn, category, content)
}

// 上报信息 上报 INFO 级别日志
func 上报信息(category, content string) {
	上报日志(LogLevelInfo, category, content)
}

// 信息 控制台打印并上报 INFO 级别日志(category 固定为 "info")
func 信息(content string) {
	fmt.Println(content)
	上报信息("info", content)
}

// 信息f 格式化版本的信息日志
func 信息f(format string, args ...interface{}) {
	信息(fmt.Sprintf(format, args...))
}

// 警告 控制台打印并上报 WARN 级别日志(category 固定为 "info")
func 警告(content string) {
	fmt.Println(content)
	上报警告("info", content)
}

// 警告f 格式化版本的警告日志
func 警告f(format string, args ...interface{}) {
	警告(fmt.Sprintf(format, args...))
}

// 错误 控制台打印并上报 ERROR 级别日志(category 固定为 "info")
func 错误(content string) {
	fmt.Println(content)
	上报错误("info", content)
}

// 错误f 格式化版本的错误日志
func 错误f(format string, args ...interface{}) {
	错误(fmt.Sprintf(format, args...))
}
