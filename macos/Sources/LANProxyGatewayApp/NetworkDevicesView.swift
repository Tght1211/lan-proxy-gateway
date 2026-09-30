import SwiftUI

struct NetworkDevicesView: View {
    @EnvironmentObject private var model: AppModel
    @State private var method="wifi"
    @State private var sheet:NetworkSheet?
    @AppStorage("hotspotMemoSSID") private var memoSSID = "GrandLine"
    var body: some View {
        ScrollPage {
            Text("选择接入方式，让设备使用同一套出口与规则。").font(.system(size:13)).foregroundStyle(Theme.muted)
            StudioTabs(title: "接入方式", selection: $method, items: [("wifi", "Wi-Fi 热点 · 推荐", "wifi"), ("gateway", "静态 IP", "network"), ("pac", "PAC", "list.bullet.rectangle"), ("http", "HTTP 代理", "globe")])
            Panel {
                if method == "wifi" {
                    VStack(alignment:.leading,spacing:26) {
                        HStack(spacing:18) {
                            StudioIcon("wifi").frame(width:38,height:38).foregroundStyle(Theme.cyan)
                            VStack(alignment:.leading,spacing:6) { Text(memoSSID).font(.system(size:22,weight:.semibold)); Text("让手机、电脑与游戏主机共享网络").font(.system(size:13)).foregroundStyle(Theme.muted) }
                            Spacer(); StudioTag(text:model.hotspotControlState.title)
                        }
                        HStack(spacing:24) {
                            accessSummary("网络来源", "以太网 → Wi-Fi")
                            accessSummary("热点状态", model.hotspotControlState.title)
                            accessSummary("地址分配", "自动获取")
                        }
                        Divider()
                        HStack { Text("在 macOS 创建热点，在这里管理分流。").font(.caption).foregroundStyle(Theme.muted);Spacer();Button("设置 Wi-Fi 与连接设备"){sheet = .access("wifi")}.buttonStyle(StudioButtonStyle(primary:true)) }
                    }
                } else { NetworkAccessInstructions(method:method) }
            }
            HStack {Text("已记录设备").font(.headline);Spacer();Text("自动识别 · 可在详情中修正").font(.caption).foregroundStyle(Theme.muted)}
            Panel {
                VStack(spacing:0) {
                    ForEach(model.visibleDevices) { device in
                        Button {sheet = .device(device.name)} label:{
                            HStack(spacing:16) {
                                StudioIcon(deviceIcon(label:model.effectiveDeviceLabel(for:device.name))).frame(width:26,height:26)
                                VStack(alignment:.leading,spacing:4){Text(model.effectiveDeviceLabel(for:device.name).nonEmpty ?? device.name).font(.headline);Text(deviceSubtitle(device.name)).font(.system(.caption,design:.monospaced)).foregroundStyle(Theme.muted)}
                                Spacer();Text(shortBytes(device.total)).monospacedDigit();Image(systemName:"chevron.right").font(.caption).foregroundStyle(Theme.muted)
                            }.foregroundStyle(model.isDeviceInactive(device) ? Theme.muted : Theme.text).opacity(model.isDeviceInactive(device) ? 0.45 : 1).padding(.vertical,15).contentShape(Rectangle())
                        }.buttonStyle(.plain)
                        Divider()
                    }
                    if model.visibleDevices.isEmpty { Text("设备访问网络后会出现在这里").foregroundStyle(Theme.muted).padding(24) }
                }
            }
        }.sheet(item:$sheet){NetworkSheetContent(destination:$0).environmentObject(model)}.task{await model.refreshHotspot()}
    }
    private func accessSummary(_ title: String, _ value: String) -> some View {
        VStack(alignment:.leading,spacing:8) {
            Text(title).font(.caption).foregroundStyle(Theme.muted)
            Text(value).font(.system(size:14))
        }.frame(maxWidth:.infinity,alignment:.leading)
    }
    private func deviceSubtitle(_ ip: String) -> String {
        let connection = ((model.stats?.relay.active ?? []) + (model.stats?.relay.recent ?? [])).first { $0.srcIP == ip }
        let ingress = connection.map { NetworkIngress.resolve($0, hotspot:model.stats?.hotspot).title } ?? "暂无近期连接"
        return model.effectiveDeviceLabel(for:ip).isEmpty ? ingress : "\(ip) · \(ingress)"
    }
}

struct NetworkAccessInstructions: View {
    @EnvironmentObject private var model: AppModel
    let method:String
    var body: some View {
        VStack(alignment:.leading,spacing:18) {
            Text(method == "gateway" ? "静态网关接入":method == "pac" ? "自动代理 PAC":"HTTP 代理接入").font(.title3.weight(.semibold))
            let address=model.status?.gateway.localIP ?? "—"
            if method == "gateway" {
                SetupValue("网关",address)
                SetupValue("DNS",model.status?.dns.enabled == true && model.status?.dns.hijack == true ? address : "保留原有 DNS")
                Text("设备与 Mac 在同一局域网；为设备预留一个静态 IP，再填写上面的网关与 DNS 设置。").foregroundStyle(Theme.muted)
                if model.status?.accessMode == "hotspot" {
                    Text("当前核心使用热点接入模式。切换为静态网关会改变现有热点接管。").font(.caption).foregroundStyle(Theme.yellow)
                    Button("切换为静态网关接入"){Task{await model.configureHotspot("use-lan")}}.disabled(model.isBusy)
                }
            } else {
                if method == "pac" { SetupValue("PAC URL","http://\(address):\(model.status?.httpProxy?.port ?? 17894)/proxy.pac") }
                else { HStack {SetupValue("服务器",address);SetupValue("端口","\(model.status?.httpProxy?.port ?? 17894)")} }
                Text("保持设备原来的 IP 与网关，在网络设置中填写代理。仅遵循代理设置的应用生效。").font(.caption).foregroundStyle(Theme.muted)
                LANHTTPProxySettingsPanel()
            }
        }.frame(maxWidth:.infinity,alignment:.leading)
    }
}
