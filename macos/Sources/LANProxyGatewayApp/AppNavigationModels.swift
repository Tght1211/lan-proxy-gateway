import Foundation

enum AppSection: String, CaseIterable, Identifiable {
    case overview = "网络总览"
    case devices = "设备接入"
    case connections = "访问记录"
    case settings = "设置"

    var id: String { rawValue }

    var systemImage: String {
        switch self {
        case .overview: return "command"
        case .devices: return "desktopcomputer.and.macbook"
        case .connections: return "list.bullet.rectangle.portrait"
        case .settings: return "gearshape"
        }
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
