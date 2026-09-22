import Foundation

enum GatewayClientError: LocalizedError {
    case engineNotFound
    case commandFailed(String)
    case invalidOutput(String)

    var errorDescription: String? {
        switch self {
        case .engineNotFound:
            return "App 中没有找到 gateway 核心程序，请重新安装。"
        case .commandFailed(let message):
            return message
        case .invalidOutput(let message):
            return "无法解析 gateway 返回的数据：\(message)"
        }
    }
}

struct GatewayClient {
    var bundledEngineURL: URL? {
        Bundle.main.resourceURL?.appendingPathComponent("gateway")
    }

    private var engineURL: URL? {
        if let bundledEngineURL,
           FileManager.default.isExecutableFile(atPath: bundledEngineURL.path) {
            return bundledEngineURL
        }
        let developmentCandidates = [
            URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
                .appendingPathComponent("gateway"),
            URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
                .deletingLastPathComponent().appendingPathComponent("gateway"),
            URL(fileURLWithPath: "/usr/local/bin/gateway"),
            URL(fileURLWithPath: "/opt/homebrew/bin/gateway")
        ]
        return developmentCandidates.first {
            FileManager.default.isExecutableFile(atPath: $0.path)
        }
    }

    func status() async throws -> GatewayStatus {
        let data = try await run(arguments: ["status", "--json"], privileged: false).data
        let decoder = JSONDecoder()
        do {
            return try decoder.decode(GatewayStatus.self, from: data)
        } catch {
            throw GatewayClientError.invalidOutput(error.localizedDescription)
        }
    }

    func stats(apiPort: Int, configFile: String) async throws -> RuntimeStats {
        let url = URL(string: "http://127.0.0.1:\(apiPort)/api/stats")!
        var request = URLRequest(url: url)
        let tokenURL = URL(fileURLWithPath: configFile).deletingLastPathComponent().appendingPathComponent("api-token")
        let token = try String(contentsOf: tokenURL, encoding: .utf8).trimmingCharacters(in: .whitespacesAndNewlines)
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        request.timeoutInterval = 2
        let (data, response) = try await URLSession.shared.data(for: request)
        guard let http = response as? HTTPURLResponse, http.statusCode == 200 else {
            throw GatewayClientError.commandFailed("核心服务状态接口暂时不可用。")
        }
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try decoder.decode(RuntimeStats.self, from: data)
    }

    func initialize() async throws -> String {
        try await output(arguments: ["init"], privileged: false)
    }

    func start() async throws -> String {
        try await output(arguments: ["start"], privileged: true)
    }

    func stop() async throws -> String {
        try await output(arguments: ["stop"], privileged: true)
    }

    func restart() async throws -> String {
        try await output(arguments: ["restart"], privileged: true)
    }

    func setProxy(type: String, host: String, port: Int) async throws -> String {
        try await output(
            arguments: ["system-proxy", "on", "--type", type, "--host", host, "--port", String(port)],
            privileged: true
        )
    }

    func testProxy(type: String, host: String, port: Int) async throws -> String {
        try await output(
            arguments: ["system-proxy", "test", "--type", type, "--host", host, "--port", String(port)],
            privileged: false
        )
    }

    func setDirect() async throws -> String {
        try await output(arguments: ["system-proxy", "off"], privileged: true)
    }

    func learningAction(_ action: String, host: String) async throws -> String {
        try await output(arguments: ["learning", action, host], privileged: false)
    }

    func setRoutingRules(_ rules: [RoutingRule]) async throws -> String {
        let data = try JSONEncoder().encode(rules)
        guard let json = String(data: data, encoding: .utf8) else {
            throw GatewayClientError.invalidOutput("无法编码分流规则")
        }
        return try await output(arguments: ["routing", "set", "--rules-json", json], privileged: false)
    }

    func setLANHTTPProxy(enabled: Bool, port: Int, auth: String, username: String, password: String?) async throws -> String {
        guard let engineURL else { throw GatewayClientError.engineNotFound }
        var config: [String: Any] = ["enabled": enabled, "port": port, "auth": auth, "username": username]
        if let password { config["password"] = password }
        let input = try JSONSerialization.data(withJSONObject: config)
        return try await runProcess(executable: engineURL, arguments: ["http-proxy", "set"], input: input).text
    }

    func installService() async throws -> String {
        guard let source = bundledEngineURL ?? engineURL else {
            throw GatewayClientError.engineNotFound
        }
        let installed = "/usr/local/bin/gateway"
        let command = [
            "/bin/mkdir -p /usr/local/bin",
            "/bin/cp \(shellQuote(source.path)) \(shellQuote(installed))",
            "/bin/chmod 755 \(shellQuote(installed))",
            "/usr/bin/env \(privilegedEnvironment()) \(shellQuote(installed)) service install"
        ].joined(separator: " && ")
        return try await runPrivilegedShell(command).text
    }

    func uninstallService() async throws -> String {
        try await output(arguments: ["service", "uninstall"], privileged: true)
    }

    func readLog(path: String) async -> String {
        await Task.detached {
            guard let handle = FileHandle(forReadingAtPath: path) else { return "" }
            defer { try? handle.close() }
            do {
                let end = try handle.seekToEnd()
                try handle.seek(toOffset: end > 262_144 ? end - 262_144 : 0)
                let data = try handle.readToEnd() ?? Data()
                return String(decoding: data, as: UTF8.self).split(separator: "\n", omittingEmptySubsequences: false).suffix(500).joined(separator: "\n")
            } catch { return "读取日志失败：\(error.localizedDescription)" }
        }.value
    }

    func exportAgentSkill(to destination: URL) async throws -> String {
        try await output(arguments: ["skill", "export", "--output", destination.path, "--force"], privileged: false)
    }

    func serviceStatus() async throws -> String {
        try await output(arguments: ["service", "status"], privileged: false)
    }

    func installCLI() async throws -> String {
        guard let source = bundledEngineURL,
              FileManager.default.isExecutableFile(atPath: source.path) else {
            throw GatewayClientError.engineNotFound
        }
        let command = [
            "/bin/mkdir -p /usr/local/bin",
            "/bin/cp \(shellQuote(source.path)) /usr/local/bin/gateway",
            "/bin/chmod 755 /usr/local/bin/gateway"
        ].joined(separator: " && ")
        return try await runPrivilegedShell(command).text
    }

    func natDiag(apiPort: Int, configFile: String) async throws -> NATDiagResult {
        let url = URL(string: "http://127.0.0.1:\(apiPort)/api/nat-diag")!
        var request = URLRequest(url: url)
        let tokenURL = URL(fileURLWithPath: configFile).deletingLastPathComponent().appendingPathComponent("api-token")
        let token = try String(contentsOf: tokenURL, encoding: .utf8).trimmingCharacters(in: .whitespacesAndNewlines)
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        request.timeoutInterval = 10
        let (data, response) = try await URLSession.shared.data(for: request)
        guard let http = response as? HTTPURLResponse, http.statusCode == 200 else {
            throw GatewayClientError.commandFailed("NAT 诊断接口暂时不可用。")
        }
        return try JSONDecoder().decode(NATDiagResult.self, from: data)
    }

    private func output(arguments: [String], privileged: Bool) async throws -> String {
        try await run(arguments: arguments, privileged: privileged).text
    }

    private func run(arguments: [String], privileged: Bool) async throws -> CommandResult {
        guard let engineURL else { throw GatewayClientError.engineNotFound }
        if privileged {
            let args = arguments.map(shellQuote).joined(separator: " ")
            return try await runPrivilegedShell(
                "/usr/bin/env \(privilegedEnvironment()) \(shellQuote(engineURL.path)) \(args)"
            )
        }
        return try await runProcess(executable: engineURL, arguments: arguments)
    }

    private func runPrivilegedShell(_ command: String) async throws -> CommandResult {
        let escaped = command
            .replacingOccurrences(of: "\\", with: "\\\\")
            .replacingOccurrences(of: "\"", with: "\\\"")
        return try await runProcess(
            executable: URL(fileURLWithPath: "/usr/bin/osascript"),
            arguments: ["-e", "do shell script \"\(escaped)\" with administrator privileges"]
        )
    }

    func runProcess(executable: URL, arguments: [String], input: Data? = nil) async throws -> CommandResult {
        try await Task.detached {
            let process = Process()
            let stdin = Pipe()
            if input != nil { process.standardInput = stdin }
            let stdout = Pipe()
            let stderr = Pipe()
            process.executableURL = executable
            process.arguments = arguments
            process.standardOutput = stdout
            process.standardError = stderr
            do {
                try process.run()
            } catch {
                throw GatewayClientError.commandFailed(error.localizedDescription)
            }
            if let input {
                try stdin.fileHandleForWriting.write(contentsOf: input)
                try stdin.fileHandleForWriting.close()
            }
            async let out = stdout.fileHandleForReading.readToEnd() ?? Data()
            async let err = stderr.fileHandleForReading.readToEnd() ?? Data()
            process.waitUntilExit()
            let (outputData, errorData) = try await (out, err)
            guard process.terminationStatus == 0 else {
                let message = String(decoding: errorData.isEmpty ? outputData : errorData, as: UTF8.self)
                    .trimmingCharacters(in: .whitespacesAndNewlines)
                throw GatewayClientError.commandFailed(message.isEmpty ? "gateway 命令执行失败。" : message)
            }
            return CommandResult(data: outputData)
        }.value
    }

    private func privilegedEnvironment() -> String {
        userIdentityEnvironment().map { "\($0.key)=\(shellQuote($0.value))" }
            .sorted().joined(separator: " ")
    }

    private func userIdentityEnvironment() -> [String: String] {
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        let username = NSUserName()
        return [
            "HOME": home,
            "SUDO_USER": username,
            "SUDO_UID": String(getuid()),
            "SUDO_GID": String(getgid())
        ]
    }

    private func shellQuote(_ value: String) -> String {
        "'" + value.replacingOccurrences(of: "'", with: "'\\''") + "'"
    }
}

struct CommandResult {
    let data: Data
    var text: String {
        String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
    }
}
