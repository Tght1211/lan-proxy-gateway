import Foundation

@main struct ModelDecodingTests {
    static func main() throws { try ModelDecodingTests().testIngressCompatibility(); print("Model decoding tests passed") }
    func testIngressCompatibility() throws {
        let old = try JSONDecoder().decode(RelayStats.self, from: Data("{\"recent\":[{\"id\":1,\"started_at\":0}]}".utf8))
        precondition(old.ingress.isEmpty && old.recent[0].ingressTitle == "网关接入")
        let current = try JSONDecoder().decode(RelayStats.self, from: Data("{\"active\":[{\"id\":2,\"started_at\":0,\"ingress\":\"http-proxy\"}]}".utf8))
        precondition(current.active[0].isHTTPProxy && current.active[0].ingressTitle == "HTTP 代理")
    }

}
