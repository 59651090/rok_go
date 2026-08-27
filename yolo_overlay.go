package main

import (
	"fmt"

	"github.com/Dasongzi1366/AutoGo/imgui"
)

// showYoloOverlay 是否显示 YOLO 检测框覆盖层
var showYoloOverlay = false

// 切换Yolo覆盖层显示 开关屏幕上的 YOLO 检测框
func 切换Yolo覆盖层显示() {
	showYoloOverlay = !showYoloOverlay
	fmt.Printf("YOLO 覆盖层: %v\n", showYoloOverlay)
}

// drawYoloOverlay 在屏幕上绘制 YOLO 检测框和标签
func drawYoloOverlay() {
	if !showYoloOverlay {
		return
	}

	viewport := imgui.MainViewport()
	workPos := viewport.WorkPos()
	workSize := viewport.WorkSize()

	imgui.SetNextWindowPosV(workPos, imgui.CondAlways, imgui.Vec2{})
	imgui.SetNextWindowSizeV(workSize, imgui.CondAlways)

	// 透明、无标题栏、无交互、无背景的全屏窗口
	flags := imgui.WindowFlagsNoTitleBar |
		imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoScrollbar |
		imgui.WindowFlagsNoInputs |
		imgui.WindowFlagsNoBackground |
		imgui.WindowFlagsNoSavedSettings |
		imgui.WindowFlagsNoBringToFrontOnFocus

	imgui.PushStyleColorVec4(imgui.ColWindowBg, imgui.Vec4{X: 0, Y: 0, Z: 0, W: 0})
	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.Vec2{})

	imgui.BeginV("##YoloOverlay", nil, flags)
	drawList := imgui.WindowDrawList()

	for _, r := range lastYoloResults {
		if r.Width <= 0 || r.Height <= 0 {
			continue
		}
		x1 := float32(r.X)
		y1 := float32(r.Y)
		x2 := float32(r.X + r.Width)
		y2 := float32(r.Y + r.Height)

		// 框颜色: 根据置信度从红到绿
		color := scoreToColor(r.Score)

		// 画矩形框
		drawList.AddRectV(
			imgui.Vec2{X: x1, Y: y1},
			imgui.Vec2{X: x2, Y: y2},
			imgui.ColorU32Vec4(color), 0, imgui.DrawFlagsNone, 2,
		)

		// 标签文字: 类别名 + 置信度 (用 RawLabel 避免 imgui 默认字体不支持中文)
		text := fmt.Sprintf("%s %.0f%%", r.RawLabel, r.Score*100)
		textPos := imgui.Vec2{X: x1, Y: y1 - 18}
		if textPos.Y < 0 {
			textPos.Y = y1 + 2
		}

		// 文字(带简单描边效果,先画黑字再画白字)
		drawList.AddTextVec2(
			imgui.Vec2{X: textPos.X + 1, Y: textPos.Y + 1},
			imgui.ColorU32Vec4(imgui.Vec4{X: 0, Y: 0, Z: 0, W: 0.8}),
			text,
		)
		drawList.AddTextVec2(
			textPos,
			imgui.ColorU32Vec4(imgui.Vec4{X: 1, Y: 1, Z: 1, W: 1}),
			text,
		)
	}

	imgui.End()
	imgui.PopStyleVar()
	imgui.PopStyleColor()
}

// scoreToColor 根据置信度返回颜色 (0=红, 1=绿)
func scoreToColor(score float64) imgui.Vec4 {
	if score < 0.5 {
		return imgui.Vec4{X: 1, Y: 0.2, Z: 0.2, W: 0.9}
	}
	if score < 0.7 {
		return imgui.Vec4{X: 1, Y: 0.8, Z: 0.2, W: 0.9}
	}
	return imgui.Vec4{X: 0.2, Y: 1, Z: 0.2, W: 0.9}
}
