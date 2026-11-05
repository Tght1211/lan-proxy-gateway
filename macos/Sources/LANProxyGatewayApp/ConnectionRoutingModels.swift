import Darwin
import Foundation

enum ConnectionRouteAction: String, CaseIterable {
    case direct
    case proxy

    var title: String { self == .direct ? "直连" : "代理" }
}

enum ConnectionRoutingError: LocalizedError {
    case rulesChanged
    case notApplied
    case unavailable

    var errorDescription: String? {
        switch self {
        case .rulesChanged: return "分流规则正在被其他操作更新，请稍后重试。"
        case .notApplied: return "未能确认快捷规则已保存，请刷新配置规则后重试。"
        case .unavailable: return "无法读取当前分流规则，未保存快捷规则。"
        }
    }
}

struct ConnectionRoutingShortcut {
    let host: String

    init?(host: String) {
        let normalized = host.trimmingCharacters(in: .whitespacesAndNewlines)
            .lowercased().trimmingCharacters(in: CharacterSet(charactersIn: "."))
        var ipv4 = in_addr()
        var ipv6 = in6_addr()
        guard !normalized.isEmpty, normalized.utf8.count <= 253,
              inet_pton(AF_INET, normalized, &ipv4) != 1,
              inet_pton(AF_INET6, normalized, &ipv6) != 1 else { return nil }
        let labels = normalized.split(separator: ".", omittingEmptySubsequences: false)
        guard labels.allSatisfy({ label in
            !label.isEmpty && label.utf8.count <= 63 && label.first != "-" && label.last != "-" &&
                label.utf8.allSatisfy { (97...122).contains($0) || (48...57).contains($0) || $0 == 45 }
        }) else { return nil }
        self.host = normalized
    }

    func action(in rules: [RoutingRule]) -> ConnectionRouteAction? {
        guard let rule = rules.first(where: matches), !rule.learned else { return nil }
        return ConnectionRouteAction(rawValue: rule.action)
    }

    func updating(_ rules: [RoutingRule], action: ConnectionRouteAction) -> [RoutingRule] {
        var rule = rules.first(where: matches) ?? RoutingRule(type: "domain", value: host, action: action.rawValue)
        if rule.learned || rule.group.isEmpty { rule.group = "流量快捷规则" }
        rule.type = "domain"
        rule.value = host
        rule.action = action.rawValue
        rule.learned = false
        return [rule] + rules.filter { !matches($0) }
    }

    func save(action: ConnectionRouteAction,
              readRules: () async throws -> [RoutingRule],
              writeRules: ([RoutingRule]) async throws -> Void) async throws {
        for _ in 0..<3 {
            let current = try await readRules()
            let latest = try await readRules()
            guard Self.sameRules(current, latest) else { continue }
            try await writeRules(updating(latest, action: action))
            let saved = try await readRules()
            guard let first = saved.first, matches(first), !first.learned,
                  first.action == action.rawValue else { throw ConnectionRoutingError.notApplied }
            return
        }
        throw ConnectionRoutingError.rulesChanged
    }

    static func sameRules(_ first: [RoutingRule], _ second: [RoutingRule]) -> Bool {
        first.count == second.count && zip(first, second).allSatisfy { left, right in
            left.type == right.type && left.value == right.value && left.action == right.action &&
                left.group == right.group && left.learned == right.learned
        }
    }

    private func matches(_ rule: RoutingRule) -> Bool {
        rule.type == "domain" && rule.value.lowercased()
            .trimmingCharacters(in: CharacterSet(charactersIn: ".")) == host
    }
}
