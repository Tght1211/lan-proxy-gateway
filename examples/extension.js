// lan-proxy-gateway 扩展脚本示例
// 同时兼容 Clash Verge Rev 扩展脚本格式
//
// 功能：住宅代理链式出口 + AI 服务分流（与内置 residential_chain 功能等价）
//
// 什么时候用这个脚本，而不是 gateway.yaml 的 residential_chain？
//   1. 你同时使用 Clash Verge Rev，希望一份脚本两边通用
//   2. 你需要在链式代理基础上追加更多自定义逻辑（residential_chain 满足不了）
//   否则直接用 residential_chain 配置项即可，更简单，无需维护 JS 文件。
//
// 使用方法（gateway）：
//   1. 复制此文件到安全位置，如 /etc/gateway/script.js
//   2. 修改下方【用户配置区】的变量
//   3. 在 gateway.yaml 中设置：
//        extension:
//          mode: script
//          script_path: /etc/gateway/script.js
//   4. extension.residential_chain 配置可以保留，但当前不会生效
//   5. 重启网关：sudo gateway restart
//
// 使用方法（Clash Verge Rev）：
//   1. 修改下方变量
//   2. 将整个文件内容粘贴到 Clash Verge Rev → 订阅 → 脚本 中

function main(config) {
  // =============================================
  // ★★★ 用户配置区 - 只需要改这里 ★★★
  // =============================================
  const PROXY_SERVER = "your-proxy-server.example.com"; // 住宅代理 IP 或域名
  const PROXY_PORT = 443; // 住宅代理端口
  const PROXY_USERNAME = "your-username"; // 用户名（无需认证则留空）
  const PROXY_PASSWORD = "your-password"; // 密码（无需认证则留空）
  const PROXY_TYPE = "socks5"; // socks5 / http
  const AIRPORT_GROUP = "Auto"; // 机场代理组名，如 "自动选择"
  // =============================================

  const residentialProxy = {
    name: "Residential-Proxy",
    type: PROXY_TYPE,
    server: PROXY_SERVER,
    port: PROXY_PORT,
    username: PROXY_USERNAME,
    password: PROXY_PASSWORD,
    "dialer-proxy": AIRPORT_GROUP,
    udp: false,
    "skip-cert-verify": true,
  };

  if (!config.proxies) config.proxies = [];
  config.proxies.push(residentialProxy);

  if (!config["proxy-groups"]) config["proxy-groups"] = [];
  config["proxy-groups"].unshift({
    name: "AI Only",
    type: "select",
    proxies: ["Residential-Proxy", "DIRECT", AIRPORT_GROUP],
  });

  const priorityRules = [
    // --- 【IP 验证及 AI 域名走住宅代理】 ---
    "DOMAIN,checkip.amazonaws.com,AI Only",
    "DOMAIN,ipwho.is,AI Only",
    "DOMAIN,ping0.cc,AI Only",
    "DOMAIN-SUFFIX,anthropic.com,AI Only",
    "DOMAIN-SUFFIX,claude.ai,AI Only",
    "DOMAIN-SUFFIX,claude.com,AI Only",
    "DOMAIN-SUFFIX,claudeusercontent.com,AI Only",
    "DOMAIN-SUFFIX,openai.com,AI Only",
    "DOMAIN-SUFFIX,chatgpt.com,AI Only",
    "DOMAIN-SUFFIX,oaistatic.com,AI Only",
    "PROCESS-NAME,Claude,AI Only",
    "PROCESS-NAME,Antigravity,AI Only",

    // --- 【Cursor IDE 走住宅代理】 ---
    "PROCESS-NAME,Cursor,AI Only",
    "DOMAIN-SUFFIX,cursor.sh,AI Only",
    "DOMAIN-SUFFIX,cursor-cdn.com,AI Only",
    "DOMAIN-SUFFIX,cursorapi.com,AI Only",
    "DOMAIN,downloads.cursor.com,AI Only",
    "DOMAIN,anysphere-binaries.s3.us-east-1.amazonaws.com,AI Only",
  ];

  if (!config["rule-providers"]) config["rule-providers"] = {};

  const aiProviders = {
    "OpenAI-Rule":
      "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/OpenAI/OpenAI.yaml",
    "Claude-Rule":
      "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Claude/Claude.yaml",
    "Gemini-Rule":
      "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Gemini/Gemini.yaml",
  };

  for (const [name, url] of Object.entries(aiProviders)) {
    config["rule-providers"][name] = {
      type: "http",
      behavior: "domain",
      url: url,
      path: `./ruleset/${name}.yaml`,
      interval: 86400,
    };
    priorityRules.push(`RULE-SET,${name},AI Only`);
  }

  if (!config.rules) config.rules = [];
  config.rules = [...priorityRules, ...config.rules];

  return config;
}
