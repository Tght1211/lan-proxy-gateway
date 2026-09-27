import AppKit
import Foundation

@MainActor
final class AppModel: ObservableObject {
    @Published var hotspot: HotspotStatus?
    @Published var hotspotError: String?
    @Published var hotspotOperation: String?
    @Published var hotspotActionError: String?

    var hotspotControlState: HotspotControlState {
        HotspotControlState.resolve(operation: hotspotOperation, error: hotspotActionError,
            desired: hotspot?.enabled == true, running: isRunning,
            hasStats: stats != nil, runtime: stats?.hotspot)
    }

    var hotspotControlDetail: String {
        switch hotspotControlState {
        case .off: return "发现 Wi-Fi 不代表已接管。点击开启后，游戏机流量才会按本 App 的规则转发。"
        case .enabling: return "正在更新并重启后台核心。若系统弹出管理员授权，请完成授权；不要重复点击。"
        case .stopping: return "正在撤销接管。系统 Wi-Fi 共享会保留，可继续提供普通网络。"
        case .verifying: return "正在读取运行中核心的确认结果，保存配置并不代表已经生效。"
        case .enabled: return "后台核心已确认流量接管。现在可连接热点，在游戏机上测试网络。"
        case .waiting: return stats?.hotspot?.message ?? hotspot?.message ?? "等待共享网络和核心准备就绪。"
        case .needsCoreUpdate: return "配置已保存，但运行中的旧核心不支持热点状态。点击「更新核心并开启接管」完成更新。"
        case .unavailable: return "核心尚未运行或状态接口不可用，不能确认已开启。请重试；不要把 Wi-Fi 已连接当作代理已生效。"
        case .failed: return hotspotActionError ?? stats?.hotspot?.message ?? "请重试并检查核心日志。"
        }
    }
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
    @Published var autoDeviceLabels: [String: String] = [:]
    @Published var isBusy = false
    @Published var notice: String?
    @Published var errorMessage: String?
    @Published var coreUpgradeRecommended = false
    @Published var natDiag: NATDiagResult?
    @Published var isNATDiagRunning = false
    @Published var themeID: String = UserDefaults.standard.string(forKey: "appThemeID") ?? "cream" {
        didSet { UserDefaults.standard.set(themeID, forKey: "appThemeID") }
    }

    let client = GatewayClient()
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
            Task { @MainActor [weak self] in self?.isLiveScrolling = true }
        }
        NotificationCenter.default.addObserver(forName: NSScrollView.didEndLiveScrollNotification, object: nil, queue: .main) { [weak self] _ in
            Task { @MainActor [weak self] in self?.isLiveScrolling = false }
        }
        timer = Timer.scheduledTimer(withTimeInterval: 3, repeats: true) { [weak self] _ in
            Task { @MainActor in
                guard let self, !self.isLiveScrolling else { return }
                await self.refresh(silent: true)
            }
        }
        Task {
            await refresh(silent: true)
        }
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
                    let runtime = try await client.stats(apiPort: latest.ports.api, configFile: latest.configFile)
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
            recomputeAutoDeviceLabels()
            if selectedSection == .settings {
                let latestLog = await client.readLog(path: latest.logFile)
                if latestLog != logText { logText = latestLog }
            }
            if !silent { showNotice("状态已刷新") }
        } catch {
            if !silent { errorMessage = error.localizedDescription }
        }
    }

    func refreshHotspot() async {
        do {
            hotspot = try await client.hotspotStatus()
            hotspotError = nil
        } catch {
            hotspot = nil
            hotspotError = error.localizedDescription
        }
    }

    func configureHotspot(_ action: String) async {
        guard !isBusy else { return }
        isBusy = true
        hotspotOperation = action
        hotspotActionError = nil
        errorMessage = nil
        defer { isBusy = false; hotspotOperation = nil }
        do {
            _ = try await client.configureHotspot(action)
            hotspotOperation = "verify"
            var confirmed = false
            // Verify the running process, not just the config written by the CLI.
            for _ in 0..<12 {
                let latest = try await client.status()
                status = latest
                if latest.running, let live = try? await client.stats(apiPort: latest.ports.api, configFile: latest.configFile) {
                    stats = live
                    if let runtime = live.hotspot {
                        if action == "enable" && runtime.enabled && runtime.applied { confirmed = true }
                        if action == "disable" && !runtime.enabled && !runtime.applied { confirmed = true }
                    }
                    if action == "use-lan" && latest.accessMode != "hotspot" { confirmed = true }
                    if confirmed { break }
                } else { stats = nil }
                try await Task.sleep(nanoseconds: 1_000_000_000)
            }
            if confirmed {
                showNotice(action == "disable" ? "热点接管已停止" : "接入已确认生效")
            } else {
                throw GatewayClientError.commandFailed(stats?.hotspot?.message ?? "配置已保存，但后台核心没有确认接管结果。可能仍在运行旧版核心；请重试更新，或查看核心日志。")
            }
        } catch {
            hotspotActionError = error.localizedDescription
        }
        await refreshHotspot()
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

    func learningAction(_ action: String, host: String) async {
        _ = await performAsync("规则建议已更新") { try await self.client.learningAction(action, host: host) }
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

    func runNATDiag() {
        guard let port = status?.ports.api, let configFile = status?.configFile else { return }
        isNATDiagRunning = true
        Task {
            do {
                natDiag = try await client.natDiag(apiPort: port, configFile: configFile)
            } catch {
                errorMessage = "NAT 诊断失败: \(error.localizedDescription)"
            }
            isNATDiagRunning = false
        }
    }

    func setServiceEnabled(_ enabled: Bool) {
        if enabled { installService() } else { uninstallService() }
    }

    func deviceLabel(for ip: String) -> String { deviceLabels[ip] ?? "" }

    // effectiveDeviceLabel prefers the user's manual label; otherwise the
    // traffic-inferred auto label (in-memory only, cleared when offline).
    func effectiveDeviceLabel(for ip: String) -> String {
        if let manual = deviceLabels[ip], !manual.isEmpty { return manual }
        return autoDeviceLabels[ip] ?? ""
    }

    func isAutoLabeled(_ ip: String) -> Bool {
        (deviceLabels[ip] ?? "").isEmpty && autoDeviceLabels[ip] != nil
    }

	func adaptiveDeviceState(for ip: String) -> DeviceAdaptiveState? {
		stats?.deviceAdaptive?.devices.first { $0.device == ip }
	}

    func deviceOverride(for ip: String) -> String {
        for rule in status?.routing ?? [] where rule.type == "src-ip" && rule.value == ip {
            return rule.action
        }
        return ""
    }

    func setDeviceOverride(_ ip: String, action: String) {
        var rules = status?.routing ?? []
        rules.removeAll { $0.type == "src-ip" && $0.value == ip }
        if action == "proxy" || action == "direct" || action == "reject" {
            rules.insert(RoutingRule(type: "src-ip", value: ip, action: action, group: "设备开关"), at: 0)
        }
        Task {
            _ = await applyRoutingRules(rules)
            await refresh(silent: true)
        }
    }

    private func recomputeAutoDeviceLabels() {
        guard let deviceGroups = stats?.relay.deviceServices, !deviceGroups.isEmpty else {
            if !autoDeviceLabels.isEmpty { autoDeviceLabels = [:] }
            return
        }
        let now = Date()
        var labels: [String: String] = [:]
        for group in deviceGroups {
            guard let lastSeen = group.services.map(\.lastSeen).max(),
                  now.timeIntervalSince(lastSeen) < 600 else { continue }
            let names = Set(group.services.map(\.name))
            if names.contains("Nintendo") {
                labels[group.device] = "Switch"
            } else if names.contains("PlayStation") {
                labels[group.device] = "PlayStation"
            } else if names.contains("Steam") {
                labels[group.device] = "电脑"
            } else if !names.isDisjoint(with: ["微信", "抖音", "小红书", "TikTok"]) {
                labels[group.device] = "手机"
            }
        }
        if labels != autoDeviceLabels { autoDeviceLabels = labels }
    }

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

    @Published var isInstallingCLI = false
    @Published var cliNeedsCoreRestart = false

    func installCLI() {
        guard !isInstallingCLI else { return }
        isInstallingCLI = true
        Task {
            let ok = await performAsync("CLI 已安装到 /usr/local/bin/gateway") { try await self.client.installCLI() }
            isInstallingCLI = false
            if ok && isRunning { cliNeedsCoreRestart = true }
        }
    }

    func restartForNewCLI() {
        Task {
            let ok = await performAsync("核心服务已用新版本重启") { try await self.client.restart() }
            if ok { cliNeedsCoreRestart = false }
        }
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

    func showNotice(_ text: String) {
        noticeTask?.cancel()
        notice = text
        noticeTask = Task {
            try? await Task.sleep(for: .seconds(4))
            guard !Task.isCancelled else { return }
            notice = nil
        }
    }
}
