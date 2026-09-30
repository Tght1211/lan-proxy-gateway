import AppKit
import Charts
import SwiftUI

extension ThemePalette {
    static let playBridge = ThemePalette(
        id: "playBridge", name: "PlayBridge", isDark: false,
        canvas: Color(red: 0.95, green: 0.96, blue: 0.98),
        sidebar: Color(red: 0.93, green: 0.95, blue: 0.98),
        panel: .white, panelRaised: Color(red: 0.96, green: 0.97, blue: 0.99),
        border: Color.primary.opacity(0.08), cyan: Color(red: 0.18, green: 0.38, blue: 0.91),
        coral: .red, lime: Color(red: 0.13, green: 0.58, blue: 0.46), yellow: .orange,
        muted: .secondary, radius: 20, radiusSmall: 12, borderWidth: 0.5,
        shadowOpacity: 0.025, shadowRadius: 12, fontDesign: .default, canvasGradient: [])
}

struct InterfaceModePanel: View {
    @EnvironmentObject private var model: AppModel
    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 10) {
                Text("界面模式").font(.headline)
                Picker("界面模式", selection: $model.interfaceMode) {
                    ForEach(InterfaceMode.allCases) { Text($0.title).tag($0) }
                }.pickerStyle(.segmented).labelsHidden()
                Text("仅切换界面布局与交互。共用同一核心、配置和连接，不会重启或中断网络。")
                    .font(.caption).foregroundStyle(.secondary)
            }
        }
    }
}

struct InterfaceModeSuggestion: View {
    @EnvironmentObject private var model: AppModel
    @AppStorage("playBridge.invitationDismissed") private var dismissed = false
    var body: some View {
        if !dismissed {
            HStack(spacing: 12) {
                Image(systemName: "gamecontroller").foregroundStyle(Theme.cyan)
                VStack(alignment: .leading, spacing: 3) {
                    Text("试试 PlayBridge 界面").font(.subheadline.weight(.semibold))
                    Text("面向游戏热点的简洁布局。仅切换界面，网络与配置保持不变。")
                        .font(.caption).foregroundStyle(.secondary)
                }
                Spacer()
                Button("切换界面") { model.interfaceMode = .playBridge; dismissed = true }
                Button { dismissed = true } label: { Image(systemName: "xmark") }
                    .buttonStyle(.plain).help("暂不切换；以后可在设置中切换")
            }.padding(14).background(Theme.cyan.opacity(0.06))
        }
    }
}

private enum BridgePage: String, CaseIterable, Identifiable {
    case hotspot = "游戏热点", devices = "设备", traffic = "流量", source = "加速源", settings = "设置"
    var id: String { rawValue }
    var symbol: String {
        switch self {
        case .hotspot: return "wifi"
        case .devices: return "gamecontroller"
        case .traffic: return "chart.xyaxis.line"
        case .source: return "arrow.triangle.branch"
        case .settings: return "slider.horizontal.3"
        }
    }
    var subtitle: String {
        switch self {
        case .hotspot: return "让 Mac 成为游戏设备与网络之间的桥梁。"
        case .devices: return "在这里查看通过热点使用网络的设备。"
        case .traffic: return "了解流量去了哪里，以及如何到达。"
        case .source: return "连接你自己的代理，也可以直接上网。"
        case .settings: return "保持简单，需要时再展开更多工具。"
        }
    }
}

struct PlayBridgeView: View {
    @EnvironmentObject private var model: AppModel
    @State private var page: BridgePage? = .hotspot
    @State private var onboarding = false
    @State private var proxy = false
    @State private var rules = false
    @State private var legacyTool: AppSection?
    @AppStorage("playBridge.showRules") private var showRules = false
    @AppStorage("playBridge.showAdvanced") private var showAdvanced = false
    private var network: HotspotStatus? { model.stats?.hotspot ?? model.hotspot }
    private var rows: [DailyUsage] {
        guard let network else { return [] }
        return hotspotUsage(model.stats?.usageHistory ?? [], network: network, date: usageDate())
    }
    private var addresses: [String] { Array(Set(rows.map(\.device))).sorted() }
    private var currentPage: BridgePage { page ?? .hotspot }

    var body: some View {
        NavigationSplitView {
            VStack(alignment: .leading, spacing: 24) {
                HStack(spacing: 10) {
                    Image(systemName: "point.3.connected.trianglepath.dotted")
                        .font(.system(size: 25, weight: .medium)).foregroundStyle(Theme.cyan)
                    VStack(alignment: .leading, spacing: 3) {
                        Text("PlayBridge").font(.system(size: 20, weight: .bold))
                        Text("YOUR MAC. YOUR NETWORK.").font(.system(size: 8, weight: .medium)).tracking(1)
                            .foregroundStyle(.secondary)
                    }
                }.padding(.horizontal, 20).padding(.top, 30)
                List(BridgePage.allCases, selection: $page) { item in
                    Label(item.rawValue, systemImage: item.symbol).tag(item)
                        .font(.system(size: 14, weight: .medium)).padding(.vertical, 9)
                }.listStyle(.sidebar).scrollContentBackground(.hidden)
                VStack(alignment: .leading, spacing: 8) {
                    Label(model.hotspotControlState.title, systemImage: "circle.fill")
                        .font(.caption).foregroundStyle(model.hotspotControlState == .enabled ? Theme.lime : .secondary)
                    Text("LAN Proxy Gateway 核心").font(.caption2).foregroundStyle(.secondary)
                    Button("界面模式…") { page = .settings }.buttonStyle(.link).font(.caption)
                }.padding(20)
            }.background(Theme.sidebar).navigationSplitViewColumnWidth(min: 210, ideal: 220, max: 260)
        } detail: {
            ScrollView {
                VStack(alignment: .leading, spacing: 24) {
                    header
                    if model.coreUpgradeRecommended { CoreCompatibilityBar() }
                    switch currentPage {
                    case .hotspot: dashboard
                    case .devices: deviceList
                    case .traffic: trafficDetails
                    case .source: sourceSettings
                    case .settings: settings
                    }
                }.padding(30).frame(maxWidth: 1280).frame(maxWidth: .infinity)
            }.background(Theme.canvas)
        }
        .sheet(isPresented: $onboarding) { PlayBridgeSetupSheet().environmentObject(model) }
        .sheet(isPresented: $proxy) { ProxyConfigSheet().environmentObject(model) }
        .sheet(isPresented: $rules) { RoutingRulesEditor(rules: model.status?.routing ?? []).environmentObject(model) }
        .sheet(item: $legacyTool) { tool in
            VStack(spacing: 0) {
                HStack {
                    Text("高级工具 · " + tool.rawValue).font(.headline)
                    Spacer()
                    Button("完成") { legacyTool = nil }
                }.padding(20)
                Divider()
                switch tool {
                case .devices: DevicesView()
                case .connections: ConnectionsView()
                default: SettingsView()
                }
            }.frame(width: 1060, height: 720).background(Theme.canvas).environmentObject(model)
        }
        .task { await model.refreshHotspot() }
    }

    private var header: some View {
        HStack {
            VStack(alignment: .leading, spacing: 8) {
                Text(currentPage.rawValue).font(.system(size: 29, weight: .bold))
                Text(currentPage.subtitle).font(.subheadline).foregroundStyle(.secondary)
            }
            Spacer()
            if currentPage == .hotspot || currentPage == .devices {
                Button { onboarding = true } label: { Label("设置热点", systemImage: "plus") }
                    .buttonStyle(.borderedProminent).controlSize(.large)
            }
        }
    }

    private var dashboard: some View {
        VStack(spacing: 20) {
            topology
            HStack(spacing: 16) {
                metric("今日已记录", bytes(rows.reduce(0) { $0 + $1.total }), "热点网段 · 核心转发流量", "arrow.up.arrow.down", Theme.cyan)
                metric("通过代理", bytes(rows.filter { $0.egress == "proxy" }.reduce(0) { $0 + $1.total }), "使用你配置的加速源", "arrow.triangle.branch", Theme.cyan)
                metric("直接连接", bytes(rows.filter { $0.egress == "direct" }.reduce(0) { $0 + $1.total }), "通过本机网络访问", "globe", Theme.lime)
            }
            PlayBridgeEqualHeightRow {
                PlayBridgeThroughput(fillHeight: true)
                PlayBridgeStability()
            }
            PlayBridgeEqualHeightRow {
                Panel(fillHeight: true) {
                    VStack(alignment: .leading, spacing: 12) {
                        Label("兼容性优先的 Wi-Fi", systemImage: "wifi").font(.headline)
                        Text("默认参考港版主机。近距离优先试 5 GHz / 36；兼容性需要时回退 2.4 GHz / 1、6、11。")
                            .font(.subheadline).foregroundStyle(.secondary)
                        Spacer(minLength: 0)
                        Button("查看机型与信道建议") { onboarding = true }.buttonStyle(.link)
                    }.frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
                }
                Panel(fillHeight: true) {
                    VStack(alignment: .leading, spacing: 12) {
                        Label("从基础网络开始", systemImage: "sparkles").font(.headline)
                        Text("核心提供 DNS 缓存与并发上游查询。实际效果取决于网络环境；远端线路加速可选用你自己的代理。")
                            .font(.subheadline).foregroundStyle(.secondary)
                        Spacer(minLength: 0)
                        Button("配置加速源") { proxy = true }.buttonStyle(.link)
                    }.frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
                }
            }
            scopeNote
        }
    }

    private var topology: some View {
        VStack(alignment: .leading, spacing: 22) {
            HStack {
                Label("你的网络桥梁", systemImage: "point.3.connected.trianglepath.dotted").font(.headline)
                Spacer()
                Text(model.hotspotControlState.title)
                    .font(.caption.weight(.medium)).foregroundStyle(Theme.cyan)
            }
            HStack(spacing: 0) {
                VStack(spacing: 14) {
                    topologyNode("直接连接", subtitle: "Mac 的互联网连接", icon: "globe", tint: Theme.lime) { page = .source }
                    topologyNode("自定义加速源", subtitle: model.status?.egress == "proxy" ? (model.status?.proxy ?? "已配置") : "可选 · 使用自己的代理", icon: "arrow.triangle.branch", tint: Theme.cyan) { proxy = true }
                }.frame(maxWidth: .infinity)
                bridgeLine
                Button { onboarding = true } label: {
                    VStack(spacing: 14) {
                        ZStack {
                            Circle().stroke(Theme.cyan.opacity(0.07), lineWidth: 1).frame(width: 160, height: 160)
                            Circle().stroke(Theme.cyan.opacity(0.12), lineWidth: 1).frame(width: 128, height: 128)
                            RoundedRectangle(cornerRadius: 23).fill(LinearGradient(colors: [.white, Color(red: 0.82, green: 0.87, blue: 0.96)], startPoint: .topLeading, endPoint: .bottomTrailing))
                                .frame(width: 96, height: 96).rotationEffect(.degrees(-8))
                                .shadow(color: Theme.cyan.opacity(0.16), radius: 18, y: 10)
                            Image(systemName: "wifi").font(.system(size: 36, weight: .medium)).foregroundStyle(Theme.cyan)
                        }
                        Text("Mac · 游戏热点").font(.headline)
                        Text(network?.available == true ? (network?.ip ?? "热点已检测") : "完成系统共享设置")
                            .font(.caption).foregroundStyle(.secondary)
                    }
                }.buttonStyle(.plain).help("查看热点设置与状态").frame(width: 184)
                bridgeLine
                VStack(spacing: 14) {
                    if addresses.isEmpty {
                        topologyNode("游戏设备", subtitle: "连接热点后产生流量即可发现", icon: "gamecontroller", tint: Theme.cyan) { page = .devices }
                        Text("PS5 · Switch · 更多设备").font(.caption).foregroundStyle(.secondary)
                    } else {
                        ForEach(Array(addresses.prefix(2)), id: \.self) { address in
                            topologyNode(model.effectiveDeviceLabel(for: address), subtitle: address, icon: "gamecontroller", tint: Theme.cyan) { page = .devices }
                        }
                        if addresses.count > 2 { Button("查看全部 \(addresses.count) 台设备") { page = .devices }.buttonStyle(.link) }
                    }
                }.frame(maxWidth: .infinity)
            }.padding(.vertical, 4)
            Divider()
            HStack {
                Image(systemName: "info.circle").foregroundStyle(Theme.cyan)
                Text(model.hotspotActionError ?? model.hotspotError ?? network?.message ?? "先创建系统热点，再由 PlayBridge 接管网络。")
                    .font(.caption).foregroundStyle(.secondary)
                Spacer()
                Button("检测") { Task { await model.refreshHotspot(); await model.refresh(silent: true) } }
                    .disabled(model.isBusy)
                if model.hotspotControlState == .enabled {
                    Button("停止接管") { Task { await model.configureHotspot("disable") } }.disabled(model.isBusy)
                        .help("停止 App 接管；系统 Wi-Fi 共享需在系统设置中关闭")
                } else {
                    Button("接入引导") { onboarding = true }.buttonStyle(.bordered)
                }
            }
        }.padding(24)
            .background(LinearGradient(colors: [.white, Theme.cyan.opacity(0.045)], startPoint: .topLeading, endPoint: .bottomTrailing))
            .clipShape(RoundedRectangle(cornerRadius: 22))
            .overlay(RoundedRectangle(cornerRadius: 22).stroke(Theme.cyan.opacity(0.1)))
    }

    private var bridgeLine: some View {
        HStack(spacing: 0) {
            Circle().fill(Theme.cyan.opacity(0.35)).frame(width: 5, height: 5)
            Rectangle().fill(Theme.cyan.opacity(0.25)).frame(height: 1)
            Image(systemName: "chevron.right").font(.system(size: 9, weight: .bold)).foregroundStyle(Theme.cyan.opacity(0.5))
        }.frame(minWidth: 20, maxWidth: 54).padding(.horizontal, 8).accessibilityHidden(true)
    }

    private func topologyNode(_ title: String, subtitle: String, icon: String, tint: Color, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            VStack(alignment: .leading, spacing: 8) {
                Image(systemName: icon).font(.system(size: 22)).foregroundStyle(tint)
                Text(title).font(.system(size: 13, weight: .semibold)).lineLimit(1)
                Text(subtitle).font(.system(size: 10)).foregroundStyle(.secondary).lineLimit(2)
            }.frame(maxWidth: .infinity, alignment: .leading).padding(15)
                .background(.white.opacity(0.85)).clipShape(RoundedRectangle(cornerRadius: 15))
                .overlay(RoundedRectangle(cornerRadius: 15).stroke(tint.opacity(0.12)))
        }.buttonStyle(.plain)
    }

    private func metric(_ title: String, _ value: String, _ detail: String, _ icon: String, _ tint: Color) -> some View {
        Panel {
            VStack(alignment: .leading, spacing: 12) {
                Label(title, systemImage: icon).font(.caption).foregroundStyle(tint)
                Text(model.stats?.usageHistory == nil || network?.available != true ? "—" : value)
                    .font(.system(size: 26, weight: .semibold, design: .rounded)).monospacedDigit()
                Text(detail).font(.caption2).foregroundStyle(.secondary)
            }.frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    private var deviceList: some View {
        Panel {
            VStack(alignment: .leading, spacing: 18) {
                Text("今天使用过热点的设备 · \(addresses.count)").font(.headline)
                if addresses.isEmpty {
                    Text("还没有设备流量。让主机连接 Mac 创建的热点，保持 IP 与 DNS 自动获取，然后访问网络。")
                        .foregroundStyle(.secondary).padding(.vertical, 30)
                }
                ForEach(addresses, id: \.self) { address in
                    HStack(spacing: 16) {
                        Image(systemName: "gamecontroller").font(.title2).foregroundStyle(Theme.cyan)
                        VStack(alignment: .leading, spacing: 4) {
                            Text(model.effectiveDeviceLabel(for: address)).font(.headline)
                            Text(address).font(.caption).foregroundStyle(.secondary)
                        }
                        Spacer()
                        Text(hasActiveConnection(address) ? "有转发连接" : "今天使用过").font(.caption).foregroundStyle(.secondary)
                        Text(bytes(rows.filter { $0.device == address }.reduce(0) { $0 + $1.total })).monospacedDigit()
                    }.padding(.vertical, 8)
                    Divider()
                }
                scopeNote
                if showAdvanced { Button("打开完整设备管理…") { legacyTool = .devices } }
            }
        }
    }

    private func hasActiveConnection(_ address: String) -> Bool {
        model.stats?.relay.active.contains { $0.srcIP == address && !$0.isHTTPProxy } == true
    }

    private var trafficDetails: some View {
        PlayBridgeTrafficView()
    }

    private var sourceSettings: some View {
        VStack(alignment: .leading, spacing: 20) {
            Panel {
                VStack(alignment: .leading, spacing: 16) {
                    Label("默认出口", systemImage: "arrow.triangle.branch").font(.headline)
                    Text(model.status?.egress == "proxy" ? "使用自定义代理" : "直接连接").font(.title2.weight(.semibold))
                    Text(model.status?.egress == "proxy" ? (model.status?.proxy ?? "已配置代理") : "使用 Mac 自己的互联网连接，不需要提供 VPN。")
                        .foregroundStyle(.secondary).textSelection(.enabled)
                    Text("加速源由你提供。将本机 VPN / 代理软件开放的 SOCKS5 或 HTTP 地址与端口填入此处；实际流量仍按共用分流规则选择出口。")
                        .font(.subheadline).foregroundStyle(.secondary)
                    Button("配置出口…") { proxy = true }.buttonStyle(.borderedProminent)
                }
            }
            Panel {
                VStack(alignment: .leading, spacing: 12) {
                    Text("自动学习网站出口").font(.headline)
                    Text("陌生网站先直连；连接失败再试代理，收到响应后按学习策略处理。可调整学习次数与保存方式，也可撤销或暂停指定域名。")
                        .font(.subheadline).foregroundStyle(.secondary)
                    FallbackLearningBadge()
                }
            }
            if showRules {
                Panel {
                    VStack(alignment: .leading, spacing: 12) {
                        Text("分流规则").font(.headline)
                        Text("与经典界面共用。按设备、域名和服务选择直连或代理，隐藏此入口不会停止已有规则。")
                            .font(.subheadline).foregroundStyle(.secondary)
                        Button("管理分流规则…") { rules = true }
                    }
                }
            } else {
                Text("需要自定义分流？在设置中显示「分流规则」工具。已有规则始终生效。")
                    .font(.caption).foregroundStyle(.secondary)
            }
        }
    }

    private var settings: some View {
        VStack(alignment: .leading, spacing: 20) {
            InterfaceModePanel()
            Panel {
                VStack(alignment: .leading, spacing: 18) {
                    Text("按需显示工具").font(.headline)
                    Toggle("分流规则", isOn: $showRules)
                    Text("在加速源页面显示规则编辑入口。").font(.caption).foregroundStyle(.secondary)
                    Divider()
                    Toggle("高级工具", isOn: $showAdvanced)
                    Text("显示完整设备管理与原有系统设置。开关只控制入口展示，不会启停已有功能。")
                        .font(.caption).foregroundStyle(.secondary)
                }.toggleStyle(.switch)
            }
            HotspotSystemSetupGuide()
            if showAdvanced {
                HStack {
                    Button("高级设置…") { legacyTool = .settings }
                    Button("全部接入方式的连接记录…") { legacyTool = .connections }
                }
            }
            Text("PlayBridge 是 LAN Proxy Gateway 的界面模式。当前沿用同一核心，无需另装一套软件。")
                .font(.caption).foregroundStyle(.secondary)
        }
    }

    private var scopeNote: some View {
        Text("统计仅包含核心记录的当前热点网段流量，按本地日期汇总；未接管、绕过核心的流量不计入。历史记录按当前网段归类，旧记录可能未区分直连与代理。")
            .font(.caption2).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
    }

    private func bytes(_ value: Int64) -> String { ByteCountFormatter.string(fromByteCount: value, countStyle: .file) }
}

private struct PlayBridgeThroughput: View {
    var fillHeight = false
    @EnvironmentObject private var model: AppModel
    @State private var seconds = 60
    @State private var inspectedAt: Date?
    private var inspectedPoint: HotspotRatePoint? {
        guard let inspectedAt else { return points.last }
        return points.min { abs($0.at.timeIntervalSince(inspectedAt)) < abs($1.at.timeIntervalSince(inspectedAt)) }
    }
    private var window: HotspotChartWindow {
        HotspotChartWindow(points: model.hotspotRates, seconds: seconds, now: Date())
    }
    private var points: [HotspotRatePoint] { window.points }
    var body: some View {
        Panel(fillHeight: fillHeight) {
            VStack(alignment: .leading, spacing: 18) {
                HStack {
                    VStack(alignment: .leading, spacing: 5) {
                        Text("吞吐量").font(.headline)
                        Text("↓ \(rate(inspectedPoint?.down))    ↑ \(rate(inspectedPoint?.up))")
                            .font(.caption).foregroundStyle(.secondary).monospacedDigit()
                    }
                    Spacer()
                    Picker("时间范围", selection: $seconds) {
                        Text("1 分钟").tag(60)
                        Text("5 分钟").tag(300)
                    }.pickerStyle(.segmented).labelsHidden().frame(width: 140)
                }
                Text(inspectedAt?.formatted(date: .omitted, time: .standard) ?? "最近 \(seconds / 60) 分钟 · 未采集的时段留空")
                    .font(.caption2).foregroundStyle(.secondary)
                Spacer(minLength: 0)
                if points.isEmpty {
                    VStack(spacing: 10) {
                        Image(systemName: "waveform.path").font(.title).foregroundStyle(Theme.cyan.opacity(0.5))
                        Text("等待热点流量采样").font(.subheadline)
                        Text("接管生效后，根据核心计数变化显示真实速率。")
                            .font(.caption).foregroundStyle(.secondary)
                    }.frame(maxWidth: .infinity).frame(height: 155)
                } else {
                    Chart(points) { point in
                        LineMark(x: .value("时间", point.at), y: .value("KB/s", point.down / 1000), series: .value("方向", "下载"))
                            .foregroundStyle(by: .value("方向", "下载"))
                        LineMark(x: .value("时间", point.at), y: .value("KB/s", point.up / 1000), series: .value("方向", "上传"))
                            .foregroundStyle(by: .value("方向", "上传"))
                    }.chartForegroundStyleScale(["下载": Theme.cyan, "上传": Theme.lime])
                        .chartXScale(domain: window.domain)
                        .chartXAxis {
                            AxisMarks(values: [window.domain.lowerBound,
                                               window.domain.lowerBound.addingTimeInterval(Double(seconds) / 2),
                                               window.domain.upperBound]) { _ in
                                AxisGridLine()
                                AxisValueLabel(format: .dateTime.hour().minute().second())
                            }
                        }
                        .chartYAxisLabel("KB/s").chartYScale(domain: .automatic(includesZero: true))
                        .frame(height: 155)
                        .chartOverlay { chart in
                            GeometryReader { geometry in
                                Rectangle().fill(.clear).contentShape(Rectangle())
                                    .onContinuousHover { phase in
                                        switch phase {
                                        case .active(let location):
                                            let origin = geometry[chart.plotAreaFrame].origin
                                            inspectedAt = chart.value(atX: location.x - origin.x, as: Date.self)
                                        case .ended: inspectedAt = nil
                                        }
                                    }
                            }
                        }
                        .accessibilityLabel("最近 \(seconds / 60) 分钟热点吞吐曲线，下载 \(rate(points.last?.down))，上传 \(rate(points.last?.up))")
                }
            }.frame(maxHeight: fillHeight ? .infinity : nil, alignment: .topLeading)
        }
        .onChange(of: seconds) { _ in inspectedAt = nil }
    }
    private func rate(_ value: Double?) -> String {
        guard let value else { return "—" }
        return ByteCountFormatter.string(fromByteCount: Int64(value), countStyle: .file) + "/s"
    }
}
