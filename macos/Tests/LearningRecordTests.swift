import Foundation

@main struct LearningRecordTests {
    static func snapshot(rules: [RoutingRule], candidates: [[String: Any]] = [], ignored: [String] = []) throws -> FallbackStats {
        let records = rules.map { ["type": $0.type, "value": $0.value, "action": $0.action, "group": $0.group] }
        let data = try JSONSerialization.data(withJSONObject: ["learned": records, "candidates": candidates, "ignored": ignored])
        return try JSONDecoder().decode(FallbackStats.self, from: data)
    }

    static func main() throws {
        precondition(LearningCountLabel.compact(-1) == "0")
        precondition(LearningCountLabel.compact(999) == "999")
        precondition(LearningCountLabel.compact(1_000) == "1k")
        precondition(LearningCountLabel.compact(1_234) == "1.2k")
        precondition(LearningCountLabel.compact(10_000) == "10k")
        precondition(LearningCountLabel.compact(999_999) == "999.9k")
        precondition(LearningCountLabel.compact(1_234_567) == "1.2m")
        precondition(!LearningCountLabel.compact(Int.max).isEmpty)

        var index = LearningRecordsIndex()
        let empty = index.page(state: .saved, number: Int.max, pageSize: 0)
        precondition(empty.entries.isEmpty && empty.number == 0 && empty.pageCount == 1)
        precondition(empty.firstRecord == 0 && empty.lastRecord == 0 && empty.pageSize == 1)
        precondition(!index.update(fallback: nil, connections: []))

        let rules = (0..<10_000).reversed().map {
            RoutingRule(type: "domain", value: String(format: "domain-%05d.example", $0), action: "proxy")
        }
        let data = try snapshot(rules: rules)
        let started = Date()
        precondition(index.update(fallback: data, connections: []))
        let initialMilliseconds = Date().timeIntervalSince(started) * 1_000
        precondition(index.count(for: .saved) == 10_000)
        let first = index.page(state: .saved)
        precondition(first.entries.count == 50 && first.pageCount == 200 && first.total == 10_000)
        precondition(first.entries.first?.host == "domain-00000.example")
        precondition(first.firstRecord == 1 && first.lastRecord == 50)
        let last = index.page(state: .saved, number: Int.max)
        precondition(last.number == 199 && last.entries.count == 50)
        precondition(last.firstRecord == 9_951 && last.lastRecord == 10_000)
        let everyID = (0..<first.pageCount).flatMap { index.page(state: .saved, number: $0).entries.map(\.id) }
        precondition(everyID.count == 10_000 && Set(everyID).count == 10_000, "pagination must not drop or repeat rules")
        let filtered = index.page(state: .saved, search: "  DOMAIN-09999  ", number: 100)
        precondition(filtered.matched == 1 && filtered.number == 0 && filtered.entries[0].host == "domain-09999.example")
        let prefix = index.page(state: .saved, search: "domain-09")
        precondition(prefix.matched == 1_000 && prefix.pageCount == 20)
        precondition(index.page(state: .saved, search: "not-present.test").entries.isEmpty)
        precondition(index.page(state: .saved, pageSize: Int.max).entries.count == 200)

        let decodedAgain = try snapshot(rules: rules)
        precondition(!index.update(fallback: decodedAgain, connections: []), "new decoded UUIDs must not invalidate identical learning records")
        precondition(index.page(state: .saved).entries.map(\.id) == first.entries.map(\.id))

        let small = try snapshot(rules: Array(rules.prefix(51)))
        precondition(index.update(fallback: small, connections: []))
        let shortened = index.page(state: .saved, number: 199)
        precondition(shortened.number == 1 && shortened.entries.count == 1 && shortened.lastRecord == 51)
        precondition(index.update(fallback: nil, connections: []))
        precondition(index.page(state: .saved, number: 199).entries.isEmpty)

        let duplicate = RoutingRule(type: "domain", value: "duplicate.test", action: "proxy")
        let mixed = try snapshot(rules: [duplicate, duplicate], candidates: [
            ["host": "older.test", "count": 2, "last_at": 100],
            ["host": "newer.test", "count": 1, "last_at": 200]
        ], ignored: ["paused.test", "paused.test"])
        precondition(index.update(fallback: mixed, connections: []))
        precondition(index.count(for: .pending) == 2 && index.count(for: .saved) == 2 && index.count(for: .ignored) == 2)
        let pending = index.page(state: .pending)
        precondition(pending.entries[0].host == "newer.test" && pending.entries[1].confirmations == 2)
        precondition(Set(index.page(state: .saved).entries.map(\.id)).count == 2)
        precondition(Set(index.page(state: .ignored).entries.map(\.id)).count == 2)

        let connection = try JSONDecoder().decode(ConnectionInfo.self, from: Data(#"{"id":1,"started_at":0,"src_ip":"192.0.2.1","dst_host":"duplicate.test","service":"Searchable service"}"#.utf8))
        precondition(index.update(fallback: mixed, connections: [connection]))
        precondition(index.page(state: .saved, search: "SEARCHABLE").matched == 2)
        precondition(!index.update(fallback: mixed, connections: [connection]))

        let classified = try snapshot(rules: [
            RoutingRule(type: "domain", value: "saved.test", action: "proxy", group: "自动学习 · Google"),
            RoutingRule(type: "domain-suffix", value: "legacy.test", action: "direct", group: "自动学习 · Google"),
            RoutingRule(type: "domain", value: "other.test", action: "proxy", group: "自动学习 · YouTube"),
            RoutingRule(type: "ip-cidr", value: "192.0.2.0/24", action: "reject", group: "用户保留分组"),
            RoutingRule(type: "future-scope", value: "unknown.co.uk", action: "future-action", group: "自动学习")
        ], candidates: [["host": "pending.test", "count": 1, "last_at": 300]], ignored: ["paused.test"])
        let conflict = try JSONDecoder().decode(ConnectionInfo.self, from: Data(#"{"id":2,"started_at":0,"src_ip":"192.0.2.1","dst_host":"SAVED.TEST.","service":"Different live service"}"#.utf8))
        precondition(index.update(fallback: classified, connections: [conflict]))
        precondition(index.services(for: .saved) == [
            LearningServiceCategory(name: "Google", count: 2),
            LearningServiceCategory(name: "YouTube", count: 1),
            LearningServiceCategory(name: "未分类", count: 2)
        ])
        precondition(index.page(state: .saved, service: "Google").matched == 2, "persisted service must survive missing or conflicting live traffic")
        let combined = index.page(state: .saved, service: "Google", action: "direct", ruleType: "domain-suffix")
        precondition(combined.matched == 1 && combined.entries[0].host == "legacy.test")
        precondition(combined.entries[0].scopeTitle == "域名及子域名" && combined.entries[0].routeTitle == "直连（旧版保留）")
        precondition(index.page(state: .saved, action: "reject").entries[0].scopeTitle == "IP 网段")
        precondition(index.page(state: .saved, search: "用户保留分组").matched == 1)
        let unknown = index.page(state: .saved, search: "unknown.co.uk").entries[0]
        precondition(unknown.service == "未分类" && unknown.scopeTitle == "其他范围（future-scope）" && unknown.routeTitle == "其他路由（future-action）")
        precondition(index.page(state: .saved, search: "other", service: "Google").matched == 0)
        precondition(index.page(state: .saved).sections.map(\.service) == ["Google", "YouTube", "未分类"])
        precondition(index.page(state: .saved, service: "Google", number: 1, pageSize: 1).sections[0].entries.count == 1)
        precondition(index.page(state: .pending).entries[0].action.isEmpty)
        precondition(index.page(state: .ignored).entries[0].action.isEmpty, "paused learning is not a reject routing policy")
        precondition(!index.update(fallback: classified, connections: []), "persisted categories must not change when live connections disappear")

        let largeClassifiedRules = rules.enumerated().map { position, rule in
            RoutingRule(type: position % 3 == 0 ? "domain-suffix" : "domain", value: rule.value,
                        action: position % 2 == 0 ? "proxy" : "direct",
                        group: position % 2 == 0 ? "自动学习 · Google" : "自动学习 · YouTube")
        }
        let largeClassified = try snapshot(rules: largeClassifiedRules)
        precondition(index.update(fallback: largeClassified, connections: []))
        precondition(index.services(for: .saved).map(\.count) == [5_000, 5_000])
        let largeFiltered = index.page(state: .saved, service: "Google", action: "proxy", ruleType: "domain-suffix")
        precondition(largeFiltered.matched == 1_667 && largeFiltered.pageCount == 34)
        let filteredIDs = (0..<largeFiltered.pageCount).flatMap {
            index.page(state: .saved, service: "Google", action: "proxy", ruleType: "domain-suffix", number: $0).entries.map(\.id)
        }
        precondition(filteredIDs.count == 1_667 && Set(filteredIDs).count == 1_667)
        print(String(format: "Learning record tests passed (10,000-row initial index: %.2f ms)", initialMilliseconds))
    }
}
