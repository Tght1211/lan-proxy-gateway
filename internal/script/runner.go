// Package script runs a user-provided JavaScript "enhancer" over the mihomo
// config YAML, same shape as Clash Verge Rev 的 "Script" feature:
//
//	function main(config) { /* mutate */ return config; }
//
// 调用 Apply(scriptPath, configYAML) 得到被 JS 修改过后的 YAML。
// 失败返回原 YAML 前的 error，不污染 input。
//
// 实现思路：
//  1. YAML → Go generic value → JSON（避免 Go↔JS 复杂类型映射）
//  2. 在 goja VM 里 JSON.parse 成 JS 对象
//  3. 调 main(config)
//  4. JSON.stringify 结果 → Go generic value → YAML
//
// 沙箱：脚本即代码执行（goja 跑任意 JS，脚本路径来自配置）。为防止恶意/写错的
// 脚本把渲染整条管线挂死或 panic 掉整个进程，执行受两道保护：
//   - 超时中断：超过 defaultScriptTimeout 用 goja.Interrupt 强制中止死循环。
//   - panic 恢复：goja 内部或类型断言 panic 被 recover 成普通 error，不杀进程。
package script

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dop251/goja"
	"gopkg.in/yaml.v3"
)

// defaultScriptTimeout 是增强脚本单次执行的墙钟上限。脚本在 render 阶段同步跑，
// 正常应在毫秒级完成；给到秒级足够宽松，又能兜住死循环。
const defaultScriptTimeout = 5 * time.Second

// Apply reads the JS script, runs main(config) against configYAML, returns
// the modified YAML. The script must export a `main(config) -> config` function.
func Apply(scriptPath string, configYAML []byte) ([]byte, error) {
	return applyWithTimeout(scriptPath, configYAML, defaultScriptTimeout)
}

// applyWithTimeout 是带超时/recover 的实现，timeout 可注入便于测试。
func applyWithTimeout(scriptPath string, configYAML []byte, timeout time.Duration) (out []byte, err error) {
	scriptCode, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("读取脚本失败: %w", err)
	}

	// YAML → interface{} → JSON
	var configObj interface{}
	if err := yaml.Unmarshal(configYAML, &configObj); err != nil {
		return nil, fmt.Errorf("解析 config YAML 失败: %w", err)
	}
	jsonBytes, err := json.Marshal(configObj)
	if err != nil {
		return nil, fmt.Errorf("config → JSON 失败: %w", err)
	}

	// panic 恢复：任何 goja 内部/类型断言 panic 转成 error，绝不杀进程。
	defer func() {
		if r := recover(); r != nil {
			out = nil
			err = fmt.Errorf("脚本执行 panic（已隔离，不影响网关）: %v", r)
		}
	}()

	vm := goja.New()
	// 超时看门狗：到点用 Interrupt 让正在跑的 JS 抛出 *InterruptedError，
	// 死循环会就此中止并从下面的调用处返回 error。
	timer := time.AfterFunc(timeout, func() {
		vm.Interrupt(fmt.Sprintf("脚本执行超时（>%s），疑似死循环已中止", timeout))
	})
	defer timer.Stop()

	if _, err := vm.RunScript(scriptPath, string(scriptCode)); err != nil {
		return nil, fmt.Errorf("脚本加载失败: %w", err)
	}

	mainFn, ok := goja.AssertFunction(vm.Get("main"))
	if !ok {
		return nil, fmt.Errorf("脚本里找不到 main 函数（签名应为 function main(config) { return config; }）")
	}

	// 在 JS 里 JSON.parse 出 config，避开 Go→JS 的深层类型转换坑
	parseFn, _ := goja.AssertFunction(vm.Get("JSON").ToObject(vm).Get("parse"))
	configVal, err := parseFn(goja.Undefined(), vm.ToValue(string(jsonBytes)))
	if err != nil {
		return nil, fmt.Errorf("JSON.parse 失败: %w", err)
	}

	result, err := mainFn(goja.Undefined(), configVal)
	if err != nil {
		return nil, fmt.Errorf("脚本 main() 报错: %w", err)
	}

	stringifyFn, _ := goja.AssertFunction(vm.Get("JSON").ToObject(vm).Get("stringify"))
	jsonResult, err := stringifyFn(goja.Undefined(), result)
	if err != nil {
		return nil, fmt.Errorf("JSON.stringify 失败: %w", err)
	}

	var resultObj interface{}
	if err := json.Unmarshal([]byte(jsonResult.String()), &resultObj); err != nil {
		return nil, fmt.Errorf("解析脚本返回值失败: %w", err)
	}
	output, err := yaml.Marshal(resultObj)
	if err != nil {
		return nil, fmt.Errorf("生成 YAML 失败: %w", err)
	}
	return output, nil
}
