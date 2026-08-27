package main

import (
	"strings"
	"time"

	"github.com/Dasongzi1366/AutoGo/app"
)

// 顶号等待时间 检测到顶号后的倒计时等待时长,由配置决定
var 顶号等待时间 time.Duration

// 顶号关键词 OCR 识别结果中包含任意一个关键词即判定为顶号
var 顶号关键词 = []string{
	"其他设备",
	"登录了",
	"已经在",
	"账号在",
	"其他地方",
	"被迫下线",
	"重新登录",
}

// 检测顶号 返回 true 表示检测到账号被顶
// 通过 OCR 检测弹窗文字,命中顶号关键词即判定
func 检测顶号() bool {
	text, _ := Ocr(472, 290, 808, 382)
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}

	for _, kw := range 顶号关键词 {
		if strings.Contains(text, kw) {
			信息f("检测到顶号弹窗: %s (命中关键词: %s)", text, kw)
			return true
		}
	}
	return false
}

// 检测到顶号则处理 如果检测到顶号,调用处理顶号并返回 true
// 用于子流程中快速检查并中断当前操作
func 检测到顶号则处理() bool {
	if 检测顶号() {
		处理顶号(stopChan)
		return true
	}
	return false
}

// 处理顶号 关闭游戏、显示倒计时、等待、然后重新登录
func 处理顶号(stop <-chan struct{}) {
	错误("检测到顶号,关闭游戏并开始倒计时等待")

	// 先强制关闭游戏,避免顶号弹窗一直卡在前台
	app.ForceStop(pkg)
	time.Sleep(2 * time.Second)

	设置顶号等待倒计时(顶号等待时间)
	defer 清除顶号等待倒计时()

	select {
	case <-time.After(顶号等待时间):
	case <-stop:
		信息("顶号等待被停止")
		return
	}

	信息("顶号等待结束,尝试重新登录")
	检查游戏运行()
	进入游戏(stop)
}
