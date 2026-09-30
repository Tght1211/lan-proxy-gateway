import SwiftUI

struct NetworkExitsView: View {
    @EnvironmentObject private var model: AppModel
    @State private var sheet: NetworkSheet?
    @State private var testing = false
    @State private var testResult: String?
    private var today:[DailyUsage] { (model.stats?.usageHistory ?? []).filter{$0.date == usageDate()} }
    private var rejected:[ConnectionInfo] { ((model.stats?.relay.active ?? [])+(model.stats?.relay.recent ?? [])).filter(\.rejected) }
    var body:some View {
        ScrollPage {
            HStack { Text("管理网络出口，查看各自的流量与稳定性。").font(.system(size:13)).foregroundStyle(Theme.muted);Spacer();Button(model.status?.proxy == nil ? "＋ 配置代理源":"编辑代理源"){sheet = .proxy} }
            LazyVGrid(columns:[GridItem(.flexible()),GridItem(.flexible())],alignment:.leading,spacing:18) {
                Panel {
                    VStack(alignment:.leading,spacing:20) {
                        heading("代理出口","cloud",Theme.cyan,model.status?.proxy == nil ? "未配置":"HTTP / SOCKS")
                        Text(model.status?.proxy ?? "配置一个代理源，作为独立网络出口").font(.system(size:12,design:.monospaced)).foregroundStyle(Theme.muted).textSelection(.enabled)
                        HStack {
                            metric("今日流量",shortBytes(total("proxy")));Spacer()
                            metric("最近响应",model.stats?.egress == "proxy" ? model.stats?.health.history.last.map{$0.ok ? formatMS($0.latencyMS):"未响应"} ?? "—":"—")
                        }
                        Divider()
                        if model.stats?.egress == "proxy" { NetworkStabilityCard(compact:true) }
                        else {
                            VStack(alignment:.leading,spacing:12) {Text("最近 5 分钟稳定性").font(.system(size:13,weight:.medium));HStack(spacing:3){ForEach(0..<30,id:\.self){_ in RoundedRectangle(cornerRadius:3).fill(Theme.border.opacity(0.4)).frame(height:32)}};Text("等待此出口的探测记录").font(.caption).foregroundStyle(Theme.muted)}
                        }
                        Spacer(minLength:0)
                        HStack { Button("配置代理源"){sheet = .proxy};Spacer();Button(testing ? "检测中…":"测试代理") {testing=true;Task{let result=await model.testProxyAsync();testResult=result.message;testing=false}}.disabled(testing || model.isBusy) }
                        if let testResult {Text(testResult).font(.caption).foregroundStyle(Theme.muted)}
                    }.frame(maxWidth:.infinity,minHeight:360,alignment:.topLeading)
                }
                Panel {
                    VStack(alignment:.leading,spacing:20) {
                        heading("本机直连","globe",Theme.lime,"DIRECT")
                        Text("使用 Mac 当前网络").font(.system(size:12)).foregroundStyle(Theme.muted)
                        HStack {metric("今日流量",shortBytes(total("direct")));Spacer();metric("今日连接","\(today.filter{$0.egress == "direct"}.reduce(Int64(0)){$0+$1.connections}) 条")}
                        Divider()
                        HStack {metric("下载",shortBytes(today.filter{$0.egress == "direct"}.reduce(0){$0+$1.down}));Spacer();metric("上传",shortBytes(today.filter{$0.egress == "direct"}.reduce(0){$0+$1.up}))}
                        VStack(alignment:.leading,spacing:10) {
                            Text("主要访问域名").font(.system(size:13,weight:.medium))
                            ForEach(topDirectDomains,id:\.0){domain,bytes in HStack{Text(domain).lineLimit(1);Spacer();Text(shortBytes(bytes)).monospacedDigit()}.font(.system(size:11)).foregroundStyle(Theme.muted)}
                            if topDirectDomains.isEmpty { Text("等待直连流量记录").font(.caption).foregroundStyle(Theme.muted) }
                        }
                        Spacer(minLength:0)
                        HStack {Text("未配置域名默认直连").font(.caption).foregroundStyle(Theme.muted);Spacer();Button("流量明细 ↗"){sheet = .direct}}
                    }.frame(maxWidth:.infinity,minHeight:360,alignment:.topLeading)
                }
                Panel {
                    VStack(alignment:.leading,spacing:20) {
                        heading("拒绝","ban",Theme.coral,"REJECT")
                        Text("在本机终止连接").font(.system(size:12)).foregroundStyle(Theme.muted)
                        HStack {metric("最近触发","\(rejected.count) 次");Spacer();metric("拒绝规则","\((model.status?.routing ?? []).filter{$0.action == "reject"}.count) 条")}
                        Divider()
                        Text("最近触发域名").font(.system(size:13,weight:.medium))
                        ForEach(Array(rejected.prefix(3))) { c in
                            Button {sheet = .reject} label:{HStack{Text(c.dstHost).lineLimit(1);Spacer();Text("已拒绝").foregroundStyle(Theme.coral);Image(systemName:"chevron.right").font(.system(size:9))}.font(.system(size:12)).padding(.vertical,7)}.buttonStyle(.plain)
                        }
                        if rejected.isEmpty {Text("暂无触发记录").font(.caption).foregroundStyle(Theme.muted)}
                        Spacer(minLength:0)
                        HStack {Text("拒绝请求不转发到目标").font(.caption).foregroundStyle(Theme.muted);Spacer();Button("规则与触发记录 ↗"){sheet = .reject}}
                    }.frame(maxWidth:.infinity,minHeight:300,alignment:.topLeading)
                }
            }
        }.sheet(item:$sheet){NetworkSheetContent(destination:$0).environmentObject(model)}
    }
    private func total(_ exit:String)->Int64 {today.filter{$0.egress == exit}.reduce(0){$0+$1.total}}
    private var topDirectDomains:[(String,Int64)] {
        Dictionary(grouping:today.filter{$0.egress == "direct" && $0.destination?.isEmpty == false},by:{$0.destination!}).map{($0.key,$0.value.reduce(Int64(0)){$0+$1.total})}.sorted{$0.1>$1.1}.prefix(3).map{$0}
    }
    private func heading(_ title:String,_ icon:String,_ tint:Color,_ tag:String)->some View {HStack(spacing:12){StudioIcon(icon).frame(width:25,height:25).foregroundStyle(tint);Text(title).font(.system(size:17,weight:.semibold));Spacer();StudioTag(text:tag,tint:tint)}}
    private func metric(_ title:String,_ value:String)->some View {VStack(alignment:.leading,spacing:9){Text(title).font(.system(size:11)).foregroundStyle(Theme.muted);Text(model.stats == nil ? "—":value).font(.system(size:23,weight:.semibold)).monospacedDigit()}}
}
