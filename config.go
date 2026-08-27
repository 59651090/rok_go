package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Dasongzi1366/AutoGo/files"
)

// configFile 本地配置文件路径
const configFile = "/sdcard/rokGo_config.json"

// AppConfig 应用配置
type AppConfig struct {
	// 顶号等待分钟 检测到顶号后的等待时间(分钟)
	顶号等待分钟 int32 `json:"顶号等待分钟"`
	// 铁手间隔分钟 铁手监控每轮之间的休息时间(分钟)
	铁手间隔分钟 int32 `json:"铁手间隔分钟"`
}

// defaultConfig 默认配置
var defaultConfig = AppConfig{
	顶号等待分钟:   30,
	铁手间隔分钟: 5,
}

// 顶号等待分钟输入 绑定到 UI 控件的变量
var 顶号等待分钟输入 int32 = 30

// 铁手间隔分钟输入 绑定到 UI 控件的变量
var 铁手间隔分钟输入 int32 = 5

// 当前配置 全局配置实例
var 当前配置 AppConfig

// 加载配置 从本地文件加载配置,不存在则使用默认配置
func 加载配置() {
	当前配置 = defaultConfig
	顶号等待分钟输入 = defaultConfig.顶号等待分钟
	铁手间隔分钟输入 = defaultConfig.铁手间隔分钟

	if !files.Exists(configFile) {
		fmt.Println("配置文件不存在,使用默认配置")
		return
	}

	data := files.Read(configFile)
	if data == "" {
		fmt.Println("配置文件为空,使用默认配置")
		return
	}

	var cfg AppConfig
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		fmt.Printf("配置文件解析失败: %v, 使用默认配置\n", err)
		return
	}

	// 基本校验
	if cfg.顶号等待分钟 <= 0 {
		cfg.顶号等待分钟 = defaultConfig.顶号等待分钟
	}
	if cfg.铁手间隔分钟 <= 0 {
		cfg.铁手间隔分钟 = defaultConfig.铁手间隔分钟
	}

	当前配置 = cfg
	顶号等待分钟输入 = cfg.顶号等待分钟
	铁手间隔分钟输入 = cfg.铁手间隔分钟
	fmt.Printf("配置加载成功: 顶号等待=%d分钟 铁手间隔=%d分钟\n", 当前配置.顶号等待分钟, 当前配置.铁手间隔分钟)
}

// 保存配置 将当前配置写入本地文件
func 保存配置() {
	当前配置.顶号等待分钟 = 顶号等待分钟输入
	当前配置.铁手间隔分钟 = 铁手间隔分钟输入
	data, err := json.MarshalIndent(当前配置, "", "  ")
	if err != nil {
		fmt.Printf("配置序列化失败: %v\n", err)
		return
	}

	files.Write(configFile, string(data))
	fmt.Printf("配置已保存: 顶号等待=%d分钟 铁手间隔=%d分钟\n", 当前配置.顶号等待分钟, 当前配置.铁手间隔分钟)
}

// 应用配置 将加载的配置应用到运行时的全局变量
func 应用配置() {
	顶号等待时间 = time.Duration(当前配置.顶号等待分钟) * time.Minute
	fmt.Printf("配置已应用: 顶号等待=%v\n", 顶号等待时间)
}
