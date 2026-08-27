package main

import (
	"fmt"
	"math"

	"github.com/Dasongzi1366/AutoGo/device"
	"github.com/Dasongzi1366/AutoGo/imgui"
)

// 悬浮球状态
type FloatingBall struct {
	X      float32
	Y      float32
	Radius float32

	ScreenWidth  int
	ScreenHeight int

	// 拖动
	IsDragging  bool
	DragStartX  float32
	DragStartY  float32
	DragOffsetX float32
	DragOffsetY float32

	// 菜单
	IsExpanded bool
	ExpandAnim float32

	// 位置
	IsOnRightSide bool

	// 自动吸附动画
	IsAnimating  bool
	AnimProgress float32
	AnimStartX   float32
	AnimTargetX  float32

	// 半隐藏（贴边时只露出一小条）
	IsHalfHidden    bool
	HalfHideAnim    float32 // 0=完全显示 1=半隐藏
	HalfHideDirLeft bool

	// 外部状态
	IsRunning bool
}

var ball *FloatingBall

func initFloatingBall() {
	w, h, _, _ := device.GetDisplayInfo(0)
	if w == 0 {
		w, h = 1080, 1920
	}
	ball = &FloatingBall{
		Radius:        36,
		ScreenWidth:   w,
		ScreenHeight:  h,
		X:             float32(w) - 36 - 10,
		Y:             float32(h) / 2,
		IsOnRightSide: true,
	}
}

// 设置悬浮球位置 直接设置悬浮球坐标
func 设置悬浮球位置(x, y float32) {
	if ball == nil {
		initFloatingBall()
	}
	ball.X = x
	ball.Y = y
	ball.IsHalfHidden = false
	ball.HalfHideAnim = 0
}

func drawFloatingBall() {
	// 运行中完全隐藏悬浮球（停止脚本后自动恢复）
	if isRunning {
		return
	}

	if ball == nil {
		initFloatingBall()
	}

	updateBallAnimations()

	// 计算实际显示位置（半隐藏时偏移到屏幕外）
	displayX := ball.X
	if ball.IsHalfHidden {
		hideOffset := ball.Radius*2 - 6 // 只露出6px
		if ball.HalfHideDirLeft {
			displayX = ball.X - hideOffset
		} else {
			displayX = ball.X + hideOffset
		}
	}

	// 计算窗口大小
	var windowW, windowH float32
	if ball.IsExpanded || ball.ExpandAnim > 0 {
		spacing := float32(80) * easeOutCubic(ball.ExpandAnim)
		if ball.IsOnRightSide {
			windowW = spacing*3 + 100
		} else {
			windowW = spacing*3 + 100
		}
		windowH = 100
	} else {
		windowW = ball.Radius*2 + 20
		windowH = ball.Radius*2 + 20
	}

	var windowX, windowY float32
	if ball.IsExpanded || ball.ExpandAnim > 0 {
		if ball.IsOnRightSide {
			windowX = displayX - 80*3*easeOutCubic(ball.ExpandAnim) - 50
		} else {
			windowX = displayX - 50
		}
		windowY = ball.Y - 50
	} else {
		windowX = displayX - ball.Radius - 10
		windowY = ball.Y - ball.Radius - 10
	}

	imgui.SetNextWindowPosV(imgui.Vec2{X: windowX, Y: windowY}, imgui.CondAlways, imgui.Vec2{})
	imgui.SetNextWindowSizeV(imgui.Vec2{X: windowW, Y: windowH}, imgui.CondAlways)
	imgui.PushStyleColorVec4(imgui.ColWindowBg, imgui.Vec4{X: 0, Y: 0, Z: 0, W: 0})
	imgui.PushStyleVarFloat(imgui.StyleVarWindowBorderSize, 0)
	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.Vec2{})

	flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoScrollbar | imgui.WindowFlagsNoBackground |
		imgui.WindowFlagsNoSavedSettings | imgui.WindowFlagsNoMove

	imgui.BeginV("##FloatBall", nil, flags)
	drawList := imgui.WindowDrawList()

	handleBallDragging()

	if ball.IsExpanded || ball.ExpandAnim > 0 {
		drawExpandedMenu(drawList)
	} else {
		drawSmallBall(drawList)
	}

	imgui.End()
	imgui.PopStyleVar()
	imgui.PopStyleVar()
	imgui.PopStyleColor()
}

// 绘制小球
func drawSmallBall(drawList *imgui.DrawList) {
	pos := imgui.Vec2{X: ball.X, Y: ball.Y}

	// 阴影
	drawList.AddCircleFilled(
		imgui.Vec2{X: pos.X + 3, Y: pos.Y + 3},
		ball.Radius,
		imgui.ColorU32Vec4(imgui.Vec4{X: 0, Y: 0, Z: 0, W: 0.3}),
	)

	// 光晕
	for i := 3; i > 0; i-- {
		glowRadius := ball.Radius + float32(i)*4
		alpha := 0.1 / float32(i)
		var c imgui.Vec4
		if ball.IsRunning {
			c = imgui.Vec4{X: 0.2, Y: 0.8, Z: 0.3, W: alpha}
		} else {
			c = imgui.Vec4{X: 0.2, Y: 0.6, Z: 1.0, W: alpha}
		}
		drawList.AddCircleFilled(pos, glowRadius, imgui.ColorU32Vec4(c))
	}

	// 主球体
	var mainColor imgui.Vec4
	if ball.IsRunning {
		mainColor = imgui.Vec4{X: 0.2, Y: 0.8, Z: 0.3, W: 0.95}
	} else {
		mainColor = imgui.Vec4{X: 0.2, Y: 0.6, Z: 1.0, W: 0.95}
	}
	drawList.AddCircleFilled(pos, ball.Radius, imgui.ColorU32Vec4(mainColor))

	// 高光
	drawList.AddCircleFilled(
		imgui.Vec2{X: pos.X - 10, Y: pos.Y - 10},
		ball.Radius*0.3,
		imgui.ColorU32Vec4(imgui.Vec4{X: 1, Y: 1, Z: 1, W: 0.5}),
	)
}

// 绘制展开菜单
func drawExpandedMenu(drawList *imgui.DrawList) {
	anim := easeOutCubic(ball.ExpandAnim)
	buttonRadius := float32(30)
	spacing := float32(80) * anim

	// 按钮：Logo(收起)、停止/开始
	type btnInfo struct {
		pos      imgui.Vec2
		color    imgui.Vec4
		index    int
		iconText string
	}

	var buttons []btnInfo
	if ball.IsOnRightSide {
		buttons = []btnInfo{
			{imgui.Vec2{X: ball.X - spacing, Y: ball.Y}, imgui.Vec4{X: 0.6, Y: 0.6, Z: 0.6, W: 1.0}, 0, "X"},
			{imgui.Vec2{X: ball.X - spacing*2, Y: ball.Y}, startStopColor(), 1, startStopText()},
		}
	} else {
		buttons = []btnInfo{
			{imgui.Vec2{X: ball.X + spacing, Y: ball.Y}, imgui.Vec4{X: 0.6, Y: 0.6, Z: 0.6, W: 1.0}, 0, "X"},
			{imgui.Vec2{X: ball.X + spacing*2, Y: ball.Y}, startStopColor(), 1, startStopText()},
		}
	}

	// 连接线
	if len(buttons) >= 2 {
		drawList.AddLineV(buttons[0].pos, buttons[1].pos, imgui.ColorU32Vec4(imgui.Vec4{X: 1, Y: 1, Z: 1, W: 0.2 * anim}), 2)
	}

	for _, btn := range buttons {
		// 阴影
		drawList.AddCircleFilled(
			imgui.Vec2{X: btn.pos.X + 2, Y: btn.pos.Y + 2},
			buttonRadius,
			imgui.ColorU32Vec4(imgui.Vec4{X: 0, Y: 0, Z: 0, W: 0.3}),
		)
		// 按钮主体
		drawList.AddCircleFilled(btn.pos, buttonRadius, imgui.ColorU32Vec4(btn.color))
		// 高光
		drawList.AddCircleFilled(
			imgui.Vec2{X: btn.pos.X - 6, Y: btn.pos.Y - 6},
			buttonRadius*0.25,
			imgui.ColorU32Vec4(imgui.Vec4{X: 1, Y: 1, Z: 1, W: 0.3}),
		)
		// 图标
		drawBallButtonIcon(drawList, btn.pos, buttonRadius, btn.index)
		// 点击检测
		if ball.IsExpanded && anim > 0.9 {
			checkBallButtonClick(btn.pos, buttonRadius, btn.index)
		}
	}
}

func startStopColor() imgui.Vec4 {
	if ball.IsRunning {
		return imgui.Vec4{X: 0.9, Y: 0.3, Z: 0.3, W: 1.0}
	}
	return imgui.Vec4{X: 0.3, Y: 0.8, Z: 0.4, W: 1.0}
}

func startStopText() string {
	if ball.IsRunning {
		return "■"
	}
	return "▶"
}

func drawBallButtonIcon(drawList *imgui.DrawList, pos imgui.Vec2, radius float32, index int) {
	iconColor := imgui.ColorU32Vec4(imgui.Vec4{X: 1, Y: 1, Z: 1, W: 0.9})
	size := radius * 0.4

	switch index {
	case 0: // X 关闭
		offset := size * 0.7
		drawList.AddLineV(
			imgui.Vec2{X: pos.X - offset, Y: pos.Y - offset},
			imgui.Vec2{X: pos.X + offset, Y: pos.Y + offset},
			iconColor, 3,
		)
		drawList.AddLineV(
			imgui.Vec2{X: pos.X + offset, Y: pos.Y - offset},
			imgui.Vec2{X: pos.X - offset, Y: pos.Y + offset},
			iconColor, 3,
		)
	case 1: // 开始/停止
		if ball.IsRunning {
			// 停止方形
			drawList.AddRectFilled(
				imgui.Vec2{X: pos.X - size, Y: pos.Y - size},
				imgui.Vec2{X: pos.X + size, Y: pos.Y + size},
				iconColor,
			)
		} else {
			// 播放三角
			drawList.AddTriangleFilled(
				imgui.Vec2{X: pos.X - size*0.5, Y: pos.Y - size},
				imgui.Vec2{X: pos.X - size*0.5, Y: pos.Y + size},
				imgui.Vec2{X: pos.X + size, Y: pos.Y},
				iconColor,
			)
		}
	}
}

// 拖动逻辑
func handleBallDragging() {
	mousePos := imgui.MousePos()

	dx := mousePos.X - ball.X
	dy := mousePos.Y - ball.Y
	distance := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if imgui.IsMouseDown(imgui.MouseButtonLeft) {
		if !ball.IsDragging && !ball.IsExpanded && distance < ball.Radius+10 {
			ball.IsDragging = true
			ball.DragStartX = mousePos.X
			ball.DragStartY = mousePos.Y
			ball.DragOffsetX = mousePos.X - ball.X
			ball.DragOffsetY = mousePos.Y - ball.Y
			// 拖动时取消半隐藏
			ball.IsHalfHidden = false
			ball.HalfHideAnim = 0
		}
		if ball.IsDragging {
			ball.X = mousePos.X - ball.DragOffsetX
			ball.Y = mousePos.Y - ball.DragOffsetY
			// 边界限制（用实时视口尺寸，适配横竖屏切换）
			viewport := imgui.MainViewport()
			workSize := viewport.WorkSize()
			screenW := int(workSize.X)
			screenH := int(workSize.Y)
			if ball.X < ball.Radius {
				ball.X = ball.Radius
			}
			if ball.X > float32(screenW)-ball.Radius {
				ball.X = float32(screenW) - ball.Radius
			}
			if ball.Y < ball.Radius {
				ball.Y = ball.Radius
			}
			if ball.Y > float32(screenH)-ball.Radius {
				ball.Y = float32(screenH) - ball.Radius
			}
		}
	} else if imgui.IsMouseReleased(imgui.MouseButtonLeft) {
		if ball.IsDragging {
			dragDist := float32(math.Sqrt(float64(
				(mousePos.X-ball.DragStartX)*(mousePos.X-ball.DragStartX) +
					(mousePos.Y-ball.DragStartY)*(mousePos.Y-ball.DragStartY))))

			if dragDist < 10 && !ball.IsExpanded {
				// 点击 → 展开菜单
				ball.IsExpanded = true
			} else if dragDist >= 10 {
				// 拖动结束 → 吸附边缘
				startAutoAlign()
			}
			ball.IsDragging = false
		} else if !ball.IsExpanded && distance < ball.Radius+10 {
			// 半隐藏时点击 → 取消半隐藏
			if ball.IsHalfHidden {
				ball.IsHalfHidden = false
				ball.HalfHideAnim = 0
			}
		}
	}
}

// 点击按钮
func checkBallButtonClick(pos imgui.Vec2, radius float32, buttonIndex int) {
	if !imgui.IsMouseReleased(imgui.MouseButtonLeft) {
		return
	}
	mousePos := imgui.MousePos()
	dx := mousePos.X - pos.X
	dy := mousePos.Y - pos.Y
	distance := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if distance < radius {
		ball.IsExpanded = false
		switch buttonIndex {
		case 0: // 关闭 → 停止脚本 + 半隐藏
			if ball.IsRunning {
				fmt.Println("点击关闭，停止脚本")
				ball.IsRunning = false
				isRunning = false
				select {
				case stopChan <- struct{}{}:
				default:
				}
			}
			ball.IsHalfHidden = true
			ball.HalfHideDirLeft = !ball.IsOnRightSide
		case 1: // 开始/停止
			if ball.IsRunning {
				fmt.Println("点击停止")
				ball.IsRunning = false
				isRunning = false
				// 通过 stopChan 通知 gameMain 停止
				select {
				case stopChan <- struct{}{}:
				default:
				}
			} else {
				fmt.Println("点击开始")
				ball.IsRunning = true
				isRunning = true
				// 启动游戏主逻辑
				go 游戏主逻辑(stopChan)
			}
		}
	}
}

// 自动吸附边缘
func startAutoAlign() {
	ball.IsAnimating = true
	ball.AnimProgress = 0
	ball.AnimStartX = ball.X

	if ball.X > float32(ball.ScreenWidth)/2 {
		ball.IsOnRightSide = true
		ball.AnimTargetX = float32(ball.ScreenWidth) - ball.Radius - 10
	} else {
		ball.IsOnRightSide = false
		ball.AnimTargetX = ball.Radius + 10
	}
}

// 更新动画
func updateBallAnimations() {
	// 吸附动画
	if ball.IsAnimating {
		ball.AnimProgress += 0.05
		if ball.AnimProgress >= 1.0 {
			ball.AnimProgress = 1.0
			ball.IsAnimating = false
			ball.X = ball.AnimTargetX
			// 吸附完成后自动半隐藏
			if !ball.IsExpanded {
				ball.IsHalfHidden = true
				ball.HalfHideDirLeft = !ball.IsOnRightSide
			}
		} else {
			t := easeOutCubic(ball.AnimProgress)
			ball.X = ball.AnimStartX + (ball.AnimTargetX-ball.AnimStartX)*t
		}
	}

	// 半隐藏动画
	if ball.IsHalfHidden && ball.HalfHideAnim < 1.0 {
		ball.HalfHideAnim += 0.08
		if ball.HalfHideAnim > 1.0 {
			ball.HalfHideAnim = 1.0
		}
	} else if !ball.IsHalfHidden && ball.HalfHideAnim > 0.0 {
		ball.HalfHideAnim -= 0.12
		if ball.HalfHideAnim < 0.0 {
			ball.HalfHideAnim = 0.0
		}
	}

	// 菜单展开动画
	if ball.IsExpanded {
		if ball.ExpandAnim < 1.0 {
			ball.ExpandAnim += 0.08
			if ball.ExpandAnim > 1.0 {
				ball.ExpandAnim = 1.0
			}
		}
	} else {
		if ball.ExpandAnim > 0.0 {
			ball.ExpandAnim -= 0.08
			if ball.ExpandAnim < 0.0 {
				ball.ExpandAnim = 0.0
			}
		}
	}
}

// 缓动函数
func easeOutCubic(t float32) float32 {
	t = t - 1
	return t*t*t + 1
}
