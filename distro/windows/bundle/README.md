Place `wintun.dll` alongside `xray.exe` in the `x86_64` and `x86` bundles before building MSI/ZIP artifacts.
The Windows installer harvests all files from these bundle directories and copies them into the installation `bin` folder.
