import AppKit
import SwiftUI

struct WiFiGuideView: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.dismiss) private var dismiss
    @State private var step=0
    @State private var category="通用设备"
    @State private var console="PS5"
    @State private var region="港版"
    @State private var band="5 GHz"
    @State private var ssid=UserDefaults.standard.string(forKey:"hotspotMemoSSID") ?? "GrandLine"
    @State private var security=UserDefaults.standard.string(forKey:"hotspotMemoSecurity") ?? "WPA2/WPA3 个人级"
    @State private var password=""
    @State private var message=""
    @State private var passwordLoaded=false
    var body: some View {
        VStack(alignment:.leading,spacing:20) {
            HStack { Text("Wi-Fi 接入指南").font(.title2.weight(.semibold));Spacer();Button { dismiss() } label:{Image(systemName:"xmark")}.buttonStyle(.plain).accessibilityLabel("关闭") }
            HStack {
                ForEach(Array(["Mac 共享","Wi-Fi 设置","连接设备"].enumerated()),id:\.offset) { index,title in
                    HStack { Text(index < step ? "✓":"\(index+1)").frame(width:26,height:26).background(index == step ? Theme.cyan.opacity(0.2):Theme.panelRaised).clipShape(Circle());Text(title).foregroundStyle(index == step ? Color.primary:Theme.muted) };if index<2 {Spacer()}
                }
            }.font(.callout)
            Divider()
            ScrollView {
                VStack(alignment:.leading,spacing:18) {
                    if step == 0 {
                        Text("把 Mac 的网络共享给 Wi-Fi").font(.title3.weight(.semibold))
                        SetupValue("系统设置", "通用 → 共享 → 互联网共享")
                        SetupValue("共享来源 / 共享给", "以太网 → Wi-Fi")
                        Text("先打开 Mac 的 Wi-Fi，再设置热点名称、安全性与密码。").foregroundStyle(Theme.muted)
                        Button("打开系统共享设置") { NSWorkspace.shared.open(URL(string:"x-apple.systempreferences:com.apple.Sharing-Settings.extension")!) }.buttonStyle(StudioButtonStyle(primary:true))
                    } else if step == 1 {
                        StudioTabs(title:"接入设备",selection:$category,items:[("通用设备","手机 / 电脑","smartphone"),("游戏主机","游戏主机","gamecontroller")])
                        if category == "游戏主机" {
                            HStack {
                                StudioSelect(title:"主机型号",selection:$console,options:["PS5","PS5 Slim","PS5 Pro","Switch","Switch Lite","Switch OLED","Switch 2"].map{($0,$0)})
                                StudioSelect(title:"主机销售版本",selection:$region,options:["港版","日版","美版","其他"].map{($0,$0)})
                            }
                        }
                        StudioTabs(title:"使用环境",selection:$band,items:[("5 GHz","近距离 · 5 GHz",""),("2.4 GHz","隔墙 / 兼容 · 2.4 GHz","")])
                        HStack { Text("建议优先尝试").foregroundStyle(Theme.muted);Spacer();Text(band == "5 GHz" ? "36":"1 / 6 / 11").font(.system(size:30,weight:.semibold)).foregroundStyle(Theme.cyan);Text("信道") }
                        Text(band == "5 GHz" ? "备选 40 / 44 / 48，以 Mac 当地可选项为准。":"选择干扰较少的可用信道；不同时开启双频。") .font(.caption).foregroundStyle(Theme.muted)
                        Divider()
                        HStack { Text("网络名称").foregroundStyle(Theme.muted); TextField("GrandLine",text:$ssid).textFieldStyle(StudioFieldStyle()) }
                        HStack { Text("安全性").foregroundStyle(Theme.muted);StudioSelect(title:"安全性",selection:$security,options:["WPA2/WPA3 个人级","WPA2 个人级","WPA3 个人级"].map{($0,$0)}) }
                        HStack { SecureField("密码备忘（至少 8 位）",text:$password).textFieldStyle(StudioFieldStyle());Button("读取备忘") { do {password=try WiFiMemoStore.password();passwordLoaded=true;message="已从钥匙串读取"} catch {message=error.localizedDescription} } }
                        if category == "游戏主机" { Text("Switch / Lite / OLED 请选择兼容 WPA2 的安全性；地区指硬件销售版本。").font(.caption).foregroundStyle(Theme.muted) }
                        HStack { Button("保存备忘") { saveMemo() }.buttonStyle(StudioButtonStyle());Text(message).font(.caption).foregroundStyle(Theme.muted) }
                        Text("这里仅保存备忘。实际生效请在系统 Wi-Fi 选项中填写相同内容。").font(.caption).foregroundStyle(Theme.muted)
                    } else {
                        Text("连接 \(ssid.isEmpty ? "你的热点":ssid)").font(.title3.weight(.semibold))
                        Text(category == "通用设备" ? "在手机或电脑的 Wi-Fi 列表选择热点，输入密码。":console.hasPrefix("PS5") ? "设置 → 网络 → 设定 → 设定互联网连接。":"系统设置 → 互联网 → 互联网设置。")
                        Text("IP 与 DNS 自动获取，设备端代理关闭。").foregroundStyle(Theme.muted)
                        Divider()
                        Label(model.hotspotControlState.title,systemImage:model.hotspotControlState == .enabled ? "checkmark.circle":"wifi").foregroundStyle(Theme.cyan)
                        Text(model.hotspotControlDetail).font(.caption).foregroundStyle(Theme.muted)
                        Button(model.hotspotControlState == .enabled ? "停止热点接管":"开启热点接管") { Task {await model.configureHotspot(model.hotspotControlState == .enabled ? "disable":"enable")} }.buttonStyle(StudioButtonStyle(primary:true)).disabled(model.isBusy)
                        Text("完成后访问网页或运行主机连接测试，再查看流量记录。").font(.caption).foregroundStyle(Theme.muted)
                    }
                }.frame(maxWidth:.infinity,alignment:.leading)
            }
            Divider()
            HStack { Text("系统负责创建热点，App 负责分流。").font(.caption).foregroundStyle(Theme.muted);Spacer();if step>0 {Button("上一步"){step-=1}};Button(step == 2 ? "完成":"下一步"){if step==2 {dismiss()}else{step+=1}}.buttonStyle(StudioButtonStyle(primary:true)) }
        }.padding(26).frame(width:680,height:620).foregroundStyle(Theme.text).background(Theme.panel).task{await model.refreshHotspot()}
    }
    private func saveMemo() {
        guard !ssid.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty,ssid.utf8.count <= 32 else {message="网络名称需为 1–32 字节";return}
        guard password.isEmpty || ((8...63).contains(password.utf8.count) && password.utf8.allSatisfy { (32...126).contains($0) }) else {message="密码需为 8–63 位可打印英文字符";return}
        do {
            if !password.isEmpty || passwordLoaded {try WiFiMemoStore.save(password:password)}
            UserDefaults.standard.set(ssid,forKey:"hotspotMemoSSID");UserDefaults.standard.set(security,forKey:"hotspotMemoSecurity");message="备忘已保存，密码存于系统钥匙串"
        } catch {message=error.localizedDescription}
    }
}
