; EncodeSwarmr Desktop — Inno Setup 6 installer
;
; Build:
;   ISCC.exe /DDesktopVersion=1.2.0 /O"dist" installer\desktop-setup.iss
;
; Or via Makefile:
;   make installer-desktop VERSION=1.2.0
;
; Requires Inno Setup 6: https://jrsoftware.org/isdl.php
;   choco install innosetup

#ifndef DesktopVersion
  #define DesktopVersion "0.0.0-dev"
#endif

#define AppName    "EncodeSwarmr Desktop"
#define AppURL     "https://github.com/badskater/encodeswarmr"
#define AppExeName "encodeswarmr-desktop.exe"

; ── [Setup] ────────────────────────────────────────────────────────────────────
[Setup]
AppId={{B8A3F2D1-7E4C-4B5A-9D6F-1A2B3C4D5E6F}
AppName={#AppName}
AppVersion={#DesktopVersion}
AppPublisher=badskater
AppPublisherURL={#AppURL}
AppSupportURL={#AppURL}/issues
DefaultDirName={autopf}\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
OutputBaseFilename=encodeswarmr-desktop-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
; Uninstall info
UninstallDisplayName={#AppName}
UninstallDisplayIcon={app}\{#AppExeName}

; ── [Languages] ───────────────────────────────────────────────────────────────
[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

; ── [Tasks] ───────────────────────────────────────────────────────────────────
[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

; ── [Files] ───────────────────────────────────────────────────────────────────
[Files]
Source: "..\bin\encodeswarmr-desktop.exe"; DestDir: "{app}"; Flags: ignoreversion

; ── [Icons] ───────────────────────────────────────────────────────────────────
[Icons]
Name: "{group}\{#AppName}";                    Filename: "{app}\{#AppExeName}"
Name: "{group}\{cm:UninstallProgram,{#AppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#AppName}";              Filename: "{app}\{#AppExeName}"; Tasks: desktopicon

; ── [Run] ─────────────────────────────────────────────────────────────────────
[Run]
Filename: "{app}\{#AppExeName}"; Description: "{cm:LaunchProgram,{#AppName}}"; Flags: nowait postinstall skipifsilent
