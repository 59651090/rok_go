package main

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/Dasongzi1366/AutoGo/device"
	"github.com/Dasongzi1366/AutoGo/files"
)

// machineIDFile 本地持久化机器码的备用文件
const machineIDFile = "/sdcard/rokGo_machine_id.txt"

// cachedMachineCode 缓存,避免每次重复读取设备信息
var cachedMachineCode string

// 获取机器码 返回一个稳定的设备标识
//
// 优先级:
//  1. Android ID(最稳定,推荐)
//  2. IMEI
//  3. 硬件 Serial
//  4. WiFi MAC
//  5. 本地生成并持久化的 UUID(兜底)
func 获取机器码() string {
	if cachedMachineCode != "" {
		return cachedMachineCode
	}

	var code string
	switch {
	case device.GetAndroidId() != "":
		code = device.GetAndroidId()
	case device.GetImei() != "":
		code = device.GetImei()
	case device.Serial != "":
		code = device.Serial
	case device.GetWifiMac() != "":
		code = strings.ReplaceAll(device.GetWifiMac(), ":", "")
	default:
		code = getOrCreateFallbackID()
	}

	cachedMachineCode = code
	return code
}

// 初始化机器码 自动获取机器码并设置到日志上报的 machine 字段
func 初始化机器码() {
	设置机器标识(获取机器码())
}

// getOrCreateFallbackID 生成一个本地持久化的 UUID 作为兜底机器码
func getOrCreateFallbackID() string {
	if files.Exists(machineIDFile) {
		if id := files.Read(machineIDFile); id != "" {
			return id
		}
	}
	id := generateUUID()
	files.Write(machineIDFile, id)
	return id
}

// generateUUID 生成一个简化版 UUID(v4)
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// variant 和 version 位,符合 UUID v4
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
