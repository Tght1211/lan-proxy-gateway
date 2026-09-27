import Charts
import SwiftUI

struct DeviceOnboardingSheet: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.dismiss) private var dismiss
    @State private var method = "hotspot"
    @State private var advanced = false
    @State private var batch = 0
    @State private var candidates: [String] = []
    @State private var probing = false
    @State private var probed = false

    private var pool: [String] {
        suggestedDeviceIPPool(
            gateway: model.status?.gateway.localIP ?? "",
            occupied: Set(model.stats?.relay.devices.map(\.name) ?? [])
        )
    }

    var body: some View {
        let gateway = model.status?.gateway.localIP.nonEmpty ?? "--"
        VStack(spacing: 0) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 3) {
                    Text("接入新设备").font(.title3.weight(.semibold))
                    Text("Switch / PS5 推荐连接代理 Wi-Fi，地址由系统自动分配。")
                        .font(.caption).foregroundStyle(Theme.muted)
                }
                Spacer()
                Button { dismiss() } label: { Image(systemName: "xmark") }
                    .buttonStyle(IconButtonStyle())
            }
            .padding(20)
            .background(Theme.panel)
            Divider().overlay(Theme.border)

            HStack {
                Button("代理 Wi-Fi · 推荐") { method = "hotspot"; advanced = false }
                    .buttonStyle(.borderedProminent)
                Spacer()
                Button(advanced ? "收起其他方式" : "其他接入方式…") {
                    advanced.toggle()
                    method = advanced ? "gateway" : "hotspot"
                }.buttonStyle(.bordered)
            }.padding(.horizontal, 20).padding(.top, 16)
            if advanced {
                Picker("其他接入方式", selection: $method) {
                    Text("手动网关").tag("gateway")
                    Text("HTTP 代理").tag("http")
                    Text("自动代理 PAC").tag("pac")
                }.pickerStyle(.segmented).padding(.horizontal, 20).padding(.top, 8)
            }

            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    if method == "hotspot" {
                        HotspotOnboardingPanel()
                    } else if method == "gateway" {
                        if model.status?.accessMode == "hotspot" {
                            Text("当前只接管代理 Wi-Fi。使用下面的手动方式前，请先切换接入模式。")
                                .font(.caption).foregroundStyle(Theme.yellow)
                            Button("切换为手动网关接入") {
                                Task { await model.configureHotspot("use-lan") }
                            }.disabled(model.isBusy)
                        }

                        VStack(alignment: .leading, spacing: 0) {
                            OnboardingStep(number: "1", title: "打开设备的网络设置", detail: "Switch / PS5 / Apple TV / 手机的 Wi-Fi 或有线网络里，选择「手动 / 静态 IP」")
                            OnboardingStep(number: "2", title: "填写一个候选 IP", detail: probed ? "以下地址暂未响应探测，不代表未被占用；请先在路由器中预留" : "高级设置：请在路由器中预留设备地址，避免与自动分配冲突")
                            HStack(spacing: 6) {
                                Text(suggestionPrefix(candidates.isEmpty ? pool : candidates))
                                    .font(.system(size: 13, weight: .semibold, design: .monospaced))
                                ForEach(candidates, id: \.self) { address in
                                    Button {
                                        NSPasteboard.general.clearContents()
                                        NSPasteboard.general.setString(address, forType: .string)
                                    } label: {
                                        Text(".\(address.split(separator: ".").last.map(String.init) ?? address)")
                                            .font(.system(size: 13, weight: .semibold, design: .monospaced))
                                            .foregroundStyle(Theme.cyan)
                                            .padding(.horizontal, 10).frame(height: 28)
                                            .background(Theme.cyan.opacity(0.08))
                                            .overlay(RoundedRectangle(cornerRadius: 5).stroke(Theme.cyan.opacity(0.35), lineWidth: 0.8))
                                            .clipShape(RoundedRectangle(cornerRadius: 5))
                                    }
                                    .buttonStyle(.plain)
                                    .help("点击复制 \(address)")
                                }
                                if probing {
                                    ProgressView().controlSize(.small)
                                    Text("正在探测占用...").font(.caption2).foregroundStyle(Theme.muted)
                                } else if candidates.isEmpty {
                                    Text("本批候选均被占用，请换一批").font(.caption).foregroundStyle(Theme.yellow)
                                }
                                Spacer(minLength: 8)
                                Button {
                                    batch += 1
                                    refreshCandidates()
                                } label: {
                                    Label("换一批", systemImage: "arrow.triangle.2.circlepath").font(.caption)
                                }
                                .buttonStyle(.bordered).controlSize(.small)
                                .disabled(probing || pool.count <= 5)
                            }
                            .padding(.leading, 34).padding(.bottom, 14)
                            OnboardingStep(number: "3", title: "网关和 DNS 都填写本机地址", detail: "网关指向旁路由流量才会经过它；DNS 也指向旁路由才能识别域名、按域名分流。主路由无需任何改动")
                            HStack(spacing: 10) {
                                SetupValue("网关 / DNS", gateway)
                                SetupValue("子网掩码", "255.255.255.0")
                            }
                            .padding(.leading, 34).padding(.bottom, 14)
                            OnboardingStep(number: "4", title: "保存并测试网络", detail: "设备上的代理设置保持关闭或不填写；连通后会自动出现在设备列表中")
                        }
                        Label("候选地址请确认不在路由器 DHCP 分配范围内，避免地址冲突。", systemImage: "exclamationmark.triangle")
                            .font(.caption).foregroundStyle(Theme.yellow)
                    } else {
                        proxyInstructions(gateway: gateway)
                    }
                }
                .padding(20)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)

            Divider().overlay(Theme.border)
            HStack {
                Spacer()
                Button("完成") { dismiss() }.buttonStyle(ActionButtonStyle(tint: Theme.cyan))
            }
            .padding(.horizontal, 20).padding(.vertical, 14)
            .background(Theme.panel)
        }
        .frame(width: 700, height: 720)
        .background(Theme.canvasBackground)
        .onChange(of: method) { value in
            if value == "gateway" { refreshCandidates() }
        }
    }

    @ViewBuilder
    private func proxyInstructions(gateway: String) -> some View {
        let proxy = model.status?.httpProxy
        if proxy?.enabled != true {
            Text("请先在设置中开启局域网 HTTP 代理并保存。")
                .foregroundStyle(Theme.yellow)
            Button("前往代理设置") {
                model.selectedSection = .settings
                dismiss()
            }.buttonStyle(.bordered)
        } else if let proxy {
            OnboardingStep(number: "1", title: "连接同一局域网", detail: "设备保持原来的 IP、网关和 DNS 设置。")
            if method == "pac" {
                OnboardingStep(number: "2", title: "选择自动代理配置", detail: "在设备 Wi-Fi 或系统代理设置中，选择「自动 / PAC」，填入以下网址。")
                SetupValue("PAC 网址", "http://\(gateway):\(proxy.port)/proxy.pac")
                Text("PAC 负责告诉设备使用哪个代理，分流仍由本网关处理；与手动方式共用端口，无需启动额外服务。")
                    .font(.caption).foregroundStyle(Theme.muted)
            } else {
            OnboardingStep(number: "2", title: "选择手动 HTTP 代理", detail: "HTTP 与 HTTPS 代理都填写以下服务器和端口。")
            HStack {
                SetupValue("服务器", gateway)
                SetupValue("端口", String(proxy.port))
            }
            }
            OnboardingStep(number: "3", title: proxy.auth == "basic" ? "填写代理认证" : "无需认证", detail: proxy.auth == "basic" ? "用户名：\(proxy.username)。密码使用代理设置中保存的密码，在客户端代理认证中填写。" : "客户端无需填写用户名和密码。")
            if method == "pac" && proxy.auth == "basic" {
                Text("PAC 不包含用户名和密码；设备还需支持代理认证。若系统的自动代理模式无法输入或弹出认证，请使用手动代理。")
                    .font(.caption).foregroundStyle(Theme.yellow)
            }
            OnboardingStep(number: "4", title: "保存并访问网页", detail: "仅遵循系统或应用代理设置的流量会经过这里。连接显示在拓扑的「HTTP 代理」入口。")
            Text("外网接入需要另行配置 TCP 隧道，再将地址和端口替换为隧道入口。")
                .font(.caption).foregroundStyle(Theme.muted)
            if !model.isRunning {
                Text("核心尚未运行，请启动后再连接。").font(.caption).foregroundStyle(Theme.yellow)
            }
        }
    }

    private func refreshCandidates() {
        let all = pool
        guard !all.isEmpty else {
            candidates = []
            return
        }
        let batchCount = (all.count + 4) / 5
        let start = (batch % batchCount) * 5
        let slice = Array(all.dropFirst(start).prefix(5))
        candidates = slice
        probed = false
        probing = true
        Task {
            let occupied = await probeOccupiedAddresses(slice)
            candidates = slice.filter { !occupied.contains($0) }
            probing = false
            probed = true
        }
    }
}

struct OnboardingStep: View {
    let number: String
    let title: String
    let detail: String

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Text(number).font(.caption.weight(.bold)).foregroundStyle(Color.white)
                .frame(width: 22, height: 22).background(Theme.cyan).clipShape(Circle())
            VStack(alignment: .leading, spacing: 3) {
                Text(title).font(.system(size: 13, weight: .semibold))
                if !detail.isEmpty {
                    Text(detail).font(.caption).foregroundStyle(Theme.muted)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }
            Spacer(minLength: 0)
        }
        .padding(.bottom, 12)
    }
}
