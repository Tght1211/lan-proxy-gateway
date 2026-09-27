//go:build !darwin

package hotspot

func Detect() Status {
	return Status{Message: "代理 Wi-Fi 引导目前仅支持通过网线上网的 Mac。"}
}
