import Foundation

struct RoutingEditorFilter: Equatable {
    var query = ""
    var action = "all"
    var origin = "all"

    var search: String { query.trimmingCharacters(in: .whitespacesAndNewlines) }
    var isActive: Bool { !search.isEmpty || action != "all" || origin != "all" }

    func matches(_ rule: RoutingRule, group: String) -> Bool {
        guard action == "all" || rule.action == action else { return false }
        guard origin == "all" || (origin == "learned" ? rule.learned : !rule.learned) else { return false }
        return search.isEmpty || group.localizedCaseInsensitiveContains(search) ||
            rule.value.localizedCaseInsensitiveContains(search) || rule.type.localizedCaseInsensitiveContains(search)
    }
}

struct RoutingEditorPage {
    struct Entry: Identifiable {
        let id: UUID
        let ruleIndex: Int
    }

    let entries: [Entry]
    let number: Int
    let pageCount: Int
    let total: Int
    let pageSize: Int
    let canDragReorder: Bool

    var firstRecord: Int { total == 0 ? 0 : number * pageSize + 1 }
    var lastRecord: Int { total == 0 ? 0 : firstRecord + entries.count - 1 }
}

struct RoutingEditorProjection {
    let visibleGroupIndices: [Int]
    let ruleIndices: [[Int]]
    let matchedCount: Int
    let totalCount: Int
    let filter: RoutingEditorFilter

    init(groups: [(name: String, rules: [RoutingRule])], filter: RoutingEditorFilter) {
        self.filter = filter
        let indices = groups.map { group in
            group.rules.indices.filter { filter.matches(group.rules[$0], group: group.name) }
        }
        ruleIndices = indices
        visibleGroupIndices = groups.indices.filter {
            !indices[$0].isEmpty || (!filter.isActive && groups[$0].rules.isEmpty)
        }
        matchedCount = indices.reduce(0) { $0 + $1.count }
        totalCount = groups.reduce(0) { $0 + $1.rules.count }
    }

    func page(groupIndex: Int, rules: [RoutingRule], number: Int, pageSize: Int) -> RoutingEditorPage {
        let size = min(100, max(1, pageSize))
        let indices = ruleIndices.indices.contains(groupIndex) ? ruleIndices[groupIndex] : []
        let pages = indices.isEmpty ? 1 : (indices.count - 1) / size + 1
        let current = min(max(0, number), pages - 1)
        let entries = indices.dropFirst(current * size).prefix(size).compactMap { index -> RoutingEditorPage.Entry? in
            guard rules.indices.contains(index) else { return nil }
            return RoutingEditorPage.Entry(id: rules[index].id, ruleIndex: index)
        }
        return RoutingEditorPage(entries: entries, number: current, pageCount: pages, total: indices.count,
                                 pageSize: size, canDragReorder: !filter.isActive && pages == 1)
    }
}

// Presets are templates: every insertion owns fresh draft identities.
enum RoutingPresetInsertion {
    static func rules(_ template: [RoutingRule], excluding existing: [RoutingRule]) -> [RoutingRule] {
        var keys = Set(existing.map { "\($0.type)|\($0.value.lowercased())" })
        return template.compactMap { rule in
            guard keys.insert("\(rule.type)|\(rule.value.lowercased())").inserted else { return nil }
            var copy = rule
            copy.id = UUID()
            return copy
        }
    }
}

// Grouping is a presentation detail. Keep the original global priority even
// when rules from one group are interleaved with rules from other groups.
enum RoutingDraftOrder {
    static func rules(_ grouped: [RoutingRule], order: [UUID]) -> [RoutingRule] {
        let byID = Dictionary(uniqueKeysWithValues: grouped.map { ($0.id, $0) })
        let known = Set(order)
        return order.compactMap { byID[$0] } + grouped.filter { !known.contains($0.id) }
    }

    static func replacingSlots(_ order: [UUID], with reordered: [UUID]) -> [UUID] {
        let members = Set(reordered)
        var iterator = reordered.makeIterator()
        return order.map { members.contains($0) ? (iterator.next() ?? $0) : $0 }
    }
}

struct RoutingRuleContent: Hashable {
    let type, value, action, group: String
    let learned: Bool
    init(_ rule: RoutingRule) {
        type = rule.type; value = rule.value; action = rule.action
        group = rule.group; learned = rule.learned
    }
}

struct RoutingRuleChange: Identifiable {
    let id: UUID
    let before: RoutingRule?
    let after: RoutingRule?
    let oldPosition: Int?
    let newPosition: Int?
    let moved: Bool
    var kind: String {
        if before == nil { return "新增" }
        if after == nil { return "删除" }
        return RoutingRuleContent(before!) == RoutingRuleContent(after!) ? "调整顺序" : "修改"
    }
    var title: String { (after ?? before)?.value ?? "" }
    var details: String {
        guard let before, let after else { return "" }
        var fields: [String] = []
        if before.type != after.type { fields.append("匹配类型") }
        if before.value != after.value { fields.append("目标") }
        if before.action != after.action { fields.append("动作") }
        if before.group != after.group { fields.append("分组") }
        if before.learned != after.learned { fields.append("来源") }
        if moved { fields.append("优先级 #\(oldPosition ?? 0) → #\(newPosition ?? 0)") }
        return fields.joined(separator: " · ")
    }
}

enum RoutingRuleComparison {
    private struct Selector: Hashable { let type, value: String }

    // Decoding creates new UUIDs. Reconcile exact occurrences first, then
    // unambiguous same-target edits. Local edits already retain their UUIDs.
    static func aligned(_ rules: [RoutingRule], to baseline: [RoutingRule]) -> [RoutingRule] {
        var result = rules
        let baseIDs = Set(baseline.map(\.id))
        var used = Set(rules.map(\.id)).intersection(baseIDs)
        var assigned = Set(rules.indices.filter { baseIDs.contains(rules[$0].id) })
        var exact: [RoutingRuleContent: [UUID]] = [:]
        for rule in baseline.reversed() where !used.contains(rule.id) { exact[RoutingRuleContent(rule), default: []].append(rule.id) }
        for i in rules.indices where !assigned.contains(i) {
            if let id = exact[RoutingRuleContent(rules[i])]?.popLast() {
                result[i].id = id; used.insert(id); assigned.insert(i)
            }
        }
        var remainingBase: [Selector: [UUID]] = [:]
        var remainingSide: [Selector: [Int]] = [:]
        for rule in baseline where !used.contains(rule.id) {
            remainingBase[Selector(type: rule.type, value: rule.value), default: []].append(rule.id)
        }
        for i in rules.indices where !assigned.contains(i) {
            remainingSide[Selector(type: rules[i].type, value: rules[i].value), default: []].append(i)
        }
        for (key, indices) in remainingSide where indices.count == 1 {
            if let ids = remainingBase[key], ids.count == 1 { result[indices[0]].id = ids[0] }
        }
        return result
    }

    static func same(_ lhs: [RoutingRule], _ rhs: [RoutingRule]) -> Bool {
        lhs.map(RoutingRuleContent.init) == rhs.map(RoutingRuleContent.init)
    }

    static func movedIDs(baseline: [RoutingRule], rules: [RoutingRule]) -> Set<UUID> {
        let oldIDs = Set(baseline.map(\.id)), newIDs = Set(rules.map(\.id))
        let old = baseline.map(\.id).filter { newIDs.contains($0) }
        let new = rules.map(\.id).filter { oldIDs.contains($0) }
        return Set(new.difference(from: old).inferringMoves().compactMap { change in
            if case .insert(_, let id, let association) = change, association != nil { return id }
            return nil
        })
    }

    static func changes(from baseline: [RoutingRule], to rules: [RoutingRule]) -> [RoutingRuleChange] {
        let side = aligned(rules, to: baseline)
        let previous = Dictionary(uniqueKeysWithValues: baseline.enumerated().map { ($0.element.id, ($0.offset + 1, $0.element)) })
        let current = Set(side.map(\.id))
        let moved = movedIDs(baseline: baseline, rules: side)
        var changes: [RoutingRuleChange] = []
        for (i, rule) in side.enumerated() {
            let old = previous[rule.id]
            if old == nil || RoutingRuleContent(old!.1) != RoutingRuleContent(rule) || moved.contains(rule.id) {
                changes.append(RoutingRuleChange(id: rule.id, before: old?.1, after: rule,
                    oldPosition: old?.0, newPosition: i + 1, moved: moved.contains(rule.id)))
            }
        }
        for (i, rule) in baseline.enumerated() where !current.contains(rule.id) {
            changes.append(RoutingRuleChange(id: rule.id, before: rule, after: nil, oldPosition: i + 1, newPosition: nil, moved: false))
        }
        return changes
    }
}

struct RoutingRuleConflict: Identifiable {
    let id: UUID
    let baseline: RoutingRule?
    let local: RoutingRule?
    let latest: RoutingRule?
}

struct RoutingRuleMerge {
    enum Choice: String { case local, latest }
    let baseline: [RoutingRule]
    let local: [RoutingRule]
    let latest: [RoutingRule]
    let conflicts: [RoutingRuleConflict]
    let orderConflict: Bool
    private let merged: [UUID: RoutingRule]
    private let localReordered: Bool
    var needsResolution: Bool { !conflicts.isEmpty || orderConflict }

    init(baseline: [RoutingRule], local: [RoutingRule], latest: [RoutingRule]) {
        self.baseline = baseline
        let mine = RoutingRuleComparison.aligned(local, to: baseline)
        let baseIDs = Set(baseline.map(\.id))
        // Match concurrent additions too: identical additions collapse; differing
        // actions for the same new target require an explicit choice.
        let remote = RoutingRuleComparison.aligned(latest, to: baseline + mine.filter { !baseIDs.contains($0.id) })
        self.local = mine; self.latest = remote
        let old = Dictionary(uniqueKeysWithValues: baseline.map { ($0.id, $0) })
        let left = Dictionary(uniqueKeysWithValues: mine.map { ($0.id, $0) })
        let right = Dictionary(uniqueKeysWithValues: remote.map { ($0.id, $0) })
        var result: [UUID: RoutingRule] = [:]
        var conflicts: [RoutingRuleConflict] = []
        let ids = baseline.map(\.id) + mine.filter { !baseIDs.contains($0.id) }.map(\.id) + remote.filter { old[$0.id] == nil && left[$0.id] == nil }.map(\.id)
        for id in ids {
            let original = old[id], localRule = left[id], latestRule = right[id]
            let before = original.map(RoutingRuleContent.init)
            let a = localRule.map(RoutingRuleContent.init), b = latestRule.map(RoutingRuleContent.init)
            if a == before { result[id] = latestRule }
            else if b == before || a == b { result[id] = localRule }
            else { conflicts.append(RoutingRuleConflict(id: id, baseline: original, local: localRule, latest: latestRule)) }
        }
        self.merged = result; self.conflicts = conflicts
        localReordered = !RoutingRuleComparison.movedIDs(baseline: baseline, rules: mine).isEmpty
        let remoteReordered = !RoutingRuleComparison.movedIDs(baseline: baseline, rules: remote).isEmpty
        let common = Set(mine.map(\.id)).intersection(remote.map(\.id)).intersection(baseIDs)
        orderConflict = localReordered && remoteReordered && mine.map(\.id).filter { common.contains($0) } != remote.map(\.id).filter { common.contains($0) }
    }

    func resolved(choices: [UUID: Choice] = [:], order: Choice? = nil) -> [RoutingRule]? {
        guard (!orderConflict || order != nil), conflicts.allSatisfy({ choices[$0.id] != nil }) else { return nil }
        var result = merged
        for conflict in conflicts { result[conflict.id] = choices[conflict.id] == .local ? conflict.local : conflict.latest }
        let useLocal = orderConflict ? order == .local : localReordered
        let primary = useLocal ? local : latest
        let secondary = useLocal ? latest : local
        var ids = primary.map(\.id).filter { result[$0] != nil }
        var present = Set(ids)
        // Insert the other side's new rules beside its nearest surviving anchor.
        // Existing global order is retained; additions never regroup the list.
        let secondaryRules = secondary.filter { result[$0.id] != nil }
        let lastAnchor = secondaryRules.lastIndex(where: { present.contains($0.id) }) ?? -1
        var previous: UUID?
        for (index, rule) in secondaryRules.enumerated() {
            if !present.contains(rule.id) {
                let position = index > lastAnchor ? ids.count : previous.flatMap { ids.firstIndex(of: $0) }.map { $0 + 1 } ?? 0
                ids.insert(rule.id, at: position); present.insert(rule.id)
            }
            previous = rule.id
        }
        return ids.compactMap { result[$0] }
    }
}

struct RuleGroupDraft: Identifiable, Equatable {
    var id = UUID()
    var name: String // "" = ungrouped
    var rules: [RoutingRule]
    var collapsed = false
}

func makeGroupDrafts(_ rules: [RoutingRule]) -> [RuleGroupDraft] {
    var order: [String] = []
    var bucket: [String: [RoutingRule]] = [:]
    for rule in rules {
        if bucket[rule.group] == nil {
            order.append(rule.group)
            bucket[rule.group] = []
        }
        bucket[rule.group]?.append(rule)
    }
    return order.map { RuleGroupDraft(name: $0, rules: bucket[$0] ?? []) }
}

func flattenGroups(_ groups: [RuleGroupDraft]) -> [RoutingRule] {
    groups.flatMap { group -> [RoutingRule] in
        let name = group.name.trimmingCharacters(in: .whitespaces)
        return group.rules.map { rule in
            var copy = rule
            copy.group = name
            return copy
        }
    }
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
