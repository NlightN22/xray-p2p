#pragma once

#include <windows.h>

namespace xp2p::ui {

bool InitGdiPlus();
void ShutdownGdiPlus();

HICON CreateIconFromPngResource(HINSTANCE instance, int resourceId, int sizePx);

}

