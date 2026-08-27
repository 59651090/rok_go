package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/nfnt/resize"
	_ "modernc.org/sqlite"
)

// ============================================================
// 配置
// ============================================================

var (
	listenAddr      = getEnv("LISTEN_ADDR", ":14742")         // 监听端口
	uploadDir       = getEnv("UPLOAD_DIR", "./uploads")       // 截图保存目录
	dbPath          = getEnv("DB_PATH", "./ironfist.db")      // SQLite 数据库路径
	maxUploadSize   int64 = 20 * 1024 * 1024                  // 单文件最大 20MB
	maxPerPoint     int   = 3                                 // 每个坐标点保留最新几张
	retentionDays   int   = 7                                 // 记录保留天数
	cleanupInterval       = 1 * time.Hour                     // 清理任务运行间隔
	thumbWidth      uint  = 300                               // 缩略图宽度（像素）
)

// getEnv 读环境变量，没设置就用默认值
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ============================================================
// 数据结构
// ============================================================

// 铁手记录 数据库表结构
type IronFistRecord struct {
	ID            int64  `json:"id"`
	X             int    `json:"x"`
	Y             int    `json:"y"`
	Server        string `json:"server"`
	Remark        string `json:"remark"`
	Machine       string `json:"machine"`
	FileName      string `json:"-"`
	ImageURL      string `json:"image_url"`
	ThumbURL      string `json:"thumb_url"`
	RemainTime   string `json:"remain_time"`  // 铁手剩余时间
	CriticalHit  string `json:"critical_hit"` // 致命一击
	CreatedAt     string `json:"created_at"`
}

// API 响应
type ApiResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// 列表响应
type ListResponse struct {
	Total int               `json:"total"`
	List  []IronFistRecord  `json:"list"`
}

// ============================================================
// 数据库初始化
// ============================================================

var db *sql.DB

func initDB() error {
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	// 1. 先建表（不带唯一索引，避免旧数据冲突）
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ironfist (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			x INTEGER NOT NULL DEFAULT 0,
			y INTEGER NOT NULL DEFAULT 0,
			server TEXT NOT NULL DEFAULT '',
			remark TEXT NOT NULL DEFAULT '',
			machine TEXT NOT NULL DEFAULT '',
			file_name TEXT NOT NULL DEFAULT '',
			remain_time TEXT NOT NULL DEFAULT '',
			critical_hit TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_created_at ON ironfist(created_at);
	`)
	if err != nil {
		return err
	}

	// 2. 老版本升级：补字段
	if err := addColumnIfMissing("ironfist", "remain_time", "TEXT NOT NULL DEFAULT ''"); err != nil {
		fmt.Printf("[升级] remain_time 字段处理失败: %v\n", err)
	} else {
		fmt.Println("[升级] remain_time 字段就绪")
	}
	if err := addColumnIfMissing("ironfist", "critical_hit", "TEXT NOT NULL DEFAULT ''"); err != nil {
		fmt.Printf("[升级] critical_hit 字段处理失败: %v\n", err)
	} else {
		fmt.Println("[升级] critical_hit 字段就绪")
	}

	// 3. 清理同一 (server, x, y) 的重复记录，只保留最新的一条
	_, _ = db.Exec(`
		DELETE FROM ironfist
		WHERE id NOT IN (
			SELECT MAX(id) FROM ironfist GROUP BY server, x, y
		)
	`)

	// 4. 加唯一索引（现在数据已经去重了，不会冲突）
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_server_xy ON ironfist(server, x, y)`)
	if err != nil {
		fmt.Printf("[警告] 创建唯一索引失败: %v\n", err)
		// 不返回错误，让服务能启动，只是没有去重保护
	}

	return nil
}

// addColumnIfMissing 检查表字段是否存在，不存在就加
func addColumnIfMissing(table, column, def string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	exists := false
	for rows.Next() {
		var cid int
		var name string
		var rest sql.NullString // 忽略其他列
		if err := rows.Scan(&cid, &name, &rest, &rest, &rest, &rest); err == nil {
			if name == column {
				exists = true
				break
			}
		}
	}
	if exists {
		return nil
	}
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, def))
	return err
}

// ============================================================
// 上传接口 POST /api/ironfist/upload
// ============================================================

func handleUpload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.Method != http.MethodPost {
		writeJSON(w, 405, "method not allowed", nil)
		return
	}

	// 限制上传大小
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeJSON(w, 400, "上传文件过大或解析失败: "+err.Error(), nil)
		return
	}

	// 读取文件
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, 400, "读取文件字段失败: "+err.Error(), nil)
		return
	}
	defer file.Close()

	// 校验后缀
	ext := filepath.Ext(header.Filename)
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		writeJSON(w, 400, "只支持 png/jpg 格式", nil)
		return
	}

	// 读取其他字段
	x, _ := strconv.Atoi(r.FormValue("x"))
	y, _ := strconv.Atoi(r.FormValue("y"))
	server := r.FormValue("server")
	remark := r.FormValue("remark")
	machine := r.FormValue("machine")
	remainTime := r.FormValue("remain_time")
	criticalHit := r.FormValue("critical_hit")

	// 生成文件名: 时间戳_随机.png
	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("%s_%d%s", timestamp, time.Now().UnixNano()%100000, ext)
	savePath := filepath.Join(uploadDir, fileName)

	// 保存文件
	dst, err := os.Create(savePath)
	if err != nil {
		writeJSON(w, 500, "保存文件失败: "+err.Error(), nil)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		writeJSON(w, 500, "写入文件失败: "+err.Error(), nil)
		return
	}

	// 生成缩略图
	_ = generateThumbnail(savePath, fileName)

	// 先查旧记录，准备覆盖时删旧图
	var oldFileName string
	_ = db.QueryRow(
		`SELECT file_name FROM ironfist WHERE server = ? AND x = ? AND y = ?`,
		server, x, y,
	).Scan(&oldFileName)

	// 写入数据库（同一区服+坐标覆盖旧记录）
	res, err := db.Exec(
		`INSERT OR REPLACE INTO ironfist (x, y, server, remark, machine, file_name, remain_time, critical_hit)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		x, y, server, remark, machine, fileName, remainTime, criticalHit,
	)
	if err != nil {
		writeJSON(w, 500, "写入数据库失败: "+err.Error(), nil)
		return
	}

	// 删掉旧截图和缩略图
	if oldFileName != "" && oldFileName != fileName {
		_ = os.Remove(filepath.Join(uploadDir, oldFileName))
		_ = os.Remove(filepath.Join(uploadDir, "thumbs", oldFileName+".jpg"))
	}

	id, _ := res.LastInsertId()
	writeJSON(w, 0, "success", map[string]interface{}{
		"id":  id,
		"url": "/uploads/" + fileName,
	})
}

// ============================================================
// ============================================================
// 缩略图
// ============================================================

// generateThumbnail 生成缩略图，保存到 uploads/thumbs/ 目录
// 失败时返回错误，但不影响主流程
func generateThumbnail(srcPath, fileName string) error {
	thumbDir := filepath.Join(uploadDir, "thumbs")
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return err
	}

	// 打开原图
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	// 等比缩放到 thumbWidth 宽
	thumb := resize.Resize(thumbWidth, 0, img, resize.Lanczos3)

	// 存为 JPEG（体积更小）
	thumbName := fileName + ".jpg"
	thumbPath := filepath.Join(thumbDir, thumbName)
	out, err := os.Create(thumbPath)
	if err != nil {
		return err
	}
	defer out.Close()

	return jpeg.Encode(out, thumb, &jpeg.Options{Quality: 75})
}

// ============================================================
// 列表接口 GET /api/ironfist/list
// ============================================================

func handleList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.Method != http.MethodGet {
		writeJSON(w, 405, "method not allowed", nil)
		return
	}

	// 分页参数
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	server := r.URL.Query().Get("server")
	remark := r.URL.Query().Get("remark")

	// 构建查询
	where := "WHERE 1=1"
	args := []interface{}{}
	if server != "" {
		where += " AND server = ?"
		args = append(args, server)
	}
	if remark != "" {
		where += " AND remark LIKE ?"
		args = append(args, "%"+remark+"%")
	}

	// 查总数
	var total int
	countSQL := "SELECT COUNT(*) FROM ironfist " + where
	if err := db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		writeJSON(w, 500, "查询总数失败: "+err.Error(), nil)
		return
	}

	// 查列表
	offset := (page - 1) * pageSize
	listSQL := `SELECT id, x, y, server, remark, machine, file_name, remain_time, critical_hit, created_at
		FROM ironfist ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)

	rows, err := db.Query(listSQL, args...)
	if err != nil {
		writeJSON(w, 500, "查询列表失败: "+err.Error(), nil)
		return
	}
	defer rows.Close()

	list := []IronFistRecord{}
	for rows.Next() {
		var rec IronFistRecord
		var rawTime string
		if err := rows.Scan(&rec.ID, &rec.X, &rec.Y, &rec.Server, &rec.Remark, &rec.Machine, &rec.FileName, &rec.RemainTime, &rec.CriticalHit, &rawTime); err != nil {
			continue
		}
		rec.CreatedAt = formatBeijingTime(rawTime)
		rec.ImageURL = "/uploads/" + rec.FileName
		rec.ThumbURL = "/uploads/thumbs/" + rec.FileName + ".jpg"
		list = append(list, rec)
	}

	writeJSON(w, 0, "success", ListResponse{
		Total: total,
		List:  list,
	})
}

// ============================================================
// 区服列表接口 GET /api/ironfist/servers
// ============================================================

func handleServers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.Method != http.MethodGet {
		writeJSON(w, 405, "method not allowed", nil)
		return
	}

	rows, err := db.Query(`SELECT DISTINCT server FROM ironfist WHERE server != '' ORDER BY server`)
	if err != nil {
		writeJSON(w, 500, "查询区服失败: "+err.Error(), nil)
		return
	}
	defer rows.Close()

	servers := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err == nil {
			servers = append(servers, s)
		}
	}

	writeJSON(w, 0, "success", servers)
}

// ============================================================
// 查看页面 GET /ironfist
// ============================================================

var pageHTML = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>铁手监控</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; padding: 20px; }
  h1 { font-size: 20px; margin-bottom: 16px; }
  .toolbar { background: #fff; padding: 16px; border-radius: 8px; margin-bottom: 16px; display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
  .toolbar label { font-size: 14px; color: #666; }
  .toolbar input, .toolbar select { padding: 8px 12px; border: 1px solid #ddd; border-radius: 4px; font-size: 14px; background: #fff; }
  .toolbar button { padding: 8px 20px; background: #1890ff; color: #fff; border: none; border-radius: 4px; cursor: pointer; font-size: 14px; }
  .toolbar button:hover { background: #40a9ff; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 16px; }
  .card { background: #fff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 8px rgba(0,0,0,0.06); cursor: pointer; transition: transform .2s; }
  .card:hover { transform: translateY(-2px); box-shadow: 0 4px 16px rgba(0,0,0,0.1); }
  .card img { width: 100%; height: 200px; object-fit: cover; display: block; background: #f0f0f0; }
  .card-info { padding: 12px; font-size: 13px; line-height: 1.6; }
  .card-info .title { font-weight: 600; font-size: 14px; margin-bottom: 4px; color: #1890ff; }
  .card-info .meta { color: #888; }
  .pagination { margin-top: 20px; text-align: center; }
  .pagination button { padding: 6px 14px; margin: 0 4px; border: 1px solid #ddd; background: #fff; border-radius: 4px; cursor: pointer; }
  .pagination button:disabled { opacity: 0.5; cursor: not-allowed; }
  .pagination .active { background: #1890ff; color: #fff; border-color: #1890ff; }
  .pagination span { margin: 0 8px; color: #666; font-size: 14px; }
  /* 大图查看 */
  .modal { display: none; position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.85); z-index: 999; align-items: center; justify-content: center; }
  .modal.show { display: flex; }
  .modal img { max-width: 95%; max-height: 95%; border-radius: 4px; }
  .modal .close { position: absolute; top: 20px; right: 30px; color: #fff; font-size: 32px; cursor: pointer; }
</style>
</head>
<body>
<h1>🗡️ 铁手监控</h1>

<div class="toolbar">
  <label>区服：<select id="server" onchange="loadData(1)"><option value="">全部</option></select></label>
  <label>备注：<input type="text" id="remark" placeholder="模糊搜索"></label>
  <button onclick="loadData(1)">查询</button>
  <button onclick="loadData(currentPage)">刷新</button>
  <label>刷新:
    <select id="refreshInterval" onchange="changeRefresh()">
      <option value="5000">5秒</option>
      <option value="10000" selected>10秒</option>
      <option value="30000">30秒</option>
      <option value="60000">60秒</option>
      <option value="0">关闭</option>
    </select>
  </label>
</div>

<div class="grid" id="grid"></div>

<div class="pagination" id="pagination"></div>

<div class="modal" id="modal" onclick="closeModal()">
  <span class="close">&times;</span>
  <img id="modalImg" src="" alt="">
</div>

<script>
let currentPage = 1;
let totalPages = 1;
const pageSize = 20;

function loadData(page) {
  currentPage = page;
  const server = document.getElementById('server').value;
  const remark = document.getElementById('remark').value;
  let url = '/api/ironfist/list?page=' + page + '&pageSize=' + pageSize;
  if (server) url += '&server=' + encodeURIComponent(server);
  if (remark) url += '&remark=' + encodeURIComponent(remark);

  fetch(url)
    .then(r => r.json())
    .then(res => {
      if (res.code !== 0) { alert('加载失败: ' + res.msg); return; }
      const data = res.data;
      totalPages = Math.ceil(data.total / pageSize);
      renderGrid(data.list);
      renderPagination(data.total);
    });
}

function renderGrid(list) {
  const grid = document.getElementById('grid');
  if (list.length === 0) {
    grid.innerHTML = '<p style="grid-column:1/-1;text-align:center;color:#999;padding:60px;">暂无数据</p>';
    return;
  }
  grid.innerHTML = list.map(item => {
    const coord = '(' + item.x + ', ' + item.y + ')';
    const title = (item.server || '未知区服') + ' - ' + (item.remark || '未命名');
    return '<div class="card" onclick="showModal(\'' + item.image_url + '\')">' +
      '<img src="' + item.thumb_url + '" alt="" loading="lazy">' +
      '<div class="card-info">' +
        '<div class="title">' + escapeHtml(title) + '</div>' +
        '<div class="meta">坐标: ' + coord + '</div>' +
        (item.remain_time ? '<div class="meta">⏰ 剩余: ' + escapeHtml(item.remain_time) + '</div>' : '') +
        (item.critical_hit ? '<div class="meta" style="color:#ff4d4f;">⚔️ 致命: ' + escapeHtml(item.critical_hit) + '</div>' : '') +
        '<div class="meta">时间: ' + item.created_at + '</div>' +
      '</div>' +
    '</div>';
  }).join('');
}

function renderPagination(total) {
  const pg = document.getElementById('pagination');
  if (total === 0) { pg.innerHTML = ''; return; }
  let html = '';
  html += '<button ' + (currentPage <= 1 ? 'disabled' : '') + ' onclick="loadData(' + (currentPage-1) + ')">上一页</button>';
  html += '<span>第 ' + currentPage + ' / ' + totalPages + ' 页 (共 ' + total + ' 条)</span>';
  html += '<button ' + (currentPage >= totalPages ? 'disabled' : '') + ' onclick="loadData(' + (currentPage+1) + ')">下一页</button>';
  pg.innerHTML = html;
}

function showModal(imgUrl) {
  document.getElementById('modalImg').src = imgUrl;
  document.getElementById('modal').classList.add('show');
}

function closeModal() {
  document.getElementById('modal').classList.remove('show');
}

function escapeHtml(s) {
  const div = document.createElement('div');
  div.textContent = s;
  return div.innerHTML;
}

// 加载区服列表
function loadServers() {
  fetch('/api/ironfist/servers')
    .then(r => r.json())
    .then(res => {
      if (res.code !== 0 || !res.data) return;
      const sel = document.getElementById('server');
      const current = sel.value;
      sel.innerHTML = '<option value="">全部</option>';
      res.data.forEach(s => {
        const opt = document.createElement('option');
        opt.value = s;
        opt.textContent = s;
        sel.appendChild(opt);
      });
      sel.value = current;
    });
}

// 自动刷新
let refreshTimer = null;
function changeRefresh() {
  const ms = parseInt(document.getElementById('refreshInterval').value);
  if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null; }
  if (ms > 0) {
    refreshTimer = setInterval(() => {
      loadServers();
      loadData(currentPage);
    }, ms);
  }
}

// 初始加载
loadServers();
loadData(1);
changeRefresh(); // 启动默认刷新间隔
</script>
</body>
</html>
`

func handlePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, err := template.New("page").Parse(pageHTML)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	_ = tmpl.Execute(w, nil)
}

// ============================================================
// 工具函数
// ============================================================

func writeJSON(w http.ResponseWriter, code int, msg string, data interface{}) {
	resp := ApiResponse{Code: code, Msg: msg, Data: data}
	b, _ := json.Marshal(resp)
	w.Write(b)
}

// formatBeijingTime 把各种格式的时间字符串转成北京时间
func formatBeijingTime(raw string) string {
	loc, _ := time.LoadLocation("Asia/Shanghai")

	formats := []string{
		time.RFC3339,              // 2006-01-02T15:04:05Z07:00
		"2006-01-02 15:04:05",     // SQLite DEFAULT
		"2006-01-02T15:04:05Z",    // ISO UTC
		"2006-01-02 15:04:05-0700",
	}

	for _, f := range formats {
		if t, err := time.Parse(f, raw); err == nil {
			return t.In(loc).Format("2006-01-02 15:04:05")
		}
	}
	return raw
}

// ============================================================
// 清理任务
// ============================================================

// cleanupOldRecords 清理过期记录和每个坐标点超出保留数量的记录
// 同时删除对应的截图文件
func cleanupOldRecords() {
	// 1. 清理超过保留天数的记录
	cutoff := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02 15:04:05")

	rows, err := db.Query(`SELECT file_name FROM ironfist WHERE created_at < ?`, cutoff)
	if err != nil {
		fmt.Printf("[清理] 查询过期记录失败: %v\n", err)
	} else {
		var expiredFiles []string
		for rows.Next() {
			var fn string
			if err := rows.Scan(&fn); err == nil && fn != "" {
				expiredFiles = append(expiredFiles, fn)
			}
		}
		rows.Close()

		if _, err := db.Exec(`DELETE FROM ironfist WHERE created_at < ?`, cutoff); err != nil {
			fmt.Printf("[清理] 删除过期记录失败: %v\n", err)
		} else if len(expiredFiles) > 0 {
			fmt.Printf("[清理] 删除过期记录 %d 条\n", len(expiredFiles))
		}

		// 删文件（原图 + 缩略图）
		for _, fn := range expiredFiles {
			_ = os.Remove(filepath.Join(uploadDir, fn))
			_ = os.Remove(filepath.Join(uploadDir, "thumbs", fn+".jpg"))
		}
	}

	// 2. 每个坐标点(按 server + remark + x + y 分组)只保留最新 maxPerPoint 条
	rows, err = db.Query(`
		SELECT file_name FROM ironfist
		WHERE id NOT IN (
			SELECT id FROM ironfist t2
			WHERE t2.server = ironfist.server
			  AND t2.remark = ironfist.remark
			  AND t2.x = ironfist.x
			  AND t2.y = ironfist.y
			ORDER BY t2.id DESC
			LIMIT ?
		)
		ORDER BY id DESC
	`, maxPerPoint)
	if err != nil {
		fmt.Printf("[清理] 查询超额记录失败: %v\n", err)
		return
	}

	var extraFiles []string
	for rows.Next() {
		var fn string
		if err := rows.Scan(&fn); err == nil && fn != "" {
			extraFiles = append(extraFiles, fn)
		}
	}
	rows.Close()

	if len(extraFiles) == 0 {
		return
	}

	// 构建占位符
	placeholders := make([]string, len(extraFiles))
	args := make([]interface{}, len(extraFiles))
	for i, fn := range extraFiles {
		placeholders[i] = "?"
		args[i] = fn
	}

	deleteSQL := `DELETE FROM ironfist WHERE file_name IN (` +
		joinStrings(placeholders, ",") + `)`
	if _, err := db.Exec(deleteSQL, args...); err != nil {
		fmt.Printf("[清理] 删除超额记录失败: %v\n", err)
		return
	}

	for _, fn := range extraFiles {
		_ = os.Remove(filepath.Join(uploadDir, fn))
		_ = os.Remove(filepath.Join(uploadDir, "thumbs", fn+".jpg"))
	}

	fmt.Printf("[清理] 每个坐标点只保留最新 %d 条，删除超额记录 %d 条\n", maxPerPoint, len(extraFiles))
}

// startCleanupLoop 启动后台清理循环
func startCleanupLoop() {
	go func() {
		// 启动时先跑一次
		cleanupOldRecords()

		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			cleanupOldRecords()
		}
	}()
}

// joinStrings 字符串切片拼接（不引入 strings 包的额外依赖，其实有 fmt 也够了）
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for i := 1; i < len(ss); i++ {
		result += sep + ss[i]
	}
	return result
}

// ============================================================
// 主函数
// ============================================================

func main() {
	// 确保上传目录存在
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		fmt.Printf("创建上传目录失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化数据库
	if err := initDB(); err != nil {
		fmt.Printf("初始化数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// 启动后台清理任务
	startCleanupLoop()

	// 路由
	http.HandleFunc("/ironfist", handlePage)
	http.HandleFunc("/api/ironfist/upload", handleUpload)
	http.HandleFunc("/api/ironfist/list", handleList)
	http.HandleFunc("/api/ironfist/servers", handleServers)
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))

	fmt.Printf("🚀 铁手监控服务已启动\n")
	fmt.Printf("📋 查看页面:   http://localhost%s/ironfist\n", listenAddr)
	fmt.Printf("📤 上传接口:   POST http://localhost%s/api/ironfist/upload\n", listenAddr)
	fmt.Printf("📜 列表接口:   GET  http://localhost%s/api/ironfist/list\n", listenAddr)
	fmt.Printf("📁 数据库:     %s\n", dbPath)
	fmt.Printf("🖼️  截图目录:   %s\n", uploadDir)
	fmt.Printf("🧹 每点保留:   最新 %d 条\n", maxPerPoint)
	fmt.Printf("⏳ 保留天数:   %d 天\n", retentionDays)
	fmt.Printf("🔄 清理频率:   每 %v\n", cleanupInterval)

	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		fmt.Printf("服务启动失败: %v\n", err)
		os.Exit(1)
	}
}
