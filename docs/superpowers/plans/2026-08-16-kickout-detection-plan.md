# 顶号检测与等待重登框架实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在脚本主流程中接入顶号检测框架，检测到后进入可配置倒计时等待，结束后自动重新登录游戏。

**Architecture:** 复用现有 `wait_overlay.go` 的倒计时显示能力，新增独立的顶号等待状态；在 `游戏主逻辑()` 的 mode0 主循环中定期调用 `检测顶号()`；检测到后由 `处理顶号()` 完成等待、覆盖层显示和重新登录。

**Tech Stack:** Go, AutoGo (device/images/ocr/imgui), 项目内已有日志上报/等待覆盖层机制。

## Global Constraints

- 检测逻辑由用户后续补充，本次框架中 `检测顶号()` 返回 `false`。
- 顶号等待时间默认 30 分钟，使用 `time.Duration` 类型全局变量，可配置。
- 倒计时期间需监听 `stopChan`，用户点击停止可立即中断。
- 重新登录复用现有 `检查游戏运行()` 和 `进入游戏()`。
- 所有新增函数/变量使用中文命名，与项目现有风格保持一致。
- `category` 固定为 `"info"`，日志级别按语义使用 INFO/WARN/ERROR。

---

## File Structure

| 文件 | 职责 |
|------|------|
| `kickout.go` | 新增：顶号检测函数、处理函数、配置变量 |
| `wait_overlay.go` | 修改：新增顶号等待状态与覆盖层显示逻辑 |
| `main.go` | 修改：在 `游戏主逻辑()` mode0 循环中调用检测 |

---

### Task 1: 新增顶号框架文件 `kickout.go`

**Files:**
- Create: `kickout.go`
- Test: `gofmt -w kickout.go && go vet kickout.go`（`go vet` 可能因 CGO 依赖失败，以 `gofmt` 为准）

**Interfaces:**
- Produces: `var 顶号等待时间 time.Duration`
- Produces: `func 检测顶号() bool`
- Produces: `func 处理顶号(stop <-chan struct{})`
- Consumes: `func 错误(content string)`, `func 信息(content string)` from `log_report.go`
- Consumes: `func 设置顶号等待倒计时(d time.Duration)`, `func 清除顶号等待倒计时()` from `wait_overlay.go`
- Consumes: `func 检查游戏运行()`, `func 进入游戏(stop <-chan struct{})` from `main.go`

- [ ] **Step 1: 创建 `kickout.go` 并写入框架代码**

```go
package main

import "time"

// 顶号等待时间 检测到顶号后的倒计时等待时长,默认 30 分钟
var 顶号等待时间 = 30 * time.Minute

// 检测顶号 返回 true 表示检测到账号被顶
// TODO: 后续补充找图/OCR 匹配逻辑
func 检测顶号() bool {
	return false
}

// 处理顶号 显示倒计时、等待、然后重新登录
func 处理顶号(stop <-chan struct{}) {
	错误("检测到顶号,开始倒计时等待")
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
```

- [ ] **Step 2: 格式化并检查语法**

Run: `gofmt -w kickout.go`
Expected: 无输出（成功）

- [ ] **Step 3: 提交**

```bash
git add kickout.go
git commit -m "feat: add kickout detection framework"
```

---

### Task 2: 扩展等待覆盖层以支持顶号倒计时

**Files:**
- Modify: `wait_overlay.go`
- Test: `gofmt -w wait_overlay.go`

**Interfaces:**
- Produces: `func 设置顶号等待倒计时(duration time.Duration)`
- Produces: `func 清除顶号等待倒计时()`
- Produces: `func 获取顶号等待倒计时() (time.Duration, bool)`
- Modifies: `func drawWaitCountdown()` 增加顶号状态渲染分支
- Consumes: existing `waitState` pattern from `wait_overlay.go`

- [ ] **Step 1: 在 `wait_overlay.go` 顶部新增顶号状态变量**

在 `waitState` 结构体旁边新增：

```go
// kickoutState 顶号等待状态
var kickoutState struct {
	sync.Mutex
	until time.Time
}
```

- [ ] **Step 2: 新增顶号等待状态操作函数**

在 `获取等待倒计时()` 函数之后插入：

```go
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
```

- [ ] **Step 3: 修改 `drawWaitCountdown()` 增加顶号分支**

将函数开头：

```go
func drawWaitCountdown() {
	remaining, ok := 获取等待倒计时()
	if !ok {
		return
	}
```

改为优先判断顶号状态，并新增顶号渲染分支：

```go
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
```

由于原函数体较长，为了不破坏原有普通等待逻辑，建议将原函数体抽取为 `drawNormalCountdown(remaining time.Duration)`，新增 `drawKickoutCountdown(remaining time.Duration)`。

`drawKickoutCountdown` 与 `drawNormalCountdown` 结构相同，区别：
- 文字：`顶号等待: 25:32 后重登`
- 进度条颜色：红色 `imgui.Vec4{X: 0.9, Y: 0.2, Z: 0.2, W: 0.9}`
- 文字背景颜色可略深

- [ ] **Step 4: 格式化并检查语法**

Run: `gofmt -w wait_overlay.go`
Expected: 无输出（成功）

- [ ] **Step 5: 提交**

```bash
git add wait_overlay.go
git commit -m "feat: support kickout countdown overlay"
```

---

### Task 3: 在主流程中接入顶号检测

**Files:**
- Modify: `main.go`
- Test: `gofmt -w main.go`

**Interfaces:**
- Consumes: `func 检测顶号() bool` from `kickout.go`
- Consumes: `func 处理顶号(stop <-chan struct{})` from `kickout.go`

- [ ] **Step 1: 在 mode0 主循环开头调用检测**

找到 `func 游戏主逻辑(stop <-chan struct{})` 中 mode0 的循环：

```go
for {
    select {
    case <-stop:
        信息("脚本已停止")
        return
    default:
        // 检测队列：满了就跑勾选任务，没满就挖
        isFull, _ := 是否满队列(1194, 130, 1269, 162)
```

在循环开头（`select` 之前或 `default` 分支最上方）插入：

```go
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
```

- [ ] **Step 2: 格式化并检查语法**

Run: `gofmt -w main.go`
Expected: 无输出（成功）

- [ ] **Step 3: 提交**

```bash
git add main.go
git commit -m "feat: wire kickout detection into main loop"
```

---

### Task 4: 全量格式化与最终验证

**Files:**
- Modify: 无（仅验证）
- Test: 项目级 `gofmt`

- [ ] **Step 1: 格式化所有修改过的文件**

Run:
```bash
gofmt -w main.go wait_overlay.go kickout.go log_report.go machine_id.go
```
Expected: 无输出（成功）

- [ ] **Step 2: 检查关键调用是否就位**

Run:
```bash
grep -n "检测顶号\|处理顶号\|设置顶号等待倒计时\|drawKickoutCountdown" main.go wait_overlay.go kickout.go
```
Expected: 在 `kickout.go`、`wait_overlay.go`、`main.go` 中均能看到对应调用。

- [ ] **Step 3: 提交**

```bash
git add -A
git commit -m "style: format kickout-related files"
```

---

## Self-Review

1. **Spec coverage:**
   - `检测顶号()` 占位返回 `false` → Task 1 ✓
   - 30 分钟可配置等待 → Task 1 (`var 顶号等待时间`) ✓
   - 倒计时覆盖层 → Task 2 ✓
   - 主流程接入 → Task 3 ✓
   - 等待后重新登录 → Task 1 (`处理顶号()` 调用 `检查游戏运行()` + `进入游戏()`) ✓

2. **Placeholder scan:** 无 TBD/TODO（除用户明确要求的 `检测顶号()` 内部 TODO 外）。

3. **Type consistency:**
   - `处理顶号(stop <-chan struct{})` 与 `进入游戏(stop <-chan struct{})` 签名一致。
   - `设置顶号等待倒计时(duration time.Duration)` 与 `顶号等待时间` 类型一致。

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-08-16-kickout-detection-plan.md`. Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
