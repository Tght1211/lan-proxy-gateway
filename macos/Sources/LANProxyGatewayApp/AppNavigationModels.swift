import Foundation

struct AppBuildVersion {
    let value: String?

    init(_ bundleVersion: String?) {
        let trimmed = bundleVersion?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        let normalized = trimmed.hasPrefix("v") ? String(trimmed.dropFirst()) : trimmed
        value = ["", "0.0.0", "dev"].contains(normalized) ? nil : normalized
    }

    var detail: String {
        guard let value else { return "开发预览 · 未标记版本" }
        return value.contains("-") ? "开发预览 v\(value)" : "当前版本 v\(value)"
    }
}

enum AppSection: String, CaseIterable, Identifiable {
    case overview = "网络总览"
    case rules = "配置规则"
    case devices = "设备接入"
    case connections = "流量记录"
    case settings = "设置"

    var id: String { rawValue }

    var systemImage: String {
        switch self {
        case .overview: return "point.3.connected.trianglepath.dotted"
        case .rules: return "network"
        case .devices: return "gamecontroller"
        case .connections: return "activity"
        case .settings: return "settings"
        }
    }
}

struct AppSectionCache {
    private(set) var visited: Set<AppSection> = []

    mutating func visit(_ section: AppSection?) {
        visited.insert(section ?? .overview)
    }

    func sections(including section: AppSection?) -> [AppSection] {
        let current = section ?? .overview
        return AppSection.allCases.filter { visited.contains($0) || $0 == current }
    }
}

struct ContentSheetSizing {
    let width: Double
    let maximumHeight: Double

    static let information = ContentSheetSizing(width: 480, maximumHeight: 420)
    static let policy = ContentSheetSizing(width: 420, maximumHeight: 560)
    static let access = ContentSheetSizing(width: 600, maximumHeight: 680)
    static let services = ContentSheetSizing(width: 680, maximumHeight: 620)
    static let records = ContentSheetSizing(width: 620, maximumHeight: 620)
    static let workspace = ContentSheetSizing(width: 900, maximumHeight: 720)
    static let settings = ContentSheetSizing(width: 740, maximumHeight: 680)
    static let probe = ContentSheetSizing(width: 360, maximumHeight: 300)

    func fittedSize(contentHeight: Double, headerHeight: Double, availableWidth: Double, availableHeight: Double) -> CGSize {
        let fittedWidth = max(1, min(width, availableWidth))
        let heightLimit = max(1, min(maximumHeight, availableHeight))
        let fittedHeight = min(heightLimit, max(120, contentHeight + headerHeight + 1))
        return CGSize(width: fittedWidth, height: fittedHeight)
    }
}

// MARK: - NAT Diagnosis

struct NATDiagResult: Decodable {
    let natType: String
    let externalIP: String
    let externalPort: Int
    let doubleNAT: Bool
    let doubleNATDetail: String?
    let upnp: UPnPStatusResult
    let warnings: [String]?

    enum CodingKeys: String, CodingKey {
        case natType = "nat_type"
        case externalIP = "external_ip"
        case externalPort = "external_port"
        case doubleNAT = "double_nat"
        case doubleNATDetail = "double_nat_detail"
        case upnp, warnings
    }
}

struct UPnPStatusResult: Decodable {
    let available: Bool
    let deviceName: String?
    let serviceType: String?

    enum CodingKeys: String, CodingKey {
        case available
        case deviceName = "device_name"
        case serviceType = "service_type"
    }
}
