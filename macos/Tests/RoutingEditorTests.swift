import Foundation

@main struct RoutingEditorTests {
    static func main() throws {
        try testFilteringPreservesRules()
        testPagingAndStableIndices()
        testEmptyGroupsAndClamping()
        print("Routing editor filtering and paging tests passed")
    }

    static func testFilteringPreservesRules() throws {
        let manual = RoutingRule(type: "domain-suffix", value: "example.com", action: "direct", group: "混合分组")
        let learned = RoutingRule(type: "domain", value: "api.example.com", action: "proxy", group: "混合分组", learned: true)
        let rejected = RoutingRule(type: "ip-cidr", value: "203.0.113.0/24", action: "reject", group: "IP 策略")
        let groups = [(name: "混合分组", rules: [manual, learned]), (name: "IP 策略", rules: [rejected])]
        let encoder = JSONEncoder()
        encoder.outputFormatting = .sortedKeys
        let baseline = try encoder.encode(groups.flatMap(\.rules))
        let direct = RoutingEditorProjection(groups: groups, filter: RoutingEditorFilter(action: "direct"))
        precondition(direct.visibleGroupIndices == [0] && direct.ruleIndices == [[0], []])
        precondition(direct.matchedCount == 1 && direct.totalCount == 3)
        let learning = RoutingEditorProjection(groups: groups, filter: RoutingEditorFilter(origin: "learned"))
        precondition(learning.ruleIndices == [[1], []])
        let combined = RoutingEditorProjection(groups: groups, filter: RoutingEditorFilter(query: " API.EXAMPLE.COM \n", action: "proxy", origin: "learned"))
        precondition(combined.visibleGroupIndices == [0] && combined.matchedCount == 1)
        let groupSearch = RoutingEditorProjection(groups: groups, filter: RoutingEditorFilter(query: "混合"))
        precondition(groupSearch.ruleIndices[0] == [0, 1])
        let ipSearch = RoutingEditorProjection(groups: groups, filter: RoutingEditorFilter(query: "203.0.113"))
        precondition(ipSearch.visibleGroupIndices == [1])
        let manualOnly = RoutingEditorProjection(groups: groups, filter: RoutingEditorFilter(origin: "manual"))
        precondition(manualOnly.ruleIndices == [[0], [0]])
        let after = try encoder.encode(groups.flatMap(\.rules))
        precondition(after == baseline)
    }

    static func testPagingAndStableIndices() {
        let rules = (0..<10_000).map { index in
            RoutingRule(type: "domain", value: "host-\(index).example", action: index.isMultiple(of: 2) ? "proxy" : "direct",
                        group: "大量规则", learned: index.isMultiple(of: 3))
        }
        let groups = [(name: "大量规则", rules: rules)]
        let all = RoutingEditorProjection(groups: groups, filter: RoutingEditorFilter())
        let first = all.page(groupIndex: 0, rules: rules, number: 0, pageSize: 50)
        precondition(first.entries.count == 50 && first.pageCount == 200 && first.firstRecord == 1 && first.lastRecord == 50)
        precondition(!first.canDragReorder)
        let second = all.page(groupIndex: 0, rules: rules, number: 1, pageSize: 50)
        precondition(second.entries.first?.ruleIndex == 50 && second.entries.first?.id == rules[50].id)
        precondition(second.firstRecord == 51 && second.lastRecord == 100)
        let ids = (0..<first.pageCount).flatMap { all.page(groupIndex: 0, rules: rules, number: $0, pageSize: 50).entries.map(\.id) }
        precondition(ids.count == rules.count && Set(ids).count == rules.count)
        let direct = RoutingEditorProjection(groups: groups, filter: RoutingEditorFilter(action: "direct"))
        let filtered = direct.page(groupIndex: 0, rules: rules, number: 1, pageSize: 25)
        precondition(filtered.entries.first?.ruleIndex == 51)
        precondition(filtered.entries.allSatisfy { rules[$0.ruleIndex].action == "direct" })
        precondition(!filtered.canDragReorder)
        let last = all.page(groupIndex: 0, rules: rules, number: Int.max, pageSize: 100)
        precondition(last.number == 99 && last.firstRecord == 9_901 && last.lastRecord == 10_000)
        precondition(all.page(groupIndex: 0, rules: rules, number: -1, pageSize: 1_000).entries.count == 100)
    }

    static func testEmptyGroupsAndClamping() {
        let rule = RoutingRule(type: "domain", value: "one.example", action: "proxy")
        let groups = [(name: "空分组", rules: [RoutingRule]()), (name: "小分组", rules: [rule])]
        let all = RoutingEditorProjection(groups: groups, filter: RoutingEditorFilter(query: " \n "))
        precondition(all.visibleGroupIndices == [0, 1])
        let empty = all.page(groupIndex: 0, rules: [], number: Int.max, pageSize: 0)
        precondition(empty.entries.isEmpty && empty.pageCount == 1 && empty.number == 0 && empty.pageSize == 1)
        precondition(empty.firstRecord == 0 && empty.lastRecord == 0)
        precondition(all.page(groupIndex: 1, rules: [rule], number: 0, pageSize: 25).canDragReorder)
        let noMatch = RoutingEditorProjection(groups: groups, filter: RoutingEditorFilter(query: "missing"))
        precondition(noMatch.visibleGroupIndices.isEmpty && noMatch.matchedCount == 0 && noMatch.totalCount == 1)
        precondition(all.page(groupIndex: 99, rules: [], number: 0, pageSize: 50).entries.isEmpty)
    }
}
