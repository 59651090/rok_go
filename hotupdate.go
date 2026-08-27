package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Dasongzi1366/AutoGo/files"
	"github.com/Dasongzi1366/AutoGo/https"
	"github.com/Dasongzi1366/AutoGo/system"
	"github.com/Dasongzi1366/AutoGo/utils"
)

// 热更新说明：
// 1. 在服务器上放一个 JSON 配置文件(格式见下方 UpdateInfo)
// 2. 启动时 HotUpdate() 请求该 JSON,比较 currentVersion 与服务器 version
// 3. 如果版本号不同,下载对应架构的二进制文件,覆盖当前运行的可执行文件(os.Args[0])
// 4. 调用 system.RestartSelf() 自动重启进程,加载新版本
//
// 注意：这不是"运行中不重启换代码",而是"启动时自动下载+覆盖+重启"

// requestURLs 热更新 JSON 配置地址(可配置多个,会打乱顺序尝试)
var requestURLs = []string{
	"http://124.220.39.233:14741/go.json",
}

// currentVersion 当前本地版本号,每次发新版时记得同步更新
var currentVersion = "3"

// found 是否成功请求到更新配置
var found = false

// UpdateInfo 热更新 JSON 配置结构
// 示例：
//
//	{
//	  "version": "2",
//	  "arm64": ["https://你的域名/xmmubin/arm64-v8a-release-packso"],
//	  "arm32": [""],
//	  "x8664": ["https://你的域名/xmmubin/x86_64-release-packso"],
//	  "x8632": [""],
//	  "md5": ["51194ec07baa79a4672573ded4408099"]
//	}
type UpdateInfo struct {
	Version string   `json:"version"`
	URL     []string `json:"url"`
	MD5     []string `json:"md5"`
	Arm64   []string `json:"arm64"`
	Arm32   []string `json:"arm32"`
	X8664   []string `json:"x8664"`
	X8632   []string `json:"x8632"`
}

// ObtainArchitecture 获取设备架构
func ObtainArchitecture() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm64" // 真机架构
	case "amd64":
		return "x8664" // 64位 x86 模拟器架构
	case "386":
		return "x8632" // 32位 x86 模拟器架构
	default:
		return "0" // 其他架构直接返回
	}
}

// ShuffleURLs 打乱 URL 顺序,多线路时避免每次都请求同一个
func ShuffleURLs(urls []string) []string {
	rand.Seed(time.Now().UnixNano())
	for i := len(urls) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		urls[i], urls[j] = urls[j], urls[i]
	}
	return urls
}

// HttpGet 发送 HTTP/HTTPS GET 请求,支持超时设置(单位:秒)
func HttpGet(url string, timeout int64) (int64, string) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return 0, ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		return int64(resp.StatusCode), ""
	}
	return int64(resp.StatusCode), string(body)
}

// HotUpdate 热更新主流程
func HotUpdate() {
	信息f("当前架构:%s", ObtainArchitecture())
	utils.Sleep(100)

	// 1. 打乱请求 URL 顺序并请求配置
	shuffledURLs := ShuffleURLs(append([]string{}, requestURLs...))
	var info UpdateInfo
	for _, url := range shuffledURLs {
		statusCode, data := HttpGet(url, 5)
		if statusCode == 200 {
			if err := json.Unmarshal([]byte(data), &info); err != nil {
				错误f("JSON解析失败:%v", err)
				continue
			}
			信息f("服务器版本:%s", info.Version)
			found = true
			break
		}
	}

	// 2. 请求失败则提示并重启重试
	if !found {
		utils.Toast("热更新配置请求失败", -1, -1, -1)
		time.Sleep(10 * time.Second)
		system.RestartSelf()
	}

	// 3. 服务器版本号 <= 本地版本号则无需更新
	localVer, errLocal := strconv.Atoi(currentVersion)
	serverVer, errServer := strconv.Atoi(info.Version)
	if errLocal != nil || errServer != nil {
		msg := fmt.Sprintf("版本号格式错误--- 本地版本:%s 服务器版本:%s", currentVersion, info.Version)
		错误(msg)
		utils.Toast(msg, -1, -1, -1)
		time.Sleep(1000 * time.Millisecond)
		return
	}
	if serverVer <= localVer {
		msg := fmt.Sprintf("无需更新--- 本地版本:%s 服务器版本:%s", currentVersion, info.Version)
		信息(msg)
		utils.Toast(fmt.Sprintf("本地版本:%s\n服务器版本:%s", currentVersion, info.Version), -1, -1, -1)
		time.Sleep(1000 * time.Millisecond)
		return
	}

	// 4. 服务器版本号更大：下载对应架构的新二进制
	var downloadURL []string
	arch := ObtainArchitecture()
	switch arch {
	case "arm64":
		downloadURL = info.Arm64
	case "x8664":
		downloadURL = info.X8664
	case "x8632":
		downloadURL = info.X8632
	default:
		downloadURL = info.Arm64
	}

	// 打乱下载地址顺序
	downloadURL = ShuffleURLs(downloadURL)
	downloadStatus := false
	utils.Toast("正在更新脚本", -1, -1, -1)

	for _, url := range downloadURL {
		if strings.TrimSpace(url) == "" {
			continue
		}
		code, newData := https.Get(strings.TrimSpace(url), 0)
		if code == 200 {
			信息("下载成功,即将重启脚本")
			downloadStatus = true
			// 覆盖当前运行的可执行文件
			files.WriteBytes(os.Args[0], newData)
			system.RestartSelf()
			// RestartSelf 不会返回; 如果返回了说明重启失败
		} else {
			fmt.Printf("下载文件失败,HTTP状态码: %d\n", code)
		}
	}

	// 5. 下载失败则提示并重启重试
	if !downloadStatus {
		utils.Toast("更新失败", -1, -1, -1)
		time.Sleep(10 * time.Second)
		system.RestartSelf()
	}
}
