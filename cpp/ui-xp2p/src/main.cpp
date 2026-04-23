#include "tray_app.h"

#include "png_icon.h"

#include <windows.h>
#include <objbase.h>

int WINAPI wWinMain(HINSTANCE instance, HINSTANCE, PWSTR, int) {
    HANDLE mutex = CreateMutexW(nullptr, TRUE, L"Global\\ui-xp2p");
    if (!mutex) {
        return 1;
    }
    if (GetLastError() == ERROR_ALREADY_EXISTS) {
        CloseHandle(mutex);
        return 0;
    }

    int exitCode = 0;
    HRESULT comInit = CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED);
    xp2p::ui::InitGdiPlus();
    try {
        xp2p::ui::TrayApp app(instance);
        exitCode = app.Run();
    } catch (...) {
        exitCode = 10;
    }
    xp2p::ui::ShutdownGdiPlus();
    if (SUCCEEDED(comInit)) {
        CoUninitialize();
    }

    CloseHandle(mutex);
    return exitCode;
}
