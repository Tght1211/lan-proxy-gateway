import SwiftUI

struct LearningSettingsView: View {
    @EnvironmentObject private var model: AppModel
    @State private var settings = LearningSettings(enabled: true, confirmations: 1, autoSave: true)
    @State private var loaded = false
    private var valid: Bool { settings.maxDirectWaitSeconds >= settings.directWaitSeconds }
    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 24) {
                HStack(spacing:14) {
                    StudioIcon("sparkles").frame(width:30,height:30).foregroundStyle(Theme.cyan)
                    VStack(alignment:.leading,spacing:5) {
                        Text("自动选择可用路径").font(.system(size:19,weight:.semibold))
                        Text("为未配置的域名补充策略").font(.system(size:12)).foregroundStyle(Theme.muted)
                    }
                    Spacer(); Toggle("自动学习", isOn: $settings.enabled).toggleStyle(.switch).font(.system(size:12))
                }
                HStack(alignment:.top,spacing:22) {
                    VStack(alignment:.leading,spacing:12) {
                        stage("01 · 先直连", "等待响应数据")
                        number("首次直连等待",value:$settings.directWaitSeconds,choices:[1,3,5,10,15,30],unit:"秒")
                    }
                    VStack(alignment:.leading,spacing:12) {
                        stage("02 · 再尝试", "无响应，尝试代理")
                        field("尝试出口") { StudioSelect(title:"尝试出口",selection:.constant("proxy"),options:[("proxy",model.status?.proxy == nil ? "尚未配置代理源":"代理出口")]).disabled(true) }
                    }
                    VStack(alignment:.leading,spacing:12) {
                        stage("03 · 确认后学习", "有响应才记住路径")
                        number("有效响应确认",value:$settings.confirmations,choices:Array(1...10),unit:"次")
                    }
                }
                Divider()
                LazyVGrid(columns:Array(repeating:GridItem(.flexible(),alignment:.leading),count:3),alignment:.leading,spacing:22) {
                    number("代理等待",value:$settings.proxyWaitSeconds,choices:[1,3,5,10,15,30],unit:"秒")
                    field("双方失败后") { StudioSelect(title:"双方失败后",selection:.constant("extend"),options:[("extend","逐次延长直连等待")]).disabled(true) }
                    number("直连等待上限",value:$settings.maxDirectWaitSeconds,choices:[5,10,15,30,60],unit:"秒")
                    number("失败后代理冷却",value:$settings.cooldownSeconds,choices:[5,10,30,60,120,300,600],unit:"秒")
                    field("记录方式") { StudioSelect(title:"记录方式",selection:Binding(get:{settings.autoSave ? "auto":"manual"},set:{settings.autoSave = $0 == "auto"}),options:[("auto","自动保存"),("manual","由我确认")]) }
                    number("临时判断保留",value:$settings.memoryMinutes,choices:[1,5,10,15,30,60],unit:"分钟")
                }
                if !settings.responsePolicySupported { Text("当前核心不支持调整等待参数；安装新版 CLI 并重启核心后可编辑。").font(.caption).foregroundStyle(Theme.yellow) }
                HStack(alignment:.center,spacing:18) {
                    Text(valid ? "双方失败不学习；直连等待逐次翻倍。仅对可安全重试的连接换路。":"等待上限不能小于首次等待。")
                        .font(.system(size:11)).foregroundStyle(valid ? Theme.muted:Theme.coral)
                    Spacer()
                    Button("保存学习配置") {
                        guard let data=try? JSONEncoder().encode(settings),let json=String(data:data,encoding:.utf8) else{return}
                        Task { await model.learningAction("configure",host:json) }
                    }.buttonStyle(StudioButtonStyle(primary:true)).disabled(model.isBusy || !loaded || !valid)
                }
                Label("明确规则及其兜底链优先；自动学习不会覆盖它们。",systemImage:"checkmark.shield")
                    .font(.system(size:12)).foregroundStyle(Theme.cyan)
            }
        }.onReceive(model.$stats) { stats in
            if !loaded,let current=stats?.fallback?.settings {settings=current;loaded=true}
        }
    }
    private func stage(_ title:String,_ headline:String)->some View {
        VStack(alignment:.leading,spacing:12) {
            Rectangle().fill(Theme.border).frame(height:2)
            Text(title).font(.system(size:11)).foregroundStyle(Theme.muted)
            Text(headline).font(.system(size:13,weight:.semibold))
        }
    }
    private func field<Content:View>(_ title:String,@ViewBuilder content:()->Content)->some View {
        VStack(alignment:.leading,spacing:9){Text(title).font(.system(size:11)).foregroundStyle(Theme.muted);content()}.frame(maxWidth:.infinity,alignment:.leading)
    }
    private func number(_ title:String,value:Binding<Int>,choices:[Int],unit:String)->some View {
        field(title) { StudioSelect(title:title,selection:Binding(get:{String(value.wrappedValue)},set:{if let number=Int($0){value.wrappedValue=number}}),options:Array(Set(choices+[value.wrappedValue])).sorted().map{(String($0),"\($0) \(unit)")}).disabled(!settings.responsePolicySupported && title != "有效响应确认") }
    }
}
