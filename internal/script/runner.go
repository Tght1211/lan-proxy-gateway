package script

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dop251/goja"
	"gopkg.in/yaml.v3"
)

// Apply reads the JS script at scriptPath, runs main(config) on the given
// YAML content, and returns the modified YAML bytes.
//
// The script must export a function with the signature:
//
//	function main(config) { ...; return config; }
//
// This is the same format used by Clash Verge Rev extension scripts.
func Apply(scriptPath string, yamlContent []byte) ([]byte, error) {
	scriptCode, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("读取脚本文件失败: %w", err)
	}

	// YAML → generic Go value → JSON
	var configObj interface{}
	if err := yaml.Unmarshal(yamlContent, &configObj); err != nil {
		return nil, fmt.Errorf("解析配置 YAML 失败: %w", err)
	}
	jsonBytes, err := json.Marshal(configObj)
	if err != nil {
		return nil, fmt.Errorf("配置转换为 JSON 失败: %w", err)
	}

	// Run in goja
	vm := goja.New()
	if _, err := vm.RunScript(scriptPath, string(scriptCode)); err != nil {
		return nil, fmt.Errorf("脚本执行失败: %w", err)
	}

	mainFn, ok := goja.AssertFunction(vm.Get("main"))
	if !ok {
		return nil, fmt.Errorf("脚本中未找到 main 函数")
	}

	// Parse JSON config inside JS engine (avoids Go↔JS type mapping issues)
	jsonStr := vm.ToValue(string(jsonBytes))
	parseFn, _ := goja.AssertFunction(vm.Get("JSON").ToObject(vm).Get("parse"))
	configVal, err := parseFn(goja.Undefined(), jsonStr)
	if err != nil {
		return nil, fmt.Errorf("JSON.parse 失败: %w", err)
	}

	// Call main(config)
	result, err := mainFn(goja.Undefined(), configVal)
	if err != nil {
		return nil, fmt.Errorf("脚本 main() 执行失败: %w", err)
	}

	// Stringify result back to JSON
	stringifyFn, _ := goja.AssertFunction(vm.Get("JSON").ToObject(vm).Get("stringify"))
	jsonResult, err := stringifyFn(goja.Undefined(), result)
	if err != nil {
		return nil, fmt.Errorf("JSON.stringify 失败: %w", err)
	}

	// JSON → Go map → YAML
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
