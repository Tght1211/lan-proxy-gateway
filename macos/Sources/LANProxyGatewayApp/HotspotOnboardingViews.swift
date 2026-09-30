import AppKit
import SwiftUI

struct HotspotOnboardingPanel: View {
    @EnvironmentObject private var model: AppModel
    @State private var console = "Switch"
    @State private var showProxy = false
    @State private var showRules = false
    @State private var showHelp = false
    @State private var checking = false
    @State private var checkedAt: Date?

    private var applied: Bool {
        model.hotspotControlState == .enabled
    }

    private var canEnable: Bool {
        !model.isBusy && model.hotspot?.available == true
    }

    private var enableTitle: String {
        if model.hotspotControlState.isWorking { return model.hotspotControlState.title }
        if model.hotspotControlState == .needsCoreUpdate { return "更新核心并开启接管" }
        if model.hotspot?.stage == "wifi_off" { return "先打开 Mac 的 Wi-Fi" }
        if model.hotspot?.available != true { return "等待热点启动" }
        return "开启热点接管"
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack(spacing: 14) {
                Image(systemName: "wifi").font(.system(size: 28)).foregroundStyle(Theme.cyan)
                VStack(alignment: .leading, spacing: 4) {
                    Text("为游戏机准备专属 Wi-Fi").font(.headline)
                    Text("不用填写 IP、网关或 DNS。Mac 继续通过网线上网。")
                        .font(.caption).foregroundStyle(Theme.muted)
                }
            }
            Divider()
            OnboardingStep(number: "1", title: "在系统设置中创建 Wi-Fi", detail: "先打开 Mac 的 Wi-Fi。再到「通用 → 共享 → 互联网共享」，从以太网共享给 Wi-Fi。设置名称和密码后，依次点击「好」「完成」，确认共享开关已开启。这里不需要填写任何代理。")
            HotspotSystemSetupGuide()
            HStack {
                Button("打开系统共享设置") {
                    if let url = URL(string: "x-apple.systempreferences:com.apple.Sharing-Settings.extension") {
                        NSWorkspace.shared.open(url)
                    }
                }.buttonStyle(.bordered)
                if model.hotspot?.stage == "wifi_off" {
                    Button("打开 Wi-Fi 设置") {
                        if let url = URL(string: "x-apple.systempreferences:com.apple.wifi-settings-extension") {
                            NSWorkspace.shared.open(url)
                        }
                    }.buttonStyle(.bordered)
                }
                Button(checking ? "检测中…" : "重新检测") {
                    Task { await checkHotspot(); await model.refresh(silent: true) }
                }
                    .buttonStyle(.bordered)
                    .disabled(checking)
            }
            Label(model.hotspotError ?? model.hotspot?.message ?? "正在检测 Wi-Fi 共享…",
                  systemImage: model.hotspot?.available == true ? "checkmark.circle.fill" : "info.circle")
                .font(.caption).foregroundStyle(model.hotspot?.available == true ? Theme.cyan : Theme.muted)
                .fixedSize(horizontal: false, vertical: true)

            if let checkedAt {
                Text("上次检测：\(checkedAt.formatted(date: .omitted, time: .standard))")
                    .font(.caption2).foregroundStyle(Theme.muted)
            }

            OnboardingStep(number: "2", title: "选择出口并开启热点接管", detail: "可直接上网，也可配置自己的代理地址与端口。开启接管会应用热点接入模式并重启核心；已有连接可能中断。默认出口和分流规则与经典界面共用。")
            takeoverStatus
            if model.status?.egress == "proxy" {
                Label("已配置代理：\(model.status?.proxy ?? "现有代理出口")，无需重复填写。", systemImage: "checkmark.circle")
                    .font(.caption).foregroundStyle(Theme.cyan)
            }
            HStack {
                Button(model.status?.egress == "proxy" ? "更换代理出口" : "设置代理出口") { showProxy = true }
                    .buttonStyle(.bordered)
                if applied {
                    Label("接管已开启", systemImage: "checkmark.circle.fill").font(.caption).foregroundStyle(Theme.cyan)
                    Spacer()
                    Button("停止接管") { Task { await model.configureHotspot("disable") } }
                        .disabled(model.isBusy)
                } else {
                    Button(enableTitle) {
                        Task { await model.configureHotspot("enable") }
                    }
                    .buttonStyle(ActionButtonStyle(tint: canEnable ? Theme.cyan : Theme.muted))
                    .opacity(canEnable ? 1 : 0.55)
                    .disabled(!canEnable)
                }
            }
            VStack(alignment: .leading, spacing: 8) {
                Label("已复用本 App 的分流规则", systemImage: "arrow.triangle.branch")
                    .font(.headline)
                Text("热点设备与其他接入方式共用同一套规则：先判断设备策略，再匹配域名规则，未匹配时使用默认出口。修改后即时应用。")
                    .font(.caption).foregroundStyle(Theme.muted)
                Button("管理分流规则（共用）") { showRules = true }
                    .buttonStyle(.bordered)
                Text("这里管理的是本 App 的规则。上游 Clash 等软件收到代理流量后，仍会执行它自己的规则。")
                    .font(.caption2).foregroundStyle(Theme.muted)
            }
            .padding(12).background(Theme.panelRaised)
            .clipShape(RoundedRectangle(cornerRadius: 10))
            if model.status?.egress != "proxy" {
                Text("当前默认出口为直连，无需 VPN 也可开启接管；显式代理规则仍按已有规则执行。")
                    .font(.caption).foregroundStyle(Theme.yellow)
            }
            if let error = model.errorMessage {
                Text(error).font(.caption).foregroundStyle(Theme.yellow).textSelection(.enabled)
            }

            OnboardingStep(number: "3", title: "在游戏机上连接刚才的 Wi-Fi", detail: "选择你设置的 Wi-Fi 名称，输入密码。之前手填过网络设置的设备，请改回自动获取。")
            Picker("你的设备", selection: $console) {
                Text("Nintendo Switch").tag("Switch")
                Text("PlayStation 5").tag("PS5")
            }.pickerStyle(.segmented)
            Text(console == "Switch"
                 ? "系统设置 → 互联网 → 互联网设置 → 选择新 Wi-Fi。IP 地址和 DNS 设为「自动」，代理服务器设为「不使用」。"
                 : "设置 → 网络 → 设定 → 设定互联网连接 → 选择新 Wi-Fi。IP 和 DNS 设为「自动」，DHCP 主机名「不指定」，代理服务器「不使用」。")
                .font(.callout).fixedSize(horizontal: false, vertical: true)
            Text("最后运行游戏机的「测试连接」，再打开商店或下载内容；回到设备列表查看流量。热点已开启不等于游戏联机已验证。")
                .font(.caption).foregroundStyle(Theme.muted)
            DisclosureGroup("连不上，或游戏联机受限？", isExpanded: $showHelp) {
                VStack(alignment: .leading, spacing: 8) {
                    Text("找不到 Wi-Fi：确认系统互联网共享已开启，Mac 保持唤醒；尝试在 Wi-Fi 选项中选择兼容游戏机的频段与安全方式。")
                    Text("连上但打不开商店：确认代理软件正在运行，重新检测并开启接管；在游戏机上断开后重连。")
                    Text("商店可用但联机受限：本功能主要代理 TCP，游戏与语音 UDP 仍直连。互联网共享会增加一层 NAT，联机效果需在游戏机实测；可切回家里的 Wi-Fi 对比。")
                    Text("停止接管后，系统仍会共享普通网络。要关闭这个 Wi-Fi，请在系统设置中关闭互联网共享。Mac 休眠或关机会让热点断开。")
                }.font(.caption).foregroundStyle(Theme.muted).padding(.top, 6)
            }
        }
        .sheet(isPresented: $showProxy) { ProxyConfigSheet().environmentObject(model) }
        .sheet(isPresented: $showRules) { RoutingRulesEditor(rules: model.status?.routing ?? []).environmentObject(model) }
        .task {
            while !Task.isCancelled {
                await checkHotspot()
                try? await Task.sleep(nanoseconds: 4_000_000_000)
            }
        }
    }
    private var takeoverStatus: some View {
        let state = model.hotspotControlState
        let tint = state == .enabled ? Theme.cyan : Theme.yellow
        return HStack(alignment: .top, spacing: 10) {
            if state.isWorking {
                ProgressView().controlSize(.small)
            } else {
                Image(systemName: state == .enabled ? "checkmark.shield.fill" : "info.circle.fill")
                    .foregroundStyle(tint)
            }
            VStack(alignment: .leading, spacing: 5) {
                Text(state.title).font(.headline)
                Text(model.hotspotControlDetail).font(.caption)
                    .fixedSize(horizontal: false, vertical: true).textSelection(.enabled)
            }
            Spacer(minLength: 0)
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(tint.opacity(0.10))
        .clipShape(RoundedRectangle(cornerRadius: 10))
        .accessibilityElement(children: .combine)
    }

    @MainActor
    private func checkHotspot() async {
        guard !checking else { return }
        checking = true
        defer { checking = false }
        await model.refreshHotspot()
        checkedAt = Date()
    }

}


struct HotspotQuickControls: View {
    @EnvironmentObject private var model: AppModel
    let onRules: () -> Void
    let onGuide: () -> Void
    @State private var showCredentialHelp = false
    @State private var copied = false
    @State private var checking = false

    private var activeDevices: Int {
        Set((model.stats?.relay.active ?? []).filter {
            !$0.isHTTPProxy && model.stats?.hotspot?.containsClient($0.srcIP) == true
        }.map(\.srcIP)).count
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack {
                Label("代理 Wi-Fi", systemImage: "wifi").font(.headline)
                Spacer()
                Button {
                    Task {
                        checking = true
                        await model.refreshHotspot()
                        await model.refresh(silent: true)
                        checking = false
                    }
                } label: {
                    Image(systemName: "arrow.clockwise")
                }.disabled(checking).help("刷新热点状态")
            }
            Label(model.hotspotControlState.title,
                  systemImage: model.hotspotControlState == .enabled ? "checkmark.circle.fill" : "info.circle")
                .font(.caption).foregroundStyle(model.hotspotControlState == .enabled ? Theme.lime : Theme.yellow)
            Text("\(activeDevices) 台设备正在使用 · 地址自动分配")
                .font(.caption).foregroundStyle(Theme.muted)
            Divider()
            Button {
                showCredentialHelp.toggle()
            } label: {
                Label("Wi-Fi 名称、密码与频段…", systemImage: "wifi")
            }.buttonStyle(.bordered)
            if showCredentialHelp {
                VStack(alignment: .leading, spacing: 8) {
                    DisclosureGroup("主机版本与频段建议") {
                        HotspotSystemSetupGuide()
                    }
                    Text("名称、密码和频段由 macOS 管理，请以系统 Wi-Fi 选项中的内容为准。")
                    Text("进入「互联网共享」详情 →「Wi-Fi 选项」。如果选项变灰，需先关闭互联网共享；热点设备会暂时断网。查看或修改后，请重新开启共享。")
                    Button("打开系统共享设置") {
                        if let url = URL(string: "x-apple.systempreferences:com.apple.Sharing-Settings.extension") {
                            NSWorkspace.shared.open(url)
                        }
                    }.buttonStyle(.borderedProminent)
                }.font(.caption).foregroundStyle(Theme.muted)
                    .fixedSize(horizontal: false, vertical: true)
            }
            HStack {
                Text("热点网关").foregroundStyle(Theme.muted)
                Spacer()
                Text(model.stats?.hotspot?.ip.nonEmpty ?? "未检测到").monospaced()
                Button(copied ? "已复制" : "复制") {
                    guard let address = model.stats?.hotspot?.ip.nonEmpty else { return }
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(address, forType: .string)
                    copied = true
                }.disabled(model.stats?.hotspot?.ip.nonEmpty == nil)
            }.font(.caption)
            Button(action: onRules) {
                Label("管理共用分流规则", systemImage: "arrow.triangle.branch")
            }
            Button(action: onGuide) {
                Label("连接 Switch / PS5 · 接入指南", systemImage: "gamecontroller")
            }
            Divider()
            if model.hotspotControlState.isWorking {
                HStack { ProgressView().controlSize(.small); Text(model.hotspotControlState.title).font(.caption) }
            } else {
                Button(model.hotspotControlState == .enabled ? "停止热点接管" : "开启热点接管") {
                    Task { await model.configureHotspot(model.hotspotControlState == .enabled ? "disable" : "enable") }
                }
                .disabled(model.isBusy || (model.hotspotControlState != .enabled && (model.hotspot?.available != true)))
                .buttonStyle(.bordered)
            }
            Text("停止接管只停止本 App 管理热点流量，Wi-Fi 仍由系统共享。")
                .font(.caption2).foregroundStyle(Theme.muted)
            if let error = model.hotspotActionError {
                Text(error).font(.caption).foregroundStyle(Theme.yellow)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .padding(18).frame(width: 360)
        .task { await model.refreshHotspot() }
    }
}

/// Advice only: selecting a recommendation does not change system Wi-Fi settings.
struct HotspotSystemSetupGuide: View {
    @AppStorage("hotspot.prefer5GHz") private var prefer5GHz = true
    @AppStorage("hotspot.consoleModel") private var consoleModel = "switch"
    @AppStorage("hotspot.consoleRegion") private var consoleRegion = "HK"

    private var regionAdvice: String {
        switch consoleRegion {
        case "JP": return "日版：先试 36，再试 40 / 44 / 48；不要直接套用其他地区的高信道设置。"
        case "US": return "美版：先试 36，再试 40 / 44 / 48；回退 2.4 GHz 时选择 1 / 6 / 11。"
        case "OTHER": return "其他销售地区：请核对主机说明书，在 Mac 提供的合法信道中选择，并在主机上测试连接。"
        default: return "港版（默认）：PS5 / Switch 优先尝试 5 GHz 信道 36，找不到热点时再试 40 / 44 / 48。"
        }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Label("Wi-Fi 设置建议", systemImage: "slider.horizontal.3")
                .font(.headline)
            Picker("主机型号", selection: $consoleModel) {
                Text("PS5 / Slim").tag("ps5")
                Text("PS5 Pro").tag("ps5pro")
                Text("Switch / Lite / OLED").tag("switch")
                Text("Switch 2").tag("switch2")
            }
            Text(consoleModel == "switch" ? "Switch / Lite / OLED：优先使用兼容 WPA2 的安全模式。" : "采用兼容性优先的 2.4 / 5 GHz 建议；不假定系统热点提供主机的全部无线能力。")
                .foregroundStyle(Theme.muted)
            Picker("主机销售版本", selection: $consoleRegion) {
                Text("港版").tag("HK")
                Text("日版").tag("JP")
                Text("美版").tag("US")
                Text("其他").tag("OTHER")
            }.pickerStyle(.segmented)
            Text(regionAdvice)
            Text("按主机购买版本选择，不是账号地区；此选项只调整引导，不修改 Mac 的无线地区。")
                .foregroundStyle(Theme.muted)
            Picker("使用场景", selection: $prefer5GHz) {
                Text("近距离 · 5 GHz").tag(true)
                Text("隔墙 / 兼容 · 2.4 GHz").tag(false)
            }.pickerStyle(.segmented)
            Text(prefer5GHz
                 ? "在系统「Wi-Fi 选项 → 频道」中优先选择 36；也可选择 40、44 或 48（以系统提供的选项为准）。这些是 5 GHz 信道。"
                 : "在系统「Wi-Fi 选项 → 频道」中选择 1、6 或 11。这些是 2.4 GHz 信道；设备找不到 5 GHz 热点时可尝试此模式。")
            Text("共享来源选「以太网」，共享给「Wi-Fi」。设置容易辨认的名称及至少 8 位密码。Switch / Lite / OLED 需要 WPA2 兼容：在 Mac 上选择「WPA2/WPA3 个人级」，不要选择仅 WPA3。")
            Text("上述为兼容性优先的建议，不是各版本完整支持列表。信道以 Mac 所在地区可选项为准；找不到热点时先靠近 Mac，仍不可见再回退 2.4 GHz 的 1 / 6 / 11，并在主机上测试。")
                .foregroundStyle(Theme.muted)
            Text("点击「好」「完成」后开启互联网共享。修改频道后，需关闭再开启共享才能应用，已连接设备会暂时断网。")
            Text("这里仅展示设置建议，不会自动切换频段，也不代表同时开启双频热点。系统热点设置完成后，App 会自动检测；代理出口、规则与接管由本 App 管理。")
                .foregroundStyle(Theme.muted)
        }
        .font(.caption)
        .fixedSize(horizontal: false, vertical: true)
        .padding(12)
        .background(Theme.panelRaised)
        .clipShape(RoundedRectangle(cornerRadius: 10))
    }
}
