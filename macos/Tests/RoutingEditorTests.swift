import Foundation

@main struct RoutingEditorTests {
    static func main() throws {
        try testFilteringPreservesRules()
        testPagingAndStableIndices()
        testEmptyGroupsAndClamping()
        testInterleavedOrderAndTextRoundTrip()
        testPresetReinsertion()
        testDiffAndMoves()
        testMergeLearningAndDraft()
        testConflictsAndResolution()
        testConcurrentAdditionsAndDuplicates()
        testLargeMerge()
        print("Routing editor filtering, priority, diff and three-way merge tests passed")
    }


    static func rule(_ value: String, group: String = "A") -> RoutingRule {
        RoutingRule(type: "domain", value: value, action: "proxy", group: group)
    }
    static func decoded(_ rules: [RoutingRule]) -> [RoutingRule] {
        try! JSONDecoder().decode([RoutingRule].self, from: JSONEncoder().encode(rules))
    }
    static func testInterleavedOrderAndTextRoundTrip() {
        let original = [rule("a"), rule("b", group: "B"), rule("c"), RoutingRule(type: "domain", value: "learned", action: "proxy", group: "B", learned: true)]
        let groups = makeGroupDrafts(original)
        let restored = RoutingDraftOrder.rules(flattenGroups(groups), order: original.map(\.id))
        precondition(RoutingRuleComparison.same(restored, original), "display must not regroup global priorities")
        precondition(RoutingRuleComparison.changes(from: original, to: restored).isEmpty)
        let parsed = parseRuleLines(serializeRuleLines(restored) + "\n# comment\n")
        precondition(parsed.skipped.isEmpty && RoutingRuleComparison.same(parsed.rules, original))
        let order = RoutingDraftOrder.replacingSlots(original.map(\.id), with: [original[2].id, original[0].id])
        precondition(order == [original[2].id, original[1].id, original[0].id, original[3].id])
    }
    static func testPresetReinsertion() {
        let preset = [rule("preset.example")]
        var draft = RoutingPresetInsertion.rules(preset, excluding: [])
        precondition(draft[0].id != preset[0].id)
        precondition(RoutingPresetInsertion.rules(preset, excluding: draft).isEmpty)
        let order = draft.map(\.id)
        draft[0].value = "edited.example"
        draft += RoutingPresetInsertion.rules(preset, excluding: draft)
        let ordered = RoutingDraftOrder.rules(draft, order: order)
        precondition(ordered.map(\.value) == ["edited.example", "preset.example"])
        precondition(Set(ordered.map(\.id)).count == 2)
        precondition(RoutingRuleComparison.changes(from: [], to: ordered).count == 2)
    }

    static func testDiffAndMoves() {
        let base = [rule("a"), rule("b"), rule("c")]
        precondition(RoutingRuleComparison.changes(from: base, to: decoded(base)).isEmpty)
        let added = RoutingRuleComparison.changes(from: base, to: [rule("new")] + decoded(base))
        precondition(added.count == 1 && added[0].kind == "新增", "insertions must not label all later rules moved")
        var edited = base; edited[1].action = "direct"
        let changes = RoutingRuleComparison.changes(from: base, to: decoded(edited))
        precondition(changes.count == 1 && changes[0].kind == "修改" && changes[0].details == "动作")
        let reordered = RoutingRuleComparison.changes(from: base, to: [base[1], base[0], base[2]])
        precondition(reordered.count == 1 && reordered[0].kind == "调整顺序")
        let removed = RoutingRuleComparison.changes(from: base, to: [base[0], base[2]])
        precondition(removed.count == 1 && removed[0].kind == "删除" && removed[0].title == "b")
    }
    static func testMergeLearningAndDraft() {
        let base = [rule("a"), rule("b", group: "B"), rule("c")]
        var draft = base; draft[0].action = "direct"
        var learned = rule("new", group: "自动学习"); learned.learned = true
        let latest = decoded(base + [learned])
        let merged = RoutingRuleMerge(baseline: base, local: draft, latest: latest).resolved()!
        precondition(merged.map(\.value) == ["a", "b", "c", "new"] && merged[0].action == "direct" && merged[3].learned)
        var external = base; external[2].group = "Moved group"
        let both = RoutingRuleMerge(baseline: base, local: draft, latest: decoded(external)).resolved()!
        precondition(both[0].action == "direct" && both[2].group == "Moved group")
        let reordering = RoutingRuleMerge(baseline: base, local: [base[2], base[0], base[1]], latest: latest).resolved()!
        precondition(reordering.map(\.value) == ["c", "a", "b", "new"], "background appends stay at the end after local reordering")
        let noEdit = RoutingRuleMerge(baseline: base, local: base, latest: decoded([base[1], base[0], base[2]])).resolved()!
        precondition(noEdit.map(\.value) == ["b", "a", "c"])
    }
    static func testConflictsAndResolution() {
        let base = [rule("a"), rule("b"), rule("c")]
        var mine = base; mine[0].action = "direct"
        var remote = base; remote[0].action = "reject"; remote.append(rule("learned"))
        let conflict = RoutingRuleMerge(baseline: base, local: mine, latest: decoded(remote))
        precondition(conflict.conflicts.count == 1 && conflict.resolved() == nil)
        let ours = conflict.resolved(choices: [base[0].id: .local])!
        let theirs = conflict.resolved(choices: [base[0].id: .latest])!
        precondition(ours[0].action == "direct" && theirs[0].action == "reject" && ours.last?.value == "learned")
        let deletion = RoutingRuleMerge(baseline: base, local: mine, latest: decoded(Array(base.dropFirst())))
        precondition(deletion.conflicts.count == 1 && deletion.resolved() == nil)
        precondition(deletion.resolved(choices: [base[0].id: .latest])!.map(\.value) == ["b", "c"])
        let orders = RoutingRuleMerge(baseline: base, local: [base[1],base[0],base[2]], latest: decoded([base[0],base[2],base[1]]))
        precondition(orders.orderConflict && orders.resolved() == nil)
        precondition(orders.resolved(order: .local)!.map(\.value) == ["b","a","c"])
        precondition(orders.resolved(order: .latest)!.map(\.value) == ["a","c","b"])
        let same = RoutingRuleMerge(baseline: base, local: mine, latest: decoded(mine))
        precondition(!same.needsResolution && same.resolved()![0].action == "direct")
    }
    static func testConcurrentAdditionsAndDuplicates() {
        let a = rule("a"), new = rule("new")
        let same = RoutingRuleMerge(baseline: [a], local: [a,new], latest: decoded([a,new]))
        precondition(same.resolved()!.count == 2)
        var other = new; other.action = "reject"
        let conflict = RoutingRuleMerge(baseline: [a], local: [a,new], latest: decoded([a,other]))
        precondition(conflict.conflicts.count == 1 && conflict.resolved() == nil)
        let duplicate = [rule("a"), rule("a")]
        precondition(RoutingRuleComparison.changes(from: duplicate, to: decoded(duplicate)).isEmpty)
        var changed = duplicate; changed[1].action = "direct"
        let diff = RoutingRuleComparison.changes(from: duplicate, to: decoded(changed))
        precondition(diff.count == 1 && diff[0].kind == "修改")
        let result = RoutingRuleMerge(baseline: duplicate, local: changed, latest: decoded(duplicate + [new])).resolved()!
        precondition(result.count == 3 && result[1].action == "direct")
    }
    static func testLargeMerge() {
        let base = (0..<10_000).map { rule("host-\($0)", group: "group-\($0 % 10)") }
        var local = base; local[4321].action = "direct"
        let start = Date()
        let merged = RoutingRuleMerge(baseline: base, local: local, latest: decoded(base + [rule("new")])).resolved()!
        precondition(merged.count == 10_001 && merged[4321].action == "direct")
        print("10,000-rule merge: \(Date().timeIntervalSince(start) * 1000) ms")
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
