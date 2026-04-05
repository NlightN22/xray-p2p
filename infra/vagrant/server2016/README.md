# Windows Server 2016 Box Notes

The `mwrock/Windows2016` box requires WinRM plaintext + basic auth to be enabled
manually before Vagrant provisioning can connect.

Run these commands in an elevated PowerShell session inside the guest:

```powershell
winrm set winrm/config/service '@{AllowUnencrypted="true"}'
winrm set winrm/config/service/auth '@{Basic="true"}'
```
