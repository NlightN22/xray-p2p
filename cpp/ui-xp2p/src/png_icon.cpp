#include "png_icon.h"

#include <cstring>

#include <objidl.h>
#include <windows.h>

#include <gdiplus.h>

namespace xp2p::ui {

static ULONG_PTR g_gdiplusToken = 0;

bool InitGdiPlus() {
    if (g_gdiplusToken != 0) {
        return true;
    }
    Gdiplus::GdiplusStartupInput input{};
    const auto st = Gdiplus::GdiplusStartup(&g_gdiplusToken, &input, nullptr);
    return st == Gdiplus::Ok;
}

void ShutdownGdiPlus() {
    if (g_gdiplusToken == 0) {
        return;
    }
    Gdiplus::GdiplusShutdown(g_gdiplusToken);
    g_gdiplusToken = 0;
}

static IStream* CreateStreamFromResource(HINSTANCE instance, int resourceId) {
    HRSRC resInfo = FindResourceW(instance, MAKEINTRESOURCEW(resourceId), RT_RCDATA);
    if (!resInfo) {
        return nullptr;
    }
    HGLOBAL resData = LoadResource(instance, resInfo);
    if (!resData) {
        return nullptr;
    }
    const DWORD size = SizeofResource(instance, resInfo);
    void* raw = LockResource(resData);
    if (!raw || size == 0) {
        return nullptr;
    }

    HGLOBAL hCopy = GlobalAlloc(GMEM_MOVEABLE, size);
    if (!hCopy) {
        return nullptr;
    }
    void* dst = GlobalLock(hCopy);
    if (!dst) {
        GlobalFree(hCopy);
        return nullptr;
    }
    std::memcpy(dst, raw, size);
    GlobalUnlock(hCopy);

    IStream* stream = nullptr;
    if (FAILED(CreateStreamOnHGlobal(hCopy, TRUE, &stream))) {
        GlobalFree(hCopy);
        return nullptr;
    }
    return stream;
}

HICON CreateIconFromPngResource(HINSTANCE instance, int resourceId, int sizePx) {
    if (sizePx <= 0) {
        return nullptr;
    }
    if (!InitGdiPlus()) {
        return nullptr;
    }

    IStream* stream = CreateStreamFromResource(instance, resourceId);
    if (!stream) {
        return nullptr;
    }
    Gdiplus::Bitmap bitmap(stream);
    stream->Release();
    if (bitmap.GetLastStatus() != Gdiplus::Ok) {
        return nullptr;
    }

    Gdiplus::Bitmap scaled(sizePx, sizePx, PixelFormat32bppARGB);
    Gdiplus::Graphics g(&scaled);
    g.SetInterpolationMode(Gdiplus::InterpolationModeHighQualityBicubic);
    g.DrawImage(&bitmap, 0, 0, sizePx, sizePx);

    HBITMAP color = nullptr;
    if (scaled.GetHBITMAP(Gdiplus::Color(0, 0, 0, 0), &color) != Gdiplus::Ok || !color) {
        return nullptr;
    }

    HBITMAP mask = CreateBitmap(sizePx, sizePx, 1, 1, nullptr);
    if (!mask) {
        DeleteObject(color);
        return nullptr;
    }

    ICONINFO ii{};
    ii.fIcon = TRUE;
    ii.hbmColor = color;
    ii.hbmMask = mask;
    HICON icon = CreateIconIndirect(&ii);
    DeleteObject(color);
    DeleteObject(mask);
    return icon;
}

}
