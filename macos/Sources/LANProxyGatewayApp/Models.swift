import Foundation

struct GatewayStatus: Decodable {
    let accessMode: String?
    let httpProxy: LANHTTPProxyStatus?
    let configured: Bool
    let running: Bool
    let egress: String
    let proxy: String?
    let routing: [RoutingRule]?
    let dns: DNSStatus
    let quicBlock: Bool
    let gateway: NetworkStatus
    let ports: PortsStatus
    let configFile: String
    let logFile: String

    enum CodingKeys: String, CodingKey {
        case accessMode = "access_mode"
        case httpProxy = "http_proxy"
        case configured, running, egress, proxy, routing, dns, gateway, ports
        case quicBlock = "quic_block"
        case configFile = "config_file"
        case logFile = "log_file"
    }
}

struct RoutingRule: Codable, Identifiable, Equatable {
    var id = UUID()
    var type: String
    var value: String
    var action: String
    var group: String
    var learned: Bool

    init(id: UUID = UUID(), type: String, value: String, action: String, group: String = "", learned: Bool = false) {
        self.id = id
        self.type = type
        self.value = value
        self.action = action
        self.group = group
        self.learned = learned
    }

    enum CodingKeys: String, CodingKey { case type, value, action, group, learned }

    init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = UUID()
        type = try values.decode(String.self, forKey: .type)
        value = try values.decode(String.self, forKey: .value)
        action = try values.decode(String.self, forKey: .action)
        group = try values.decodeIfPresent(String.self, forKey: .group) ?? ""
        learned = try values.decodeIfPresent(Bool.self, forKey: .learned) ?? false
    }
}

struct DNSStatus: Decodable {
    let enabled: Bool
    let port: Int
    let hijack: Bool
    let fakeIP: Bool

    enum CodingKeys: String, CodingKey {
        case enabled, port, hijack
        case fakeIP = "fake_ip"
    }
}

struct NetworkStatus: Decodable {
    let ipForward: Bool
    let interface: String
    let localIP: String
    let router: String

    enum CodingKeys: String, CodingKey {
        case ipForward = "IPForward"
        case interface = "Interface"
        case localIP = "LocalIP"
        case router = "Router"
    }
}

struct PortsStatus: Decodable {
    let redir: Int
    let udpRedir: Int?
    let api: Int
    let dns: Int

    enum CodingKeys: String, CodingKey {
        case redir, api, dns
        case udpRedir = "udp_redir"
    }
}

struct UDPRelayStats: Decodable {
    let sessions: Int
    let listen: String
}

struct ComponentHealth: Decodable, Identifiable {
    let name: String
    let running: Bool
    let crashes: Int

    var id: String { name }
}

struct RuntimeStats: Decodable {
    let hotspot: HotspotStatus?
    let usageHistory: [DailyUsage]?
    let httpProxy: LANHTTPProxyStatus?
    let schemaVersion: Int?
    let egress: String
    let proxy: String?
    let uptimeSec: Int64
    let relay: RelayStats
    let udpRelay: UDPRelayStats?
    let dns: DNSStats?
    let health: HealthStats
    let exitHealth: [String: HealthStats]?
    let fallback: FallbackStats?
    let egressHealth: EgressHealthStats?
	let deviceAdaptive: DeviceAdaptiveStats?
    let components: [ComponentHealth]?

    enum CodingKeys: String, CodingKey {
        case hotspot
        case usageHistory = "usage_history"
        case httpProxy = "http_proxy"
        case egress, proxy, relay, dns, health, fallback, components
        case schemaVersion = "schema_version"
        case uptimeSec = "uptime_sec"
        case exitHealth = "exit_health"
        case udpRelay = "udp_relay"
		case egressHealth = "egress_health"
		case deviceAdaptive = "device_adaptive"
    }
}

extension RuntimeStats {
    func health(for exit: String) -> HealthStats? {
        if let exitHealth { return exitHealth[exit] }
        return egress == exit ? health : nil
    }
}

struct DeviceAdaptiveStats: Decodable {
	let threshold: Int
	let windowSeconds: Int
	let directSeconds: Int
	let devices: [DeviceAdaptiveState]

	enum CodingKeys: String, CodingKey {
		case threshold, devices
		case windowSeconds = "window_seconds"
		case directSeconds = "direct_seconds"
	}

	init(from decoder: Decoder) throws {
		let v = try decoder.container(keyedBy: CodingKeys.self)
		threshold = try v.decodeIfPresent(Int.self, forKey: .threshold) ?? 5
		windowSeconds = try v.decodeIfPresent(Int.self, forKey: .windowSeconds) ?? 120
		directSeconds = try v.decodeIfPresent(Int.self, forKey: .directSeconds) ?? 900
		devices = try v.decodeIfPresent([DeviceAdaptiveState].self, forKey: .devices) ?? []
	}
}

struct DeviceAdaptiveState: Decodable, Identifiable {
	let device: String
	let mode: String
	let failureCount: Int
	let hosts: [String]
	let since: Date?
	let until: Date?
	var id: String { device }

	enum CodingKeys: String, CodingKey {
		case device, mode, hosts, since, until
		case failureCount = "failure_count"
	}

	init(from decoder: Decoder) throws {
		let v = try decoder.container(keyedBy: CodingKeys.self)
		device = try v.decode(String.self, forKey: .device)
		mode = try v.decodeIfPresent(String.self, forKey: .mode) ?? "observing"
		failureCount = try v.decodeIfPresent(Int.self, forKey: .failureCount) ?? 0
		hosts = try v.decodeIfPresent([String].self, forKey: .hosts) ?? []
		since = try v.decodeIfPresent(Date.self, forKey: .since)
		until = try v.decodeIfPresent(Date.self, forKey: .until)
	}
}

struct EgressHealthStats: Decodable {
    let proxyDown: Bool
    let since: Date?
    let actions: [EgressAction]
    let alerts: [String]
    let directFailures: [DirectFailure]
    let failureStats: [EgressFailureStat]
    let alertThreshold: Int
    let alertWindowSec: Int
    let statsWindowSec: Int

    enum CodingKeys: String, CodingKey {
        case actions, alerts
        case proxyDown = "proxy_down"
        case since
        case directFailures = "direct_failures"
        case failureStats = "failure_stats"
        case alertThreshold = "alert_threshold"
        case alertWindowSec = "alert_window_sec"
        case statsWindowSec = "stats_window_sec"
    }

    init(from decoder: Decoder) throws {
        let v = try decoder.container(keyedBy: CodingKeys.self)
        proxyDown = try v.decodeIfPresent(Bool.self, forKey: .proxyDown) ?? false
        since = try v.decodeIfPresent(Date.self, forKey: .since)
        actions = try v.decodeIfPresent([EgressAction].self, forKey: .actions) ?? []
        alerts = try v.decodeIfPresent([String].self, forKey: .alerts) ?? []
        directFailures = try v.decodeIfPresent([DirectFailure].self, forKey: .directFailures) ?? []
        failureStats = try v.decodeIfPresent([EgressFailureStat].self, forKey: .failureStats) ?? []
        alertThreshold = try v.decodeIfPresent(Int.self, forKey: .alertThreshold) ?? 3
        alertWindowSec = try v.decodeIfPresent(Int.self, forKey: .alertWindowSec) ?? 600
        statsWindowSec = try v.decodeIfPresent(Int.self, forKey: .statsWindowSec) ?? 3600
    }
}

struct EgressAction: Decodable, Identifiable {
    let at: Date
    let text: String
    var id: String { "\(at.timeIntervalSince1970)-\(text)" }
}

struct DirectFailure: Decodable, Identifiable {
    let device: String
    let host: String
    let reason: String
    let count: Int
    let lastAt: Date?
    var id: String { "\(device)-\(host)" }

    enum CodingKeys: String, CodingKey {
        case device, host, reason, count
        case lastAt = "last_at"
    }

    init(from decoder: Decoder) throws {
        let v = try decoder.container(keyedBy: CodingKeys.self)
        device = try v.decodeIfPresent(String.self, forKey: .device) ?? ""
        host = try v.decodeIfPresent(String.self, forKey: .host) ?? ""
        reason = try v.decodeIfPresent(String.self, forKey: .reason) ?? ""
        count = try v.decodeIfPresent(Int.self, forKey: .count) ?? 1
        lastAt = try v.decodeIfPresent(Date.self, forKey: .lastAt)
    }
}

struct EgressFailureStat: Decodable, Identifiable {
    let device: String
    let host: String
    let reason: String
    let count: Int
    let lastAt: Date?
    let suppressed: Bool
    let alerting: Bool
    var id: String { "\(device)-\(host)" }

    enum CodingKeys: String, CodingKey {
        case device, host, reason, count, suppressed, alerting
        case lastAt = "last_at"
    }

    init(from decoder: Decoder) throws {
        let v = try decoder.container(keyedBy: CodingKeys.self)
        device = try v.decodeIfPresent(String.self, forKey: .device) ?? ""
        host = try v.decodeIfPresent(String.self, forKey: .host) ?? ""
        reason = try v.decodeIfPresent(String.self, forKey: .reason) ?? ""
        count = try v.decodeIfPresent(Int.self, forKey: .count) ?? 0
        lastAt = try v.decodeIfPresent(Date.self, forKey: .lastAt)
        suppressed = try v.decodeIfPresent(Bool.self, forKey: .suppressed) ?? false
        alerting = try v.decodeIfPresent(Bool.self, forKey: .alerting) ?? false
    }
}

struct LearningSettings: Codable {
    var enabled: Bool
    var confirmations: Int
    var autoSave: Bool
    var directWaitSeconds = 5
    var proxyWaitSeconds = 5
    var maxDirectWaitSeconds = 30
    var cooldownSeconds = 30
    var memoryMinutes = 10
    var responsePolicySupported = false
    enum CodingKeys: String, CodingKey {
        case enabled, confirmations
        case autoSave = "auto_save"
        case directWaitSeconds = "direct_wait_seconds", proxyWaitSeconds = "proxy_wait_seconds"
        case maxDirectWaitSeconds = "max_direct_wait_seconds", cooldownSeconds = "cooldown_seconds", memoryMinutes = "memory_minutes"
    }
    init(enabled: Bool, confirmations: Int, autoSave: Bool) {
        self.enabled = enabled; self.confirmations = confirmations; self.autoSave = autoSave
    }
    init(from decoder: Decoder) throws {
        let v = try decoder.container(keyedBy: CodingKeys.self)
        enabled = try v.decode(Bool.self, forKey: .enabled)
        confirmations = try v.decode(Int.self, forKey: .confirmations)
        autoSave = try v.decode(Bool.self, forKey: .autoSave)
        directWaitSeconds = try v.decodeIfPresent(Int.self, forKey: .directWaitSeconds) ?? 5
        proxyWaitSeconds = try v.decodeIfPresent(Int.self, forKey: .proxyWaitSeconds) ?? 5
        maxDirectWaitSeconds = try v.decodeIfPresent(Int.self, forKey: .maxDirectWaitSeconds) ?? 30
        cooldownSeconds = try v.decodeIfPresent(Int.self, forKey: .cooldownSeconds) ?? 30
        memoryMinutes = try v.decodeIfPresent(Int.self, forKey: .memoryMinutes) ?? 10
        responsePolicySupported = v.contains(.directWaitSeconds)
    }
}

struct FallbackStats: Decodable {
    let settings: LearningSettings?
    let strategy: String
    let ignored: [String]
    let threshold: Int
    let windowHours: Int
    let candidates: [FallbackCandidate]
    let learned: [RoutingRule]

    enum CodingKeys: String, CodingKey {
        case settings, strategy, threshold, candidates, learned, ignored
        case windowHours = "window_hours"
    }

    init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        settings = try values.decodeIfPresent(LearningSettings.self, forKey: .settings)
        strategy = try values.decodeIfPresent(String.self, forKey: .strategy) ?? "legacy"
        ignored = try values.decodeIfPresent([String].self, forKey: .ignored) ?? []
        threshold = try values.decodeIfPresent(Int.self, forKey: .threshold) ?? 3
        windowHours = try values.decodeIfPresent(Int.self, forKey: .windowHours) ?? 24
        candidates = try values.decodeIfPresent([FallbackCandidate].self, forKey: .candidates) ?? []
        learned = try values.decodeIfPresent([RoutingRule].self, forKey: .learned) ?? []
    }
}

struct FallbackCandidate: Decodable, Identifiable {
    let host: String
    let count: Int
    let lastAt: Date
    var id: String { host }

    enum CodingKeys: String, CodingKey {
        case host, count
        case lastAt = "last_at"
    }
}

struct LANHTTPProxyStatus: Decodable, Equatable {
    let enabled: Bool
    let port: Int
    let auth: String
    let username: String
    let passwordSet: Bool
    enum CodingKeys: String, CodingKey {
        case enabled, port, auth, username
        case passwordSet = "password_set"
    }
}

struct DailyUsage: Decodable {
    let egress: String?
    let proxyEndpoint: String?
    let destination: String?
    let service: String?
    var total: Int64 { up + down }
    let date: String
    let device: String
    let ingress: String
    let up: Int64
    let down: Int64
    let connections: Int64
    let lastSeen: Date
    enum CodingKeys: String, CodingKey {
        case date, device, ingress, up, down, connections, egress, destination, service
        case proxyEndpoint = "proxy_endpoint"
        case lastSeen = "last_seen"
    }
}

struct HotspotStatus: Decodable {
    let stage: String?

    let supported: Bool
    let available: Bool
    let enabled: Bool
    let applied: Bool
    let message: String
    let interface: String
    let ip: String
    let cidr: String
    let uplink: String

    func containsClient(_ address: String) -> Bool {
        func ipv4(_ value: String) -> UInt32? {
            let parts = value.split(separator: ".", omittingEmptySubsequences: false)
            guard parts.count == 4 else { return nil }
            var result: UInt32 = 0
            for part in parts {
                guard let byte = UInt8(part) else { return nil }
                result = (result << 8) | UInt32(byte)
            }
            return result
        }
        let parts = cidr.split(separator: "/", omittingEmptySubsequences: false)
        guard address != ip, parts.count == 2, let bits = Int(parts[1]), (0...32).contains(bits),
              let network = ipv4(String(parts[0])), let client = ipv4(address) else { return false }
        let mask: UInt32 = bits == 0 ? 0 : UInt32.max << (32 - bits)
        return client & mask == network & mask
    }
}

// Runtime confirmation is distinct from saved settings and Wi-Fi availability.
enum HotspotControlState: Equatable {
    case off, enabling, stopping, verifying, enabled, waiting, needsCoreUpdate, unavailable, failed

    static func resolve(operation: String?, error: String?, desired: Bool,
                        running: Bool, hasStats: Bool, runtime: HotspotStatus?) -> Self {
        if operation == "enable" || operation == "use-lan" { return .enabling }
        if operation == "disable" { return .stopping }
        if operation == "verify" { return .verifying }
        if error != nil { return .failed }
        if running, let runtime, runtime.enabled {
            if runtime.applied { return .enabled }
            return runtime.stage == "apply_failed" ? .failed : .waiting
        }
        if desired {
            if !running || !hasStats { return .unavailable }
            if runtime == nil { return .needsCoreUpdate }
            return .waiting
        }
        return .off
    }

    var title: String {
        switch self {
        case .off: return "热点接管未开启"
        case .enabling: return "正在开启热点接管…"
        case .stopping: return "正在停止热点接管…"
        case .verifying: return "正在确认接管结果…"
        case .enabled: return "热点接管已开启"
        case .waiting: return "接管尚未生效 · 等待热点就绪"
        case .needsCoreUpdate: return "接管尚未生效 · 需要更新后台核心"
        case .unavailable: return "暂时无法确认接管状态"
        case .failed: return "热点接管操作失败"
        }
    }

    var isWorking: Bool { self == .enabling || self == .stopping || self == .verifying }
}
