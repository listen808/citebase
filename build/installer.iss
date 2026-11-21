; build/installer.iss — Citebase Windows 安装器
;
; 编译期需要 Inno Setup 6（产物 Setup.exe 本身无任何运行时依赖）。
; 由 scripts\package-windows.bat 的 [7/7] 步骤调用：
;   "%ISCC%" /DStageDir=dist\citebase /DBuildVersion=2.0.0 /Odist\Citebase-Setup-2.0.0.exe build\installer.iss
;
; 本文件必须存为 UTF-8 with BOM；[Code] 段必须全 ASCII —— Inno 的 Pascal Script
; 编译器对这一段的非 ASCII 文本解析不可靠（中文会让它报 Unknown identifier /
; Unknown type，已踩过）。面向用户的中文说明由 package-windows.bat 生成为
; 包内的 DATABASE-SETUP.sql 与 README.txt，安装器只引用文件路径。
;
; 设计约束（原因见 docs\release\windows-packaging.md）：
;   1. 快捷方式指向 start.bat 而不是 citebase.exe —— 静态资源按 CWD 相对路径读取
;      （app\server\server.go:154-177），exe 的 CWD 必须是安装目录。
;   2. 默认装到 {localappdata}\Citebase 而不是 Program Files —— data\logs、data\backups、
;      data\.jwt_secret 等运行期写入是硬编码相对路径（internal\log\log.go:63、
;      internal\service\admin\backup.go:37），Program Files 默认不可写。
;   3. 卸载默认保留 data 目录（索引/上传/日志），删除需用户显式确认。

#ifndef StageDir
  #error StageDir 未定义：请用 /DStageDir=<staging 目录> 调用 ISCC
#endif
#ifndef BuildVersion
  #define BuildVersion "0.0.0"
#endif

#define AppName    "Citebase"
#define AppExeName "citebase.exe"

[Setup]
AppId={{3F2A1C64-9B7D-4E52-A1F3-8D6C0B5E7A94}
AppName={#AppName}
AppVersion={#BuildVersion}
AppVerName={#AppName} {#BuildVersion}
DefaultDirName={localappdata}\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
; per-user install: the target dir lives in the user profile, no UAC elevation needed
PrivilegesRequired=lowest
OutputBaseFilename=Citebase-Setup-{#BuildVersion}
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
; only close our own process, never the user's browser / AV
CloseApplications=yes
CloseApplicationsFilter={#AppExeName}
RestartApplications=no
SetupLogging=yes
UninstallDisplayIcon={app}\{#AppExeName}
VersionInfoVersion={#BuildVersion}
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
AllowNoIcons=yes

[Files]
; the staging dir is produced by package-windows.bat and is the single source of
; truth shared by the portable zip and this installer
Source: "{#StageDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\start.bat"; WorkingDir: "{app}"
Name: "{group}\Uninstall {#AppName}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\start.bat"; WorkingDir: "{app}"

[Run]
Filename: "{app}\start.bat"; Description: "Launch Citebase (configure the database first, see DATABASE-SETUP.sql)"; WorkingDir: "{app}"; Flags: nowait postinstall skipifsilent

[Code]

// Inno has no port API; probe with netstat once and only warn (never block install)
function PortInUse(const Port: String): Boolean;
var
  ResultCode: Integer;
  Lines: TArrayOfString;
  Probe: String;
begin
  Result := False;
  Probe := ExpandConstant('{tmp}\citebase-port-probe.txt');
  if not Exec(ExpandConstant('{cmd}'),
    '/C netstat -ano ^| findstr /R /C:":' + Port + ' .*LISTENING" > "' + Probe + '"',
    '', SW_HIDE, ewWaitUntilTerminated, ResultCode) then
    Exit;
  if LoadStringsFromFile(Probe, Lines) then
    Result := Length(Lines) > 0;
end;

// Kill a running instance before install-over / uninstall, otherwise the exe
// stays locked and its files cannot be overwritten
procedure StopRunningInstance;
var
  ResultCode: Integer;
begin
  Exec(ExpandConstant('{cmd}'), '/C taskkill /F /IM {#AppExeName} >nul 2>nul',
    '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssInstall then
    StopRunningInstance;
  if CurStep = ssPostInstall then
  begin
    ; // An empty database is enough. The first start runs AutoMigrate + seedDefaults,
    ; // which creates all 94 tables, the permission list, roles, system config and
    ; // the admin account -- no need to import table.sql / data.sql.
    ; // Step-by-step Chinese instructions live in DATABASE-SETUP.sql and README.txt.
    ; // CurPageChanged is not used on purpose: TSetupPage is not resolvable in this
    ; ; // script context, ssPostInstall does the same job.
    if PortInUse('3001') then
      MsgBox('Port 3001 is already in use. Change server.port in' + #13#10 +
             ExpandConstant('{app}\configs\config.yaml') + ' before starting.' + #13#10 + #13#10 +
             'Database setup: see DATABASE-SETUP.sql in the install folder.',
             mbError, MB_OK);
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  DataDir: String;
begin
  if CurUninstallStep = usUninstall then
  begin
    StopRunningInstance;
    ; // an upgrade overwrites configs\config.yaml, keep a copy
    if FileExists(ExpandConstant('{app}\configs\config.yaml')) then
      CopyFile(ExpandConstant('{app}\configs\config.yaml'),
               ExpandConstant('{app}\configs\config.yaml.bak'), False);
  end;
  if CurUninstallStep = usPostUninstall then
  begin
    DataDir := ExpandConstant('{app}\data');
    if DirExists(DataDir) then
      if MsgBox('Delete the runtime data folder as well (data: search index, uploaded files, logs)?' + #13#10 +
                'Answer No to keep it - reinstalling into the same path keeps your data.',
                mbConfirmation, MB_YESNO) = IDYES then
        DelTree(DataDir, True, True, True);
  end;
end;
