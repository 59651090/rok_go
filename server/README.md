# 铁手监控服务端

## 功能

- 接收设备端上传的铁手截图（坐标 + 区服 + 备注 + 机器码）
- 截图存在本地 `uploads/` 目录
- 数据存在 SQLite 数据库 `ironfist.db`
- 自带 Web 查看页面，按区服/备注筛选，点击放大，60 秒自动刷新

## 接口

### 上传截图
```
POST /api/ironfist/upload
Content-Type: multipart/form-data

字段:
  file     - 截图文件 (png/jpg)
  x        - X 坐标
  y        - Y 坐标
  server   - 区服
  remark   - 备注
  machine  - 机器码
```

响应:
```json
{"code":0,"msg":"success","data":{"id":1,"url":"/uploads/xxx.png"}}
```

### 查询列表
```
GET /api/ironfist/list?page=1&pageSize=20&server=K1&remark=铁手
```

### 查看页面
```
GET /ironfist
```

## 本地运行

```bash
cd server
go mod tidy
go run main.go
```

浏览器打开: http://localhost:14742/ironfist

## 部署到服务器

### 方式一：直接编译运行
```bash
# 在服务器上
GOOS=linux GOARCH=amd64 go build -o ironfist-server main.go
chmod +x ironfist-server
./ironfist-server
```

### 方式二：systemd 守护进程
```ini
# /etc/systemd/system/ironfist.service
[Unit]
Description=IronFist Server
After=network.target

[Service]
Type=simple
WorkingDirectory=/root/ironfist
ExecStart=/root/ironfist/ironfist-server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable ironfist
systemctl start ironfist
```

### 方式三：Nginx 反代（推荐，加域名和 HTTPS）
```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://127.0.0.1:14742;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## 配置修改

编辑 `main.go` 顶部常量：
- `listenAddr` - 监听端口，默认 `:14742`
- `uploadDir` - 截图保存目录，默认 `./uploads`
- `dbPath` - SQLite 数据库路径，默认 `./ironfist.db`
- `maxUploadSize` - 单文件最大体积，默认 20MB

## 客户端（AutoGo 脚本）对应配置

在 `iron_fist.go` 里修改：
```go
const 铁手上报URL = "http://你的域名或IP:14742/api/ironfist/upload"
```
