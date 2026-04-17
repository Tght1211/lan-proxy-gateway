package rules

import (
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

type section struct {
	title string
	rules []string
}

func Render(cfg *config.Config) string {
	var sections []section

	if cfg.Rules.NintendoProxyEnabled() {
		sections = append(sections, section{title: "Nintendo / 游戏主机走代理", rules: nintendoProxyRules})
	}

	sections = append(sections, section{title: "安全浏览直连", rules: safeDirectRules})

	if cfg.Rules.AppleRulesEnabled() {
		sections = append(sections,
			section{title: "Apple 开发与验证走代理", rules: appleProxyRules},
			section{title: "Apple 日常服务直连", rules: appleDirectRules},
		)
	}

	if cfg.Rules.ChinaDirectEnabled() {
		sections = append(sections,
			section{title: "国内服务直连", rules: chinaDirectRules},
			section{title: "微信 / QQ / 腾讯生态直连", rules: tencentDirectRules},
			section{title: "小红书 / 抖音 / 头条生态直连", rules: bytedanceDirectRules},
			section{title: "国内游戏与视频生态直连", rules: chinaEntertainmentDirectRules},
		)
	}

	if cfg.Rules.AdsRejectEnabled() {
		sections = append(sections, section{title: "明显广告与跟踪拦截", rules: adRejectRules})
	}

	if cfg.Rules.GlobalProxyEnabled() {
		sections = append(sections,
			section{title: "国外常见网站走代理", rules: globalProxyRules},
			section{title: "Telegram IP 段走代理", rules: telegramIPRules},
		)
	}

	if cfg.Rules.LanDirectEnabled() {
		sections = append(sections,
			section{title: "局域网与保留地址直连", rules: lanDirectRules},
			section{title: "中国地区直连", rules: chinaGeoRules},
		)
	}

	if len(cfg.Rules.ExtraDirectRules) > 0 {
		sections = append(sections, section{title: "自定义直连规则", rules: cfg.Rules.ExtraDirectRules})
	}
	if len(cfg.Rules.ExtraRejectRules) > 0 {
		sections = append(sections, section{title: "自定义拦截规则", rules: cfg.Rules.ExtraRejectRules})
	}
	if len(cfg.Rules.ExtraProxyRules) > 0 {
		sections = append(sections, section{title: "自定义代理规则", rules: cfg.Rules.ExtraProxyRules})
	}

	sections = append(sections, section{title: "兜底规则", rules: []string{"MATCH,Proxy"}})

	var b strings.Builder
	b.WriteString("rules:\n")

	seen := make(map[string]struct{})
	for _, sec := range sections {
		writtenHeader := false
		for _, rule := range sec.rules {
			rule = strings.TrimSpace(rule)
			if rule == "" {
				continue
			}
			if _, ok := seen[rule]; ok {
				continue
			}
			seen[rule] = struct{}{}
			if !writtenHeader {
				b.WriteString("  # --- " + sec.title + " ---\n")
				writtenHeader = true
			}
			b.WriteString("  - " + rule + "\n")
		}
		if writtenHeader {
			b.WriteString("\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

var safeDirectRules = []string{
	"DOMAIN,safebrowsing.urlsec.qq.com,DIRECT",
	"DOMAIN,safebrowsing.googleapis.com,DIRECT",
}

var nintendoProxyRules = []string{
	"DOMAIN-SUFFIX,nintendo.net,Proxy",
	"DOMAIN-SUFFIX,nintendo.com,Proxy",
	"DOMAIN-SUFFIX,nintendowifi.net,Proxy",
	"DOMAIN-SUFFIX,nintendo-europe.com,Proxy",
	"DOMAIN-SUFFIX,nintendoservicecentre.co.uk,Proxy",
	"DOMAIN-SUFFIX,services.googleapis.cn,Proxy",
	"DOMAIN-SUFFIX,xn--ngstr-lra8j.com,Proxy",
}

var appleProxyRules = []string{
	"DOMAIN,developer.apple.com,Proxy",
	"DOMAIN-SUFFIX,digicert.com,Proxy",
	"DOMAIN,ocsp.apple.com,Proxy",
	"DOMAIN,ocsp.comodoca.com,Proxy",
	"DOMAIN,ocsp.usertrust.com,Proxy",
	"DOMAIN,ocsp.sectigo.com,Proxy",
	"DOMAIN,ocsp.verisign.net,Proxy",
	"DOMAIN-SUFFIX,apple-dns.net,Proxy",
	"DOMAIN,testflight.apple.com,Proxy",
	"DOMAIN,sandbox.itunes.apple.com,Proxy",
	"DOMAIN,itunes.apple.com,Proxy",
	"DOMAIN-SUFFIX,apps.apple.com,Proxy",
	"DOMAIN-SUFFIX,blobstore.apple.com,Proxy",
	"DOMAIN,cvws.icloud-content.com,Proxy",
}

var appleDirectRules = []string{
	"DOMAIN-SUFFIX,mzstatic.com,DIRECT",
	"DOMAIN-SUFFIX,itunes.apple.com,DIRECT",
	"DOMAIN-SUFFIX,icloud.com,DIRECT",
	"DOMAIN-SUFFIX,icloud-content.com,DIRECT",
	"DOMAIN-SUFFIX,me.com,DIRECT",
	"DOMAIN-SUFFIX,aaplimg.com,DIRECT",
	"DOMAIN-SUFFIX,cdn20.com,DIRECT",
	"DOMAIN-SUFFIX,cdn-apple.com,DIRECT",
	"DOMAIN-SUFFIX,akadns.net,DIRECT",
	"DOMAIN-SUFFIX,akamaiedge.net,DIRECT",
	"DOMAIN-SUFFIX,edgekey.net,DIRECT",
	"DOMAIN-SUFFIX,mwcloudcdn.com,DIRECT",
	"DOMAIN-SUFFIX,mwcname.com,DIRECT",
	"DOMAIN-SUFFIX,apple.com,DIRECT",
	"DOMAIN-SUFFIX,apple-cloudkit.com,DIRECT",
	"DOMAIN-SUFFIX,apple-mapkit.com,DIRECT",
}

var chinaDirectRules = []string{
	"DOMAIN-SUFFIX,126.com,DIRECT",
	"DOMAIN-SUFFIX,126.net,DIRECT",
	"DOMAIN-SUFFIX,127.net,DIRECT",
	"DOMAIN-SUFFIX,163.com,DIRECT",
	"DOMAIN-SUFFIX,360buyimg.com,DIRECT",
	"DOMAIN-SUFFIX,36kr.com,DIRECT",
	"DOMAIN-SUFFIX,acfun.tv,DIRECT",
	"DOMAIN-SUFFIX,aixifan.com,DIRECT",
	"DOMAIN-KEYWORD,alicdn,DIRECT",
	"DOMAIN-KEYWORD,alipay,DIRECT",
	"DOMAIN-KEYWORD,taobao,DIRECT",
	"DOMAIN-SUFFIX,amap.com,DIRECT",
	"DOMAIN-SUFFIX,autonavi.com,DIRECT",
	"DOMAIN-KEYWORD,baidu,DIRECT",
	"DOMAIN-SUFFIX,bdimg.com,DIRECT",
	"DOMAIN-SUFFIX,bdstatic.com,DIRECT",
	"DOMAIN-SUFFIX,bilibili.com,DIRECT",
	"DOMAIN-SUFFIX,bilivideo.com,DIRECT",
	"DOMAIN-SUFFIX,caiyunapp.com,DIRECT",
	"DOMAIN-SUFFIX,clouddn.com,DIRECT",
	"DOMAIN-SUFFIX,cnbeta.com,DIRECT",
	"DOMAIN-SUFFIX,cnbetacdn.com,DIRECT",
	"DOMAIN-SUFFIX,cootekservice.com,DIRECT",
	"DOMAIN-SUFFIX,csdn.net,DIRECT",
	"DOMAIN-SUFFIX,ctrip.com,DIRECT",
	"DOMAIN-SUFFIX,dgtle.com,DIRECT",
	"DOMAIN-SUFFIX,dianping.com,DIRECT",
	"DOMAIN-SUFFIX,douban.com,DIRECT",
	"DOMAIN-SUFFIX,doubanio.com,DIRECT",
	"DOMAIN-SUFFIX,duokan.com,DIRECT",
	"DOMAIN-SUFFIX,ele.me,DIRECT",
	"DOMAIN-SUFFIX,feng.com,DIRECT",
	"DOMAIN-SUFFIX,fir.im,DIRECT",
	"DOMAIN-SUFFIX,frdic.com,DIRECT",
	"DOMAIN-SUFFIX,g-cores.com,DIRECT",
	"DOMAIN-SUFFIX,godic.net,DIRECT",
	"DOMAIN-SUFFIX,gtimg.com,DIRECT",
	"DOMAIN-SUFFIX,hongxiu.com,DIRECT",
	"DOMAIN-SUFFIX,hxcdn.net,DIRECT",
	"DOMAIN-SUFFIX,iciba.com,DIRECT",
	"DOMAIN-SUFFIX,ifeng.com,DIRECT",
	"DOMAIN-SUFFIX,ifengimg.com,DIRECT",
	"DOMAIN-SUFFIX,ipip.net,DIRECT",
	"DOMAIN-SUFFIX,iqiyi.com,DIRECT",
	"DOMAIN-SUFFIX,jd.com,DIRECT",
	"DOMAIN-SUFFIX,jianshu.com,DIRECT",
	"DOMAIN-SUFFIX,le.com,DIRECT",
	"DOMAIN-SUFFIX,lecloud.com,DIRECT",
	"DOMAIN-SUFFIX,lemicp.com,DIRECT",
	"DOMAIN-SUFFIX,luoo.net,DIRECT",
	"DOMAIN-SUFFIX,meituan.com,DIRECT",
	"DOMAIN-SUFFIX,meituan.net,DIRECT",
	"DOMAIN-SUFFIX,mi.com,DIRECT",
	"DOMAIN-SUFFIX,miaopai.com,DIRECT",
	"DOMAIN-SUFFIX,microsoft.com,DIRECT",
	"DOMAIN-SUFFIX,microsoftonline.com,DIRECT",
	"DOMAIN-SUFFIX,miui.com,DIRECT",
	"DOMAIN-SUFFIX,miwifi.com,DIRECT",
	"DOMAIN-SUFFIX,mob.com,DIRECT",
	"DOMAIN-SUFFIX,netease.com,DIRECT",
	"DOMAIN-SUFFIX,office.com,DIRECT",
	"DOMAIN-SUFFIX,office365.com,DIRECT",
	"DOMAIN-KEYWORD,officecdn,DIRECT",
	"DOMAIN-SUFFIX,oschina.net,DIRECT",
	"DOMAIN-SUFFIX,pstatp.com,DIRECT",
	"DOMAIN-SUFFIX,qcloud.com,DIRECT",
	"DOMAIN-SUFFIX,qdaily.com,DIRECT",
	"DOMAIN-SUFFIX,qhimg.com,DIRECT",
	"DOMAIN-SUFFIX,qhres.com,DIRECT",
	"DOMAIN-SUFFIX,qidian.com,DIRECT",
	"DOMAIN-SUFFIX,qihucdn.com,DIRECT",
	"DOMAIN-SUFFIX,qiniu.com,DIRECT",
	"DOMAIN-SUFFIX,qiniucdn.com,DIRECT",
	"DOMAIN-SUFFIX,qiyipic.com,DIRECT",
	"DOMAIN-SUFFIX,ruguoapp.com,DIRECT",
	"DOMAIN-SUFFIX,segmentfault.com,DIRECT",
	"DOMAIN-SUFFIX,sinaapp.com,DIRECT",
	"DOMAIN-SUFFIX,smzdm.com,DIRECT",
	"DOMAIN-SUFFIX,sogou.com,DIRECT",
	"DOMAIN-SUFFIX,sogoucdn.com,DIRECT",
	"DOMAIN-SUFFIX,sohu.com,DIRECT",
	"DOMAIN-SUFFIX,speedtest.net,DIRECT",
	"DOMAIN-SUFFIX,sspai.com,DIRECT",
	"DOMAIN-SUFFIX,suning.com,DIRECT",
	"DOMAIN-SUFFIX,taobao.com,DIRECT",
	"DOMAIN-SUFFIX,tmall.com,DIRECT",
	"DOMAIN-SUFFIX,tudou.com,DIRECT",
	"DOMAIN-SUFFIX,upaiyun.com,DIRECT",
	"DOMAIN-SUFFIX,upyun.com,DIRECT",
	"DOMAIN-SUFFIX,weibo.com,DIRECT",
	"DOMAIN-SUFFIX,xiami.com,DIRECT",
	"DOMAIN-SUFFIX,ximalaya.com,DIRECT",
	"DOMAIN-SUFFIX,xmcdn.com,DIRECT",
	"DOMAIN-SUFFIX,xunlei.com,DIRECT",
	"DOMAIN-SUFFIX,yhd.com,DIRECT",
	"DOMAIN-SUFFIX,youdao.com,DIRECT",
	"DOMAIN-SUFFIX,youku.com,DIRECT",
	"DOMAIN-SUFFIX,zhihu.com,DIRECT",
	"DOMAIN-SUFFIX,zhimg.com,DIRECT",
	"DOMAIN-SUFFIX,zoho.com,DIRECT",
}

var tencentDirectRules = []string{
	"DOMAIN-SUFFIX,qq.com,DIRECT",
	"DOMAIN-SUFFIX,qqurl.com,DIRECT",
	"DOMAIN-SUFFIX,weixin.qq.com,DIRECT",
	"DOMAIN-SUFFIX,wechat.com,DIRECT",
	"DOMAIN-SUFFIX,wechatapp.com,DIRECT",
	"DOMAIN-SUFFIX,wechatpay.cn,DIRECT",
	"DOMAIN-SUFFIX,servicewechat.com,DIRECT",
	"DOMAIN-SUFFIX,qpic.cn,DIRECT",
	"DOMAIN-SUFFIX,qlogo.cn,DIRECT",
	"DOMAIN-SUFFIX,url.cn,DIRECT",
	"DOMAIN-SUFFIX,tencent.com,DIRECT",
	"DOMAIN-SUFFIX,tenpay.com,DIRECT",
	"DOMAIN-SUFFIX,wechatgame.com,DIRECT",
	"DOMAIN-SUFFIX,wegame.com,DIRECT",
	"DOMAIN-SUFFIX,pvp.qq.com,DIRECT",
	"DOMAIN-SUFFIX,game.qq.com,DIRECT",
	"DOMAIN-SUFFIX,qqgame.qq.com,DIRECT",
	"PROCESS-NAME,WeChat,DIRECT",
	"PROCESS-NAME,QQ,DIRECT",
	"PROCESS-NAME,TIM,DIRECT",
	"PROCESS-NAME,WeCom,DIRECT",
}

var bytedanceDirectRules = []string{
	"DOMAIN-SUFFIX,xiaohongshu.com,DIRECT",
	"DOMAIN-SUFFIX,xhscdn.com,DIRECT",
	"DOMAIN-SUFFIX,xhslink.com,DIRECT",
	"DOMAIN-SUFFIX,douyin.com,DIRECT",
	"DOMAIN-SUFFIX,douyinpic.com,DIRECT",
	"DOMAIN-SUFFIX,douyinvod.com,DIRECT",
	"DOMAIN-SUFFIX,iesdouyin.com,DIRECT",
	"DOMAIN-SUFFIX,amemv.com,DIRECT",
	"DOMAIN-SUFFIX,snssdk.com,DIRECT",
	"DOMAIN-SUFFIX,toutiao.com,DIRECT",
	"DOMAIN-SUFFIX,toutiaocdn.com,DIRECT",
	"DOMAIN-SUFFIX,ixigua.com,DIRECT",
	"DOMAIN-SUFFIX,byteimg.com,DIRECT",
	"DOMAIN-SUFFIX,bytecdn.cn,DIRECT",
	"DOMAIN-SUFFIX,bytedance.com,DIRECT",
	"DOMAIN-SUFFIX,bytedance.net,DIRECT",
	"DOMAIN-SUFFIX,ibytedtos.com,DIRECT",
	"PROCESS-NAME,Douyin,DIRECT",
}

var chinaEntertainmentDirectRules = []string{
	"DOMAIN-SUFFIX,kuaishou.com,DIRECT",
	"DOMAIN-SUFFIX,ksapisrv.com,DIRECT",
	"DOMAIN-SUFFIX,ks-cdn.com,DIRECT",
	"DOMAIN-SUFFIX,huya.com,DIRECT",
	"DOMAIN-SUFFIX,douyu.com,DIRECT",
	"DOMAIN-SUFFIX,qqmusic.qq.com,DIRECT",
	"DOMAIN-SUFFIX,kugou.com,DIRECT",
	"DOMAIN-SUFFIX,kuwo.cn,DIRECT",
	"DOMAIN-SUFFIX,pvp.qq.com,DIRECT",
	"DOMAIN-SUFFIX,gamecenter.qq.com,DIRECT",
	"DOMAIN-SUFFIX,tencentgames.com,DIRECT",
	"DOMAIN-SUFFIX,wetest.net,DIRECT",
	"PROCESS-NAME,QQMusic,DIRECT",
	"PROCESS-NAME,HonorOfKings,DIRECT",
}

var adRejectRules = []string{
	"DOMAIN-KEYWORD,admarvel,REJECT",
	"DOMAIN-KEYWORD,admaster,REJECT",
	"DOMAIN-KEYWORD,adsage,REJECT",
	"DOMAIN-KEYWORD,adsmogo,REJECT",
	"DOMAIN-KEYWORD,adsrvmedia,REJECT",
	"DOMAIN-KEYWORD,adwords,REJECT",
	"DOMAIN-KEYWORD,adservice,REJECT",
	"DOMAIN-SUFFIX,appsflyer.com,REJECT",
	"DOMAIN-KEYWORD,domob,REJECT",
	"DOMAIN-SUFFIX,doubleclick.net,REJECT",
	"DOMAIN-SUFFIX,googlesyndication.com,REJECT",
	"DOMAIN-SUFFIX,googleadservices.com,REJECT",
	"DOMAIN-SUFFIX,googletagmanager.com,REJECT",
	"DOMAIN-KEYWORD,duomeng,REJECT",
	"DOMAIN-KEYWORD,guanggao,REJECT",
	"DOMAIN-KEYWORD,lianmeng,REJECT",
	"DOMAIN-SUFFIX,mmstat.com,REJECT",
	"DOMAIN-KEYWORD,mopub,REJECT",
	"DOMAIN-KEYWORD,omgmta,REJECT",
	"DOMAIN-KEYWORD,openx,REJECT",
	"DOMAIN-KEYWORD,partnerad,REJECT",
	"DOMAIN-KEYWORD,umeng,REJECT",
	"DOMAIN-SUFFIX,vungle.com,REJECT",
	"DOMAIN-SUFFIX,gdt.qq.com,REJECT",
	"DOMAIN-SUFFIX,ad.tencent.com,REJECT",
	"DOMAIN-SUFFIX,union.baidu.com,REJECT",
	"DOMAIN-SUFFIX,pos.baidu.com,REJECT",
	"DOMAIN-SUFFIX,mobads.baidu.com,REJECT",
	"DOMAIN-SUFFIX,ad.xiaomi.com,REJECT",
	"DOMAIN-SUFFIX,tracking.miui.com,REJECT",
	"DOMAIN-SUFFIX,pangle.io,REJECT",
	"DOMAIN-SUFFIX,pangolin-sdk-toutiao.com,REJECT",
}

var globalProxyRules = []string{
	"DOMAIN-KEYWORD,amazon,Proxy",
	"DOMAIN-KEYWORD,google,Proxy",
	"DOMAIN-KEYWORD,gmail,Proxy",
	"DOMAIN-KEYWORD,youtube,Proxy",
	"DOMAIN-KEYWORD,facebook,Proxy",
	"DOMAIN-SUFFIX,fb.me,Proxy",
	"DOMAIN-SUFFIX,fbcdn.net,Proxy",
	"DOMAIN-KEYWORD,twitter,Proxy",
	"DOMAIN-KEYWORD,instagram,Proxy",
	"DOMAIN-KEYWORD,dropbox,Proxy",
	"DOMAIN-SUFFIX,twimg.com,Proxy",
	"DOMAIN-KEYWORD,blogspot,Proxy",
	"DOMAIN-SUFFIX,youtu.be,Proxy",
	"DOMAIN-KEYWORD,whatsapp,Proxy",
	"DOMAIN-KEYWORD,github,Proxy",
	"DOMAIN-SUFFIX,githubusercontent.com,Proxy",
	"DOMAIN-SUFFIX,cloudflare.com,Proxy",
	"DOMAIN-SUFFIX,cloudfront.net,Proxy",
	"DOMAIN-SUFFIX,wikipedia.org,Proxy",
	"DOMAIN-SUFFIX,wikimedia.org,Proxy",
	"DOMAIN-SUFFIX,medium.com,Proxy",
	"DOMAIN-SUFFIX,reddit.com,Proxy",
	"DOMAIN-SUFFIX,stackoverflow.com,Proxy",
	"DOMAIN-SUFFIX,v2ex.com,Proxy",
	"DOMAIN-SUFFIX,telegram.org,Proxy",
	"DOMAIN-SUFFIX,telegra.ph,Proxy",
	"DOMAIN-SUFFIX,spotify.com,Proxy",
	"DOMAIN-SUFFIX,twitch.tv,Proxy",
	"DOMAIN-SUFFIX,discord.com,Proxy",
	"DOMAIN-SUFFIX,discord.gg,Proxy",
	"DOMAIN-SUFFIX,discordapp.com,Proxy",
	"DOMAIN-SUFFIX,netflix.com,Proxy",
	"DOMAIN-SUFFIX,nflxvideo.net,Proxy",
	"DOMAIN-SUFFIX,openai.com,Proxy",
	"DOMAIN-SUFFIX,anthropic.com,Proxy",
	"DOMAIN-SUFFIX,claude.ai,Proxy",
	"DOMAIN-SUFFIX,claude.com,Proxy",
	"DOMAIN-SUFFIX,chatgpt.com,Proxy",
	"DOMAIN-SUFFIX,oaistatic.com,Proxy",
	"DOMAIN-SUFFIX,bing.com,Proxy",
	"DOMAIN-SUFFIX,linkedin.com,Proxy",
	"DOMAIN-SUFFIX,playstation.com,Proxy",
	"DOMAIN-SUFFIX,playstation.net,Proxy",
	"DOMAIN-SUFFIX,playstationnetwork.com,Proxy",
	"DOMAIN-SUFFIX,sony.com,Proxy",
	"DOMAIN-SUFFIX,steamcommunity.com,Proxy",
}

var telegramIPRules = []string{
	"IP-CIDR,91.108.4.0/22,Proxy,no-resolve",
	"IP-CIDR,91.108.8.0/21,Proxy,no-resolve",
	"IP-CIDR,91.108.16.0/22,Proxy,no-resolve",
	"IP-CIDR,91.108.56.0/22,Proxy,no-resolve",
	"IP-CIDR,149.154.160.0/20,Proxy,no-resolve",
}

var lanDirectRules = []string{
	"DOMAIN-SUFFIX,local,DIRECT",
	"DOMAIN-SUFFIX,lan,DIRECT",
	"DOMAIN,injections.adguard.org,DIRECT",
	"DOMAIN,local.adguard.org,DIRECT",
	"IP-CIDR,127.0.0.0/8,DIRECT",
	"IP-CIDR,169.254.0.0/16,DIRECT",
	"IP-CIDR,172.16.0.0/12,DIRECT",
	"IP-CIDR,192.168.0.0/16,DIRECT",
	"IP-CIDR,10.0.0.0/8,DIRECT",
	"IP-CIDR,17.0.0.0/8,DIRECT",
	"IP-CIDR,100.64.0.0/10,DIRECT",
	"IP-CIDR,224.0.0.0/4,DIRECT",
	"IP-CIDR6,fe80::/10,DIRECT",
}

var chinaGeoRules = []string{
	"DOMAIN-SUFFIX,cn,DIRECT",
	"DOMAIN-KEYWORD,-cn,DIRECT",
	"GEOIP,CN,DIRECT",
}
