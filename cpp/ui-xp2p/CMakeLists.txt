cmake_minimum_required(VERSION 3.20)

if(WIN32)
    project(ui-xp2p LANGUAGES CXX RC)
else()
    project(ui-xp2p LANGUAGES CXX)
endif()

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

option(XP2P_UI_BUILD_TESTS "Build ui-xp2p unit tests" OFF)
option(XP2P_UI_ENABLE_COVERAGE "Enable coverage flags (GNU only)" OFF)

add_library(ui-xp2p-logic STATIC
    src/ui_logic.cpp
    src/ui_logic.h
)
target_compile_definitions(ui-xp2p-logic PRIVATE UNICODE _UNICODE WIN32_LEAN_AND_MEAN NOMINMAX)

if(XP2P_UI_ENABLE_COVERAGE AND CMAKE_CXX_COMPILER_ID STREQUAL "GNU")
    target_compile_options(ui-xp2p-logic PRIVATE -O0 -g --coverage)
    target_link_options(ui-xp2p-logic PRIVATE --coverage)
endif()

if(WIN32)
    set(XP2P_UI_ICON_PATH "${CMAKE_CURRENT_LIST_DIR}/../../assets/xp2p.ico")
    file(TO_CMAKE_PATH "${XP2P_UI_ICON_PATH}" XP2P_UI_ICON_PATH)
    set(XP2P_UI_PNG_DISABLED_PATH "${CMAKE_CURRENT_LIST_DIR}/../../assets/Disabled.png")
    set(XP2P_UI_PNG_ENABLED_PATH "${CMAKE_CURRENT_LIST_DIR}/../../assets/Enabled.png")
    set(XP2P_UI_PNG_ENABLING_PATH "${CMAKE_CURRENT_LIST_DIR}/../../assets/Enabling.png")
    file(TO_CMAKE_PATH "${XP2P_UI_PNG_DISABLED_PATH}" XP2P_UI_PNG_DISABLED_PATH)
    file(TO_CMAKE_PATH "${XP2P_UI_PNG_ENABLED_PATH}" XP2P_UI_PNG_ENABLED_PATH)
    file(TO_CMAKE_PATH "${XP2P_UI_PNG_ENABLING_PATH}" XP2P_UI_PNG_ENABLING_PATH)
    configure_file(
        "${CMAKE_CURRENT_LIST_DIR}/resources/ui-xp2p.rc.in"
        "${CMAKE_CURRENT_BINARY_DIR}/ui-xp2p.rc"
        @ONLY
    )

    add_executable(ui-xp2p WIN32
        "${CMAKE_CURRENT_BINARY_DIR}/ui-xp2p.rc"
        src/png_icon.cpp
        src/png_icon.h
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

    if(MSVC)
        target_compile_options(ui-xp2p PRIVATE /W4 /WX-)
        set_property(TARGET ui-xp2p PROPERTY MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")
    else()
        target_compile_options(ui-xp2p PRIVATE -Wall -Wextra -Wpedantic)
    endif()

    target_link_libraries(ui-xp2p PRIVATE ui-xp2p-logic advapi32 shell32 user32 ole32 gdiplus)
    if(MINGW)
        target_link_options(ui-xp2p PRIVATE -municode)
    endif()
endif()

if(XP2P_UI_BUILD_TESTS)
    enable_testing()
    add_executable(ui-xp2p-tests
        tests/ui_logic_tests.cpp
    )
    target_link_libraries(ui-xp2p-tests PRIVATE ui-xp2p-logic)
    if(XP2P_UI_ENABLE_COVERAGE AND CMAKE_CXX_COMPILER_ID STREQUAL "GNU")
        target_compile_options(ui-xp2p-tests PRIVATE -O0 -g --coverage)
        target_link_options(ui-xp2p-tests PRIVATE --coverage)
    endif()
    add_test(NAME ui-xp2p-tests COMMAND ui-xp2p-tests)
endif()
