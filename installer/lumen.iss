; Lumen installer — Inno Setup 6.
;
; Per-user install by design: goes to %LOCALAPPDATA%\Programs\Lumen, needs no
; admin rights and raises no UAC prompt. Lumen only ever touches the current
; user's own Plex tokens and cache, so there is nothing a machine-wide install
; would buy us except a consent dialog.
;
; Build via ..\build.ps1 -Installer, which passes MyAppVersion in. Compiling
; this file directly falls back to 0.0.0-dev.
;
;   ISCC.exe /DMyAppVersion=1.0.0 installer\lumen.iss

#ifndef MyAppVersion
  #define MyAppVersion "0.0.0-dev"
#endif

#define MyAppName        "Lumen"
#define MyAppPublisher   "Byron"
#define MyAppURL         "https://github.com/dknz7/lumen"
#define MyAppExeName     "lumen.exe"

[Setup]
; Never change AppId — it is how Windows recognises an upgrade rather than a
; second parallel install.
AppId={{7660B14F-CBB4-4FED-8727-3C58D2043245}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}/issues
AppUpdatesURL={#MyAppURL}/releases
VersionInfoVersion={#MyAppVersion}

DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
DisableDirPage=auto
LicenseFile=..\LICENSE
OutputDir=..\dist
OutputBaseFilename=LumenSetup
SetupIconFile=..\assets\lumen.ico
UninstallDisplayIcon={app}\{#MyAppExeName}
UninstallDisplayName={#MyAppName}

; Per-user: {autopf} resolves to {localappdata}\Programs under lowest privileges.
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog

; Refuse to run the 32-bit installer path; Lumen is amd64 only.
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.19041

Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern

; Matches the mutex lumen.exe creates at startup. Lets Setup detect a running
; instance and ask the user to close it rather than failing on a locked file.
AppMutex=Lumen.SingleInstance.Mutex

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon";  Description: "Create a &desktop shortcut"; GroupDescription: "Shortcuts:"
Name: "startupicon";  Description: "Start Lumen when I sign in to Windows"; GroupDescription: "Startup:"; Flags: unchecked

[Files]
Source: "..\lumen.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE";   DestDir: "{app}"; DestName: "LICENSE.txt"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}";                 Filename: "{app}\{#MyAppExeName}"
Name: "{group}\{#MyAppName} on GitHub";       Filename: "{#MyAppURL}"
Name: "{autodesktop}\{#MyAppName}";           Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon
Name: "{userstartup}\{#MyAppName}";           Filename: "{app}\{#MyAppExeName}"; Parameters: "--tray"; Tasks: startupicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "Launch {#MyAppName}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
Type: filesandordirs; Name: "{app}"

[Code]
var
  DownloadPage: TDownloadWizardPage;

{ WebView2 registers its version under one of these keys. Machine-wide install
  lands in HKLM (WOW6432Node even on x64 — the updater is 32-bit); per-user
  lands in HKCU. Checking all three is the documented detection method. }
function WebView2Installed: Boolean;
var
  V: String;
begin
  Result :=
    (RegQueryStringValue(HKLM, 'SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', V) and (V <> '') and (V <> '0.0.0.0')) or
    (RegQueryStringValue(HKLM, 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', V) and (V <> '') and (V <> '0.0.0.0')) or
    (RegQueryStringValue(HKCU, 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', V) and (V <> '') and (V <> '0.0.0.0'));
end;

function OnDownloadProgress(const Url, Filename: String; const Progress, ProgressMax: Int64): Boolean;
begin
  Result := True;
end;

procedure InitializeWizard;
begin
  DownloadPage := CreateDownloadPage(
    SetupMessage(msgWizardPreparing),
    'Lumen needs the Microsoft Edge WebView2 runtime to draw its window.',
    @OnDownloadProgress);
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if (CurPageID = wpReady) and not WebView2Installed then
  begin
    DownloadPage.Clear;
    DownloadPage.Add(
      'https://go.microsoft.com/fwlink/p/?LinkId=2124703',
      'MicrosoftEdgeWebview2Setup.exe', '');
    DownloadPage.Show;
    try
      try
        DownloadPage.Download;
      except
        { Not fatal. Windows 11 and current Windows 10 ship WebView2 anyway, and
          a user behind a proxy shouldn't be blocked from installing Lumen. }
        SuppressibleMsgBox(
          'Could not download the WebView2 runtime:' + #13#10#13#10 +
          GetExceptionMessage + #13#10#13#10 +
          'Setup will continue. If Lumen fails to open a window, install WebView2 ' +
          'manually from https://developer.microsoft.com/microsoft-edge/webview2/',
          mbInformation, MB_OK, IDOK);
      end;
    finally
      DownloadPage.Hide;
    end;
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  ResultCode: Integer;
begin
  if CurStep = ssInstall then
  begin
    if not WebView2Installed and FileExists(ExpandConstant('{tmp}\MicrosoftEdgeWebview2Setup.exe')) then
      Exec(ExpandConstant('{tmp}\MicrosoftEdgeWebview2Setup.exe'), '/silent /install',
           '', SW_SHOW, ewWaitUntilTerminated, ResultCode);
  end;
end;

{ Config holds the user's Plex tokens and their whole cache. Deleting it on
  uninstall without asking is the kind of thing that makes people angry when
  they were only reinstalling. }
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  DataDir: String;
begin
  if CurUninstallStep = usPostUninstall then
  begin
    DataDir := ExpandConstant('{userappdata}\Lumen');
    if DirExists(DataDir) then
    begin
      if SuppressibleMsgBox(
           'Also delete your Lumen settings and cache?' + #13#10#13#10 +
           DataDir + #13#10#13#10 +
           'This removes your saved Plex sign-in, server list, preferences and ' +
           'cached artwork. Choose No if you are reinstalling or upgrading.',
           mbConfirmation, MB_YESNO or MB_DEFBUTTON2, IDNO) = IDYES then
        DelTree(DataDir, True, True, True);
    end;
  end;
end;
