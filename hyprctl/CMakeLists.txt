cmake_minimum_required(VERSION 3.19)

project(
    hyprctl
    DESCRIPTION "Control utility for Hyperland"
)

pkg_check_modules(hyprctl_deps REQUIRED IMPORTED_TARGET hyprutils>=0.2.4 hyprwire re2)

file(GLOB_RECURSE HYPRCTL_SRCFILES CONFIGURE_DEPENDS "src/*.cpp" "hw-protocols/*.cpp" "include/*.hpp")

add_executable(hyprctl ${HYPRCTL_SRCFILES})

target_link_libraries(hyprctl PUBLIC PkgConfig::hyprctl_deps)
target_include_directories(hyprctl PRIVATE "hw-protocols")

# Hyprwire

function(hyprprotocol protoPath protoName)
  set(path ${CMAKE_CURRENT_SOURCE_DIR}/${protoPath})
  add_custom_command(
    OUTPUT ${CMAKE_CURRENT_SOURCE_DIR}/hw-protocols/${protoName}-client.cpp
           ${CMAKE_CURRENT_SOURCE_DIR}/hw-protocols/${protoName}-client.hpp
           ${CMAKE_CURRENT_SOURCE_DIR}/hw-protocols/${protoName}-spec.hpp
    COMMAND hyprwire-scanner --client ${path}/${protoName}.xml
            ${CMAKE_CURRENT_SOURCE_DIR}/hw-protocols/
    WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR})
  target_sources(hyprctl PRIVATE hw-protocols/${protoName}-client.cpp
                                   hw-protocols/${protoName}-client.hpp
                                   hw-protocols/${protoName}-spec.hpp)
endfunction()

hyprprotocol(hw-protocols hyprpaper_core)

# binary: hyperctl, with hyprctl as an alias
set_target_properties(hyprctl PROPERTIES OUTPUT_NAME hyperctl)
install(TARGETS hyprctl)
hyperland_install_alias(${CMAKE_INSTALL_BINDIR} hyperctl hyprctl)

# shell completions: hyprctl's are generated from hyprctl.usage, hyperctl's wrap them
foreach(NAME hyprctl hyperctl)
  install(FILES ${CMAKE_CURRENT_SOURCE_DIR}/${NAME}.bash
          DESTINATION ${CMAKE_INSTALL_DATADIR}/bash-completion/completions
          RENAME ${NAME})
  install(FILES ${CMAKE_CURRENT_SOURCE_DIR}/${NAME}.zsh
          DESTINATION ${CMAKE_INSTALL_DATADIR}/zsh/site-functions
          RENAME _${NAME})
endforeach()
install(FILES ${CMAKE_CURRENT_SOURCE_DIR}/hyprctl.fish ${CMAKE_CURRENT_SOURCE_DIR}/hyperctl.fish
        DESTINATION ${CMAKE_INSTALL_DATADIR}/fish/vendor_completions.d)

# hyperctl-tui (Go): pretty `hyperctl` output and `hyperctl tui`. hyperctl falls
# back to the plain output when it is missing. Nix builds it as its own package.
option(BUILD_HYPERCTL_TUI "Build the hyperctl-tui helper (needs Go)" ON)
set(HYPERCTL_TUI_PATH "" CACHE STRING "Absolute path hyperctl tries for hyperctl-tui after its own directory")
if(HYPERCTL_TUI_PATH)
  target_compile_definitions(hyprctl PRIVATE HYPERCTL_TUI_PATH="${HYPERCTL_TUI_PATH}")
endif()

find_program(GO_EXECUTABLE go)
if(BUILD_HYPERCTL_TUI AND GO_EXECUTABLE)
  set(TUI_SRC ${CMAKE_SOURCE_DIR}/hyperctl-tui)
  set(TUI_BIN ${CMAKE_CURRENT_BINARY_DIR}/hyperctl-tui)
  set(TUI_COMP ${CMAKE_CURRENT_BINARY_DIR}/tui-completions)
  file(GLOB_RECURSE TUI_GO_SOURCES CONFIGURE_DEPENDS ${TUI_SRC}/*.go)

  add_custom_command(
    OUTPUT ${TUI_BIN}
    COMMAND ${CMAKE_COMMAND} -E env CGO_ENABLED=0 GOFLAGS=-mod=readonly ${GO_EXECUTABLE} build -trimpath
            -ldflags "-s -w -X github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/cli.Version=${HYPRLAND_VERSION}"
            -o ${TUI_BIN} .
    WORKING_DIRECTORY ${TUI_SRC}
    DEPENDS ${TUI_GO_SOURCES} ${TUI_SRC}/go.mod ${TUI_SRC}/go.sum
    COMMENT "Building hyperctl-tui (Go)"
    VERBATIM)

  add_custom_command(
    OUTPUT ${TUI_COMP}/hyperctl-tui.bash ${TUI_COMP}/_hyperctl-tui ${TUI_COMP}/hyperctl-tui.fish
    COMMAND ${CMAKE_COMMAND} -E make_directory ${TUI_COMP}
    COMMAND sh -c "'${TUI_BIN}' completion bash > '${TUI_COMP}/hyperctl-tui.bash'"
    COMMAND sh -c "'${TUI_BIN}' completion zsh > '${TUI_COMP}/_hyperctl-tui'"
    COMMAND sh -c "'${TUI_BIN}' completion fish > '${TUI_COMP}/hyperctl-tui.fish'"
    DEPENDS ${TUI_BIN}
    VERBATIM)

  add_custom_target(hyperctl_tui ALL DEPENDS ${TUI_BIN} ${TUI_COMP}/hyperctl-tui.bash)

  install(PROGRAMS ${TUI_BIN} DESTINATION ${CMAKE_INSTALL_BINDIR})
  install(FILES ${TUI_COMP}/hyperctl-tui.bash
          DESTINATION ${CMAKE_INSTALL_DATADIR}/bash-completion/completions
          RENAME hyperctl-tui)
  install(FILES ${TUI_COMP}/_hyperctl-tui DESTINATION ${CMAKE_INSTALL_DATADIR}/zsh/site-functions)
  install(FILES ${TUI_COMP}/hyperctl-tui.fish DESTINATION ${CMAKE_INSTALL_DATADIR}/fish/vendor_completions.d)
elseif(BUILD_HYPERCTL_TUI)
  message(WARNING "Go not found: hyperctl-tui (pretty hyperctl output, `hyperctl tui`) will not be built")
endif()
