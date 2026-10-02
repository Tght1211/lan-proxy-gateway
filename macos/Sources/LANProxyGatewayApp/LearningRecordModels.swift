import Foundation

enum LearningRecordState: String, CaseIterable {
    case pending = "待处理"
    case saved = "已保存"
    case ignored = "已忽略"

    var title: String {
        switch self {
        case .pending: return "待确认"
        case .saved: return "已保存"
        case .ignored: return "暂停学习"
        }
    }
}

enum LearningCountLabel {
    static func compact(_ count: Int) -> String {
        let count = max(0, count)
        if count >= 1_000_000 { return abbreviated(count, divisor: 1_000_000, suffix: "m") }
        if count >= 1_000 { return abbreviated(count, divisor: 1_000, suffix: "k") }
        return String(count)
    }

    private static func abbreviated(_ count: Int, divisor: Int, suffix: String) -> String {
        let whole = count / divisor
        let fraction = (count % divisor) / (divisor / 10)
        return fraction == 0 ? "\(whole)\(suffix)" : "\(whole).\(fraction)\(suffix)"
    }
}

struct LearningRecordEntry: Identifiable, Equatable {
    let id: String
    let state: LearningRecordState
    let host: String
    let service: String
    let group: String
    let ruleType: String
    let action: String
    let confirmations: Int
    let lastAt: Date?
    let searchableText: String

    var scopeTitle: String {
        switch ruleType {
        case "domain": return "仅此域名"
        case "domain-suffix": return "域名及子域名"
        case "ip-cidr": return "IP 网段"
        case "src-ip": return "来源设备"
        default: return ruleType.isEmpty ? "尚未保存规则" : "其他范围（\(ruleType)）"
        }
    }

    var routeTitle: String {
        switch action {
        case "proxy": return "代理"
        case "direct": return "直连（旧版保留）"
        case "reject": return "拒绝"
        default: return action.isEmpty ? "" : "其他路由（\(action)）"
        }
    }
}

struct LearningServiceCategory: Identifiable, Equatable {
    var id: String { name }
    let name: String
    let count: Int
}

struct LearningRecordSection: Identifiable {
    var id: String { service }
    let service: String
    var entries: [LearningRecordEntry]
}

struct LearningRecordPage {
    let entries: [LearningRecordEntry]
    let total: Int
    let matched: Int
    let number: Int
    let pageCount: Int
    let pageSize: Int

    var sections: [LearningRecordSection] {
        guard entries.first?.state == .saved else {
            return [LearningRecordSection(service: "", entries: entries)]
        }
        var result: [LearningRecordSection] = []
        for entry in entries {
            if result.last?.service == entry.service {
                result[result.count - 1].entries.append(entry)
            } else {
                result.append(LearningRecordSection(service: entry.service, entries: [entry]))
            }
        }
        return result
    }

    var firstRecord: Int { matched == 0 ? 0 : number * pageSize + 1 }
    var lastRecord: Int { min(matched, (number + 1) * pageSize) }
}

struct LearningRecordsIndex {
    private var source: [LearningRecordEntry] = []
    private var records: [LearningRecordState: [LearningRecordEntry]] = [:]
    private var categories: [LearningRecordState: [LearningServiceCategory]] = [:]

    func count(for state: LearningRecordState) -> Int { records[state]?.count ?? 0 }
    func services(for state: LearningRecordState) -> [LearningServiceCategory] { categories[state] ?? [] }

    private static func serviceName(_ value: String) -> String? {
        let value = value.trimmingCharacters(in: .whitespacesAndNewlines)
        return value.isEmpty || value.contains(".") || ["未知目标", "未解析域名", "未识别流量", "IP 地址流量"].contains(value) ? nil : value
    }

    mutating func update(fallback: FallbackStats?, connections: [ConnectionInfo]) -> Bool {
        var services: [String: String] = [:]
        for connection in connections {
            let host = connection.dstHost.lowercased().trimmingCharacters(in: CharacterSet(charactersIn: "."))
            if services[host] == nil, let service = Self.serviceName(connection.service) {
                services[host] = service
            }
        }
        var next: [LearningRecordEntry] = []
        var identities: [String: Int] = [:]
        func append(state: LearningRecordState, host: String, ruleType: String = "",
                    action: String = "", group: String = "", confirmations: Int = 0, lastAt: Date? = nil) {
            let identity = "\(state.rawValue)|\(ruleType)|\(action)|\(host)"
            let duplicate = identities[identity, default: 0]
            identities[identity] = duplicate + 1
            let prefix = "自动学习 · "
            let savedService = group.hasPrefix(prefix) ? Self.serviceName(String(group.dropFirst(prefix.count))) : nil
            let normalizedHost = host.lowercased().trimmingCharacters(in: CharacterSet(charactersIn: "."))
            let service = savedService ?? services[normalizedHost] ?? "未分类"
            next.append(LearningRecordEntry(id: "\(identity)|\(duplicate)", state: state, host: host,
                                            service: service, group: group, ruleType: ruleType, action: action,
                                            confirmations: confirmations, lastAt: lastAt,
                                            searchableText: "\(host) \(service) \(group) \(ruleType) \(action)".lowercased()))
        }
        for candidate in fallback?.candidates ?? [] {
            append(state: .pending, host: candidate.host, confirmations: candidate.count, lastAt: candidate.lastAt)
        }
        for rule in fallback?.learned ?? [] {
            append(state: .saved, host: rule.value, ruleType: rule.type, action: rule.action, group: rule.group)
        }
        for host in fallback?.ignored ?? [] { append(state: .ignored, host: host) }
        guard next != source else { return false }
        source = next
        records = Dictionary(grouping: next, by: \.state)
        for state in LearningRecordState.allCases {
            var counts: [String: Int] = [:]
            for record in records[state] ?? [] { counts[record.service, default: 0] += 1 }
            categories[state] = counts.map { LearningServiceCategory(name: $0.key, count: $0.value) }.sorted {
                if ($0.name == "未分类") != ($1.name == "未分类") { return $1.name == "未分类" }
                return $0.name < $1.name
            }
            records[state]?.sort { first, second in
                if state == .pending, first.lastAt != second.lastAt {
                    return (first.lastAt ?? .distantPast) > (second.lastAt ?? .distantPast)
                }
                if state == .saved, first.service != second.service {
                    if (first.service == "未分类") != (second.service == "未分类") { return second.service == "未分类" }
                    return first.service < second.service
                }
                return first.host == second.host ? first.id < second.id : first.host < second.host
            }
        }
        return true
    }

    func page(state: LearningRecordState, search: String = "", service: String = "",
              action: String = "", ruleType: String = "", number: Int = 0,
              pageSize: Int = 50) -> LearningRecordPage {
        let rows = records[state] ?? []
        let query = search.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        let size = min(200, max(1, pageSize))
        let matches = rows.indices.filter {
            (query.isEmpty || rows[$0].searchableText.contains(query)) &&
            (service.isEmpty || rows[$0].service == service) &&
            (action.isEmpty || rows[$0].action == action) &&
            (ruleType.isEmpty || rows[$0].ruleType == ruleType)
        }
        let pageCount = max(1, (matches.count + size - 1) / size)
        let current = min(max(0, number), pageCount - 1)
        let start = current * size
        let entries = matches[start..<min(start + size, matches.count)].map { rows[$0] }
        return LearningRecordPage(entries: entries, total: rows.count, matched: matches.count,
                                  number: current, pageCount: pageCount, pageSize: size)
    }
}
