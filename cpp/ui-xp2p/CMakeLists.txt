cmake_minimum_required(VERSION 3.20)

project(ui-xp2p LANGUAGES CXX RC)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

add_executable(ui-xp2p WIN32
    resources/ui-xp2p.rc
    src/logging.cpp
    src/logging.h
    src/path_utils.cpp
    src/path_utils.h
    src/service_manager.cpp
    src/service_manager.h
    src/tray_app.cpp
    src/tray_app.h
    src/main.cpp
)

target_compile_definitions(ui-xp2p PRIVATE UNICODE _UNICODE WIN32_LEAN_AND_MEAN NOMINMAX)
target_compile_options(ui-xp2p PRIVATE /W4 /WX-)

set_property(TARGET ui-xp2p PROPERTY MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")

target_link_libraries(ui-xp2p PRIVATE advapi32 shell32 user32)

