package main

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/Dasongzi1366/AutoGo/files"
	"github.com/Dasongzi1366/AutoGo/images"
	"github.com/Dasongzi1366/AutoGo/ime"
)

// ============================================================
// 铁手监控点配置 —— 后续你填充，这里留好字段
// ============================================================

// 铁手坐标点 描述一个需要监控的铁手点坐标和元信息
type 铁手坐标点 struct {
	X  int    // 地图 X 坐标
	Y  int    // 地图 Y 坐标
	区服 string // 区服标识（如 K1、国服-18服 等）
	备注 string // 备注说明（如 "铁手 1 号点"）
}

// 铁手监控点列表 —— 后续你在这里填充具体坐标
var 铁手监控点 = []铁手坐标点{
	// 示例格式（实际填充时删除示例）：
	// {X: 1234, Y: 5678, 区服: "K1", 备注: "铁手 1 号点"},
	// {X: 2345, Y: 6789, 区服: "K1", 备注: "铁手 2 号点"},
	{X: 134, Y: 673, 区服: "", 备注: "关1"},
	{X: 45, Y: 887, 区服: "", 备注: "关1"},
	{X: 195, Y: 885, 区服: "", 备注: "关1"},
	{X: 45, Y: 1035, 区服: "", 备注: "关1"},
	{X: 105, Y: 1155, 区服: "", 备注: "关1"},
	{X: 315, Y: 1067, 区服: "", 备注: "关1"},
	{X: 285, Y: 1005, 区服: "", 备注: "关1"},
	{X: 316, Y: 915, 区服: "", 备注: "关1"},
}

// 截图上传地址 —— 后续改成你真实的接口
const 铁手上报URL = "http://124.220.39.233:14742/api/ironfist/upload"

// 铁手截图保存目录
const 铁手目录 = "/sdcard"

const 铁手区服 = ""

// ============================================================
// 监控铁手主入口
// ============================================================

// 监控铁手 主流程：遍历所有坐标点，逐个去截图并上传
func 监控铁手() {
	信息("开始监控铁手")

	// 0.
	去城内()
	// 1. 先出城
	if !是否在城外() {
		信息("不在城外，先去城外")
		去城外()
		time.Sleep(2 * time.Second)
	}

	// 没配置坐标点就直接返回
	if len(铁手监控点) == 0 {
		警告("铁手监控点为空，跳过")
		return
	}

	// 2. 遍历所有坐标点
	for idx, point := range 铁手监控点 {
		// 中途被顶号则先处理
		if 检测到顶号则处理() {
			return
		}

		信息f("铁手监控 第%d个: X=%d Y=%d 区服=%s 备注=%s",
			idx+1, point.X, point.Y, point.区服, point.备注)

		// 3. 移动到该坐标
		/* if !移动到坐标(point.X, point.Y) {
			错误f("移动到坐标失败: X=%d Y=%d，跳过", point.X, point.Y)
			continue
		} */

		// 3.

		//拉起输入框
		点击(370, 18, 2000)
		time.Sleep(1 * time.Second)

		点击(624, 140, 2000)
		//输入鼠标 x
		ime.InputText(fmt.Sprintf("%d", point.X))
		//输入完成
		//点击(1024, 355, 2000)

		点击(789, 140, 2000)
		//输入鼠标 y
		ime.InputText(fmt.Sprintf("%d", point.Y))
		//输入完成
		//点击(1024, 355, 2000)

		result := 找图并点击("坐标搜索1.png|坐标搜索2.png|坐标搜索3.png", 0.5, 2000, "000000", 0, 0, 0, 0, windowSize.Width/2)
		if result.Found {
			time.Sleep(5 * time.Second)
			信息("找到坐标搜索")
		} else {
			点击(884, 144, 5000)
			信息("没找到坐标搜索  使用坐标！")
			time.Sleep(5 * time.Second)
		}

		剩余时间 := ""
		致命一击 := ""

		// 寻找棺材，OCR 识别剩余时间和致命一击
		result = 找图并点击(图组("棺材", 1, 11), 0.7, 3000, "000000", 0, 0, 0, 0, 0)
		if result.Found {
			信息("找到棺材")
			time.Sleep(3 * time.Second)

			result = 找图("蓝色集合1.png|蓝色集合2.png", 0.75, "000000", 0, 0, 0, 0, 0)
			if result.Found {
				// OCR 识别区域
				sx1, sy1 := result.Cx-160, result.Cy-120
				sx2, sy2 := sx1+90, sy1+35
				剩余时间, _ = Ocr(sx1, sy1, sx2, sy2)

				mx1, my1 := result.Cx-180, result.Cy-85
				mx2, my2 := result.Cx+200, my1+53
				致命一击, _ = Ocr(mx1, my1, mx2, my2)

				// 信息f("识别结果: 剩余时间=%s 致命一击=%s", 剩余时间, 致命一击)

				// 调试：保存 OCR 区域截图，方便确认范围对不对
				// 调准后可以注释掉
				// 保存区域截图(sx1, sy1, sx2, sy2, fmt.Sprintf("ocr_shengyu_%d_%d", point.X, point.Y))
				// 保存区域截图(mx1, my1, mx2, my2, fmt.Sprintf("ocr_zhiming_%d_%d", point.X, point.Y))
			}
		}

		// 4. 截图
		screenshotPath, imgData := 截图保存(point)
		if len(imgData) == 0 {
			错误("截图失败，跳过")
			continue
		}

		// 5. 上传截图（上传完成后删除本地文件，避免越积越多）
		go func() {
			上传铁手截图(screenshotPath, point, imgData, 剩余时间, 致命一击)
			files.Remove(screenshotPath)
		}()

		time.Sleep(1 * time.Second)
	}

	信息("铁手监控全部完成")
}

// ============================================================
// 截图 & 保存
// ============================================================

// 截图保存 对当前屏幕截图，保存到 /sdcard/ironfist/ 目录
// 返回 文件路径 和 PNG 字节数据，失败返回 ("", nil)
func 截图保存(point 铁手坐标点) (string, []byte) {
	img := images.CaptureScreen(0, 0, 0, 0, 0)
	if img == nil {
		错误("截图失败: CaptureScreen 返回 nil")
		return "", nil
	}

	// 构造文件名: 区服_备注_X_Y_时间戳.png
	dir := 铁手目录
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%d_%d_%s.png",
		point.区服, point.备注, point.X, point.Y, timestamp)
	filepath := dir + "/" + filename

	// 确保目录存在
	files.EnsureDir(dir)

	// 用 images.Save 保存（项目里 ScreenShot 函数也是这么用的）
	if !images.Save(img, filepath, 100) {
		错误f("截图保存失败: %s", filepath)
		return "", nil
	}

	// 读出来用于上传
	imgData := files.ReadBytes(filepath)

	信息f("截图已保存: %s (%d 字节)", filepath, len(imgData))
	return filepath, imgData
}

// 保存区域截图 截取指定区域并保存到 /sdcard/ironfist/ 目录
// 用于调试 OCR 识别范围是否正确
func 保存区域截图(x1, y1, x2, y2 int, name string) {
	img := images.CaptureScreen(x1, y1, x2, y2, 0)
	if img == nil {
		return
	}

	dir := 铁手目录
	files.EnsureDir(dir)
	filepath := dir + "/" + name + ".png"

	if images.Save(img, filepath, 100) {
		信息f("区域截图已保存: %s", filepath)
	} else {
		错误f("区域截图保存失败: %s", filepath)
	}
}

// ============================================================
// 上传截图
// ============================================================

// 上传铁手截图 把截图文件和坐标信息一起上传到服务端
// 异步调用，imgData 是 PNG 字节数据
func 上传铁手截图(filepath string, point 铁手坐标点, imgData []byte, 剩余时间 string, 致命一击 string) {
	信息f("上传铁手截图: %s (%d 字节)", filepath, len(imgData))

	if len(imgData) == 0 {
		错误("上传截图失败: 数据为空")
		return
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 写入文件字段（只用文件名，不用完整路径）
	part, err := writer.CreateFormFile("file", files.GetName(filepath))
	if err != nil {
		错误f("创建表单文件失败: %v", err)
		return
	}
	_, _ = part.Write(imgData)

	// 写入其他字段
	_ = writer.WriteField("x", strconv.Itoa(point.X))
	_ = writer.WriteField("y", strconv.Itoa(point.Y))
	_ = writer.WriteField("server", 铁手区服)
	_ = writer.WriteField("remark", point.备注)
	_ = writer.WriteField("machine", 获取机器码())
	_ = writer.WriteField("remain_time", 剩余时间)
	_ = writer.WriteField("critical_hit", 致命一击)

	_ = writer.Close()

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", 铁手上报URL, body)
	if err != nil {
		错误f("创建上传请求失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		错误f("上传截图失败: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		错误f("上传截图状态码: %d", resp.StatusCode)
	} else {
		信息f("截图上传成功: %s", filepath)
	}
}
