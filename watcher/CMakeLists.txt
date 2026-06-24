#
# The relative source and module paths seem to work inconsistently on Windows
# and Nix (flakes). This file used to live around the build/in and
# build/in/cmake directories. For now, it seems easiest to leave it here, as
# one megafile, in the project's root directory.
#

cmake_policy(SET CMP0048 NEW) # For VERSION in project()
cmake_policy(SET CMP0069 NEW) # For better IPO support
cmake_minimum_required(VERSION 3.11)

set(WTR_WATCHER_MAIN_PROJECT ON)
if(DEFINED PROJECT_SOURCE_DIR)
  set(WTR_WATCHER_MAIN_PROJECT OFF)
endif()

project(
  wtr.watcher
  VERSION 0.14.5 # hook: tool/release
  DESCRIPTION "watcher: a filesystem watcher"
  HOMEPAGE_URL "github.com/e-dant/watcher"
  LANGUAGES
    CXX
    C
)

include(CheckIPOSupported)
include(FetchContent)
include(GNUInstallDirs)

enable_testing()

#
# Options, Variable & Constants
#

option(BUILD_LIB        "Create targets for the watcher-c libraries" ON)
option(BUILD_BIN        "Create targets for the CLI binaries" ON)
option(BUILD_PKG_CONFIG "Create targets for the pkg-config files" ON)
option(BUILD_HDR        "Create targets for the headers (both the C++ single-header library and the watcher-c library header)" ON)
option(BUILD_TESTING    "Create targets for the test programs" OFF)
option(BUILD_SAN        "Mega-option to allow sanitizers" OFF)
option(BUILD_ASAN       "Create targets address-sanitized libraries and binaries" OFF)
option(BUILD_MSAN       "Create targets memory-sanitized libraries and binaries" OFF)
option(BUILD_TSAN       "Create targets thread-sanitized libraries and binaries" OFF)
option(BUILD_UBSAN      "Create targets undefined-behavior-sanitized libraries and binaries" OFF)

set(WTR_WATCHER_CXX_STD 17)

set(CMAKE_EXPORT_COMPILE_COMMANDS 1)

set(IS_CC_CLANG 0)
set(IS_CC_ANYCLANG 0)
set(IS_CC_APPLECLANG 0)
set(IS_CC_GCC 0)
set(IS_CC_MSVC 0)
if(CMAKE_CXX_COMPILER_ID     STREQUAL "MSVC")
  set(IS_CC_MSVC 1)
elseif(CMAKE_CXX_COMPILER_ID STREQUAL "GNU")
  set(IS_CC_GCC 1)
elseif(CMAKE_CXX_COMPILER_ID MATCHES  "Clang")
  set(IS_CC_ANYCLANG 1)
  if(CMAKE_CXX_COMPILER_ID   STREQUAL "AppleClang")
    set(IS_CC_APPLECLANG 1)
  else()
    set(IS_CC_CLANG 1)
  endif()
endif()

if(NOT CMAKE_BUILD_TYPE)
  set(CMAKE_BUILD_TYPE "Release")
endif()

if(NOT IS_CC_MSVC)
  # It's not that we don't want these, it's just that I hate Windows.
  # Also, MSVC doesn't support some of these arguments, so it's not possible.
  set(COMPILE_OPTIONS
    "${COMPILE_OPTIONS}"
    "-Wall"
    "-Wextra"
    "-Werror"
    "-Wno-unused-function"
    "-Wno-unneeded-internal-declaration"
  )
endif()

if(NOT WIN32 AND NOT IS_CC_MSVC)
  # TODO: strict-aliasing
  set(COMPILE_OPTIONS
    "${COMPILE_OPTIONS}"
    "-Wno-ignored-optimization-argument" # For Android's clang
    "-Wno-unused-command-line-argument" # For clang-11
    "-fno-exceptions"
    "-fno-rtti"
    "-fstrict-enums"
    "-fstrict-overflow"
  )
  if(IS_CC_CLANG)
    set(COMPILE_OPTIONS
      "${COMPILE_OPTIONS}"
      "-fstrict-return"
      "-fstrict-float-cast-overflow"
    )
  endif()
  if(NOT IS_CC_APPLECLANG)
    set(COMPILE_OPTIONS
      "${COMPILE_OPTIONS}"
      "-fexpensive-optimizations"
    )
  endif()
endif()

if(ANDROID)
  # Android's stdlib ("bionic") doesn't need to be linked with (p)threads.
  set(LINK_LIBRARIES "${LINK_LIBRARIES}")
else()
  find_package(Threads REQUIRED)
  set(LINK_LIBRARIES
    "${LINK_LIBRARIES}"
    "Threads::Threads"
  )
  if(APPLE)
    list(APPEND LINK_LIBRARIES
      "-framework CoreFoundation"
      "-framework CoreServices"
    )
  endif()
endif()

set(INCLUDE_PATH_DEVEL "devel/include")

set(ALLOWED_asan        0)
set(ALLOWED_msan        0)
set(ALLOWED_tsan        0)
set(ALLOWED_ubsan       0)
set(ALLOWED_stacksan    0)
set(ALLOWED_dataflowsan 0)
set(ALLOWED_cfisan      0)
set(ALLOWED_kcfisan     0)
if((NOT WIN32 OR IS_CC_MSVC) AND (BUILD_ASAN OR BUILD_SAN))
  set(ALLOWED_asan      1)
endif()
if(IS_CC_CLANG AND NOT ANDROID AND (BUILD_MSAN OR BUILD_SAN))
  set(ALLOWED_msan      1)
endif()
if(NOT ANDROID AND NOT WIN32 AND (BUILD_TSAN OR BUILD_SAN))
  set(ALLOWED_tsan      1)
endif()
if(NOT WIN32 AND (BUILD_UBSAN OR BUILD_SAN))
  set(ALLOWED_ubsan     1)
endif()
set(SAN_NAMES                       "asan" "msan" "tsan" "ubsan")
if(IS_CC_MSVC)
  set(CCLL_EXTOPT_SET_ASAN          "/fsanitize=address")
else()
  set(CCLL_EXTOPT_SET_ASAN          "-fno-omit-frame-pointer" "-fsanitize=address")
endif()
set(CCLL_EXTOPT_SET_MSAN            "-fno-omit-frame-pointer" "-fsanitize=memory")
set(CCLL_EXTOPT_SET_TSAN            "-fno-omit-frame-pointer" "-fsanitize=thread")
set(CCLL_EXTOPT_SET_UBSAN           "-fno-omit-frame-pointer" "-fsanitize=undefined")
set(CCLL_EXTOPT_SET_STACKSAN        "-fno-omit-frame-pointer" "-fsanitize=safe-stack")
set(CCLL_EXTOPT_SET_DATAFLOWSAN     "-fno-omit-frame-pointer" "-fsanitize=dataflow")
set(CCLL_EXTOPT_SET_CFISAN          "-fno-omit-frame-pointer" "-fsanitize=cfi")
set(CCLL_EXTOPT_SET_KCFISAN         "-fno-omit-frame-pointer" "-fsanitize=kcfi")

set(SAN_SUPPORTED)
foreach(SAN ${SAN_NAMES})
  if(ALLOWED_${SAN})
    list(APPEND SAN_SUPPORTED ${SAN})
  endif()
endforeach()

message(STATUS "Supported sanitizers on ${CMAKE_SYSTEM}/${CMAKE_CXX_COMPILER_ID}: ${SAN_SUPPORTED}")

#
# Functions
#

function(wtr_add_bin_target
      NAME
      BIN_COMPONENT_NAME
      IS_TEST
      SRC_SET
      CC_OPT_SET
      LL_OPT_SET
      INCLUDE_PATH
      LLIB_SET)
  if(NOT IS_TEST AND NOT BUILD_BIN)
    message(STATUS "${NAME}: Skipped (BUILD_BIN=${BUILD_BIN})")
    return()
  elseif(NOT IS_TEST)
    message(STATUS "${NAME}: Added (BUILD_BIN=${BUILD_BIN})")
  endif()
  if(IS_TEST AND NOT BUILD_TESTING)
    message(STATUS "${NAME}: Skipped (BUILD_TESTING=${BUILD_TESTING})")
    return()
  elseif(IS_TEST)
    message(STATUS "${NAME}: Added (BUILD_TESTING=${BUILD_TESTING})")
  endif()
  add_executable("${NAME}" "${SRC_SET}")
  set_property(TARGET "${NAME}" PROPERTY CXX_STANDARD "${WTR_WATCHER_CXX_STD}")
  if(NOT WIN32 AND NOT IS_CC_MSVC AND NOT IS_CC_APPLECLANG)
    target_compile_options("${NAME}" PRIVATE "${CC_OPT_SET};-fwhole-program")
  else()
    target_compile_options("${NAME}" PRIVATE "${CC_OPT_SET}")
  endif()
  target_link_options("${NAME}" PRIVATE "${LL_OPT_SET}")
  target_include_directories("${NAME}" PUBLIC "${INCLUDE_PATH}")
  target_link_libraries("${NAME}" PRIVATE "${LLIB_SET}")
  check_ipo_supported(RESULT ipo_supported)
  if(ipo_supported)
    set_property(TARGET "${NAME}" PROPERTY INTERPROCEDURAL_OPTIMIZATION TRUE)
  endif()
  if(APPLE)
    set_property(
      TARGET "${NAME}"
      PROPERTY XCODE_ATTRIBUTE_PRODUCT_BUNDLE_IDENTIFIER "org.${NAME}"
    )
  endif()
  if(IS_TEST)
    if(DEFINED ENV{WTR_WATCHER_USE_SYSTEM_SNITCH})
      find_package(snitch REQUIRED)
    else()
      FetchContent_Declare(
        snitch
        GIT_REPOSITORY https://github.com/cschreib/snitch.git
        # v1.1.1 Doesn't show time as nice as v1.0.0
        # Tuesday, June 29th, 2023 @ v1.1.1
        GIT_TAG        5ad2fffebf31f3e6d56c2c0ab27bc45d01da2f05
        # Friday, January 20th, 2023 @ v1.0.0
        # GIT_TAG        ea200a0830394f8e0ef732064f0935a77c003bd6
        # Saturday, January 7th, 2023 @ main
        # GIT_TAG        8165d6c85353f9c302ce05f1c1c47dcfdc6aeb2c
        # Tuesday, December 18th, 2022 @ v0.1.3
        # GIT_TAG        f313bccafe98aaef617af3bf457d091d8d50cdcd
        # Friday, December 2nd, 2022 @ main
        # GIT_TAG        c0b6ac4efe4019e4846e8967fe21de864b0cc1ed
      )
      FetchContent_MakeAvailable(snitch)
    endif()
    add_test(NAME "${NAME}" COMMAND "${NAME}")
  endif()
  if(WTR_WATCHER_MAIN_PROJECT AND BIN_COMPONENT_NAME)
    install(
      TARGETS "${NAME}"
      DESTINATION "${CMAKE_INSTALL_PREFIX}/bin"
      COMPONENT "${BIN_COMPONENT_NAME}"
    )
  endif()
endfunction()

function(wtr_add_lib_target NAME OUTPUT_NAME SRC_SET INC_SET LIB_TYPE)
  if(NOT BUILD_LIB)
    message(STATUS "${NAME}: Skipped (BUILD_LIB=${BUILD_LIB})")
    return()
  endif()
  message(STATUS "${NAME}: Added (BUILD_LIB=${BUILD_LIB})")
  add_library("${NAME}" "${LIB_TYPE}" "${SRC_SET}")
  target_include_directories("${NAME}" PRIVATE "${INC_SET}")
  set_property(TARGET "${NAME}" PROPERTY CXX_STANDARD "${WTR_WATCHER_CXX_STD}")
  set_property(TARGET "${NAME}" PROPERTY VERSION "${PROJECT_VERSION}")
  set_property(TARGET "${NAME}" PROPERTY SOVERSION "${PROJECT_VERSION_MAJOR}")
  set_property(TARGET "${NAME}" PROPERTY POSITION_INDEPENDENT_CODE ON)
  set_property(TARGET "${NAME}" PROPERTY OUTPUT_NAME "${OUTPUT_NAME}")
  target_compile_options("${NAME}" PRIVATE "${COMPILE_OPTIONS}")
  if("${NAME}" STREQUAL "watcher-c-shared" AND LINUX)
    message(STATUS "watcher-c-shared: Adding version script")
    target_link_options("${NAME}" PRIVATE "${LINK_OPTIONS};-Wl,--version-script=${CMAKE_SOURCE_DIR}/watcher-c/watcher-c.version")
  else()
    target_link_options("${NAME}" PRIVATE "${LINK_OPTIONS}")
  endif()
  target_link_libraries("${NAME}" PRIVATE "${LINK_LIBRARIES}")
  if(APPLE)
    set_property(TARGET "${NAME}" PROPERTY INSTALL_RPATH "/usr/local/lib")
    set_property(TARGET "${NAME}" PROPERTY BUILD_RPATH "/usr/local/lib")
  endif()
  if(WTR_WATCHER_MAIN_PROJECT)
    install(
      TARGETS "${NAME}"
      DESTINATION "${CMAKE_INSTALL_LIBDIR}"
      COMPONENT "lib"
    )
  endif()
endfunction()

function(wtr_add_hdr_target NAME HDR_SET)
  if(NOT BUILD_HDR)
    message(STATUS "${NAME}: Skipped (BUILD_HDR=${BUILD_HDR})")
    return()
  endif()
  message(STATUS "${NAME}: Added (BUILD_HDR=${BUILD_HDR})")
  add_library("${NAME}" INTERFACE "${HDR_SET}")
  target_include_directories("${NAME}" INTERFACE "${INCLPATH}")
  if(WTR_WATCHER_MAIN_PROJECT)
    install(FILES "${HDR_SET}" DESTINATION "${CMAKE_INSTALL_INCLUDEDIR}/wtr" COMPONENT "include")
  endif()
endfunction()

function(wtr_add_rel_bin_target NAME SRC_SET)
  wtr_add_bin_target(
    "${NAME}"
    "bin"
    "OFF" # is test
    "${SRC_SET}"
    "${COMPILE_OPTIONS}"
    "${LINK_OPTIONS}"
    "include"
    "${LINK_LIBRARIES}"
  )
endfunction()

function(wtr_add_test_bin_target NAME SRC_SET)
  wtr_add_bin_target(
    "${NAME}"
    "test-bin"
    "ON"  # is test
    "${SRC_SET}"
    "${COMPILE_OPTIONS}"
    "${LINK_OPTIONS}"
    "${INCLUDE_PATH_DEVEL}"
    "${LINK_LIBRARIES};snitch::snitch"
  )
endfunction()

function(wtr_add_san_rel_bin_target NAME SRC_SET SAN)
  wtr_add_bin_target(
    "${NAME}.${SAN}"
    "test-bin"
    "OFF" # is test
    "${SRC_SET}"
    "${COMPILE_OPTIONS};${CCLL_EXTOPT_SET_${SAN}}"
    "${LINK_OPTIONS};${CCLL_EXTOPT_SET_${SAN}}"
    "${INCLUDE_PATH_DEVEL}"
    "${LINK_LIBRARIES}"
  )
endfunction()

function(wtr_add_san_test_bin_target NAME SRC_SET SAN)
  wtr_add_bin_target(
    "${NAME}.${SAN}"
    "test-bin"
    "ON" # is test
    "${SRC_SET}"
    "${COMPILE_OPTIONS};${CCLL_EXTOPT_SET_${SAN}}"
    "${LINK_OPTIONS};${CCLL_EXTOPT_SET_${SAN}}"
    "${INCLUDE_PATH_DEVEL}"
    "${LINK_LIBRARIES};snitch::snitch"
  )
endfunction()

function(wtr_add_autosan_bin_target NAME SRC_SET IS_TEST)
  if(IS_TEST)
    wtr_add_test_bin_target("${NAME}" "${SRC_SET}")
  else()
    wtr_add_rel_bin_target("${NAME}" "${SRC_SET}")
  endif()
  foreach(SAN ${SAN_NAMES})
    string(TOUPPER ${SAN} UPPER_SAN)
    if(NOT BUILD_SAN AND NOT BUILD_${UPPER_SAN})
      message(STATUS "${NAME}.${SAN}: Skipped (BUILD_SAN=${BUILD_SAN}, BUILD_${UPPER_SAN}=${BUILD_${UPPER_SAN}})")
    elseif(NOT ALLOWED_${SAN})
      message(STATUS "${NAME}.${SAN}: Skipped (Unsupported on ${CMAKE_SYSTEM}/${CMAKE_CXX_COMPILER_ID})")
    elseif(IS_TEST)
      wtr_add_san_test_bin_target("${NAME}" "${SRC_SET}" "${SAN}")
    else()
      wtr_add_san_rel_bin_target("${NAME}" "${SRC_SET}" "${SAN}")
    endif()
  endforeach()
endfunction()

function(wtr_add_autosan_rel_bin_target NAME SRC_SET)
  wtr_add_autosan_bin_target("${NAME}" "${SRC_SET}" OFF)
endfunction()

function(wtr_add_autosan_test_bin_target NAME SRC_SET)
  wtr_add_autosan_bin_target("${NAME}" "${SRC_SET}" ON)
endfunction()

function(wtr_add_san_lib_target NAME OUTPUT_NAME SRC_SET INC_SET LIB_TYPE SAN)
  string(TOUPPER "${SAN}" UPPER_SAN)
  if (NOT BUILD_LIB)
    message(STATUS "${NAME}: Skipped (BUILD_LIB=${BUILD_LIB})")
    return()
  elseif (NOT BUILD_SAN AND NOT BUILD_${UPPER_SAN})
    message(STATUS "${NAME}: Skipped (BUILD_SAN=${BUILD_SAN}, BUILD_${UPPER_SAN}=${BUILD_${UPPER_SAN}})")
    return()
  elseif (NOT ALLOWED_${SAN})
    message(STATUS "${NAME}: Skipped (Unsupported on ${CMAKE_SYSTEM}/${CMAKE_CXX_COMPILER_ID})")
    return()
  endif()
  message(STATUS "${NAME}: Added (BUILD_SAN=${BUILD_SAN}, BUILD_${UPPER_SAN}=${BUILD_${UPPER_SAN}}, BUILD_LIB=${BUILD_LIB})")
  wtr_add_lib_target("${NAME}" "${OUTPUT_NAME}" "${SRC_SET}" "${INC_SET}" "${LIB_TYPE}")
  target_compile_options("${NAME}" PRIVATE "${COMPILE_OPTIONS};${CCLL_EXTOPT_SET_${UPPER_SAN}}")
  target_link_options("${NAME}" PRIVATE "${LINK_OPTIONS};${CCLL_EXTOPT_SET_${UPPER_SAN}}")
endfunction()

function(wtr_add_autosan_lib_target NAME OUTPUT_NAME SRC_SET INC_SET LIB_TYPE)
  wtr_add_lib_target("${NAME}" "${OUTPUT_NAME}" "${SRC_SET}" "${INC_SET}" "${LIB_TYPE}")
  foreach(SAN ${SAN_NAMES})
    string(TOUPPER ${SAN} UPPER_SAN)
    wtr_add_san_lib_target("${NAME}.${SAN}" "${OUTPUT_NAME}.${SAN}" "${SRC_SET}" "${INC_SET}" "${LIB_TYPE}" "${SAN}")
  endforeach()
endfunction()

function(wtr_add_pkg_config_target NAME SRC)
  if(BUILD_PKG_CONFIG)
    message(STATUS "${NAME}: Added (BUILD_PKG_CONFIG=${BUILD_PKG_CONFIG})")
    configure_file("${SRC}" "${CMAKE_BINARY_DIR}/${NAME}" @ONLY)
    if(WTR_WATCHER_MAIN_PROJECT)
      install(FILES "${CMAKE_BINARY_DIR}/${NAME}" DESTINATION "${CMAKE_INSTALL_LIBDIR}/pkgconfig" COMPONENT pkgconfig)
    endif()
  else()
    message(STATUS "${NAME}: Skipped (BUILD_PKG_CONFIG=${BUILD_PKG_CONFIG})")
  endif()
endfunction()

#
# Actual work
#

add_compile_definitions(WTR_WATCHER_VERSION_S="${PROJECT_VERSION}")

wtr_add_hdr_target(
  "wtr.hdr_watcher"
  "include/wtr/watcher.hpp"
)

wtr_add_hdr_target(
  "watcher-c-hdr"
  "watcher-c/include/wtr/watcher-c.h"
)

wtr_add_autosan_lib_target(
  "watcher-c-shared"
  "watcher-c"
  "watcher-c/src/watcher-c.cpp"
  "watcher-c/include;${CMAKE_CURRENT_LIST_DIR}/include"
  "SHARED"
)

wtr_add_autosan_lib_target(
  "watcher-c-static"
  "watcher-c"
  "watcher-c/src/watcher-c.cpp"
  "watcher-c/include;${CMAKE_CURRENT_LIST_DIR}/include"
  "STATIC"
)

wtr_add_autosan_rel_bin_target(
  "wtr.watcher"
  "src/wtr/watcher/main.cpp"
)

wtr_add_autosan_rel_bin_target(
  "tw"
  "src/wtr/tiny_watcher/main.cpp"
)

set(WTR_TEST_WATCHER_SOURCE_SET
  "devel/src/wtr/test_watcher/test_concurrency.cpp"
  "devel/src/wtr/test_watcher/test_event_targets.cpp"
  "devel/src/wtr/test_watcher/test_new_directories.cpp"
  "devel/src/wtr/test_watcher/test_simple.cpp"
  "devel/src/wtr/test_watcher/test_performance.cpp"
  "devel/src/wtr/test_watcher/test_openclose.cpp"
)
wtr_add_autosan_test_bin_target(
  "wtr.test_watcher"
  "${WTR_TEST_WATCHER_SOURCE_SET}"
)

# Used in the tool/test suite. Platforms vary
# in their mv(1) implementations. We smooth
# over that by using the `rename` system call,
# which doesn't vary much at all.
wtr_add_bin_target(
  "portable-destructive-rename"
  "test-bin"
  ON
  "devel/src/portable-destructive-rename/main.c"
  ""
  ""
  ""
  ""
)

set(PC_WATCHER_LIBS_PRIVATE "${LINK_LIBRARIES}")
set(PC_WATCHER_LIBDIR "${CMAKE_INSTALL_LIBDIR}")
set(PC_WATCHER_INCLUDEDIR "${CMAKE_INSTALL_INCLUDEDIR}/wtr")
wtr_add_pkg_config_target("watcher.pc" "watcher.pc.in")

set(PC_LIBWATCHER_C_LIBS_PRIVATE "${LINK_LIBRARIES}")
set(PC_LIBWATCHER_C_LIBDIR "${CMAKE_INSTALL_LIBDIR}")
set(PC_LIBWATCHER_C_INCLUDEDIR "${CMAKE_INSTALL_INCLUDEDIR}/wtr")
wtr_add_pkg_config_target("watcher-c.pc" "watcher-c/watcher-c.pc.in")

if(BUILD_TESTING)
  message(STATUS "wtr.test_tool_test_all: Added (BUILD_TESTING=${BUILD_TESTING})")
  add_test(
    NAME wtr.test_tool_test_all
    COMMAND sh -c "PATH=$<TARGET_FILE_DIR:portable-destructive-rename>:$ENV{PATH} WATCHER=$<TARGET_FILE:wtr.watcher> ${CMAKE_SOURCE_DIR}/tool/test/all"
  )
  foreach(SAN ${SAN_NAMES})
    if(ALLOWED_${SAN} AND (BUILD_SAN OR BUILD_${SAN}))
      message(STATUS "wtr.test_tool_test_all.${SAN}: Added (BUILD_SAN=${BUILD_SAN}, BUILD_${SAN}=${BUILD_${SAN}})")
      add_test(
        NAME wtr.test_tool_test_all.${SAN}
        COMMAND sh -c "PATH=$<TARGET_FILE_DIR:portable-destructive-rename>:$ENV{PATH} WATCHER=$<TARGET_FILE:wtr.watcher.${SAN}> ${CMAKE_SOURCE_DIR}/tool/test/all"
      )
    else()
      message(STATUS "wtr.test_tool_test_all.${SAN}: Skipped (BUILD_SAN=${BUILD_SAN}, BUILD_${SAN}=${BUILD_${SAN}}, ALLOWED_${SAN}=${ALLOWED_${SAN}})")
    endif()
  endforeach()
else()
  message(STATUS "wtr.test_tool_test_all: Skipped (BUILD_TESTING=${BUILD_TESTING})")
endif()
