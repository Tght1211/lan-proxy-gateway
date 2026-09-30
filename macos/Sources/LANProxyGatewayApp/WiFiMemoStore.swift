import Foundation
import Security

/// Passwords never enter UserDefaults, logs, exports or agent snapshots.
enum WiFiMemoStore {
    private static let service = "LANProxyGateway.WiFiMemo"
    private static var query: [String:Any] { [kSecClass as String:kSecClassGenericPassword,kSecAttrService as String:service,kSecAttrAccount as String:"hotspot"] }
    static func password() throws -> String {
        var q=query;q[kSecReturnData as String]=true;q[kSecMatchLimit as String]=kSecMatchLimitOne
        var result:CFTypeRef?
        let status=SecItemCopyMatching(q as CFDictionary,&result)
        if status == errSecItemNotFound { return "" }
        guard status == errSecSuccess,let data=result as? Data else { throw failure(status) }
        return String(data:data,encoding:.utf8) ?? ""
    }
    static func save(password:String) throws {
        if password.isEmpty { let status=SecItemDelete(query as CFDictionary);guard status == errSecSuccess || status == errSecItemNotFound else {throw failure(status)};return }
        let data=Data(password.utf8)
        let status=SecItemUpdate(query as CFDictionary,[kSecValueData as String:data] as CFDictionary)
        if status == errSecItemNotFound {
            var q=query;q[kSecValueData as String]=data;q[kSecAttrAccessible as String]=kSecAttrAccessibleWhenUnlockedThisDeviceOnly
            let added=SecItemAdd(q as CFDictionary,nil);guard added == errSecSuccess else {throw failure(added)}
        } else if status != errSecSuccess { throw failure(status) }
    }
    private static func failure(_ code:OSStatus)->NSError { NSError(domain:NSOSStatusErrorDomain,code:Int(code),userInfo:[NSLocalizedDescriptionKey:(SecCopyErrorMessageString(code,nil) as String?) ?? "无法访问系统钥匙串"]) }
}
