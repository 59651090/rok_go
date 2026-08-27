package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Dasongzi1366/AutoGo/app"
	"github.com/Dasongzi1366/AutoGo/images"
	"github.com/Dasongzi1366/AutoGo/imgui"
	"github.com/Dasongzi1366/AutoGo/motion"
)

// 万国觉醒包名
const pkg = "com.lilithgames.rok.offical.cn"

// debugMode 控制是否显示测试功能按钮(true=显示, false=隐藏)
const debugMode = false

// 屏幕尺寸
var windowSize = struct {
	Width  int
	Height int
}{Width: 720, Height: 1280}

// 全局变量（悬浮球和主逻辑共用）
var (
	stopChan            = make(chan struct{}, 1) // 停止信号通道
	selectedMode int32  = 0                      // 当前选中的功能模式（默认采集混资）
	isRunning    bool                            // 是否正在运行
	showWindow   = true                          // 设置窗口是否显示

	领取资源开关 = true // 默认打勾：是否领取资源（采集队列满时才领）
	换角色开关  = true // 默认打勾：是否换角色
	领箱子开关  = true // 默认打勾：是否领箱子
)

var roleIndex = 1 //切换角色下标

var rolePos = []struct{ X, Y int }{
	{434, 270},
	{888, 270},
	{434, 413},
	{888, 413},
	{434, 542},
	{888, 542},
}

// ========== 游戏逻辑 ==========

// 检查游戏运行 判断游戏是否在运行，如果未运行则启动，同时获取屏幕实际尺寸
func 检查游戏运行() {

	if app.IsInstalled(pkg) {
		信息("万国觉醒正在运行，等待5秒...")
		currentPkg := app.CurrentPackage()
		if currentPkg != pkg {
			app.Launch(pkg, 0) // 切回前台
		}
		time.Sleep(5 * time.Second)
	} else {
		信息("万国觉醒未运行，正在启动...等待10秒")
		app.Launch(pkg, 0)
		time.Sleep(10 * time.Second)
	}
	// 获取屏幕实际尺寸（游戏横屏后宽高会互换）
	img := images.CaptureScreen(0, 0, 0, 0, 0)
	if img != nil {
		b := img.Bounds()
		windowSize.Width = b.Dx()
		windowSize.Height = b.Dy()
		fmt.Printf("屏幕尺寸: %dx%d\n", windowSize.Width, windowSize.Height)
	}
}

// 进入游戏 检测登录界面信号图，点击屏幕中心进入
// 若长时间检测不到进入游戏标志，则杀掉 APP 重新启动，避免卡死
func 进入游戏(stop <-chan struct{}) {
	信息("进入游戏流程")

	const 进入游戏超时 = 2 * time.Minute
	startTime := time.Now()

	for {
		select {
		case <-stop:
			信息("进入游戏被停止")
			return
		default:
		}

		// 超时保护：超过 2 分钟还没进入游戏，杀掉 APP 重来
		if time.Since(startTime) > 进入游戏超时 {
			错误("进入游戏超时，杀掉 APP 重新启动")
			app.ForceStop(pkg)
			time.Sleep(5 * time.Second)
			检查游戏运行()
			startTime = time.Now()
			continue
		}

		信息("检测是否进入游戏")
		// 城外
		result := 找图("城外1.png|城外2.png|城外3.png|剪裁14.png|剪裁15.png|剪裁16.png", 0.7, "000000", 0, 0, 0, 0, 0)
		if result.Found {
			信息("已检测到城外，进入游戏完成")
			return
		}
		//城内
		result = 找图("剪裁51.png|剪裁52.png|剪裁53.png", 0.75, "000000", 0, 0, 0, 0, 0)
		if result.Found {
			信息("已检测到城内，进入游戏完成")
			return
		}
		//任务
		result = 找图("任务1.png|任务2.png|任务3.png|任务4.png", 0.75, "000000", 0, 0, 0, 0, 0)
		if result.Found {
			信息("已检测到任务界面，进入游戏完成")
			return
		}

		result = 找图("登录_信号.png|16+.png|162.png", 0.7, "000000", 0, 0, 0, 0, 0)
		if result.Found {
			motion.Click(windowSize.Width/2, windowSize.Height/2, 0, 0)
			time.Sleep(10 * time.Second)
		}

		ocrresult, _ := Ocr(565, 584, 729, 620)
		if ocrresult != "" {
			if strings.Contains(ocrresult, "进入游戏") {
				motion.Click(windowSize.Width/2, windowSize.Height/2, 0, 0)
				time.Sleep(10 * time.Second)
			}
		}

		time.Sleep(2 * time.Second)
	}

}

// 保证游戏前台 调用找图前的前置检查：
// APP 没装/没运行 → 走登录流程；在运行但不在前台 → 切前台；在前台 → 直接返回
func 保证游戏前台() {
	if !app.IsInstalled(pkg) {
		信息("游戏未运行，走登录流程")
		检查游戏运行()
		进入游戏(stopChan)
		return
	}
	// 已运行：不在前台就切回前台
	if app.CurrentPackage() != pkg {
		信息("游戏不在前台，切回前台")
		app.Launch(pkg, 0)
		time.Sleep(5 * time.Second)
	}
}

// 是否在城外 检测是否在城外（找城外特征图）
func 是否在城外() bool {
	信息("执行-检测是否在城外")
	保证游戏前台()
	result := 找图("城外1.png|城外2.png|城外3.png|剪裁14.png|剪裁15.png|剪裁16.png|城外6.png|城外7.png|城外8.png|城外9.png|城外10.png", 0.7, "000000", 0, 0, 0, 0, 0)
	return result.Found
}

// 是否在城内 检测是否在城内（找城内特征图）
func 是否在城内() bool {
	信息("执行-检测是否在城内")
	保证游戏前台()
	result := 找图("剪裁51.png|剪裁52.png|剪裁53.png", 0.75, "000000", 0, 0, 0, 0, 0)
	return result.Found
}

// 去城外 从城内去城外：点击城门出城
func 去城外() {
	找图并点击("剪裁51.png|剪裁52.png|剪裁53.png", 0.8, 1000, "000000", 0, 0, 0, 0, 0)
	time.Sleep(2 * time.Second)
}

func 去城内() {
	找图并点击("城外1.png|城外2.png|城外3.png|剪裁14.png|剪裁15.png|剪裁16.png|城外6.png|城外7.png|城外8.png|城外9.png|城外10.png", 0.8, 1000, "000000", 0, 0, 0, 0, 0)
	time.Sleep(2 * time.Second)
}

// 点击帮助 点击帮助按钮（联盟互助）
func 点击帮助() {
	找图并点击("帮助1.png|帮助2.png|帮助3.png|帮助4.png|帮助5.png|帮助6.png", 0.8, 1000, "000000", 0, 0, 0, 0, 0)
	time.Sleep(1500 * time.Millisecond)
}

// 找混资图标 判断是否找到混资图标（城外资源点）
func 找混资图标() bool {
	result := 找图("剪裁30.png|剪裁31.png|城外3.png|剪裁32.png|剪裁33.png|chengwaihun1.png|chengwaihun2.png|chengwaihun3.png|chengwaihun4.png|chengwaihun5.png|chengwaihun6.png|chengwaihun7.png|chengwaihun8.png|chengwaihun9.png", 0.7, "000000", 0, 0, 0, 0, 0)
	return result.Found
}

// 挖混资一次 单次挖混资：搜资源→找图标→选位置→创建部队→行军
func 挖混资一次() {
	// 第一步：搜索城外资源
	result := 找图并点击(
		"城外搜索1.png|城外搜索2.png|城外搜索3.png|城外搜索4.png|城外搜索5.png|城外搜索6.png|城外搜索7.png|城外搜索8.png|城外搜索9.png|城外搜索10.png|城外搜索11.png",
		0.5, 1000, "000000", 0, 0, 0, 0, 0)
	if result.Found {
		time.Sleep(1 * time.Second)

		// 第二步：找混资图标
		if 找混资图标() {
			信息("找到 混资按钮")

			// 第三步：随机选一个混资点位置并点击
			positions := []struct{ X, Y int }{
				{453, 634}, {636, 634}, {830, 634}, {1081, 634},
			}
			idx := 随机整数(0, len(positions)-1)
			p := positions[idx]
			motion.Click(p.X, p.Y, 0, 0)
			time.Sleep(2 * time.Second)

			// 减号
			result = 找图("jian.png", 0.7, "000000", 0, 0, 0, 0, 0)
			if result.Found {
				for i := 0; i < 5; i++ {
					motion.Click(result.X, result.Y, 0, 0)
					time.Sleep(300 * time.Millisecond)
				}
			}
			time.Sleep(1 * time.Second)

			// 第四步：点击搜索按钮
			result := 找图并点击(
				"搜索4.png|搜索5.png|搜索6.png|搜索7.png|剪裁38.png|剪裁55.png|剪裁56.png|城外搜索文字1.png|城外搜索文字2.png|城外搜索文字3.png|城外搜索文字4.png|城外搜索文字5.png",
				0.5, 1000, "000000", 0, 0, 0, 0, 0)
			if result.Found {
				信息("找到搜索按钮")
				time.Sleep(3 * time.Second)

				// 第五步：点击采集按钮
				result := 找图并点击(
					"剪裁41.png|剪裁58.png|剪裁59.png|剪裁61.png|剪裁62.png|剪裁63.png|城外采集1.png|城外采集2.png|城外采集3.png|城外采集4.png|城外采集5.png|城外采集6.png",
					0.8, 1000, "000000", 0, 0, 0, 0, 0)
				if result.Found {
					信息("找到采集按钮")
					time.Sleep(2 * time.Second)

					// 第六步：点击创建部队
					result := 找图并点击("剪裁44.png|剪裁45.png", 0.8, 1000, "000000", 0, 0, 0, 0, 0)
					if result.Found {
						信息("找到创建部队")
						time.Sleep(3 * time.Second)
						// 第七步：点击行军（如果做指定队列，从这里开始修改）
						result := 找图并点击("行军48.png", 0.8, 1000, "000000", 0, 0, 0, 0, 0)
						if result.Found {
							信息("创建部队界面")
						} else {
							错误("没有找到城外 行军按钮！")
							按键返回() // 返回上一步
						}
					} else {
						错误("没有找到城外 创建部队按钮！")
						按键返回()
					}
				} else {
					错误("没有找到城外 采集按钮！")
					按键返回()
				}
			} else {
				错误("没有找到城外 搜索文字按钮！")
				按键返回()
			}
		} else {
			错误("没有找到城外 混资按钮！")
		}
	} else {
		错误("没有找到城外 搜索按钮！")
	}
	time.Sleep(2 * time.Second)
}

// 挖混资默认队伍 功能挖混资源默认队伍：检测城内/城外 → 城内先出城，城外直接挖一次
// 注：队列是否满由外层 mode0 循环统一检测，这里不再重复判定，避免等待放大
func 挖混资默认队伍() {
	信息("功能挖混资源默认队伍")
	按键返回()

	// 中途被顶号则先处理
	if 检测到顶号则处理() {
		return
	}

	if 是否在城外() {
		信息("在城外，挖一次")
		挖混资一次()
	} else {
		信息("在城内，先去城外")
		去城外()
	}
}

// 游戏主逻辑 主逻辑循环：启动游戏 → 进入 → 并行跑{单选主功能 + 独立勾选任务}
// stop 为停止信号通道，收到信号后立即退出循环
func 游戏主逻辑(stop <-chan struct{}) {
	// 退出时统一复位运行状态：悬浮球恢复显示
	defer func() {
		isRunning = false
		if ball != nil {
			ball.IsRunning = false
		}
	}()

	time.Sleep(2 * time.Second)
	检查游戏运行()   // 确认游戏在运行
	进入游戏(stop) // 进入游戏主界面
	点击帮助()     // 先点一次帮助

	if selectedMode == 0 {
		// 模式0：默认队伍采集混资。队列满时触发勾选任务，空了继续挖
		for {
			select {
			case <-stop:
				信息("脚本已停止")
				return
			default:
				// 检测是否被顶号
				if 检测顶号() {
					处理顶号(stop)
					continue
				}

				// 检测队列：满了就跑勾选任务，没满就挖
				isFull, _ := 是否满队列(1194, 130, 1269, 162)
				if isFull {
					点击帮助()
					信息("队列满，执行勾选任务")
					// 队列满空档：按优先级跑勾选任务（领资源→领箱子→换角色）
					执行勾选任务()
					随机等待(60*1000, 2*60*1000)
				} else {
					挖混资默认队伍()
					time.Sleep(5 * time.Second)
				}
			}
		}
	} else if selectedMode == 1 {
		// 模式1：只帮助（只点联盟互助，不挖矿）
		信息("只帮助模式启动！")
		for {
			select {
			case <-stop:
				信息("脚本已停止")
				return
			default:
				点击帮助()
				time.Sleep(2 * time.Second)
			}
		}
	} else if selectedMode == 2 {

		// 模式2：挖宝石
		信息("挖宝石模式启动！")
		for {
			select {
			case <-stop:
				信息("脚本已停止")
				return
			default:
				// 检测是否被顶号
				if 检测顶号() {
					处理顶号(stop)
					continue
				}

				挖宝石()
				time.Sleep(5 * time.Second)
			}
		}
	} else if selectedMode == 3 {
		// 模式3：监控铁手
		信息("监控铁手模式启动！")
		for {
			select {
			case <-stop:
				信息("脚本已停止")
				return
			default:
				// 检测是否被顶号
				if 检测顶号() {
					处理顶号(stop)
					continue
				}

				监控铁手()
				// 一轮监控完，等待一段时间再重新跑
				间隔分钟 := int(当前配置.铁手间隔分钟)
				信息f("铁手监控完成一轮，等待 %d 分钟后重试", 间隔分钟)
				for i := 0; i < 间隔分钟*60; i++ {
					select {
					case <-stop:
						信息("脚本已停止")
						return
					default:
					}
					if 检测顶号() {
						处理顶号(stop)
						break
					}
					time.Sleep(1 * time.Second)
				}
			}
		}
	}

	// 兜底:仅等待停止信号
	for {
		select {
		case <-stop:
			信息("脚本已停止")
			return
		}
	}
}

// 执行勾选任务 按"领资源→领箱子→换角色"优先级跑一遍已勾选的独立任务
// 仅在 mode0 队列满时调用，每次执行时换角色只推进一个角色
func 执行勾选任务() {
	// 1) 领取资源（优先级最高）
	if 领取资源开关 {
		领城内资源()
	}

	// 中途被顶号则先处理,不再继续后面的步骤
	if 检测到顶号则处理() {
		return
	}

	// 收到停止信号就不再继续后面的步骤
	select {
	case <-stopChan:
		信息("勾选任务被停止")
		return
	default:
	}
	// 2) 领箱子
	if 领箱子开关 {
		if !领箱子() {
			警告("领箱子失败,跳过")
		}
	}
	select {
	case <-stopChan:
		信息("勾选任务被停止")
		return
	default:
	}
	// 3) 换角色（本次只换到下一个角色）
	if 换角色开关 {
		按键返回()
		换角色()
	}
}

// 领城内资源
func 领城内资源() {
	信息("领城内资源")
	按键返回()

	// 中途被顶号则先处理
	if 检测到顶号则处理() {
		return
	}

	if 是否在城内() {

	} else {
		去城内()
	}
	value := ""
	for i := 1; i <= 8; i++ {
		if i != 8 {
			value = value + "城内资源" + strconv.Itoa(i) + ".png|"
		} else {
			value = value + "城内资源" + strconv.Itoa(i) + ".png"
		}
	}
	for i := 0; i < 10; i++ {
		result := 找图(value, 0.8, "000000", 0, 0, 0, 0, 0)
		if !result.Found {
			continue
		}
		// 跳过禁用区域 (1,573)-(545,718) 内的坐标
		if result.X >= 1 && result.X <= 545 && result.Y >= 573 && result.Y <= 718 {
			//警告f("城内资源坐标在禁用区域内,跳过: (%d,%d)", result.X, result.Y)
			continue
		}
		motion.Click(result.X, result.Y, 0, 0)
		time.Sleep(2 * time.Second)
	}
}

// 领箱子 领取游戏内各种奖励箱子
// 返回 true 表示成功进入领奖流程,false 表示失败或放弃
func 领箱子() bool {
	信息("开始领箱子")

	// 中途被顶号则先处理
	if 检测到顶号则处理() {
		return false
	}

	const maxAttempt = 3
	for attempt := 0; attempt < maxAttempt; attempt++ {
		if attempt > 0 {
			fmt.Printf("领箱子第 %d 次尝试...\n", attempt+1)
		}

		按键返回()

		// 第一步：尝试点击联盟图标
		result := 找图并点击("联盟1.png|联盟2.png|联盟3.png", 0.7, 1000, "000000", 0, 0, 0, 0, 0)
		if !result.Found {
			警告("未找到联盟图标")

			// 检查快捷栏是否已经展开
			result = 找图("统帅1.png|战役1.png|道具1.png|邮件2.png|邮件1.png", 0.8, "000000", 0, 0, 0, 0, 0)
			if result.Found {
				错误("快捷栏已展开但未找到联盟图标,放弃领箱子")
				continue
			}

			// 尝试展开快捷栏
			警告("未找到快捷栏目,尝试展开")
			result = 找图并点击("快捷栏1.png|快捷栏2.png|快捷栏3.png|快捷栏4.png", 0.70, 2000, "000000", 0, 0, 0, 0, 0)
			if !result.Found {
				错误("找不到快捷栏按钮,放弃领箱子")
				continue
			}
			time.Sleep(1 * time.Second)
			continue // 再试一次
		}

		// 找到联盟图标,进入领奖流程
		time.Sleep(2 * time.Second)

		// 找礼物/箱子图标并点击
		result = 找图并点击("礼物.png|礼物1.png|礼物2.png|礼物3.png|礼物4.png|礼物5.png", 0.8, 1000, "000000", 0, 0, 0, 0, 0)
		if !result.Found {
			错误("未找到礼物/箱子")
			continue // 改为重试,不要直接放弃
		}
		time.Sleep(2 * time.Second)

		for i := 0; i < 8; i++ {
			点击(340, 418, 200)
		}

		//普通
		time.Sleep(2 * time.Second)
		点击(675, 206, 2000)
		time.Sleep(1 * time.Second)
		点击(675, 206, 2000)
		time.Sleep(1 * time.Second)
		点击(1109, 203, 2000)
		time.Sleep(1 * time.Second)
		resultText, _ := Ocr(601, 120, 675, 153)
		if resultText == "奖励" {
			result = 找图并点击("蓝色确定1.png|蓝色确定2.png", 0.8, 2000, "000000", 0, 0, 0, 0, 0)
			time.Sleep(2 * time.Second)
		}

		//稀有
		time.Sleep(2 * time.Second)
		点击(924, 206, 2000)
		time.Sleep(1 * time.Second)
		点击(924, 206, 2000)
		time.Sleep(1 * time.Second)
		点击(1109, 203, 2000)
		time.Sleep(10 * time.Second)

		// 循环点击"再开启",直到界面不再出现该字样
		for i := 0; i < 30; i++ {
			resultText, _ = Ocr(566, 628, 723, 675)
			if !strings.Contains(resultText, "再开启") {
				break
			}
			点击(645, 651, 1000)
			time.Sleep(10 * time.Second)
		}

		按键(motion.KEYCODE_BACK, 2000)
		time.Sleep(1 * time.Second)
		按键(motion.KEYCODE_BACK, 2000)
		time.Sleep(1 * time.Second)

		return true
	}

	错误("领箱子尝试 3 次仍未成功,放弃")
	return false
}

// 换角色 切换到下一个角色，返回 true 表示已切换成功，false 表示全部完成或失败
func 换角色() bool {
	// 收到停止信号立即退出
	select {
	case <-stopChan:
		信息("换角色被停止")
		return false
	default:
	}

	// 中途被顶号则先处理
	if 检测到顶号则处理() {
		return false
	}

	信息f("换角色: 第 %d 个", roleIndex)
	time.Sleep(2 * time.Second)

	点击(41, 29, 2000)

	result := 找图并点击("设置1.png|设置2.png|设置3.png|设置4.png", 0.7, 1000, "000000", 0, 0, 0, 0, 0)
	if !result.Found {
		错误("未找到-设置")
		return false
	}
	time.Sleep(2 * time.Second)

	result = 找图并点击("角色管理1.png|角色管理2.png|角色管理3.png|角色管理4.png", 0.7, 1000, "000000", 0, 0, 0, 0, 0)
	if !result.Found {
		错误("未找到-角色管理")
		return false
	}
	time.Sleep(2 * time.Second)

	// 识别角色总数
	脚色总数Str, _ := Ocr(1007, 153, 1072, 197)
	脚色总数, err := strconv.Atoi(strings.TrimSpace(脚色总数Str))
	if err != nil || 脚色总数 <= 0 {
		fmt.Printf("识别角色总数失败: '%s'，默认用10\n", 脚色总数Str)
		脚色总数 = 10
	}
	fmt.Printf("角色总数: %d\n", 脚色总数)

	// 已经超过总数，重置后继续下一圈
	if roleIndex > 脚色总数 {
		信息("全部角色切换完成一圈，继续循环！")
		roleIndex = 1
	}

	// 超过6个角色，需要滑动让对应行显示出来
	if roleIndex > 6 {
		// 超出几行（每行2个角色）：第7,8个超1行，第9,10个超2行
		超出行数 := (roleIndex-7)/2 + 1
		// 每行滑动135像素
		滑动像素 := 超出行数 * 154 //135
		// 上滑：从下往上滑（无惯性，精确停止）
		无惯性滑动(636, 542, 640, 558-滑动像素, 800)
		time.Sleep(2 * time.Second)

		// 滑动后固定点第3行位置：左{434,542} 右{888,542}
		列 := (roleIndex - 1) % 2 // 0=左列, 1=右列
		fmt.Println("列=", 列)
		if 列 == 0 {
			点击(434, 542, 2000)
		} else {
			点击(888, 542, 2000)
		}
	} else {
		点击(rolePos[roleIndex-1].X, rolePos[roleIndex-1].Y, 2000)
	}

	ocrtext, _ := Ocr(577, 117, 707, 158)
	if ocrtext == "角色登入" {
		点击(820, 511, 10000)
		roleIndex = roleIndex + 1
		进入游戏(stopChan)
		return true
	}

	// 没识别到"角色登入"，跳过这个
	roleIndex = roleIndex + 1
	return false
}

// ========== main 入口 ==========

func main() {
	// 初始化机器码,后续日志上报会带上 machine 字段
	初始化机器码()

	// 加载本地配置并应用
	加载配置()
	应用配置()

	// 启动时先检查热更新; debug 模式下跳过,方便本地调试
	if !debugMode {
		HotUpdate()
	}

	// 初始化 imgui 渲染引擎
	imgui.Init()
	// 初始化 OCR 引擎（PPOCR v5）
	初始化Ocr()

	// UI 窗口是否可见
	showWindow := true

	// 主渲染循环
	imgui.Run(func() {
		// ===== 绘制悬浮球（始终可见） =====
		drawFloatingBall()

		// ===== 绘制 YOLO 检测框覆盖层 =====
		drawYoloOverlay()

		// ===== 绘制等待倒计时 =====
		drawWaitCountdown()

		// ===== 主设置窗口（点开始后隐藏） =====
		if !showWindow {
			return
		}

		// 设置窗口初始大小和位置（首次出现时生效）
		imgui.SetNextWindowSizeV(imgui.Vec2{X: 500, Y: 400}, imgui.CondOnce)
		imgui.SetNextWindowPosV(imgui.Vec2{X: 100, Y: 100}, imgui.CondOnce, imgui.Vec2{X: 0, Y: 0})
		imgui.BeginV("夸夸助手", &showWindow, 0)

		// 标题（版本号与热更新 currentVersion 保持一致）
		imgui.Text(fmt.Sprintf("夸夸助手 v%s.0", currentVersion))
		imgui.Separator()
		imgui.Spacing()

		// 功能选择单选框（竖排，避免超宽）
		imgui.Text("功能选择：")
		if imgui.RadioButtonIntPtr("默认队伍采集混", &selectedMode, 0) {
		}
		if imgui.RadioButtonIntPtr("只帮助", &selectedMode, 1) {
		}
		if imgui.RadioButtonIntPtr("宝石-未完成", &selectedMode, 2) {
		}
		if imgui.RadioButtonIntPtr("监控铁手", &selectedMode, 3) {
		}

		imgui.Spacing()

		// 独立勾选功能（与功能选择平级，可同时勾多个）
		imgui.Text("独立功能：")
		imgui.Checkbox("领箱子", &领箱子开关)
		imgui.Checkbox("领取资源", &领取资源开关)
		imgui.Checkbox("换角色", &换角色开关)

		imgui.Spacing()
		imgui.Separator()
		imgui.Spacing()

		// 顶号等待时间配置
		imgui.Text("顶号设置：")
		if imgui.SliderInt("顶号等待分钟", &顶号等待分钟输入, 1, 120) {
			保存配置()
			应用配置()
		}
		imgui.SameLineV(0, 10)
		imgui.Text(fmt.Sprintf("当前: %d 分钟", 顶号等待分钟输入))

		imgui.Spacing()

		// 铁手监控间隔配置
		imgui.Text("铁手设置：")
		if imgui.SliderInt("铁手间隔分钟", &铁手间隔分钟输入, 1, 120) {
			保存配置()
			应用配置()
		}
		imgui.SameLineV(0, 10)
		imgui.Text(fmt.Sprintf("当前: %d 分钟", 铁手间隔分钟输入))

		imgui.Spacing()
		imgui.Separator()
		imgui.Spacing()

		// 测试按钮区（仅 debug 模式显示）
		if debugMode {
			imgui.Text("测试功能：")
			if imgui.Button("OCR") {
				//Ocr(0, 0, 0, 0)
				//Ocr(1007, 153, 1072, 197)
				Ocr(472, 290, 808, 382)

				ScreenShot("")
			}
			imgui.SameLineV(0, 10)
			if imgui.Button("YOLO") {
				results := YOLO检测(0, 0, 0, 0)
				if len(results) == 0 {
					fmt.Println("YOLO 未检测到目标")
				} else {
					for _, r := range results {
						fmt.Printf("YOLO: %s (%.2f) at (%d,%d) %dx%d\n", r.Label, r.Score, r.X, r.Y, r.Width, r.Height)
					}
				}
			}
			imgui.SameLineV(0, 10)
			if imgui.Button("YOLO截图") {
				path := ScreenShot("yolo_test")
				fmt.Printf("截图路径: %s\n", path)
				if path != "" {
					results := YOLO检测从文件(path)
					fmt.Printf("YOLO截图 检测结果数: %d\n", len(results))
					for _, r := range results {
						fmt.Printf("YOLO截图: %s (%.2f) at (%d,%d) %dx%d\n", r.Label, r.Score, r.X, r.Y, r.Width, r.Height)
					}
				}
			}
			imgui.SameLineV(0, 10)
			if imgui.Button("YOLO框") {
				切换Yolo覆盖层显示()
			}
			imgui.SameLineV(0, 10)
			if imgui.Button("YOLO找点") {
				YOLO找并点击("国内木材", 0.6, 1000, 3)
			}
			imgui.SameLineV(0, 10)
			if imgui.Button("放大视角") {
				缩放视角(true)
			}
			imgui.SameLineV(0, 10)
			if imgui.Button("缩小视角") {
				缩放视角(false)
			}
			imgui.Spacing()
		}

		// 开始按钮
		imgui.PushStyleColorVec4(imgui.ColButton, imgui.Vec4{X: 0.13, Y: 0.59, Z: 0.95, W: 1.0})
		if imgui.Button("开始") {
			if !isRunning {
				isRunning = true
				if ball != nil {
					ball.IsRunning = true
				}
				showWindow = false
				fmt.Printf("启动！功能选择: %d\n", selectedMode)
				go 游戏主逻辑(stopChan)
			}
		}
		imgui.PopStyleColor()

		imgui.End()
	})

	// 阻塞主进程，防止程序退出（imgui 在后台线程运行）
	select {}
}
