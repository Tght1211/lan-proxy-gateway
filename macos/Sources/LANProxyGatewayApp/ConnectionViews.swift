import AppKit
import Charts
import SwiftUI
import UniformTypeIdentifiers

struct ConnectionsView: View {
    @EnvironmentObject private var model: AppModel
    @State private var search = ""
    @State private var outcomeFilter = "全部"
    @State private var deviceFilter = "全部设备"
    @State private var routeFilter = "全部出口"
    @State private var unresolvedOnly = false

    // All records passing search/device/route filters; outcome chips filter on
    // top of this so their counts stay stable while one chip is selected.
    private var baseConnections: [ConnectionInfo] {
        let all = (model.stats?.relay.active ?? []) + (model.stats?.relay.recent ?? [])
        return all.filter { item in
            let label = model.effectiveDeviceLabel(for: item.srcIP)
            let matchesSearch = search.isEmpty || item.srcIP.localizedCaseInsensitiveContains(search) ||
                label.localizedCaseInsensitiveContains(search) || item.dstHost.localizedCaseInsensitiveContains(search) ||
                item.service.localizedCaseInsensitiveContains(search)
            let matchesDevice = deviceFilter == "全部设备" || item.srcIP == deviceFilter
            let matchesRoute: Bool
            switch routeFilter {
            case "代理": matchesRoute = item.viaProxy && !item.rejected
            case "直连": matchesRoute = !item.viaProxy && !item.rejected
            case "拒绝": matchesRoute = item.rejected
            default: matchesRoute = true
            }
            let matchesResolution = !unresolvedOnly || item.service == "未解析域名"
            return matchesSearch && matchesDevice && matchesRoute && matchesResolution
        }
    }

    private var connections: [ConnectionInfo] {
        baseConnections.filter { matchesOutcome($0) }
    }

    private func outcomeName(_ item: ConnectionInfo) -> String {
        switch item.outcome {
        case .active: return "活跃"
        case .waiting: return "等待响应"
        case .success: return "成功"
        case .noData: return "无数据"
        case .failed: return "失败"
        case .rejected: return "拒绝"
        }
    }

    private func matchesOutcome(_ item: ConnectionInfo) -> Bool {
        switch outcomeFilter {
        case "全部": return true
        case "出口回退": return item.fallback
        default: return outcomeName(item) == outcomeFilter
        }
    }

    private func outcomeCount(_ name: String) -> Int {
        if name == "出口回退" { return baseConnections.filter(\.fallback).count }
        return baseConnections.filter { outcomeName($0) == name }.count
    }

    private var successRateText: String {
        let total = baseConnections.count
        guard total > 0 else { return "--" }
        let good = baseConnections.filter { !$0.rejected && $0.down > 0 }.count
        return "\(good * 100 / total)%"
    }

    private var devices: [String] {
        Array(Set(((model.stats?.relay.active ?? []) + (model.stats?.relay.recent ?? [])).map(\.srcIP))).sorted()
    }

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Image(systemName: "magnifyingglass").foregroundStyle(Theme.muted)
                TextField("搜索设备、服务或域名", text: $search).textFieldStyle(.plain)
                    .frame(minWidth: 180)
                Divider().frame(height: 22)
                StudioSelect(title:"设备筛选",selection:$deviceFilter,options:[("全部设备","全部设备")]+devices.map{($0,model.effectiveDeviceLabel(for:$0).nonEmpty ?? $0)}).frame(width:150)
                StudioSelect(title:"出口筛选",selection:$routeFilter,options:["全部出口","代理","直连","拒绝"].map{($0,$0)}).frame(width:120)
                Toggle("仅未解析域名", isOn: $unresolvedOnly).toggleStyle(.checkbox).font(.caption)
                Image(systemName: "info.circle").foregroundStyle(Theme.muted)
                    .help("设备直接连接 IP，或 DNS 映射不可用时无法还原域名。仍会记录目标 IP、端口、流量、时间和出口；HTTPS 加密下无法识别具体操作内容。")
                Spacer(minLength: 8)
                Text("\(connections.count) 条记录").font(.caption).foregroundStyle(Theme.muted)
            }
            .padding(.horizontal, 16).frame(height: 44).background(Theme.panel)
            Divider().overlay(Theme.border)

            VStack(spacing: 12) {
                HStack(spacing: 8) {
                    OutcomeChip(label: "全部", count: baseConnections.count, color: Theme.muted,
                                selected: outcomeFilter == "全部") { outcomeFilter = "全部" }
                    OutcomeChip(label: "等待响应", count: outcomeCount("等待响应"), color: Theme.muted,
                                selected: outcomeFilter == "等待响应") { outcomeFilter = "等待响应" }
                    OutcomeChip(label: "活跃", count: outcomeCount("活跃"), color: Theme.cyan,
                                selected: outcomeFilter == "活跃") { outcomeFilter = "活跃" }
                    OutcomeChip(label: "已响应", count: outcomeCount("成功"), color: Theme.lime,
                                selected: outcomeFilter == "成功") { outcomeFilter = "成功" }
                    OutcomeChip(label: "无数据", count: outcomeCount("无数据"), color: Theme.muted,
                                selected: outcomeFilter == "无数据") { outcomeFilter = "无数据" }
                    OutcomeChip(label: "失败", count: outcomeCount("失败"), color: Theme.yellow,
                                selected: outcomeFilter == "失败") { outcomeFilter = "失败" }
                    OutcomeChip(label: "拒绝", count: outcomeCount("拒绝"), color: Theme.coral,
                                selected: outcomeFilter == "拒绝") { outcomeFilter = "拒绝" }
                    OutcomeChip(label: "出口回退", count: outcomeCount("出口回退"), color: Theme.yellow,
                                selected: outcomeFilter == "出口回退") { outcomeFilter = "出口回退" }
                    Spacer(minLength: 8)
                    FallbackLearningBadge()
                    HStack(spacing: 5) {
                        Text("响应率").font(.caption2).foregroundStyle(Theme.muted)
                        Text(successRateText)
                            .font(.system(size: 13, weight: .bold, design: .monospaced))
                            .foregroundStyle(Theme.lime)
                    }
                    .help("响应率 = 收到目标端数据的连接 / 当前筛选范围内全部记录；等待响应不计入，HTTPS 下无法判断业务是否成功")
                }

                Table(connections) {
                    TableColumn("设备") { item in
                        VStack(alignment: .leading, spacing: 1) {
                            if let label = model.effectiveDeviceLabel(for: item.srcIP).nonEmpty {
                                HStack(spacing: 4) {
                                    Text(label).fontWeight(.medium)
                                    if model.isAutoLabeled(item.srcIP) {
                                        Text("自动").font(.system(size: 8, weight: .bold)).foregroundStyle(Theme.muted)
                                    }
                                }
                            }
                            Text(item.srcIP).font(.system(size: 10, design: .monospaced)).foregroundStyle(Theme.muted)
                        }
                    }.width(min: 110, ideal: 140)
                    TableColumn("接入方式") { Text($0.ingressTitle).font(.caption) }.width(min: 70, ideal: 85)
                    TableColumn("识别服务") { Text($0.service).fontWeight(.medium) }.width(min: 90, ideal: 120)
                    TableColumn("目标域名 / 地址") { item in
                        HStack(spacing: 4) {
                            if item.isUDP {
                                Text("UDP")
                                    .font(.system(size: 8, weight: .bold, design: .monospaced))
                                    .padding(.horizontal, 3).padding(.vertical, 1)
                                    .background(Theme.yellow.opacity(0.15))
                                    .foregroundStyle(Theme.yellow)
                                    .clipShape(RoundedRectangle(cornerRadius: 3))
                            }
                            Text("\(item.dstHost.trimmingCharacters(in: CharacterSet(charactersIn: "."))):\(String(item.dstPort))")
                                .font(.system(.body, design: .monospaced))
                                .help("\(item.isUDP ? "UDP " : "")\(item.dstHost):\(item.dstPort)")
                        }
                    }.width(min: 220, ideal: 320)
                    TableColumn("出口") { item in
                        if item.fallback && !item.rejected {
                            Text(item.viaProxy ? "回退代理" : "回退直连")
                                .foregroundStyle(Theme.yellow)
                                .help(item.viaProxy ? "直连拨号失败后切换代理；收到响应数据后自动记住该域名使用代理" : "代理拨号失败后回退直连")
                        } else {
                            Text(item.rejected ? "REJECT" : (item.viaProxy ? "PROXY" : "DIRECT"))
                                .foregroundStyle(item.rejected ? Theme.coral : (item.viaProxy ? Theme.cyan : Theme.yellow))
                        }
                    }.width(72)
                    TableColumn("结果") { item in
                        HStack(spacing: 6) {
                            Circle().fill(outcomeColor(item.outcome)).frame(width: 6, height: 6)
                            Text(item.outcome.label)
                                .foregroundStyle(outcomeColor(item.outcome))
                                .lineLimit(1)
                        }
                        .help(outcomeHelp(item.outcome))
                    }.width(min: 80, ideal: 110)
                    TableColumn("流量") { Text(bytes($0.up + $0.down)) }.width(76)
                }
                .scrollContentBackground(.hidden)
                .background(Theme.panel)
                .overlay(RoundedRectangle(cornerRadius: Theme.radius).stroke(Theme.border, lineWidth: Theme.borderWidth))
                .clipShape(RoundedRectangle(cornerRadius: Theme.radius))
                .shadow(color: Color.black.opacity(Theme.shadowOpacity), radius: Theme.shadowRadius, y: 2)
            }
            .padding(16)
        }
        .background(Theme.canvasBackground)
    }

    private func outcomeColor(_ outcome: ConnectionOutcome) -> Color {
        switch outcome {
        case .active: return Theme.cyan
        case .waiting: return Theme.muted
        case .success: return Theme.lime
        case .noData: return Theme.muted
        case .failed: return Theme.yellow
        case .rejected: return Theme.coral
        }
    }

    private func outcomeHelp(_ outcome: ConnectionOutcome) -> String {
        switch outcome {
        case .active: return "连接进行中，已收到目标响应"
        case .waiting: return "尚未收到目标响应，不计为成功"
        case .success: return "收到目标端数据；不代表 HTTPS 页面或业务成功"
        case .noData: return "连接建立但没有数据传输，可能被远端关闭"
        case .failed(let reason): return "连接失败：\(reason)。HTTPS 加密流量无法看到 404/502 等应用层状态码"
        case .rejected: return "被分流规则拒绝"
        }
    }
}

struct OutcomeChip: View {
    let label: String
    let count: Int
    let color: Color
    let selected: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 6) {
                Circle().fill(color).frame(width: 6, height: 6)
                Text(label).font(.system(size: 11, weight: .medium))
                Text("\(count)")
                    .font(.system(size: 11, weight: .bold, design: .monospaced))
                    .foregroundStyle(color)
            }
            .padding(.horizontal, 10).padding(.vertical, 5)
            .background(selected ? color.opacity(0.14) : Theme.panel)
            .overlay(RoundedRectangle(cornerRadius: Theme.radiusSmall).stroke(selected ? color.opacity(0.55) : Theme.border, lineWidth: 0.8))
            .clipShape(RoundedRectangle(cornerRadius: Theme.radiusSmall))
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .help("点击筛选\(label == "全部" ? "全部记录" : "「\(label)」的记录")")
    }
}

struct ProxyConfigSheet: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.dismiss) private var dismiss
    @State private var mode = "socks5"
    @State private var isSaving = false
    @State private var isTesting = false
    @State private var saveError: String?
    @State private var testResult: (ok: Bool, message: String)?

    var body: some View {
        VStack(spacing: 0) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 3) {
                    Text("网络出口").font(.title3.weight(.semibold))
                    Text(activeExitSubtitle).font(.caption).foregroundStyle(Theme.muted)
                }
                Spacer()
                LiveBadge(active: model.isRunning && (model.stats?.health.healthy ?? false))
                Button { dismiss() } label: { Image(systemName: "xmark") }
                    .buttonStyle(IconButtonStyle())
            }
            .padding(20)
            .background(Theme.panel)
            Divider().overlay(Theme.border)

            VStack(alignment: .leading, spacing: 18) {
                HStack(alignment: .top, spacing: 24) {
                    VStack(alignment: .leading, spacing: 5) {
                        Text("当前出口").fieldLabel()
                        Text(activeExitTitle)
                            .font(.system(size: 20, weight: .semibold, design: .rounded))
                            .lineLimit(1).minimumScaleFactor(0.72)
                    }
                    .frame(minWidth: 210, alignment: .leading)
                    Spacer(minLength: 0)
                    VStack(alignment: .leading, spacing: 5) {
                        Label("公网 IP", systemImage: "globe.asia.australia")
                            .font(.caption2).foregroundStyle(Theme.muted)
                        Text(model.stats?.health.egressIdentity?.ip ?? "正在检测")
                            .font(.system(size: 15, weight: .semibold, design: .monospaced))
                            .lineLimit(1)
                            .textSelection(.enabled)
                    }
                    VStack(alignment: .leading, spacing: 5) {
                        Label("出口地区", systemImage: "mappin.and.ellipse")
                            .font(.caption2).foregroundStyle(Theme.muted)
                        Text(egressLocation(model.stats?.health.egressIdentity))
                            .font(.system(size: 15, weight: .semibold))
                            .lineLimit(1)
                        if let isp = model.stats?.health.egressIdentity?.isp?.nonEmpty {
                            Text(isp).font(.caption2).foregroundStyle(Theme.muted).lineLimit(1)
                        }
                    }
                }
                Divider().overlay(Theme.border)
                VStack(alignment: .leading, spacing: 12) {
                    VStack(alignment: .leading, spacing: 3) {
                        Text("出口方式").sectionLabel()
                        Text("代理填写 Clash / Mihomo / sing-box 的本机混合端口，保存后立即应用到新连接")
                            .font(.caption).foregroundStyle(Theme.muted)
                    }
                    HStack(alignment: .bottom, spacing: 12) {
                        VStack(alignment: .leading, spacing: 7) {
                            Text("出口模式").fieldLabel()
                            StudioSelect(title:"出口模式",selection:$mode,options:[("direct","无代理 · 直连"),("socks5","SOCKS5 代理"),("http","HTTP CONNECT 代理")]).frame(width:190)
                        }
                        VStack(alignment: .leading, spacing: 7) {
                            Text("代理地址").fieldLabel()
                            TextField("127.0.0.1", text: $model.proxyHost)
                                .textFieldStyle(DarkFieldStyle())
                                .disabled(mode == "direct")
                        }
                        .opacity(mode == "direct" ? 0.4 : 1)
                        VStack(alignment: .leading, spacing: 7) {
                            Text("端口").fieldLabel()
                            TextField("7897", value: $model.proxyPort, format: .number.grouping(.never))
                                .textFieldStyle(DarkFieldStyle())
                                .frame(width: 96)
                                .disabled(mode == "direct")
                        }
                        .opacity(mode == "direct" ? 0.4 : 1)
                        Button {
                            testProxy()
                        } label: {
                            if isTesting {
                                ProgressView().controlSize(.small).frame(width: 60)
                            } else {
                                Label("测试代理", systemImage: "bolt.horizontal")
                            }
                        }
                        .buttonStyle(StudioButtonStyle())
                        .frame(height: 36)
                        .disabled(mode == "direct" || isTesting || isSaving)
                        .help("按当前填写的地址测试连通性，不会保存配置")
                    }
                    if let testResult {
                        Label(testResult.message, systemImage: testResult.ok ? "checkmark.circle.fill" : "xmark.circle.fill")
                            .font(.caption)
                            .foregroundStyle(testResult.ok ? Theme.lime : Theme.coral)
                    }
                }
            }
            .padding(20)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)

            Divider().overlay(Theme.border)
            HStack(spacing: 8) {
                if let saveError {
                    Label(saveError, systemImage: "exclamationmark.triangle.fill")
                        .font(.caption).foregroundStyle(Theme.coral).lineLimit(2)
                }
                Spacer()
                Button("取消") { dismiss() }.buttonStyle(StudioButtonStyle()).disabled(isSaving)
                Button {
                    apply()
                } label: {
                    if isSaving {
                        ProgressView().controlSize(.small).frame(width: 56)
                    } else {
                        Text(applyTitle)
                    }
                }
                .buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                .disabled(isSaving || isTesting || model.isBusy || applyDisabled)
            }
            .padding(.horizontal, 20).padding(.vertical, 14)
            .background(Theme.panel)
        }
        .frame(width: 640, height: 400)
        .background(Theme.canvasBackground)
        .onAppear {
            mode = model.status?.egress == "proxy" ? model.proxyType : "direct"
        }
    }

    private var applyTitle: String {
        mode == "direct" ? "切换直连" : (model.isConfigured ? "应用代理" : "保存配置")
    }

    private var applyDisabled: Bool {
        mode == "direct" && model.status?.egress != "proxy"
    }

    private func apply() {
        saveError = nil
        isSaving = true
        Task {
            let ok: Bool
            if mode == "direct" {
                ok = await model.useDirectConnectionAsync()
            } else {
                model.proxyType = mode
                ok = await model.applyProxyAsync()
            }
            isSaving = false
            if ok { dismiss() } else { saveError = model.errorMessage ?? "应用失败，请检查配置" }
        }
    }

    private func testProxy() {
        testResult = nil
        isTesting = true
        Task {
            model.proxyType = mode
            testResult = await model.testProxyAsync()
            isTesting = false
        }
    }

    private var activeExitTitle: String {
        model.status?.egress == "proxy"
            ? (model.status?.proxy?.nonEmpty ?? "代理未配置")
            : "DIRECT 直连"
    }

    private var activeExitSubtitle: String {
        guard model.status?.egress == "proxy" else { return "局域网流量不经过上游代理" }
        let count = model.status?.routing?.count ?? 0
        return count == 0 ? "尚未配置分流规则" : "已配置 \(count) 条规则；连接出口由设备策略与分流规则决定"
    }
}
