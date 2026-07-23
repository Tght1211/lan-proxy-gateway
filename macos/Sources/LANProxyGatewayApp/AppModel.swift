import AppKit
import Foundation

@MainActor
final class AppModel: ObservableObject {
    @Published var status: GatewayStatus?
    @Published var stats: RuntimeStats?
    @Published var selectedSection: AppSection? = .overview
    @Published var proxyType = "socks5"
    @Published var proxyHost = "127.0.0.1"
    @Published var proxyPort = 7897
    @Published var logText = "正在读取日志..."
    @Published var serviceStatus = "正在检查..."
    @Published var isServiceInstalled = false
    @Published var deviceLabels: [String: String] = [:]
    @Published var isBusy = false
    @Published var notice: String?
    @Published var errorMessage: String?
    @Published var coreUpgradeRecommended = false
    @Published var themeID: String = UserDefaults.standard.string(forKey: "appThemeID") ?? "light" {
        didSet { UserDefaults.standard.set(themeID, forKey: "appThemeID") }
    }

    private let client = GatewayClient()
    private var timer: Timer?
    private var isRefreshing = false
    private var isLiveScrolling = false
    private var didLoadProxyConfig = false
    private var noticeTask: Task<Void, Never>?
    private let deviceLabelsKey = "deviceLabels"

    init() {
        if let stored = UserDefaults.standard.dictionary(forKey: deviceLabelsKey) as? [String: String] {
            deviceLabels = stored
        }
        NotificationCenter.default.addObserver(forName: NSScrollView.willStartLiveScrollNotification, object: nil, queue: .main) { [weak self] _ in
            self?.isLiveScrolling = true
        }
        NotificationCenter.default.addObserver(forName: NSScrollView.didEndLiveScrollNotification, object: nil, queue: .main) { [weak self] _ in
            self?.isLiveScrolling = false
        }
        timer = Timer.scheduledTimer(withTimeInterval: 3, repeats: true) { [weak self] _ in
            Task { @MainActor in
                guard let self, !self.isLiveScrolling else { return }
                await self.refresh(silent: true)
            }
        }
        Task { await refresh(silent: true) }
    }

    deinit {
        timer?.invalidate()
        noticeTask?.cancel()
    }

    var isRunning: Bool { status?.running == true }
    var isConfigured: Bool { status?.configured == true }
    var activeDeviceCount: Int {
        Set(stats?.relay.active.map(\.srcIP) ?? []).count
    }

    func refresh(silent: Bool = false) async {
        guard !isRefreshing else { return }
        isRefreshing = true
        defer { isRefreshing = false }
        do {
            let latest = try await client.status()
            status = latest
            loadProxyConfigIfNeeded(from: latest)
            if latest.running {
                do {
                    let runtime = try await client.stats(apiPort: latest.ports.api)
                    stats = runtime
                    coreUpgradeRecommended = runtime.schemaVersion != 3
                } catch {
                    stats = nil
                    coreUpgradeRecommended = true
                }
            } else {
                stats = nil
                coreUpgradeRecommended = false
            }
            if selectedSection == .settings {
                let latestLog = await client.readLog(path: latest.logFile)
                if latestLog != logText { logText = latestLog }
            }
            if !silent { showNotice("状态已刷新") }
        } catch {
            if !silent { errorMessage = error.localizedDescription }
        }
    }

    func initializeAndStart() {
        perform("核心服务已启动") {
            if !self.isConfigured { _ = try await self.client.initialize() }
            return try await self.client.start()
        }
    }

    func stop() {
        perform("核心服务已停止") { try await self.client.stop() }
    }

    func restart() {
        perform("核心服务已重启") { try await self.client.restart() }
    }

    func applyProxy() {
        let host = proxyHost.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !host.isEmpty, (1...65535).contains(proxyPort) else {
            errorMessage = "请输入有效的代理地址和端口。"
            return
        }
        perform("代理出口已更新") {
            try await self.client.setProxy(type: self.proxyType, host: host, port: self.proxyPort)
        }
    }

    @discardableResult
    func applyProxyAsync() async -> Bool {
        let host = proxyHost.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !host.isEmpty, (1...65535).contains(proxyPort) else {
            errorMessage = "请输入有效的代理地址和端口。"
            return false
        }
        return await performAsync("代理出口已更新") {
            try await self.client.setProxy(type: self.proxyType, host: host, port: self.proxyPort)
        }
    }

    @discardableResult
    func useDirectConnectionAsync() async -> Bool {
        await performAsync("已切换为直连出口") { try await self.client.setDirect() }
    }

    func testProxyAsync() async -> (ok: Bool, message: String) {
        let host = proxyHost.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !host.isEmpty, (1...65535).contains(proxyPort) else {
            return (false, "请输入有效的代理地址和端口")
        }
        do {
            _ = try await client.testProxy(type: proxyType, host: host, port: proxyPort)
            return (true, "代理连通正常")
        } catch {
            return (false, error.localizedDescription)
        }
    }

    func useDirectConnection() {
        perform("已切换为直连出口") { try await self.client.setDirect() }
    }

    @discardableResult
    func applyRoutingRules(_ rules: [RoutingRule]) async -> Bool {
        await performAsync("分流规则已更新") { try await self.client.setRoutingRules(rules) }
    }

    private func performAsync(_ success: String, operation: @escaping () async throws -> String) async -> Bool {
        guard !isBusy else { return false }
        isBusy = true
        notice = nil
        errorMessage = nil
        defer { isBusy = false }
        do {
            _ = try await operation()
            showNotice(success)
            await refresh(silent: true)
            return true
        } catch {
            errorMessage = error.localizedDescription
            return false
        }
    }

    func updateServiceStatus() {
        Task {
            let latest = (try? await client.serviceStatus()) ?? "未安装"
            serviceStatus = latest
            isServiceInstalled = latest != "未安装" && latest != "inactive"
        }
    }

    func setServiceEnabled(_ enabled: Bool) {
        if enabled { installService() } else { uninstallService() }
    }

    func deviceLabel(for ip: String) -> String { deviceLabels[ip] ?? "" }

    func setDeviceLabel(_ label: String, for ip: String) {
        let value = label.trimmingCharacters(in: .whitespacesAndNewlines)
        if value.isEmpty { deviceLabels.removeValue(forKey: ip) } else { deviceLabels[ip] = value }
        UserDefaults.standard.set(deviceLabels, forKey: deviceLabelsKey)
    }

    func installService() {
        perform("开机自启已安装") { try await self.client.installService() }
    }

    func uninstallService() {
        perform("开机自启已移除") { try await self.client.uninstallService() }
    }

    func installCLI() {
        perform("CLI 已安装到 /usr/local/bin/gateway") { try await self.client.installCLI() }
    }

    func reloadLog() {
        guard let path = status?.logFile else { return }
        Task {
            let latestLog = await client.readLog(path: path)
            if latestLog != logText { logText = latestLog }
        }
    }

    var appVersion: String {
        Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "dev"
    }

    @Published var updateStatus: String?
    @Published var updateAvailable = false
    @Published var isCheckingUpdate = false

    func checkForUpdates() {
        guard !isCheckingUpdate else { return }
        isCheckingUpdate = true
        updateStatus = "正在检查..."
        updateAvailable = false
        Task {
            defer { isCheckingUpdate = false }
            do {
                var request = URLRequest(url: URL(string: "https://api.github.com/repos/Tght1211/lan-proxy-gateway/releases/latest")!)
                request.setValue("lan-proxy-gateway-app", forHTTPHeaderField: "User-Agent")
                request.timeoutInterval = 15
                let (data, _) = try await URLSession.shared.data(for: request)
                struct Release: Decodable { let tag_name: String }
                let latest = try JSONDecoder().decode(Release.self, from: data).tag_name
                let current = appVersion.hasPrefix("v") ? appVersion : "v\(appVersion)"
                if latest == current || latest == appVersion {
                    updateStatus = "已是最新版本（\(latest)）"
                } else {
                    updateStatus = "发现新版本 \(latest)，当前 \(current)"
                    updateAvailable = true
                }
            } catch {
                updateStatus = "检查失败：\(error.localizedDescription)"
            }
        }
    }

    func revealLog() {
        guard let path = status?.logFile else { return }
        NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: path)])
    }

    private func perform(_ success: String, operation: @escaping () async throws -> String) {
        guard !isBusy else { return }
        isBusy = true
        notice = nil
        errorMessage = nil
        Task {
            do {
                _ = try await operation()
                showNotice(success)
                await refresh(silent: true)
                updateServiceStatus()
            } catch {
                errorMessage = error.localizedDescription
            }
            isBusy = false
        }
    }

    private func loadProxyConfigIfNeeded(from status: GatewayStatus) {
        guard !didLoadProxyConfig else { return }
        didLoadProxyConfig = true
        guard let proxy = status.proxy else { return }
        let parts = proxy.split(separator: " ", maxSplits: 1).map(String.init)
        guard parts.count == 2, let separator = parts[1].lastIndex(of: ":"),
              let port = Int(parts[1][parts[1].index(after: separator)...]) else { return }
        proxyType = parts[0]
        proxyHost = String(parts[1][..<separator])
        proxyPort = port
    }

    private func showNotice(_ text: String) {
        noticeTask?.cancel()
        notice = text
        noticeTask = Task {
            try? await Task.sleep(for: .seconds(4))
            guard !Task.isCancelled else { return }
            notice = nil
        }
    }
}
