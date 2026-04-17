package rules

type Provider struct {
	Name string
	URL  string
}

func NormalProbeRules(target string) []string {
	return []string{
		"DOMAIN,ipwho.is," + target,
		"DOMAIN,api.ip.sb," + target,
	}
}

func AIProxyRules(target string) []string {
	return []string{
		"DOMAIN,checkip.amazonaws.com," + target,
		"DOMAIN-SUFFIX,anthropic.com," + target,
		"DOMAIN-SUFFIX,claude.ai," + target,
		"DOMAIN-SUFFIX,claude.com," + target,
		"DOMAIN-SUFFIX,ping0.cc," + target,
		"DOMAIN-SUFFIX,claudeusercontent.com," + target,
		"DOMAIN-SUFFIX,openai.com," + target,
		"DOMAIN-SUFFIX,chatgpt.com," + target,
		"DOMAIN-SUFFIX,oaistatic.com," + target,
		"PROCESS-NAME,Claude," + target,
		"PROCESS-NAME,Antigravity," + target,
		"PROCESS-NAME,Cursor," + target,
		"DOMAIN-SUFFIX,cursor.sh," + target,
		"DOMAIN-SUFFIX,cursor-cdn.com," + target,
		"DOMAIN-SUFFIX,cursorapi.com," + target,
		"DOMAIN,downloads.cursor.com," + target,
		"DOMAIN,anysphere-binaries.s3.us-east-1.amazonaws.com," + target,
	}
}

func AIProviders() []Provider {
	return []Provider{
		{Name: "OpenAI-Rule", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/OpenAI/OpenAI.yaml"},
		{Name: "Claude-Rule", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Claude/Claude.yaml"},
		{Name: "Gemini-Rule", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Gemini/Gemini.yaml"},
	}
}
