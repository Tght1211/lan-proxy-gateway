import AppKit
import SwiftUI

struct LANHTTPProxySettingsPanel: View {
    @EnvironmentObject private var model: AppModel
    @State private var enabled = false
    @State private var port = "17894"
    @State private var auth = "none"
    @State private var username = ""
    @State private var password = ""
    @State private var passwordSet = false
    @State private var loaded = false
    @State private var saving = false
    @State private var result = ""

    private var address: String { model.status?.gateway.localIP ?? "--" }
    private var valid: Bool {
        guard let number = Int(port), (1...65535).contains(number) else { return false }
        return auth == "none" || (!username.isEmpty && !username.contains(":") && (!password.isEmpty || passwordSet))
    }
    private var runtimeText: String {
        guard model.status?.httpProxy?.enabled == true else { return "未开启" }
        guard model.isRunning else { return "已配置，启动核心后生效" }
        if let active = model.stats?.httpProxy, active != model.status?.httpProxy {
            return "配置尚未生效，请检查端口和运行日志"
        }
        if model.stats?.components?.contains(where: { $0.name == "http-proxy" && $0.running }) == true {
            return "正在监听"
        }
        return "等待核心应用配置；如持续未就绪，请检查日志或更新核心"
    }

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 16) {
                HStack {
                    Text("局域网 HTTP 代理").sectionLabel()
                    Spacer()
                    Text(runtimeText).font(.caption).foregroundStyle(Theme.muted)
                }
                Text("其他设备在 Wi-Fi 或浏览器中填写本机地址和端口，即可使用当前网关出口，无需修改设备的网关或 DNS。支持 HTTP 和 HTTPS；仅使用系统或应用代理的流量会经过这里。")
                    .font(.caption).foregroundStyle(Theme.muted)
                SettingsRow(title: "启用代理服务", detail: "使用当前直连 / VPN 代理出口及分流规则") {
                    Toggle("启用", isOn: $enabled).labelsHidden().toggleStyle(.switch)
                }
                SettingsRow(title: "服务器地址", detail: address) {
                    Button("复制地址") {
                        NSPasteboard.general.clearContents()
                        NSPasteboard.general.setString(address, forType: .string)
                    }.buttonStyle(.bordered)
                }
                SettingsRow(title: "HTTP 代理端口", detail: "客户端的 HTTP 与 HTTPS 代理填写同一端口") {
                    TextField("17894", text: $port).textFieldStyle(.roundedBorder).frame(width: 140)
                }
                if let saved = model.status?.httpProxy, saved.enabled {
                    SettingsRow(title: "自动代理 PAC", detail: "使用已保存的端口；PAC 与手动代理共用服务") {
                        Button("复制 PAC 网址") {
                            NSPasteboard.general.clearContents()
                            NSPasteboard.general.setString("http://\(address):\(saved.port)/proxy.pac", forType: .string)
                        }.buttonStyle(.bordered)
                    }
                }
                SettingsRow(title: "认证方式", detail: "这是局域网设备的接入凭据，与上游 VPN 代理账号独立") {
                    Picker("认证", selection: $auth) {
                        Text("无需认证").tag("none")
                        Text("用户名和密码").tag("basic")
                    }.labelsHidden().frame(width: 190)
                }
                if auth == "basic" {
                    SettingsRow(title: "用户名", detail: "客户端代理认证用户名") {
                        TextField("用户名", text: $username).textFieldStyle(.roundedBorder).frame(width: 220)
                    }
                    SettingsRow(title: "密码", detail: passwordSet ? "已设置；留空保留原密码" : "请输入接入密码") {
                        SecureField(passwordSet ? "留空保留原密码" : "密码", text: $password)
                            .textFieldStyle(.roundedBorder).frame(width: 220)
                    }
                }
                if model.status?.egress == "direct" {
                    Text("当前出口为直连。要共享 VPN，请先在网络总览中将出口切换到本机 VPN 的 HTTP/SOCKS5 代理。")
                        .font(.caption).foregroundStyle(Theme.yellow)
                }
                HStack {
                    Text(result).font(.caption).foregroundStyle(Theme.muted)
                    Spacer()
                    Button(saving ? "保存中…" : "保存代理设置") { save() }
                        .buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                        .disabled(!valid || saving || !model.isConfigured)
                }
            }
            .disabled(saving)
        }
        .onAppear { load() }
        .onChange(of: model.status?.httpProxy) { _ in if !loaded { load() } }
    }

    private func load() {
        guard let current = model.status?.httpProxy else { return }
        enabled = current.enabled
        port = String(current.port)
        auth = current.auth
        username = current.username
        passwordSet = current.passwordSet
        loaded = true
    }

    private func save() {
        guard let port = Int(port), valid else { return }
        saving = true
        result = ""
        Task {
            defer { saving = false }
            do {
                result = try await model.client.setLANHTTPProxy(
                    enabled: enabled, port: port, auth: auth, username: username,
                    password: password.isEmpty && passwordSet ? nil : password
                )
                password = ""
                await model.refresh(silent: true)
                load()
            } catch {
                result = "保存失败：\(error.localizedDescription)"
            }
        }
    }
}
