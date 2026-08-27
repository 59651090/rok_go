package main

import (
	"time"

	"github.com/Dasongzi1366/AutoGo/motion"
)

// 宝石滑动方向 定义地图搜索的四个滑动方向
// 滑动方向是手指滑动方向,地图会向反方向移动
var 宝石滑动方向 = []struct {
	名称 string
	dx int
	dy int
}{
	{"上", 0, -400}, // 手指向上滑,地图向下移动,显示上方区域
	{"下", 0, 400},  // 手指向下滑,地图向上移动,显示下方区域
	{"左", -400, 0}, // 手指向左滑,地图向右移动,显示左方区域
	{"右", 400, 0},  // 手指向右滑,地图向左移动,显示右方区域
}

// 挖宝石 出城→缩放→四方向滑动搜索→找到宝石后出兵采集
func 挖宝石() {
	信息("开始挖宝石")

	// 1. 确保在城外
	if !是否在城外() {
		信息("不在城外,先去城外")
		去城外()
	}

	// 2. 缩小视角一次
	信息("缩小视角")
	w, h := windowSize.Width, windowSize.Height
	if w == 0 || h == 0 {
		w, h = 720, 1280
	}
	双指缩放(w/2, h/2, 250, 0.8, 500)
	time.Sleep(1 * time.Second)

	// 3. 四方向滑动搜索宝石
	for _, dir := range 宝石滑动方向 {
		信息f("向%s滑动搜索宝石", dir.名称)

		// 每个方向滑动 3 次,逐步探索地图
		for step := 0; step < 3; step++ {
			// 中途被顶号则先处理
			if 检测到顶号则处理() {
				return
			}

			// YOLO 检测宝石
			results := YOLO检测(0, 0, 0, 0, 0.6)
			for _, r := range results {
				if r.Label == "宝石" || r.RawLabel == "class2" {
					信息f("找到宝石: (%d,%d) 置信度%.2f", r.CenterX, r.CenterY, r.Score)
					if 采集宝石(r.CenterX, r.CenterY) {
						信息("宝石采集已派出")
						return
					}
				}
			}

			// 滑动到下一个区域
			centerX, centerY := w/2, h/2
			无惯性滑动(centerX, centerY, centerX+dir.dx, centerY+dir.dy, 500)
			time.Sleep(1 * time.Second)
		}

		// 每个方向探索完后,把视角滑回中心附近
		信息f("%s方向搜索完成,回正视角", dir.名称)
		centerX, centerY := w/2, h/2
		无惯性滑动(centerX+dir.dx*3, centerY+dir.dy*3, centerX, centerY, 500)
		time.Sleep(1 * time.Second)
	}

	错误("搜索完所有方向,未找到宝石")
}

// 采集宝石 点击宝石→采集→创建部队→行军
func 采集宝石(x, y int) bool {
	信息f("点击宝石: (%d,%d)", x, y)
	motion.Click(x, y, 0, 0)
	time.Sleep(2 * time.Second)

	// 点击采集按钮(复用城外采集的图)
	result := 找图并点击(
		"剪裁41.png|剪裁58.png|剪裁59.png|剪裁61.png|剪裁62.png|剪裁63.png|城外采集1.png|城外采集2.png|城外采集3.png|城外采集4.png|城外采集5.png|城外采集6.png",
		0.8, 1000, "000000", 0, 0, 0, 0, 0)
	if !result.Found {
		警告("未找到宝石采集按钮")
		按键返回()
		return false
	}
	time.Sleep(2 * time.Second)

	// 创建部队
	result = 找图并点击("剪裁44.png|剪裁45.png", 0.8, 1000, "000000", 0, 0, 0, 0, 0)
	if !result.Found {
		错误("未找到创建部队按钮")
		按键返回()
		return false
	}
	time.Sleep(3 * time.Second)

	// 行军
	result = 找图并点击("行军48.png", 0.8, 1000, "000000", 0, 0, 0, 0, 0)
	if !result.Found {
		错误("未找到行军按钮")
		按键返回()
		return false
	}

	信息("宝石部队已行军")
	return true
}
