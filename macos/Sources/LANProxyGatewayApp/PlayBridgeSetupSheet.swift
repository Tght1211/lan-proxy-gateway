import AppKit
import SwiftUI

/// Only explicit action buttons call the gateway. Moving between steps is presentation-only.
struct PlayBridgeSetupSheet: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.dismiss) private var dismiss
    @State private var step = 0
    @State private var showProxy = false
    @State private var checking = false
    @State private var showAdvice = false
    private let steps = ["创建热点", "选择出口", "连接主机"]

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack {
                VStack(alignment: .leading, spacing: 6) {
                    Text("准备你的游戏热点").font(.title2.weight(.bold))
                    Text("Mac 通过以太网上网，再将网络共享给游戏机。")
                        .font(.subheadline).foregroundStyle(.secondary)
                }
                Spacer()
                Button { dismiss() } label: { Image(systemName: "xmark") }.buttonStyle(.plain)
                    .accessibilityLabel("关闭接入引导")
            }.padding(24)
            HStack(spacing: 12) {
                ForEach(steps.indices, id: \.self) { index in
                    Button { step = index } label: {
                        HStack(spacing: 8) {
                            Text("\(index + 1)").font(.caption.weight(.bold))
                                .frame(width: 25, height: 25)
                                .background(step == index ? Theme.cyan : Theme.cyan.opacity(0.08))
                                .foregroundStyle(step == index ? .white : Theme.cyan).clipShape(Circle())
                            Text(steps[index]).font(.subheadline.weight(.medium))
                        }.frame(maxWidth: .infinity).padding(10)
                            .background(step == index ? Theme.cyan.opacity(0.07) : .clear)
                            .clipShape(RoundedRectangle(cornerRadius: 12))
                    }.buttonStyle(.plain)
                }
            }.padding(.horizontal, 24).padding(.bottom, 20)
            Divider()
            ScrollView {
                VStack(alignment: .leading, spacing: 20) {
                    switch step {
                    case 0: createHotspot
                    case 1: chooseSource
                    default: connectConsole
                    }
                }.padding(24).frame(maxWidth: .infinity, alignment: .leading)
            }
            Divider()
            HStack {
                if step > 0 { Button("上一步") { step -= 1 } }
                Spacer()
                if step < 2 {
                    Button("下一步") { step += 1 }.buttonStyle(.borderedProminent)
                } else {
                    Button("完成") { dismiss() }.buttonStyle(.borderedProminent)
                }
            }.padding(20)
        }.frame(width: 690, height: 660).background(Theme.canvas)
            .sheet(isPresented: $showProxy) { ProxyConfigSheet().environmentObject(model) }
            .task { await model.refreshHotspot() }
    }

    private var createHotspot: some View {
        VStack(alignment: .leading, spacing: 18) {
            Label("先在 macOS 开启互联网共享", systemImage: "wifi").font(.headline)
            Text("打开 Wi-Fi，在「系统设置 → 通用 → 共享 → 互联网共享」中，从以太网共享给 Wi-Fi。设置名称与密码，保存后开启共享。")
                .foregroundStyle(.secondary)
            HStack {
                Button("打开系统共享设置") {
                    if let url = URL(string: "x-apple.systempreferences:com.apple.Sharing-Settings.extension") { NSWorkspace.shared.open(url) }
                }.buttonStyle(.borderedProminent)
                Button(checking ? "检测中…" : "重新检测") {
                    checking = true
                    Task { await model.refreshHotspot(); checking = false }
                }.disabled(checking)
            }
            Label(model.hotspotError ?? model.hotspot?.message ?? "等待检测热点", systemImage: model.hotspot?.available == true ? "checkmark.circle" : "info.circle")
                .font(.subheadline).foregroundStyle(model.hotspot?.available == true ? Theme.lime : .secondary)
            DisclosureGroup("机型、销售地区与 Wi-Fi 信道建议", isExpanded: $showAdvice) { HotspotSystemSetupGuide().padding(.top, 12) }
            Text("默认参考港版主机：近距离先试 5 GHz 信道 36；需要更广兼容性时试 2.4 GHz 信道 1 / 6 / 11。其他销售地区请展开上面的建议。")
                .font(.caption).foregroundStyle(.secondary)
            Text("实际频段与信道以 Mac 提供的选项为准。这不是双频同时开启开关。")
                .font(.caption).foregroundStyle(.secondary)
        }
    }

    private var chooseSource: some View {
        VStack(alignment: .leading, spacing: 20) {
            Label("直接上网，或使用你自己的加速源", systemImage: "arrow.triangle.branch").font(.headline)
            Panel {
                VStack(alignment: .leading, spacing: 12) {
                    Text(model.status?.egress == "proxy" ? "当前默认出口：代理" : "当前默认出口：直连").font(.headline)
                    Text(model.status?.egress == "proxy" ? (model.status?.proxy ?? "已配置") : "无需填写代理，也可以继续使用热点。")
                        .foregroundStyle(.secondary)
                    Button("配置出口…") { showProxy = true }.buttonStyle(.bordered)
                }
            }
            Text("如果希望使用 VPN / 代理线路，请先在你自己的代理软件里开启 SOCKS5 或 HTTP 端口，再填写 IP 地址和端口。PlayBridge 不提供节点或订阅。")
                .foregroundStyle(.secondary)
            Text("已有分流规则继续生效；更改默认出口不代表覆盖每一条规则。")
                .font(.caption).foregroundStyle(.secondary)
        }
    }

    private var connectConsole: some View {
        VStack(alignment: .leading, spacing: 20) {
            Label("主机连接 Wi-Fi，地址保持自动获取", systemImage: "gamecontroller").font(.headline)
            Text("在 PS5 / Switch 的网络设置中选择刚才创建的 Wi-Fi，输入密码。IP、网关、DNS 保持自动，主机端不需要填写代理。")
                .foregroundStyle(.secondary)
            Panel {
                VStack(alignment: .leading, spacing: 12) {
                    Label(model.hotspotControlState.title, systemImage: model.hotspotControlState == .enabled ? "checkmark.circle.fill" : "wifi")
                        .font(.headline).foregroundStyle(Theme.cyan)
                    Text(model.hotspotActionError ?? model.hotspotError ?? model.stats?.hotspot?.message ?? model.hotspot?.message ?? "等待系统热点")
                        .font(.subheadline).foregroundStyle(.secondary)
                    if model.hotspotControlState != .enabled {
                        Button(model.hotspotControlState.isWorking ? model.hotspotControlState.title : "开启热点接管") {
                            Task { await model.configureHotspot("enable") }
                        }.buttonStyle(.borderedProminent)
                            .disabled(model.isBusy || model.hotspot?.available != true)
                        Text("此操作会切换为热点接入并重启核心，现有连接可能暂时中断。仅切换界面模式不会执行此操作。")
                            .font(.caption).foregroundStyle(.secondary)
                    }
                }
            }
            Text("连接后在主机上测试互联网连接。首页会显示核心记录的设备和流量；未经过核心的流量不计入。")
                .font(.subheadline).foregroundStyle(.secondary)
        }
    }
}
