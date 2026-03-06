; Distributed Encoder Agent — Inno Setup 6 installer
;
; Build:
;   ISCC.exe /DAgentVersion=1.2.0 /O"dist" installer\agent-setup.iss
;
; Or via Makefile:
;   make installer VERSION=1.2.0
;
; Requires Inno Setup 6: https://jrsoftware.org/isdl.php
;   choco install innosetup

#ifndef AgentVersion
  #define AgentVersion "0.0.0-dev"
#endif

#define AppName    "Distributed Encoder Agent"
#define AppURL     "https://github.com/badskater/distributed-encoder"
#define ServiceName "distributed-encoder-agent"
#define ConfigDir   "C:\ProgramData\distributed-encoder"
#define ConfigFile  ConfigDir + "\agent.yaml"

; ── [Setup] ────────────────────────────────────────────────────────────────────
[Setup]
AppId={{8E4A7B2C-1F3D-4E5A-9C6B-0D2E8F7A3B4C}
AppName={#AppName}
AppVersion={#AgentVersion}
AppPublisher=badskater
AppPublisherURL={#AppURL}
AppSupportURL={#AppURL}/issues
DefaultDirName=C:\DistEncoder
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
PrivilegesRequired=admin
OutputBaseFilename=distencoder-agent-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
; Uninstall info
UninstallDisplayName={#AppName}
UninstallDisplayIcon={app}\distencoder-agent.exe

; ── [Languages] ───────────────────────────────────────────────────────────────
[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

; ── [Messages] ────────────────────────────────────────────────────────────────
[Messages]
WelcomeLabel2=This wizard will install [name/ver] on your computer.%n%nThe installer will:%n  - Download the agent binary from GitHub Releases%n  - Create the required directory structure%n  - Copy your TLS certificate files%n  - Write the agent configuration file%n  - Install and start the Windows Service%n%nClick Next to continue.
FinishedLabel=Setup has finished installing [name] on your computer.%n%nThe service %"distributed-encoder-agent%" is now running.%n%nNext step: approve this agent in the web UI (Farm Servers -%n Approve) or run:%n%n  docker compose exec controller /app/controller server approve <hostname>

; ── [Dirs] ────────────────────────────────────────────────────────────────────
[Dirs]
Name: "{app}\work"
Name: "{app}\logs"
Name: "{app}\certs"

; ── [Files] ───────────────────────────────────────────────────────────────────
[Files]
; No bundled binary — downloaded during install via CurStepChanged (ssInstall).
; LICENSE is included for the license page.
Source: "..\LICENSE"; DestDir: "{tmp}"; Flags: deleteafterinstall

; ── [Icons] ───────────────────────────────────────────────────────────────────
[Icons]
Name: "{group}\Uninstall {#AppName}"; Filename: "{uninstallexe}"

; ── [Code] ────────────────────────────────────────────────────────────────────
[Code]

{ ── Global variables ────────────────────────────────────────────────────── }
var
  { Wizard pages }
  PageController : TInputQueryWizardPage;
  PageAgent      : TInputQueryWizardPage;
  PageVersion    : TInputQueryWizardPage;
  PageCerts      : TInputDirWizardPage;
  PageTools      : TWizardPage;
  GToolsMemo     : TMemo;

  { Collected values (set from pages at ssInstall time) }
  GInstallDir        : String;
  GControllerAddress : String;
  GAgentHostname     : String;
  GVersion           : String;
  GCertSourceDir     : String;
  GConfigDir         : String;
  GConfigPath        : String;
  GBinaryDest        : String;

{ ── Helpers ─────────────────────────────────────────────────────────────── }

{ Escape backslashes for YAML double-quoted strings.
  C:\foo\bar  ->  C:\\foo\\bar }
function YamlEscapePath(const S: String): String;
var
  I : Integer;
begin
  Result := '';
  for I := 1 to Length(S) do
  begin
    if S[I] = '\' then
      Result := Result + '\\'
    else
      Result := Result + S[I];
  end;
end;

{ Very lightweight host:port validator — just checks a colon is present
  and neither side is empty. }
function IsValidControllerAddress(const S: String): Boolean;
var
  ColonPos : Integer;
begin
  ColonPos := Pos(':', S);
  Result := (ColonPos > 1) and (ColonPos < Length(S));
end;

{ Populate the tool-verification TMemo with FOUND/MISSING status for each
  encoding tool at its default path. }
procedure RunToolVerification;
var
  Names : array[0..5] of String;
  Paths : array[0..5] of String;
  I, Missing : Integer;
  Status, Line : String;
begin
  Names[0] := 'FFmpeg';
  Names[1] := 'FFprobe';
  Names[2] := 'x265';
  Names[3] := 'x264';
  Names[4] := 'AviSynth+';
  Names[5] := 'VapourSynth';

  Paths[0] := 'C:\Tools\ffmpeg\ffmpeg.exe';
  Paths[1] := 'C:\Tools\ffmpeg\ffprobe.exe';
  Paths[2] := 'C:\Tools\x265\x265.exe';
  Paths[3] := 'C:\Tools\x264\x264.exe';
  Paths[4] := 'C:\Program Files\AviSynth+\avs2pipemod.exe';
  Paths[5] := 'C:\Program Files\VapourSynth\vspipe.exe';

  GToolsMemo.Lines.Clear;
  GToolsMemo.Lines.Add('Tool           Path                                       Status');
  GToolsMemo.Lines.Add(StringOfChar('-', 72));

  Missing := 0;
  for I := 0 to 5 do
  begin
    if FileExists(Paths[I]) then
      Status := 'FOUND  '
    else
    begin
      Status := 'MISSING';
      Missing := Missing + 1;
    end;
    { Manual column padding — Format() width specifiers are Inno Setup safe }
    Line := Names[I];
    while Length(Line) < 15 do Line := Line + ' ';
    Line := Line + Paths[I];
    while Length(Line) < 60 do Line := Line + ' ';
    Line := Line + Status;
    GToolsMemo.Lines.Add(Line);
  end;

  GToolsMemo.Lines.Add('');
  if Missing > 0 then
  begin
    GToolsMemo.Lines.Add(IntToStr(Missing) + ' tool(s) not found at default paths.');
    GToolsMemo.Lines.Add('The agent will install, but encoding jobs requiring missing');
    GToolsMemo.Lines.Add('tools will fail. After installation, update tool paths in:');
    GToolsMemo.Lines.Add('  ' + '{#ConfigFile}');
    GToolsMemo.Lines.Add('See DEPLOYMENT.md §1.4 for download links and install notes.');
  end
  else
    GToolsMemo.Lines.Add('All tools found.');
end;

{ Write the agent.yaml configuration file to GConfigPath. }
procedure WriteAgentConfig;
var
  CertDir : String;
  Lines   : TStringList;
begin
  CertDir := GInstallDir + '\certs';

  Lines := TStringList.Create;
  try
    Lines.Add('controller:');
    Lines.Add('  address: "' + GControllerAddress + '"');
    Lines.Add('  tls:');
    Lines.Add('    cert: "' + YamlEscapePath(CertDir + '\' + GAgentHostname + '.crt') + '"');
    Lines.Add('    key:  "' + YamlEscapePath(CertDir + '\' + GAgentHostname + '.key') + '"');
    Lines.Add('    ca:   "' + YamlEscapePath(CertDir + '\ca.crt') + '"');
    Lines.Add('  reconnect:');
    Lines.Add('    initial_delay: 5s');
    Lines.Add('    max_delay: 5m');
    Lines.Add('    multiplier: 2.0');
    Lines.Add('');
    Lines.Add('agent:');
    Lines.Add('  hostname: "' + GAgentHostname + '"');
    Lines.Add('  work_dir:   "' + YamlEscapePath(GInstallDir + '\work') + '"');
    Lines.Add('  log_dir:    "' + YamlEscapePath(GInstallDir + '\logs') + '"');
    Lines.Add('  offline_db: "' + YamlEscapePath(GInstallDir + '\offline.db') + '"');
    Lines.Add('  heartbeat_interval: 30s');
    Lines.Add('  poll_interval: 10s');
    Lines.Add('  cleanup_on_success: true');
    Lines.Add('  keep_failed_jobs: 10');
    Lines.Add('');
    Lines.Add('tools:');
    Lines.Add('  ffmpeg:   "C:\\Tools\\ffmpeg\\ffmpeg.exe"');
    Lines.Add('  ffprobe:  "C:\\Tools\\ffmpeg\\ffprobe.exe"');
    Lines.Add('  x265:     "C:\\Tools\\x265\\x265.exe"');
    Lines.Add('  x264:     "C:\\Tools\\x264\\x264.exe"');
    Lines.Add('  svt_av1:  ""');
    Lines.Add('  avs_pipe: "C:\\Program Files\\AviSynth+\\avs2pipemod.exe"');
    Lines.Add('  vspipe:   "C:\\Program Files\\VapourSynth\\vspipe.exe"');
    Lines.Add('');
    Lines.Add('gpu:');
    Lines.Add('  enabled: true');
    Lines.Add('  vendor: ""');
    Lines.Add('  max_vram_mb: 0');
    Lines.Add('  monitor_interval: 5s');
    Lines.Add('');
    Lines.Add('allowed_shares: []');
    Lines.Add('');
    Lines.Add('logging:');
    Lines.Add('  level: info');
    Lines.Add('  format: json');
    Lines.Add('  max_size_mb: 100');
    Lines.Add('  max_backups: 5');
    Lines.Add('  compress: true');
    Lines.Add('  stream_buffer_size: 1000');
    Lines.Add('  stream_flush_interval: 1s');

    Lines.SaveToFile(GConfigPath);
  finally
    Lines.Free;
  end;
end;

{ Download agent binary from GitHub Releases using PowerShell.
  Returns True on success, False on failure. }
function DownloadBinary(const Version, DestPath: String): Boolean;
var
  Url, PSArgs : String;
  ResultCode  : Integer;
begin
  Url := 'https://github.com/badskater/distributed-encoder/releases/download/v' +
         Version + '/agent-windows-amd64.exe';

  PSArgs := '-NonInteractive -NoProfile -ExecutionPolicy Bypass -Command ' +
            '"[Net.ServicePointManager]::SecurityProtocol = ' +
            '[Net.SecurityProtocolType]::Tls12; ' +
            'Invoke-WebRequest ' +
            '-Uri ''' + Url + ''' ' +
            '-OutFile ''' + DestPath + ''' ' +
            '-UseBasicParsing"';

  Result := Exec(
    ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    PSArgs,
    '',
    SW_HIDE,
    ewWaitUntilTerminated,
    ResultCode
  ) and (ResultCode = 0);
end;

{ ── Inno Setup event handlers ────────────────────────────────────────────── }

procedure InitializeWizard;
begin
  GConfigDir  := '{#ConfigDir}';
  GConfigPath := '{#ConfigFile}';

  { Page 4: Controller address }
  PageController := CreateInputQueryPage(
    wpSelectDir,
    'Controller Connection',
    'Enter the gRPC address of the Distributed Encoder Controller.',
    '');
  PageController.Add('Controller address (host:port):', False);
  PageController.Values[0] := '';

  { Page 5: Agent hostname }
  PageAgent := CreateInputQueryPage(
    PageController.ID,
    'Agent Identity',
    'Enter a name that identifies this encoding server.',
    'This name appears in the web UI and log files.');
  PageAgent.Add('Agent hostname:', False);
  PageAgent.Values[0] := GetComputerNameString;

  { Page 6: Release version to download }
  PageVersion := CreateInputQueryPage(
    PageAgent.ID,
    'Agent Binary',
    'The installer will download the agent binary from GitHub Releases.',
    'Leave blank only if you have already placed the binary manually at:' + #13#10 +
    ExpandConstant('{app}') + '\distencoder-agent.exe');
  PageVersion.Add('Release version to download (e.g. 1.2.0 — without the v prefix):', False);
  PageVersion.Values[0] := '';

  { Page 7: Certificate source directory }
  PageCerts := CreateInputDirPage(
    PageVersion.ID,
    'TLS Certificates',
    'Select the folder containing the agent TLS certificate files.',
    'Required files: ca.crt,  <hostname>.crt,  <hostname>.key' + #13#10 +
    'These will be copied into the install directory.',
    False, '');
  PageCerts.Add('');

  { Page 8: Tool verification (read-only memo) }
  PageTools := CreateCustomPage(
    PageCerts.ID,
    'Encoding Tools',
    'Checking for encoding tools at their default installation paths.');

  GToolsMemo := TMemo.Create(WizardForm);
  GToolsMemo.Parent     := PageTools.Surface;
  GToolsMemo.Left       := 0;
  GToolsMemo.Top        := 0;
  GToolsMemo.Width      := PageTools.SurfaceWidth;
  GToolsMemo.Height     := PageTools.SurfaceHeight;
  GToolsMemo.ReadOnly   := True;
  GToolsMemo.ScrollBars := ssVertical;
  GToolsMemo.Font.Name  := 'Courier New';
  GToolsMemo.Font.Size  := 8;
  GToolsMemo.Color      := $F5F5F5;
end;

procedure CurPageChanged(CurPageID: Integer);
begin
  if CurPageID = PageTools.ID then
    RunToolVerification;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;

  if CurPageID = PageController.ID then
  begin
    if Trim(PageController.Values[0]) = '' then
    begin
      MsgBox('Controller address is required.' + #13#10 +
             'Example: encoder.example.com:9443', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    if not IsValidControllerAddress(Trim(PageController.Values[0])) then
    begin
      MsgBox('Address must be in host:port format.' + #13#10 +
             'Example: encoder.example.com:9443', mbError, MB_OK);
      Result := False;
      Exit;
    end;
  end;

  if CurPageID = PageAgent.ID then
  begin
    if Trim(PageAgent.Values[0]) = '' then
    begin
      MsgBox('Agent hostname is required.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
  end;

  { Cert page: missing files produce a warning but do not block. }
  { The user may not have certs ready yet — service can be started later. }
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  CertDir, SrcCA, SrcCert, SrcKey : String;
  ResultCode                       : Integer;
  MissingCerts                     : Integer;
begin
  { ── Capture page values ──────────────────────────────────────────────── }
  GInstallDir        := WizardDirValue;
  GControllerAddress := Trim(PageController.Values[0]);
  GAgentHostname     := Trim(PageAgent.Values[0]);
  GVersion           := Trim(PageVersion.Values[0]);
  GCertSourceDir     := PageCerts.Values[0];
  GBinaryDest        := GInstallDir + '\distencoder-agent.exe';

  if CurStep = ssInstall then
  begin

    { ── Step 2: Resolve agent binary ────────────────────────────────────── }
    if GVersion <> '' then
    begin
      if not DownloadBinary(GVersion, GBinaryDest) then
      begin
        MsgBox(
          'Failed to download agent binary.' + #13#10 +
          'Possible causes:' + #13#10 +
          '  - No internet access from this server' + #13#10 +
          '  - Incorrect version string (check GitHub Releases for valid tags)' + #13#10 +
          '  - GitHub is temporarily unavailable' + #13#10#13#10 +
          'Fix the issue and re-run the installer.',
          mbError, MB_OK);
        WizardForm.Close;
        Exit;
      end;
    end
    else
    begin
      if not FileExists(GBinaryDest) then
      begin
        MsgBox(
          'No version was specified and no binary was found at:' + #13#10 +
          GBinaryDest + #13#10#13#10 +
          'Either enter a version number to download, or copy' + #13#10 +
          'distencoder-agent.exe to that path before running the installer.',
          mbError, MB_OK);
        WizardForm.Close;
        Exit;
      end;
    end;

    { ── Step 3: Copy certificates ───────────────────────────────────────── }
    CertDir := GInstallDir + '\certs';
    SrcCA   := GCertSourceDir + '\ca.crt';
    SrcCert := GCertSourceDir + '\' + GAgentHostname + '.crt';
    SrcKey  := GCertSourceDir + '\' + GAgentHostname + '.key';

    if FileExists(SrcCA)   then FileCopy(SrcCA,   CertDir + '\ca.crt', False);
    if FileExists(SrcCert) then FileCopy(SrcCert, CertDir + '\' + GAgentHostname + '.crt', False);
    if FileExists(SrcKey)  then FileCopy(SrcKey,  CertDir + '\' + GAgentHostname + '.key', False);

    MissingCerts := 0;
    if not FileExists(CertDir + '\ca.crt')                             then MissingCerts := MissingCerts + 1;
    if not FileExists(CertDir + '\' + GAgentHostname + '.crt')        then MissingCerts := MissingCerts + 1;
    if not FileExists(CertDir + '\' + GAgentHostname + '.key')        then MissingCerts := MissingCerts + 1;

    if MissingCerts > 0 then
      MsgBox(
        IntToStr(MissingCerts) + ' certificate file(s) not found.' + #13#10 +
        'The service will install but will not connect to the controller' + #13#10 +
        'until all three files are present in:' + #13#10 +
        CertDir,
        mbInformation, MB_OK);

    { ── Step 4: Write agent.yaml ─────────────────────────────────────────── }
    ForceDirectories(GConfigDir);
    WriteAgentConfig;

  end; { ssInstall }

  if CurStep = ssPostInstall then
  begin

    { ── Step 6: Stop and remove any existing service ──────────────────────── }
    Exec(GBinaryDest, 'stop',                                    '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
    Exec(GBinaryDest, 'uninstall --config "' + GConfigPath + '"', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);

    { ── Step 6 (cont): Install service ────────────────────────────────────── }
    if not Exec(GBinaryDest,
                'install --config "' + GConfigPath + '"',
                '', SW_HIDE, ewWaitUntilTerminated, ResultCode)
       or (ResultCode <> 0) then
    begin
      MsgBox(
        'Service installation failed (exit code ' + IntToStr(ResultCode) + ').' + #13#10 +
        'Verify the binary at: ' + GBinaryDest + #13#10#13#10 +
        'The configuration file has been written to:' + #13#10 + GConfigPath + #13#10 +
        'You can install the service manually:' + #13#10 +
        '  ' + GBinaryDest + ' install --config "' + GConfigPath + '"',
        mbError, MB_OK);
      Exit;
    end;

    { ── Step 7: Start service ──────────────────────────────────────────────── }
    if not Exec(GBinaryDest, 'start', '', SW_HIDE, ewWaitUntilTerminated, ResultCode)
       or (ResultCode <> 0) then
      MsgBox(
        'The service was installed but could not be started.' + #13#10 +
        'Check Event Viewer → Windows Logs → Application,' + #13#10 +
        'source: distributed-encoder-agent.' + #13#10#13#10 +
        'Or check the log files at: ' + GInstallDir + '\logs',
        mbInformation, MB_OK);

  end; { ssPostInstall }
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  BinaryPath : String;
  ResultCode : Integer;
begin
  if CurUninstallStep = usUninstall then
  begin
    BinaryPath := ExpandConstant('{app}') + '\distencoder-agent.exe';
    Exec(BinaryPath, 'stop',                                    '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
    Exec(BinaryPath, 'uninstall --config "' + '{#ConfigFile}' + '"', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  end;
end;
