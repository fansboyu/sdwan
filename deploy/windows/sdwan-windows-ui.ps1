Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$ErrorActionPreference = "Stop"

$agentExe = Join-Path $PSScriptRoot "sdwan-agent.exe"
if (!(Test-Path $agentExe)) {
    $agentExe = "sdwan-agent.exe"
}

$programData = $env:ProgramData
if ([string]::IsNullOrWhiteSpace($programData)) {
    $programData = "C:\ProgramData"
}
$configDir = Join-Path $programData "sdwan"
$configPath = Join-Path $configDir "agent.json"
$wgConfigPath = Join-Path $configDir "sdwan0.conf"

function Test-IsAdmin {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Invoke-Agent {
    param([string[]]$Arguments)

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $agentExe
    foreach ($arg in $Arguments) {
        [void]$psi.ArgumentList.Add($arg)
    }
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true

    $process = [System.Diagnostics.Process]::Start($psi)
    $stdout = $process.StandardOutput.ReadToEnd()
    $stderr = $process.StandardError.ReadToEnd()
    $process.WaitForExit()

    if ($process.ExitCode -ne 0) {
        throw "sdwan-agent exited with code $($process.ExitCode)`r`n$stderr`r`n$stdout"
    }
    return $stdout.Trim()
}

function Append-Log {
    param([string]$Text)
    $timestamp = Get-Date -Format "HH:mm:ss"
    $output.AppendText("[$timestamp] $Text`r`n")
}

$form = New-Object System.Windows.Forms.Form
$form.Text = "SD-WAN Windows Client"
$form.Size = New-Object System.Drawing.Size(720, 520)
$form.StartPosition = "CenterScreen"

$labelController = New-Object System.Windows.Forms.Label
$labelController.Text = "Controller"
$labelController.Location = New-Object System.Drawing.Point(20, 22)
$labelController.Size = New-Object System.Drawing.Size(120, 24)
$form.Controls.Add($labelController)

$controllerBox = New-Object System.Windows.Forms.TextBox
$controllerBox.Text = "https://controller.englishlisten.cn"
$controllerBox.Location = New-Object System.Drawing.Point(150, 20)
$controllerBox.Size = New-Object System.Drawing.Size(520, 24)
$form.Controls.Add($controllerBox)

$labelToken = New-Object System.Windows.Forms.Label
$labelToken.Text = "Admin Token"
$labelToken.Location = New-Object System.Drawing.Point(20, 58)
$labelToken.Size = New-Object System.Drawing.Size(120, 24)
$form.Controls.Add($labelToken)

$tokenBox = New-Object System.Windows.Forms.TextBox
$tokenBox.Location = New-Object System.Drawing.Point(150, 56)
$tokenBox.Size = New-Object System.Drawing.Size(520, 24)
$form.Controls.Add($tokenBox)

$joinButton = New-Object System.Windows.Forms.Button
$joinButton.Text = "Join"
$joinButton.Location = New-Object System.Drawing.Point(20, 100)
$joinButton.Size = New-Object System.Drawing.Size(110, 34)
$form.Controls.Add($joinButton)

$connectButton = New-Object System.Windows.Forms.Button
$connectButton.Text = "Connect"
$connectButton.Location = New-Object System.Drawing.Point(145, 100)
$connectButton.Size = New-Object System.Drawing.Size(110, 34)
$form.Controls.Add($connectButton)

$syncButton = New-Object System.Windows.Forms.Button
$syncButton.Text = "Sync Once"
$syncButton.Location = New-Object System.Drawing.Point(270, 100)
$syncButton.Size = New-Object System.Drawing.Size(110, 34)
$form.Controls.Add($syncButton)

$disconnectButton = New-Object System.Windows.Forms.Button
$disconnectButton.Text = "Disconnect"
$disconnectButton.Location = New-Object System.Drawing.Point(395, 100)
$disconnectButton.Size = New-Object System.Drawing.Size(110, 34)
$form.Controls.Add($disconnectButton)

$statusButton = New-Object System.Windows.Forms.Button
$statusButton.Text = "Status"
$statusButton.Location = New-Object System.Drawing.Point(520, 100)
$statusButton.Size = New-Object System.Drawing.Size(110, 34)
$form.Controls.Add($statusButton)

$info = New-Object System.Windows.Forms.Label
$info.Text = "Config: $configPath"
$info.Location = New-Object System.Drawing.Point(20, 150)
$info.Size = New-Object System.Drawing.Size(650, 24)
$form.Controls.Add($info)

$output = New-Object System.Windows.Forms.TextBox
$output.Multiline = $true
$output.ScrollBars = "Vertical"
$output.ReadOnly = $true
$output.Location = New-Object System.Drawing.Point(20, 185)
$output.Size = New-Object System.Drawing.Size(650, 260)
$form.Controls.Add($output)

$joinButton.Add_Click({
    try {
        if ([string]::IsNullOrWhiteSpace($tokenBox.Text)) {
            throw "Admin Token is required."
        }
        New-Item -ItemType Directory -Force -Path $configDir | Out-Null
        $result = Invoke-Agent @(
            "register",
            "--config", $configPath,
            "--controller", $controllerBox.Text.Trim(),
            "--admin-token", $tokenBox.Text.Trim()
        )
        Append-Log "Joined network."
        Append-Log $result
    } catch {
        Append-Log "Join failed: $($_.Exception.Message)"
    }
})

$connectButton.Add_Click({
    try {
        if (!(Test-IsAdmin)) {
            throw "Please run this UI as Administrator."
        }
        $result = Invoke-Agent @("up", "--config", $configPath, "--out", $wgConfigPath)
        Append-Log "Connected."
        if (![string]::IsNullOrWhiteSpace($result)) {
            Append-Log $result
        }
    } catch {
        Append-Log "Connect failed: $($_.Exception.Message)"
    }
})

$syncButton.Add_Click({
    try {
        if (!(Test-IsAdmin)) {
            throw "Please run this UI as Administrator."
        }
        $result = Invoke-Agent @("daemon", "--once", "--config", $configPath, "--wg-config", $wgConfigPath)
        Append-Log "Synced once."
        if (![string]::IsNullOrWhiteSpace($result)) {
            Append-Log $result
        }
    } catch {
        Append-Log "Sync failed: $($_.Exception.Message)"
    }
})

$disconnectButton.Add_Click({
    try {
        if (!(Test-IsAdmin)) {
            throw "Please run this UI as Administrator."
        }
        $result = Invoke-Agent @("down", "--config", $configPath)
        Append-Log "Disconnected."
        if (![string]::IsNullOrWhiteSpace($result)) {
            Append-Log $result
        }
    } catch {
        Append-Log "Disconnect failed: $($_.Exception.Message)"
    }
})

$statusButton.Add_Click({
    try {
        $result = Invoke-Agent @("netmap", "--config", $configPath)
        Append-Log $result
    } catch {
        Append-Log "Status failed: $($_.Exception.Message)"
    }
})

Append-Log "Put sdwan-agent.exe next to this script, install WireGuard for Windows, then run as Administrator."
[void]$form.ShowDialog()
