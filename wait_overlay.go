package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/Dasongzi1366/AutoGo/imgui"
)

// waitState 等待倒计时状态
var waitState struct {
	sync.Mutex
	until time.Time
}

// kickoutState 顶号等待状态
var kickoutState struct {
	sync.Mutex
	until time.Time
}

// 设置等待倒计时 设置等待结束时间
func 设置等待倒计时(duration time.Duration) {
	waitState.Lock()
	defer waitState.Unlock()
	waitState.until = time.Now().Add(duration)
}

// 清除等待倒计时 清除倒计时显示
func 清除等待倒计时() {
	waitState.Lock()
	defer waitState.Unlock()
	waitState.until = time.Time{}
}

// 获取等待倒计时剩余时间
func 获取等待倒计时() (time.Duration, bool) {
	waitState.Lock()
	defer waitState.Unlock()
	if waitState.until.IsZero() {
		return 0, false
	}
	remaining := time.Until(waitState.until)
	if remaining <= 0 {
		return 0, false
	}
	return remaining, true
}

// 设置顶号等待倒计时 设置顶号等待结束时间
func 设置顶号等待倒计时(duration time.Duration) {
	kickoutState.Lock()
	defer kickoutState.Unlock()
	kickoutState.until = time.Now().Add(duration)
}

// 清除顶号等待倒计时 清除顶号倒计时显示
func 清除顶号等待倒计时() {
	kickoutState.Lock()
	defer kickoutState.Unlock()
	kickoutState.until = time.Time{}
}

// 获取顶号等待倒计时剩余时间
func 获取顶号等待倒计时() (time.Duration, bool) {
	kickoutState.Lock()
	defer kickoutState.Unlock()
	if kickoutState.until.IsZero() {
		return 0, false
	}
	remaining := time.Until(kickoutState.until)
	if remaining <= 0 {
		return 0, false
	}
	return remaining, true
}

// drawWaitCountdown 在屏幕上绘制等待倒计时
func drawWaitCountdown() {
	// 优先显示顶号等待
	if remaining, ok := 获取顶号等待倒计时(); ok {
		drawKickoutCountdown(remaining)
		return
	}

	remaining, ok := 获取等待倒计时()
	if !ok {
		return
	}

	drawNormalCountdown(remaining)
}

// drawNormalCountdown 绘制普通等待倒计时
func drawNormalCountdown(remaining time.Duration) {
	text := fmt.Sprintf("等待中: %.1fs", remaining.Seconds())
	drawCountdown(text, remaining, 120,
		imgui.Vec4{X: 0, Y: 0, Z: 0, W: 0.6},
		imgui.Vec4{X: 0.2, Y: 0.8, Z: 0.3, W: 0.9},
	)
}

// drawKickoutCountdown 绘制顶号等待倒计时
func drawKickoutCountdown(remaining time.Duration) {
	minutes := int(remaining.Minutes())
	seconds := int(remaining.Seconds()) % 60
	text := fmt.Sprintf("顶号等待: %02d:%02d 后重登", minutes, seconds)
	drawCountdown(text, remaining, float32(顶号等待时间.Seconds()),
		imgui.Vec4{X: 0, Y: 0, Z: 0, W: 0.7},
		imgui.Vec4{X: 0.9, Y: 0.2, Z: 0.2, W: 0.9},
	)
}

// drawCountdown 通用倒计时绘制
func drawCountdown(text string, remaining time.Duration, maxSeconds float32, bgColor, barColor imgui.Vec4) {
	viewport := imgui.MainViewport()
	workPos := viewport.WorkPos()
	workSize := viewport.WorkSize()

	// 在屏幕顶部居中显示
	textSize := imgui.CalcTextSize(text)
	pos := imgui.Vec2{
		X: workPos.X + workSize.X/2 - textSize.X/2,
		Y: workPos.Y + 30,
	}

	// 进度条宽度
	barWidth := float32(300)
	barHeight := float32(20)
	barX := workPos.X + workSize.X/2 - barWidth/2
	barY := pos.Y + textSize.Y + 5

	// 全屏透明窗口,只画倒计时
	imgui.SetNextWindowPosV(workPos, imgui.CondAlways, imgui.Vec2{})
	imgui.SetNextWindowSizeV(workSize, imgui.CondAlways)

	flags := imgui.WindowFlagsNoTitleBar |
		imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoScrollbar |
		imgui.WindowFlagsNoInputs |
		imgui.WindowFlagsNoBackground |
		imgui.WindowFlagsNoSavedSettings |
		imgui.WindowFlagsNoBringToFrontOnFocus

	imgui.PushStyleColorVec4(imgui.ColWindowBg, imgui.Vec4{X: 0, Y: 0, Z: 0, W: 0})
	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.Vec2{})

	imgui.BeginV("##WaitCountdown", nil, flags)
	drawList := imgui.WindowDrawList()

	// 文字背景
	drawList.AddRectFilledV(
		imgui.Vec2{X: pos.X - 10, Y: pos.Y - 4},
		imgui.Vec2{X: pos.X + textSize.X + 10, Y: pos.Y + textSize.Y + 4},
		imgui.ColorU32Vec4(bgColor), 4, imgui.DrawFlagsNone,
	)

	// 文字
	drawList.AddTextVec2(
		pos,
		imgui.ColorU32Vec4(imgui.Vec4{X: 1, Y: 1, Z: 1, W: 1}),
		text,
	)

	// 进度条背景
	drawList.AddRectFilledV(
		imgui.Vec2{X: barX, Y: barY},
		imgui.Vec2{X: barX + barWidth, Y: barY + barHeight},
		imgui.ColorU32Vec4(imgui.Vec4{X: 0.3, Y: 0.3, Z: 0.3, W: 0.6}), 4, imgui.DrawFlagsNone,
	)

	// 进度条
	progress := float32(remaining.Seconds())
	if progress > maxSeconds {
		progress = maxSeconds
	}
	fillWidth := barWidth * (progress / maxSeconds)
	drawList.AddRectFilledV(
		imgui.Vec2{X: barX, Y: barY},
		imgui.Vec2{X: barX + fillWidth, Y: barY + barHeight},
		imgui.ColorU32Vec4(barColor), 4, imgui.DrawFlagsNone,
	)

	imgui.End()
	imgui.PopStyleVar()
	imgui.PopStyleColor()
}
