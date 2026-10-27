# Windows 原生函式庫

在專案根目錄建置 c-shared DLL（Linux/macOS 交叉編譯時使用 MinGW；在 Windows 上可把 `CC` 設成 `gcc`）：

```bash
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
  go build -buildmode=c-shared -o app/windows/libs/click_spirit.dll ./ffi
```

或直接：

```bash
bash scripts/build-desktop-lib.sh windows
```

`click_spirit.dll` 必須跟 `click_spirit.exe` 放在同一個目錄。Flutter 產生 Runner 之後，把下面片段加進 `app/windows/runner/CMakeLists.txt`（`BINARY_NAME` 是 Flutter 範本裡的執行檔目標）：

```cmake
# Copy click_spirit.dll next to the executable.
add_custom_command(TARGET ${BINARY_NAME} POST_BUILD
  COMMAND ${CMAKE_COMMAND} -E copy_if_different
    "${CMAKE_CURRENT_SOURCE_DIR}/../libs/click_spirit.dll"
    "$<TARGET_FILE_DIR:${BINARY_NAME}>/click_spirit.dll"
  COMMENT "Copy click_spirit.dll next to the executable")

install(FILES "${CMAKE_CURRENT_SOURCE_DIR}/../libs/click_spirit.dll"
  DESTINATION "${CMAKE_INSTALL_PREFIX}"
  COMPONENT Runtime)
```

DLL 會在建置後被複製到 exe 旁邊，`install()` 則在安裝套件時一併帶上。Dart 端先開 exe 同目錄的 `click_spirit.dll`，再開目前工作目錄的同名檔。

Windows 的滑鼠事件走 `SendInput`。程式會盡量呼叫 `SetProcessDpiAwarenessContext`（每螢幕 DPI）或 `SetProcessDPIAware`。對完整性較高（以系統管理員身分執行）的視窗，一般權限的行程送不出輸入，這是 UIPI 的限制，不是腳本格式問題。
