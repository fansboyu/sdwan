#ifndef AppVersion
  #define AppVersion "1.2.0"
#endif

#define AppName "SD-WAN"
#define AppPublisher "EnglishListen"
#define AppExeName "sdwan-tray.exe"
#define ServiceExeName "sdwan-service.exe"
#define BuildDir SourcePath + "..\..\..\build\windows"
#define OutputDir SourcePath + "..\..\..\dist\windows"
#define TrayIcon SourcePath + "..\..\..\cmd\windows-tray\assets\logo.ico"

[Setup]
AppId={{74B677B7-D5F5-4F7B-AC2F-D28EF5E6023E}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
DefaultDirName={autopf}\SD-WAN
DefaultGroupName=SD-WAN
DisableProgramGroupPage=yes
OutputDir={#OutputDir}
OutputBaseFilename=SD-WAN-Setup-v{#AppVersion}-x64
SetupIconFile={#TrayIcon}
UninstallDisplayIcon={app}\{#AppExeName}
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
CloseApplications=yes
RestartApplications=no
MinVersion=10.0
VersionInfoVersion={#AppVersion}.0
VersionInfoProductName={#AppName}
VersionInfoDescription=SD-WAN Windows Client Installer
VersionInfoCompany={#AppPublisher}
VersionInfoCopyright=Copyright (C) {#AppPublisher}
ChangesAssociations=no

[Tasks]
Name: "desktopicon"; Description: "创建桌面快捷方式"; GroupDescription: "快捷方式："; Flags: unchecked
Name: "autostart"; Description: "登录 Windows 后自动启动 SD-WAN 托盘"; GroupDescription: "启动选项："; Flags: checkedonce

[Files]
Source: "{#BuildDir}\sdwan-service.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#BuildDir}\sdwan-tray.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#BuildDir}\wintun.dll"; DestDir: "{app}"; Flags: ignoreversion

[Dirs]
Name: "{commonappdata}\sdwan"; Permissions: admins-full users-readexec

[Icons]
Name: "{group}\SD-WAN"; Filename: "{app}\{#AppExeName}"; WorkingDir: "{app}"
Name: "{group}\卸载 SD-WAN"; Filename: "{uninstallexe}"
Name: "{autodesktop}\SD-WAN"; Filename: "{app}\{#AppExeName}"; WorkingDir: "{app}"; Tasks: desktopicon
Name: "{commonstartup}\SD-WAN"; Filename: "{app}\{#AppExeName}"; WorkingDir: "{app}"; Tasks: autostart

[Run]
Filename: "{app}\{#ServiceExeName}"; Parameters: "install-service --config ""{commonappdata}\sdwan\agent.json"" --auto-start=false"; StatusMsg: "正在安装 SD-WAN 后台服务..."; Flags: runhidden waituntilterminated
Filename: "{app}\{#ServiceExeName}"; Parameters: "start-service"; StatusMsg: "正在恢复 SD-WAN 连接..."; Flags: runhidden waituntilterminated; Check: DeviceConfigExists
Filename: "{app}\{#AppExeName}"; Description: "启动 SD-WAN"; WorkingDir: "{app}"; Flags: nowait postinstall skipifsilent runasoriginaluser

[UninstallRun]
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM {#AppExeName}"; Flags: runhidden waituntilterminated; RunOnceId: "StopTray"
Filename: "{app}\{#ServiceExeName}"; Parameters: "stop-service"; Flags: runhidden waituntilterminated; RunOnceId: "StopService"
Filename: "{app}\{#ServiceExeName}"; Parameters: "uninstall-service"; Flags: runhidden waituntilterminated; RunOnceId: "RemoveService"

[Code]
function ServiceExists(): Boolean;
var
  ResultCode: Integer;
begin
  Result := Exec(ExpandConstant('{sys}\sc.exe'), 'query SDWANService', '', SW_HIDE,
    ewWaitUntilTerminated, ResultCode) and (ResultCode = 0);
end;

function DeviceConfigExists(): Boolean;
begin
  Result := FileExists(ExpandConstant('{commonappdata}\sdwan\agent.json'));
end;

procedure StopAndRemoveExistingService();
var
  ResultCode: Integer;
  ExistingService: String;
begin
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM {#AppExeName}', '', SW_HIDE,
    ewWaitUntilTerminated, ResultCode);

  if not ServiceExists() then
    Exit;

  ExistingService := ExpandConstant('{app}\{#ServiceExeName}');
  if FileExists(ExistingService) then
  begin
    Exec(ExistingService, 'stop-service', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
    Exec(ExistingService, 'uninstall-service', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  end
  else
  begin
    Exec(ExpandConstant('{sys}\sc.exe'), 'stop SDWANService', '', SW_HIDE,
      ewWaitUntilTerminated, ResultCode);
    Sleep(1000);
    Exec(ExpandConstant('{sys}\sc.exe'), 'delete SDWANService', '', SW_HIDE,
      ewWaitUntilTerminated, ResultCode);
  end;
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  StopAndRemoveExistingService();
  Result := '';
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  LegacyStartup: String;
begin
  if CurUninstallStep = usUninstall then
  begin
    LegacyStartup := ExpandConstant('{userappdata}\Microsoft\Windows\Start Menu\Programs\Startup\sdwan-tray.cmd');
    if FileExists(LegacyStartup) then
      DeleteFile(LegacyStartup);
  end;
end;
