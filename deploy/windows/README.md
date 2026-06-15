# SD-WAN Windows 客户端说明

当前 Windows 客户端采用路线 B：

```text
sdwan-tray.exe -> SDWANService/sdwan-service.exe -> wireguard-go -> Wintun
```

也就是说：不再要求用户额外安装 WireGuard for Windows。客户端自己启动用户态 WireGuard，并通过 Wintun 创建 Windows 虚拟网卡。

## 文件清单

发布目录建议如下：

```text
C:\sdwan\
  sdwan-tray.exe
  sdwan-service.exe
  wintun.dll
```

说明：

```text
sdwan-tray.exe      托盘程序，负责 Join、Connect、Disconnect、状态展示
sdwan-service.exe   真正后台服务，负责 Controller 通信、Wintun、wireguard-go
wintun.dll          Wintun 运行库，必须和 sdwan-service.exe 放在同目录
```

历史兼容文件：

```text
sdwan-agent.exe              旧 CLI Agent，Linux/旧 Windows 路线仍可用
sdwan-windows-ui.ps1         旧 PowerShell 简单窗口
sdwan-windows-tray.ps1       旧 PowerShell 托盘
```

当前优先使用 `sdwan-tray.exe + sdwan-service.exe`。

## 推荐安装包

普通用户不需要再手工复制三个文件。可使用 Inno Setup 构建单文件安装包：

```text
dist\windows\SD-WAN-Setup-v1.2.0-x64.exe
```

在项目根目录执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\deploy\windows\build-installer.ps1
```

安装包会：

1. 请求管理员权限并检查 Windows x64。
2. 安装到 `C:\Program Files\SD-WAN`。
3. 停止并移除旧的 `SDWANService`，然后安装新服务。
4. 保留 `C:\ProgramData\sdwan` 中已有的设备配置。
5. 安装 `sdwan-service.exe`、`sdwan-tray.exe` 和 `wintun.dll`。
6. 创建开始菜单快捷方式，并可选创建桌面和开机启动快捷方式。
7. 安装完成后启动托盘，但不会在设备注册前启动后台隧道。

用户入网流程保持不变：登录 Web 管理台，复制 Admin Token，在托盘中点击 `Join from Clipboard`，然后点击 `Connect`。

卸载时会停止并移除 Windows Service，但默认保留 `C:\ProgramData\sdwan`，方便以后升级或重新安装。需要彻底清除设备身份时再手工删除该目录。

## 运行要求

1. Windows 10/11 或 Windows Server。
2. 需要管理员权限运行托盘程序。
3. `wintun.dll` 必须存在。
4. 首次启动时 Windows 可能会请求允许安装/加载 Wintun 驱动。
5. 控制器必须可以访问，例如：

```text
https://controller.englishlisten.cn
```

## 构建命令

在项目根目录执行：

```powershell
$env:GOOS="windows"
$env:GOARCH="amd64"

go build -o build\windows\sdwan-service.exe .\cmd\windows-service
go build -ldflags="-H windowsgui" -o build\windows\sdwan-tray.exe .\cmd\windows-tray
```

如果还需要旧 CLI：

```powershell
go build -o build\windows\sdwan-agent.exe .\cmd\agent
```

复制 Wintun：

```text
build\windows\wintun.dll
```

## 启动步骤

推荐使用管理员 PowerShell：

```powershell
cd C:\sdwan
.\sdwan-tray.exe
```

然后：

1. 打开主控页面。
2. 登录账号。
3. 复制页面里的 `Admin Token`。
4. 右键 Windows 托盘里的 `SD-WAN` 图标。
5. 点击 `Join from Clipboard`。
6. 再点击 `Connect`。

连接成功后，客户端会完成：

```text
1. 注册设备到 Controller
2. 保存 C:\ProgramData\sdwan\agent.json
3. 安装并启动 SDWANService
4. 创建 Wintun 网卡 sdwan0
5. 配置虚拟 IP
6. 拉取 netmap
7. 添加 bootstrap peer
8. 添加 overlay 路由
```

## 托盘菜单说明

```text
Joined               只读状态，打勾表示本机已经注册并生成 agent.json
Connected            只读状态，打勾表示 SDWANService 正在运行
Join from Clipboard  从剪贴板读取 Admin Token 并注册入网
Connect              自动安装并启动 Windows 服务 SDWANService
Restart Service      重启 SDWANService
Disconnect           停止 SDWANService
Auto Start           托盘启动后自动启动服务
Edit Settings        打开 C:\ProgramData\sdwan\tray.json
Open Config Folder   打开 C:\ProgramData\sdwan
Start with Windows   添加开机启动命令
About                显示版本
Exit                 退出托盘程序
```

说明：

```text
Joined / Connected 是状态提示，不是操作按钮。
Auto Start / Start with Windows 是可切换选项，点击后会根据结果自动打勾或取消打勾。
托盘程序会周期性刷新状态，因此服务被外部命令启动或停止后，菜单勾选也会自动更新。
```

## Windows 服务命令

服务名：

```text
SDWANService
```

手动安装：

```powershell
.\sdwan-service.exe install-service
```

启动服务：

```powershell
.\sdwan-service.exe start-service
```

停止服务：

```powershell
.\sdwan-service.exe stop-service
```

卸载服务：

```powershell
.\sdwan-service.exe uninstall-service
```

前台调试运行：

```powershell
.\sdwan-service.exe run
```

查看本地配置：

```powershell
.\sdwan-service.exe status
```

单次同步调试：

```powershell
.\sdwan-service.exe sync
```

## 本地配置文件

```text
C:\ProgramData\sdwan\agent.json
C:\ProgramData\sdwan\tray.json
```

`agent.json` 保存：

```text
device_id
device_token
private_key
public_key
virtual_ip
netmap_version
```

`tray.json` 保存：

```json
{
  "controller_url": "https://controller.englishlisten.cn",
  "auto_sync": true,
  "sync_interval_seconds": 15
}
```

## 当前通信架构

当前版本不做自研 NAT 打洞，也不做 magicsock。

第一阶段通信路径：

```text
Windows Client
  |
  | HTTPS register/poll/netmap
  v
Controller

Windows Client
  |
  | WireGuard UDP 41641
  v
Bootstrap Peer
```

Bootstrap 的作用：

```text
1. 作为固定公网 WireGuard peer
2. 让客户端先有一个稳定可连接的 peer
3. 观察客户端真实 endpoint
4. bootstrap-agent 把 endpoint 回写到 Controller
5. 其他节点通过 netmap 获取 endpoint
```

注意：当前 bootstrap 默认不是完整 relay。复杂 NAT 场景下，客户端之间直连仍可能失败。后续可以扩展成：

```text
bootstrap hub/relay 模式
DERP-like relay
自研 NAT traversal / magicsock
```

## 路由设计

服务启动后会配置：

```text
100.64.0.0/10       走 sdwan0
100.254.254.254/32  走 sdwan0
peer allowed_ips    走 sdwan0
```

其中：

```text
100.64.0.0/10       用户设备地址池
100.254.254.254/32  bootstrap 虚拟地址
```

## 常见问题

### 1. 缺少 wintun.dll

现象：

```text
create wintun failed
cannot load wintun.dll
```

解决：

```text
确认 C:\sdwan\wintun.dll 存在
确认它和 sdwan-service.exe 在同一个目录
```

### 2. 没有管理员权限

现象：

```text
创建网卡失败
安装服务失败
添加路由失败
```

解决：

```text
右键 PowerShell 或 sdwan-tray.exe
选择“以管理员身份运行”
```

### 3. 服务安装失败

先停止和卸载旧服务：

```powershell
.\sdwan-service.exe stop-service
.\sdwan-service.exe uninstall-service
```

再重新连接：

```powershell
.\sdwan-tray.exe
```

右键托盘图标点击：

```text
Connect
```

### 4. 注册后主控页面没有设备

检查是否复制的是 `Admin Token`，不是 `Device Token`。

重新注册前可以删除：

```text
C:\ProgramData\sdwan\agent.json
```

然后重新复制 Admin Token，点击 `Join from Clipboard`。

### 5. 能看到设备，但无法互通

先确认：

```powershell
.\sdwan-service.exe status
```

再检查：

```text
Controller 是否下发 bootstrap_peer
bootstrap 服务端是否有客户端握手
服务端 51872/UDP 是否开放
Windows 防火墙是否拦截
```

### 6. 如何彻底清理客户端

```powershell
cd C:\sdwan
.\sdwan-service.exe stop-service
.\sdwan-service.exe uninstall-service
Remove-Item -Recurse -Force C:\ProgramData\sdwan
```

如需删除程序目录：

```powershell
Remove-Item -Recurse -Force C:\sdwan
```

## 当前边界

已经具备：

```text
托盘程序
Windows Service
wireguard-go
Wintun
Controller 注册
netmap 同步
bootstrap peer 接入
路由配置
```

暂未完成：

```text
安装包
代码签名
完整 GUI 设置页
服务日志查看页
自动升级
relay fallback
自研 NAT 打洞
magicsock
```
