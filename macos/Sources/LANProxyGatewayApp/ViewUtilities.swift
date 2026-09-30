import AppKit
import Charts
import SwiftUI
import UniformTypeIdentifiers

struct NoticeBar: View {
    let text: String
    var body: some View { Label(text, systemImage: "checkmark.circle.fill").font(.subheadline.weight(.medium)).foregroundStyle(Color.black).padding(.horizontal, 14).frame(minHeight: 38).background(Theme.lime).clipShape(RoundedRectangle(cornerRadius: Theme.radiusSmall)).shadow(color: Color.black.opacity(0.4), radius: 10, y: 4) }
}

struct IconButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View { configuration.label.foregroundStyle(Color.primary.opacity(0.8)).frame(width: 30, height: 30).background(configuration.isPressed ? Theme.sidebar : Theme.panel).overlay(RoundedRectangle(cornerRadius: Theme.radiusSmall).stroke(Theme.border, lineWidth: Theme.borderWidth)).clipShape(RoundedRectangle(cornerRadius: Theme.radiusSmall)) }
}

struct ActionButtonStyle: ButtonStyle {
    let tint: Color
    func makeBody(configuration: Configuration) -> some View { configuration.label.font(.system(size: 12, weight: .semibold)).foregroundStyle(Theme.canvas).padding(.horizontal, 13).frame(minHeight: 30).background(tint.opacity(configuration.isPressed ? 0.72 : 0.92)).clipShape(RoundedRectangle(cornerRadius: Theme.radiusSmall)) }
}

struct DarkFieldStyle: TextFieldStyle {
    func _body(configuration: TextField<Self._Label>) -> some View { configuration.textFieldStyle(.plain).padding(.horizontal, 11).frame(height: 36).background(Theme.panelRaised).overlay(RoundedRectangle(cornerRadius: Theme.radiusSmall).stroke(Theme.border, lineWidth: 0.8)).clipShape(RoundedRectangle(cornerRadius: Theme.radiusSmall)) }
}

extension Text {
    func eyebrow() -> some View { self.font(.system(size: 10, weight: .bold)).foregroundStyle(Theme.muted) }
    func sectionLabel() -> some View { self.font(.system(size: 12, weight: .semibold)).foregroundStyle(Color.primary.opacity(0.82)) }
    func fieldLabel() -> some View { self.font(.system(size: 11, weight: .medium)).foregroundStyle(Theme.muted) }
}

extension String { var nonEmpty: String? { isEmpty ? nil : self } }

func bytes(_ value: Int64) -> String { ByteCountFormatter.string(fromByteCount: value, countStyle: .file) }
func shortBytes(_ value: Int64) -> String { ByteCountFormatter.string(fromByteCount: value, countStyle: .file) }
func speed(_ fiveSecondBytes: Int64) -> String { bytes(fiveSecondBytes / 5) + "/s" }
func uptime(_ seconds: Int64) -> String { seconds > 3600 ? "\(seconds / 3600)H" : "\(max(seconds / 60, 0))M" }
func formatMS(_ value: Double?) -> String { guard let value, value > 0 else { return "--" }; return String(format: "%.1f ms", value) }
struct LearnedRuleSection {
    let name: String
    let rules: [RoutingRule]
    var displayName: String {
        if name.hasPrefix("自动学习 · ") { return String(name.dropFirst(6)) }
        if name == "自动学习" { return "其他" }
        return name
    }
}

func learnedRulesByGroup(_ rules: [RoutingRule]) -> [LearnedRuleSection] {
    var order: [String] = []
    var bucket: [String: [RoutingRule]] = [:]
    for rule in rules {
        let key = rule.group.isEmpty ? "自动学习" : rule.group
        if bucket[key] == nil { order.append(key) }
        bucket[key, default: []].append(rule)
    }
    return order.compactMap { key in
        guard let items = bucket[key] else { return nil }
        return LearnedRuleSection(name: key, rules: items)
    }
}

func serviceColor(_ service: String) -> Color { [Theme.cyan, Theme.lime, Theme.coral, Theme.yellow][Int(service.hashValue.magnitude % 4)] }
func egressLocation(_ identity: EgressIdentity?) -> String {
    guard let identity else { return "地区待检测" }
    let country = identity.countryCode.flatMap {
        Locale(identifier: "zh-Hans").localizedString(forRegionCode: $0)
    }
    let place = identity.city?.nonEmpty ?? identity.region?.nonEmpty
    return [country, place].compactMap { $0 }.uniqued().joined(separator: " · ").nonEmpty ?? "地区未知"
}

// Parses Clash-style rule lines ("DOMAIN-SUFFIX,example.com,DIRECT").
// Unsupported types (PROCESS-NAME, ...) are skipped; unknown targets map to
// proxy. A trailing "# 自动学习" comment restores the learned marker.
let learnedRuleComment = "自动学习"

func parseRuleLines(_ text: String) -> (rules: [RoutingRule], skipped: [String]) {
    var rules: [RoutingRule] = []
    var skipped: [String] = []
    var currentGroup = ""
    for rawLine in text.split(separator: "\n", omittingEmptySubsequences: true) {
        var line = rawLine.trimmingCharacters(in: .whitespaces)
        guard !line.isEmpty else { continue }
        if line.hasPrefix("#") || line.hasPrefix("//") {
            if let range = line.range(of: "== 分组:") {
                let rest = line[range.upperBound...]
                let name = rest.replacingOccurrences(of: "==", with: "").trimmingCharacters(in: .whitespaces)
                currentGroup = name
            } else if line.contains("== 未分组 ==") {
                currentGroup = ""
            }
            continue
        }
        var learned = false
        if let hashIndex = line.firstIndex(of: "#") {
            let comment = line[line.index(after: hashIndex)...]
            learned = comment.contains(learnedRuleComment)
            line = String(line[..<hashIndex])
        }
        line = line.trimmingCharacters(in: CharacterSet(charactersIn: "\"'，,"))
            .trimmingCharacters(in: .whitespaces)
        guard !line.isEmpty else { continue }
        let parts = line.replacingOccurrences(of: "，", with: ",")
            .split(separator: ",", omittingEmptySubsequences: true)
            .map { $0.trimmingCharacters(in: .whitespaces) }
        guard parts.count >= 2 else {
            skipped.append(String(rawLine.prefix(40)))
            continue
        }
        let type: String
        switch parts[0].uppercased() {
        case "DOMAIN": type = "domain"
        case "DOMAIN-SUFFIX": type = "domain-suffix"
        case "IP-CIDR", "IP-CIDR6": type = "ip-cidr"
		case "SRC-IP": type = "src-ip"
        default:
            skipped.append(String(rawLine.prefix(40)))
            continue
        }
        let action: String
        if parts.count < 3 {
            action = "proxy"
        } else {
            switch parts[2].uppercased() {
            case "DIRECT": action = "direct"
            case "REJECT", "REJECT-DROP", "BLOCK": action = "reject"
            default: action = "proxy"
            }
        }
        rules.append(RoutingRule(type: type, value: parts[1], action: action, group: currentGroup, learned: learned))
    }
    return (rules, skipped)
}

func serializeRuleLines(_ rules: [RoutingRule]) -> String {
    var lines: [String] = []
    var currentGroup = ""
    for rule in rules {
        if rule.group != currentGroup {
            if !lines.isEmpty { lines.append("") }
            lines.append(rule.group.isEmpty ? "# == 未分组 ==" : "# == 分组: \(rule.group) ==")
            currentGroup = rule.group
        }
        let type: String
        switch rule.type {
        case "domain": type = "DOMAIN"
        case "domain-suffix": type = "DOMAIN-SUFFIX"
        case "ip-cidr": type = "IP-CIDR"
		case "src-ip": type = "SRC-IP"
        default: type = rule.type.uppercased()
        }
        let action: String
        switch rule.action {
        case "direct": action = "DIRECT"
        case "reject": action = "REJECT"
        default: action = "PROXY"
        }
        lines.append("\(type),\(rule.value),\(action)\(rule.learned ? " # \(learnedRuleComment)" : "")")
    }
    return lines.joined(separator: "\n")
}

func suggestedDeviceIPPool(gateway: String, occupied: Set<String>) -> [String] {
    let octets = gateway.split(separator: ".").compactMap { Int($0) }
    guard octets.count == 4, octets.allSatisfy({ (0...255).contains($0) }) else { return [] }
    let prefix = "\(octets[0]).\(octets[1]).\(octets[2])"
    return (201...254)
        .map { "\(prefix).\($0)" }
        .filter { $0 != gateway && !occupied.contains($0) }
}

// Best-effort liveness probe: one ping with a 300ms reply window per address.
func probeOccupiedAddresses(_ addresses: [String]) async -> Set<String> {
    await withTaskGroup(of: (String, Bool).self) { group in
        for address in addresses {
            group.addTask {
                let process = Process()
                process.executableURL = URL(fileURLWithPath: "/sbin/ping")
                process.arguments = ["-c", "1", "-W", "300", "-q", address]
                process.standardOutput = FileHandle.nullDevice
                process.standardError = FileHandle.nullDevice
                do { try process.run() } catch { return (address, false) }
                process.waitUntilExit()
                return (address, process.terminationStatus == 0)
            }
        }
        var occupied = Set<String>()
        for await (address, alive) in group where alive { occupied.insert(address) }
        return occupied
    }
}

func suggestedDeviceIPs(gateway: String, occupied: Set<String>) -> [String] {
    let octets = gateway.split(separator: ".").compactMap { Int($0) }
    guard octets.count == 4, octets.allSatisfy({ (0...255).contains($0) }) else { return [] }
    let prefix = "\(octets[0]).\(octets[1]).\(octets[2])"
    return (201...254)
        .map { "\(prefix).\($0)" }
        .filter { $0 != gateway && !occupied.contains($0) }
        .prefix(5)
        .map { $0 }
}

func suggestionsRange(_ suggestions: [String]) -> String {
    guard let first = suggestions.first else { return "暂不可用" }
    guard let last = suggestions.last, last != first else { return first }
    let lastOctet = last.split(separator: ".").last.map(String.init) ?? last
    return "\(first)–\(lastOctet)"
}

func suggestionPrefix(_ suggestions: [String]) -> String {
    guard let first = suggestions.first else { return "--" }
    return first.split(separator: ".").dropLast().joined(separator: ".")
}

extension Array where Element == String {
    func uniqued() -> [String] {
        reduce(into: []) { result, value in
            if !result.contains(value) { result.append(value) }
        }
    }
}
