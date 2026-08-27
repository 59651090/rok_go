package main

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Dasongzi1366/AutoGo/files"
	"github.com/Dasongzi1366/AutoGo/images"
	"github.com/Dasongzi1366/AutoGo/motion"
	"github.com/Dasongzi1366/AutoGo/opencv"
	"github.com/Dasongzi1366/AutoGo/ppocr"
	"github.com/Dasongzi1366/AutoGo/yolo"
)

// ========== 工具模块 ==========
// 提供常用的公用方法，供其他模块调用

// 随机整数 [min, max]
func 随机整数(min, max int) int {
	if min >= max {
		return min
	}
	return rand.Intn(max-min+1) + min
}

// 图组 批量生成图片名拼接字符串，用于找图/找图并点击
//
//	图组("棺材", 1, 10)  => "棺材1.png|棺材2.png|...|棺材10.png"
//	图组("城外", 1, 5, ".jpg")  => "城外1.jpg|城外2.jpg|...|城外5.jpg"
//
// 参数:
//   - prefix: 文件名前缀
//   - start: 起始编号
//   - end: 结束编号（包含）
//   - ext: 可选，后缀名，默认 .png
func 图组(prefix string, start, end int, ext ...string) string {
	suffix := ".png"
	if len(ext) > 0 {
		suffix = ext[0]
	}
	if start > end {
		start, end = end, start
	}
	var sb strings.Builder
	for i := start; i <= end; i++ {
		if i > start {
			sb.WriteByte('|')
		}
		sb.WriteString(prefix)
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(suffix)
	}
	return sb.String()
}

// YoloResult yolo 检测结果
type YoloResult struct {
	X        int
	Y        int
	Width    int
	Height   int
	Label    string // 中文标签
	RawLabel string // 原始英文标签(如 class0),用于屏幕覆盖层显示
	Score    float64
	CenterX  int
	CenterY  int
}

// yoloLabelMap 把 libyolo.so 能解析的英文标签映射回中文
// 顺序与 data.yaml 中的 names 保持一致(class0-class101)
var yoloLabelMap = map[string]string{
	"class0": "国外", "class1": "国内", "class2": "宝石", "class3": "旗子", "class4": "寨子",
	"class5": "宝石确认", "class6": "采集", "class7": "回城", "class8": "创建部队", "class9": "行军",
	"class10": "窗口关闭", "class11": "队列切换", "class12": "帮助", "class13": "国外木材", "class14": "国外石头",
	"class15": "国外粮食", "class16": "国外金币", "class17": "队列识别", "class18": "被采集蓝", "class19": "自己的线",
	"class20": "战争", "class21": "关闭游戏", "class22": "聊天表情", "class23": "国内粮食", "class24": "国内木材",
	"class25": "国内石头", "class26": "国内金币", "class27": "队列蓝1", "class28": "队列蓝2", "class29": "队列蓝3",
	"class30": "队列蓝4", "class31": "队列蓝5", "class32": "队列红1", "class33": "队列红2", "class34": "队列红3",
	"class35": "队列红4", "class36": "队列红5", "class37": "队列黄1", "class38": "队列黄2", "class39": "队列黄3",
	"class40": "队列黄4", "class41": "队列黄5", "class42": "队列采集中", "class43": "队列返回中", "class44": "队列寻路中",
	"class45": "战争标题", "class46": "加入按钮", "class47": "红色集结", "class48": "国外搜索按钮", "class49": "确定",
	"class50": "提示标题", "class51": "左上角返回", "class52": "距离最近", "class53": "相安无事", "class54": "队列蓝1选中",
	"class55": "队列蓝2选中", "class56": "队列蓝3选中", "class57": "队列蓝4选中", "class58": "队列蓝5选中", "class59": "蓝色集结",
	"class60": "使用按钮", "class61": "绿色加号", "class62": "绿色领取", "class63": "1级寨子", "class64": "2级寨子",
	"class65": "3级寨子", "class66": "4级寨子", "class67": "5级寨子", "class68": "6级寨子", "class69": "7级寨子",
	"class70": "8级寨子", "class71": "9级寨子", "class72": "10级寨子", "class73": "11级寨子", "class74": "12级寨子",
	"class75": "13级寨子", "class76": "14级寨子", "class77": "15级寨子", "class78": "活动寨子", "class79": "骑兵集结",
	"class80": "步兵集结", "class81": "选择步兵", "class82": "选择骑兵", "class83": "选择弓兵", "class84": "寨子确认",
	"class85": "是", "class86": "否", "class87": "蓝色搜索", "class88": "提示", "class89": "网络断开",
	"class90": "圣地", "class91": "领土", "class92": "联盟帮助按钮", "class93": "联盟仓库", "class94": "联盟礼物",
	"class95": "联盟商店", "class96": "聊天关闭", "class97": "菜单联盟", "class98": "菜单战役", "class99": "菜单邮件",
	"class100": "特权", "class101": "召回",
}

// 全局 YOLO 实例（只初始化一次）
// 模型版本: v5 或 v8
var yoloEngine *yolo.Yolo

// YoloModelConfig yolo 模型配置
type YoloModelConfig struct {
	Version    string // 模型版本，"v5" 或 "v8"
	ThreadNum  int    // CPU 线程数
	ParamPath  string // 模型参数文件路径，如 "yolo/xxx.param"
	BinPath    string // 模型二进制文件路径，如 "yolo/xxx.bin"
	LabelsPath string // 标签文件路径，如 "yolo/labels.txt"
}

// defaultYoloConfig 默认 yolo 模型配置（按需修改路径）
var defaultYoloConfig = YoloModelConfig{
	Version:    "v8",
	ThreadNum:  4,
	ParamPath:  "last.ncnn.param",
	BinPath:    "last.ncnn.bin",
	LabelsPath: "a.txt",
}

func 初始化Yolo() {
	if yoloEngine != nil {
		return
	}
	cfg := defaultYoloConfig
	// libyolo.so 读取 labels 文件异常,直接把标签内容字符串传进去
	labelsContent := files.Read(files.Path("assets/" + cfg.LabelsPath))
	if labelsContent == "" {
		错误("YOLO 标签文件读取失败")
	}
	// 把换行分隔转成逗号分隔,libyolo.so 可能按逗号解析
	labelsContent = strings.ReplaceAll(labelsContent, "\r\n", ",")
	labelsContent = strings.ReplaceAll(labelsContent, "\n", ",")
	if strings.HasSuffix(labelsContent, ",") {
		labelsContent = labelsContent[:len(labelsContent)-1]
	}
	yoloEngine = yolo.New(
		cfg.Version,
		cfg.ThreadNum,
		files.Path("assets/"+cfg.ParamPath),
		files.Path("assets/"+cfg.BinPath),
		labelsContent,
	)
	if yoloEngine == nil {
		错误("YOLO 初始化失败，请检查模型文件路径")
	} else {
		信息("YOLO 初始化成功")
	}
}

// lastYoloResults 最后一次 YOLO 检测结果,用于屏幕绘制覆盖层
var lastYoloResults []YoloResult

// YOLO检测 yolo 检测，指定屏幕区域
//
// 参数:
//   - x1, y1, x2, y2: 检测区域，默认全屏(0,0,0,0)
//   - minScore: 可选，置信度阈值（默认 0.5）
//
// 返回:
//   - []YoloResult: 检测结果列表
func YOLO检测(x1, y1, x2, y2 int, minScore ...float64) []YoloResult {
	初始化Yolo()
	if yoloEngine == nil {
		错误("YOLO 没有初始化！")
		return nil
	}
	threshold := 0.5
	if len(minScore) > 0 {
		threshold = minScore[0]
	}
	results := yoloEngine.Detect(x1, y1, x2, y2, 0)
	out := 转换Yolo结果(results)
	out = 过滤Yolo结果(out, threshold)
	lastYoloResults = out
	if len(out) == 0 {
		警告("YOLO 未检测到目标")
	} else {
		fmt.Printf("YOLO 检测到 %d 个目标:\n", len(out))
		for i, r := range out {
			fmt.Printf("  [%d] %s (%.2f) 位置:(%d,%d) 大小:%dx%d 中心:(%d,%d)\n",
				i+1, r.Label, r.Score, r.X, r.Y, r.Width, r.Height, r.CenterX, r.CenterY)
		}
	}
	return out
}

// YOLO检测从文件 对已截取的图片做 yolo 检测
//
// 参数:
//   - imgPath: 图片文件路径
//   - minScore: 可选，置信度阈值（默认 0.5）
//
// 返回:
//   - []YoloResult: 检测结果列表
func YOLO检测从文件(imgPath string, minScore ...float64) []YoloResult {
	初始化Yolo()
	if yoloEngine == nil {
		return nil
	}
	threshold := 0.5
	if len(minScore) > 0 {
		threshold = minScore[0]
	}
	results := yoloEngine.DetectFromPath(imgPath)
	out := 转换Yolo结果(results)
	out = 过滤Yolo结果(out, threshold)
	lastYoloResults = out
	return out
}

// 过滤Yolo结果 按置信度过滤结果
func 过滤Yolo结果(results []YoloResult, threshold float64) []YoloResult {
	if threshold <= 0 {
		return results
	}
	filtered := make([]YoloResult, 0, len(results))
	for _, r := range results {
		if r.Score >= threshold {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// YOLO找并点击 用 YOLO 检测指定类别并点击第一个匹配目标
//
// 参数:
//   - label: 类别名,支持中文(如 "国内木材")或英文原始标签(如 "class24")
//   - minScore: 置信度阈值(如 0.6)
//   - delay: 点击后延迟(毫秒)
//   - maxRetry: 可选,最大尝试次数(默认1次)
//
// 返回:
//   - bool: 是否找到并点击
func YOLO找并点击(label string, minScore float64, delay int, maxRetry ...int) bool {
	retry := 1
	if len(maxRetry) > 0 && maxRetry[0] > 0 {
		retry = maxRetry[0]
	}
	for i := 0; i < retry; i++ {
		results := YOLO检测(0, 0, 0, 0, minScore)
		for _, r := range results {
			if r.Label == label || r.RawLabel == label {
				time.Sleep(time.Duration(delay) * time.Millisecond)
				motion.Click(r.CenterX, r.CenterY, 0, 0)
				fmt.Printf("YOLO点击 %s (%.2f) 中心:(%d,%d)\n", r.Label, r.Score, r.CenterX, r.CenterY)
				return true
			}
		}
		if i < retry-1 {
			fmt.Printf("YOLO第%d次未找到 %s,准备重试...\n", i+1, label)
			time.Sleep(300 * time.Millisecond)
		}
	}
	fmt.Printf("YOLO %d 次都没找到: %s\n", retry, label)
	return false
}

// YOLO找并点击区域 在指定区域内检测指定类别并点击第一个匹配目标
//
// 参数:
//   - label: 类别名
//   - minScore: 置信度阈值
//   - delay: 点击后延迟(毫秒)
//   - maxRetry: 可选,最大尝试次数(默认1次)
//   - x1, y1, x2, y2: 检测区域,全屏传(0,0,0,0)
//
// 返回:
//   - bool: 是否找到并点击
func YOLO找并点击区域(label string, minScore float64, delay int, maxRetry, x1, y1, x2, y2 int) bool {
	retry := maxRetry
	if retry <= 0 {
		retry = 1
	}
	for i := 0; i < retry; i++ {
		results := YOLO检测(x1, y1, x2, y2, minScore)
		for _, r := range results {
			if r.Label == label || r.RawLabel == label {
				time.Sleep(time.Duration(delay) * time.Millisecond)
				motion.Click(r.CenterX, r.CenterY, 0, 0)
				fmt.Printf("YOLO点击 %s (%.2f) 中心:(%d,%d)\n", r.Label, r.Score, r.CenterX, r.CenterY)
				return true
			}
		}
		if i < retry-1 {
			fmt.Printf("YOLO第%d次未找到 %s,准备重试...\n", i+1, label)
			time.Sleep(300 * time.Millisecond)
		}
	}
	fmt.Printf("YOLO %d 次都没找到: %s\n", retry, label)
	return false
}

// 转换Yolo结果 将 yolo.Result 转为 YoloResult
func 转换Yolo结果(results []yolo.Result) []YoloResult {
	out := make([]YoloResult, len(results))
	for i, r := range results {
		label := r.Label
		rawLabel := r.Label
		// libyolo.so 对中文 labels 解析异常,用英文标签映射回中文
		if cn, ok := yoloLabelMap[label]; ok {
			label = cn
		}
		out[i] = YoloResult{
			X:        r.X,
			Y:        r.Y,
			Width:    r.Width,
			Height:   r.Height,
			Label:    label,
			RawLabel: rawLabel,
			Score:    r.Score,
			CenterX:  r.CenterX,
			CenterY:  r.CenterY,
		}
	}
	return out
}

// 全局 OCR 实例（只初始化一次）
var ocrEngine = ppocr.New("v5")

func 初始化Ocr() {
	if ocrEngine == nil {
		ocrEngine = ppocr.New("v5")
	}
}

// FindResult 找图结果
type FindResult struct {
	Found  bool
	X      int // 左上角 X
	Y      int // 左上角 Y
	Cx     int // 中心 X
	Cy     int // 中心 Y
	Index  int
	Width  int // 模板图片宽度
	Height int // 模板图片高度
}

// 找图 通用找图方法
//
// 参数:
//   - picPaths: 图片路径列表，多个用"|"分隔，如 "登录_信号.png|16+.png"
//   - sim: 相似度，默认0.6
//   - delta: 偏色，默认"000000"（Go 版 opencv 不使用此参数，保留接口兼容）
//   - dir: 查找方向，默认0（左上→右下）
//   - x1, y1, x2, y2: 查找区域，默认全屏(0,0,0,0)
//
// 返回:
//   - FindResult: 找图结果
func 找图(picPaths string, sim float32, delta string, dir, x1, y1, x2, y2 int) FindResult {
	paths := strings.Split(picPaths, "|")
	for i, path := range paths {
		path = strings.TrimSpace(path)
		// 自动加资源目录前缀
		if !strings.Contains(path, "/") {
			//path = "/data/local/tmp/assets/" + path
			path = files.Path("assets/" + path)
		}
		imgData := files.ReadBytes(path)
		if imgData == nil {
			fmt.Printf("找不到图片: %s\n", path)
			continue
		}
		//fmt.Printf("图片名: %s\n", path)
		fx, fy := opencv.FindImage(x1, y1, x2, y2, &imgData, false, false, sim, 0)
		if fx != -1 && fy != -1 {
			// 获取模板图片尺寸，用于计算中心点
			tplImg := images.ReadFromBytes(imgData)
			tw, th := 0, 0
			if tplImg != nil {
				tw = tplImg.Bounds().Dx()
				th = tplImg.Bounds().Dy()
			}
			cx := fx + tw/2
			cy := fy + th/2
			fmt.Printf("找到图片: %s  坐标: (%d, %d)  中心: (%d, %d)  尺寸: (%d, %d)\n", path, fx, fy, cx, cy, tw, th)
			return FindResult{Found: true, X: fx, Y: fy, Cx: cx, Cy: cy, Index: i, Width: tw, Height: th}
		}
	}
	return FindResult{Found: false, X: -1, Y: -1, Index: -1}
}

// 找图并点击 找图并点击图片中心
//
// 参数:
//   - picPaths: 图片路径列表，多个用"|"分隔
//   - sim: 相似度，默认0.8
//   - delay: 点击后延迟(毫秒)，默认1000
//   - delta: 偏色，默认"000000"
//   - dir: 查找方向，默认0
//   - x1, y1, x2, y2: 查找区域，默认全屏
//
// 返回:
//   - FindResult: 找图结果
func 找图并点击(picPaths string, sim float32, delay int, delta string, dir, x1, y1, x2, y2 int) FindResult {
	result := 找图(picPaths, sim, delta, dir, x1, y1, x2, y2)
	if result.Found {
		motion.Click(result.Cx, result.Cy, 0, 0)
		time.Sleep(time.Duration(delay) * time.Millisecond)
		fmt.Printf("点击图片中心: (%d, %d)  延迟: %dms\n", result.Cx, result.Cy, delay)
	}
	return result
}

// Ocr扩展 OCR识别（扩展版），返回完整结果列表
//
// 参数:
//   - x1, y1, x2, y2: 识别区域
//
// 返回:
//   - []ppocr.Result: 识别结果列表
func Ocr扩展(x1, y1, x2, y2 int) []ppocr.Result {
	初始化Ocr()
	if ocrEngine == nil {
		return nil
	}
	// 截取指定区域
	img := images.CaptureScreen(x1, y1, x2, y2, 0)
	if img == nil {
		return nil
	}
	// 识别文字
	results := ocrEngine.OcrFromImage(img, "")
	return results
}

// Ocr OCR识别（简化版），返回识别到的文字和原始结果列表
//
// 参数:
//   - x1, y1, x2, y2: 识别区域，默认全屏(0,0,0,0)
//
// 返回:
//   - string: 识别到的文字，失败返回空字符串
//   - []ppocr.Result: 原始识别结果列表
func Ocr(x1, y1, x2, y2 int) (string, []ppocr.Result) {
	初始化Ocr()
	if ocrEngine == nil {
		return "", nil
	}
	results := ocrEngine.Ocr(x1, y1, x2, y2, "", 0)
	if len(results) == 0 {
		return "", nil
	}

	//fmt.Printf("Ocr=: %d\n", results)
	// 拼接所有识别到的文字
	var sb strings.Builder
	for _, r := range results {
		sb.WriteString(r.Label)
	}
	text := sb.String()
	if text != "" {
		fmt.Println("OCR:" + text)
	}
	return text, results
}

// QueueInfo 队列信息
type QueueInfo struct {
	Current int
	Total   int
}

// 获取队列 获取队列信息（现队列/总队列）
// 截图指定区域，OCR识别文字，解析 "1/5" 格式
//
// 参数:
//   - x1, y1, x2, y2: 截图区域，默认(1194,130,1269,162)
//
// 返回:
//   - QueueInfo: 队列信息
func 获取队列(x1, y1, x2, y2 int) QueueInfo {
	text := 获取队列文字(x1, y1, x2, y2)
	if text != "" {
		// 尝试匹配 "数字/数字" 格式
		re := regexp.MustCompile(`(\d+)\s*/\s*(\d+)`)
		matches := re.FindStringSubmatch(text)
		if len(matches) == 3 {
			current, _ := strconv.Atoi(matches[1])
			total, _ := strconv.Atoi(matches[2])
			fmt.Printf("队列: %d/%d\n", current, total)
			return QueueInfo{Current: current, Total: total}
		}
		// 如果只有单个数字，当作现队列，总队列未知
		reNum := regexp.MustCompile(`(\d+)`)
		numMatch := reNum.FindStringSubmatch(text)
		if len(numMatch) == 2 {
			num, _ := strconv.Atoi(numMatch[1])
			fmt.Printf("队列(仅数字): %d\n", num)
			return QueueInfo{Current: num, Total: 0}
		}
	}
	return QueueInfo{Current: 0, Total: 5}
}

// 是否满队列 判定是否满队列（现队列 >= 总队列）
//
// 参数:
//   - x1, y1, x2, y2: 截图区域，默认(1194,130,1269,162)
//
// 返回:
//   - isFull: 是否满队列
//   - QueueInfo: 队列信息
func 是否满队列(x1, y1, x2, y2 int) (bool, QueueInfo) {
	info := 获取队列(x1, y1, x2, y2)
	isFull := info.Current >= info.Total
	if isFull {
		fmt.Printf("队列已满: %d/%d\n", info.Current, info.Total)
	} else {
		fmt.Printf("队列未满: %d/%d\n", info.Current, info.Total)
	}
	return isFull, info
}

// 获取队列文字 获取队列区域的原始文字
//
// 参数:
//   - x1, y1, x2, y2: 截图区域，默认(1194,130,1269,162)
//
// 返回:
//   - string: 识别到的文字，失败返回空字符串
func 获取队列文字(x1, y1, x2, y2 int) string {
	result, _ := Ocr(x1, y1, x2, y2)
	if result != "" {
		fmt.Printf("队列区域识别文字: %s\n", result)
		return result
	}
	return ""
}

// 随机等待最大 随机等待（1秒 ~ max毫秒）
//
// 参数:
//   - max: 最大等待毫秒数，默认5000
func 随机等待最大(max int) {
	if max <= 0 {
		max = 5000
	}
	r := rand.Intn(max-1000) + 1000
	time.Sleep(time.Duration(r) * time.Millisecond)
}

// 随机等待 随机等待（min ~ max毫秒）
//
// 参数:
//   - min: 最小等待毫秒数，默认1000
//   - max: 最大等待毫秒数，默认5000
func 随机等待(min, max int) {
	if min <= 0 {
		min = 1000
	}
	if max <= 0 {
		max = 5000
	}
	r := rand.Intn(max-min) + min
	fmt.Printf("等待%d\n", r)

	duration := time.Duration(r) * time.Millisecond
	设置等待倒计时(duration)
	defer 清除等待倒计时()

	// 可被停止信号打断：收到 stopChan 立即返回
	select {
	case <-time.After(duration):
	case <-stopChan:
	}
}

// 按键返回 按键返回
// 按返回键后检测是否出现"退出游戏"提示，如有则再按一次
func 按键返回() {
	motion.KeyAction(motion.KEYCODE_ESCAPE, 0)
	time.Sleep(2 * time.Second)
	result, _ := Ocr(515, 311, 760, 350)
	if result != "" {
		if strings.Contains(result, "退出游戏") {
			fmt.Println("找到退出游戏")
			motion.KeyAction(motion.KEYCODE_ESCAPE, 0)
			time.Sleep(2 * time.Second)
		}
	}
}

// 按键 模拟按键单击
//
// 参数:
//   - code: 按键码，如 motion.KEYCODE_BACK、motion.KEYCODE_HOME 等
//   - delay: 按键后延迟(毫秒)，默认500
func 按键(code, delay int) {
	motion.KeyAction(code, 0)
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	fmt.Printf("按键: %d  延迟: %dms\n", code, delay)
}

// 双指缩放 模拟双指捏合/张开,用于缩放游戏视角
//
// 参数:
//   - centerX, centerY: 缩放中心点坐标(一般取屏幕中心)
//   - distance: 双指初始间距的一半(像素)
//   - scale: 缩放比例,>1 放大(双指张开),<1 缩小(双指捏合)
//   - duration: 动作总时长(毫秒)
func 双指缩放(centerX, centerY int, distance int, scale float64, duration int) {
	if distance <= 0 {
		distance = 150
	}
	if scale <= 0 {
		scale = 1.5
	}
	if duration <= 0 {
		duration = 500
	}

	// 初始两个手指位置(水平方向)
	x1, y1 := centerX-distance, centerY
	x2, y2 := centerX+distance, centerY

	// 目标位置:按 scale 调整间距
	newDist := int(float64(distance) * scale)
	tx1, ty1 := centerX-newDist, centerY
	tx2, ty2 := centerX+newDist, centerY

	steps := 20
	dx1 := (tx1 - x1) / steps
	dy1 := (ty1 - y1) / steps
	dx2 := (tx2 - x2) / steps
	dy2 := (ty2 - y2) / steps
	stepDelay := duration / steps
	if stepDelay <= 0 {
		stepDelay = 20
	}

	fmt.Printf("双指缩放: 中心(%d,%d) 距离%d scale%.2f\n", centerX, centerY, distance, scale)

	// 按下两个手指
	motion.TouchDown(x1, y1, 0, 0)
	time.Sleep(30 * time.Millisecond)
	motion.TouchDown(x2, y2, 1, 0)
	time.Sleep(time.Duration(stepDelay) * time.Millisecond)

	// 同步移动两个手指
	for i := 1; i <= steps; i++ {
		motion.TouchMove(x1+dx1*i, y1+dy1*i, 0, 0)
		motion.TouchMove(x2+dx2*i, y2+dy2*i, 1, 0)
		time.Sleep(time.Duration(stepDelay) * time.Millisecond)
	}

	// 静止一下再抬起,避免系统误判为快速滑动
	time.Sleep(150 * time.Millisecond)
	motion.TouchUp(tx1, ty1, 0, 0)
	time.Sleep(30 * time.Millisecond)
	motion.TouchUp(tx2, ty2, 1, 0)

	time.Sleep(500 * time.Millisecond)
}

// 缩放视角 快捷封装:放大或缩小游戏视角
//
// 参数:
//   - zoomIn: true 放大(拉近距离), false 缩小(拉远距离)
func 缩放视角(zoomIn bool) {
	w, h := windowSize.Width, windowSize.Height
	if w == 0 || h == 0 {
		w, h = 720, 1080
	}
	centerX, centerY := w/2, h/2
	if zoomIn {
		双指缩放(centerX, centerY, 200, 1.25, 500) // 双指张开,200->250,放大一点
	} else {
		双指缩放(centerX, centerY, 250, 0.8, 500) // 双指捏合,250->200,缩小一点
	}
}

// 参数:
//   - x1, y1: 起点坐标
//   - x2, y2: 终点坐标
//   - duration: 滑动总时长(毫秒)
func 无惯性滑动(x1, y1, x2, y2, duration int) {
	steps := 40 // 多分几步，移动更平滑
	dx := (x2 - x1) / steps
	dy := (y2 - y1) / steps
	stepDelay := duration / steps
	if stepDelay <= 0 {
		stepDelay = 15
	}
	motion.TouchDown(x1, y1, 0, 0)
	time.Sleep(time.Duration(stepDelay) * time.Millisecond)
	for i := 1; i <= steps; i++ {
		motion.TouchMove(x1+dx*i, y1+dy*i, 0, 0)
		time.Sleep(time.Duration(stepDelay) * time.Millisecond)
	}
	// 关键：抬起前在终点静止一会儿，消除系统判定的滑动速度
	time.Sleep(150 * time.Millisecond)
	motion.TouchUp(x2, y2, 0, 0)
}

// ScreenShot 截取全屏并保存到文件
//
// 参数:
//   - name: 文件名（不含扩展名），传空则用时间戳
//
// 返回:
//   - string: 保存的文件路径，失败返回空字符串
func ScreenShot(name string) string {
	img := images.CaptureScreen(0, 0, 0, 0, 0)
	if img == nil {
		错误("截图失败")
		return ""
	}
	if name == "" {
		name = time.Now().Format("20060102_150405")
	}
	path := "/sdcard/" + name + ".png"
	if images.Save(img, path, 100) {
		fmt.Printf("截图已保存: %s\n", path)
		return path
	}
	错误("截图保存失败")
	return ""
}

// 点击 点击指定坐标
//
// 参数:
//   - x, y: 点击坐标
//   - delay: 点击后延迟(毫秒)，默认500
func 点击(x, y, delay int) {
	motion.Click(x, y, 0, 0)
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	fmt.Printf("点击: (%d, %d)  延迟: %dms\n", x, y, delay)
}

func 检测多余窗口() {
	result := 找图并点击("liaotian.png", 0.7, 2000, "000000", 0, 0, 0, 0, 0)
	if result.Found {
		fmt.Printf("检测到聊天界面")
	}

}
