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

`click_spirit.dll` 必須跟 `click_spirit.exe` 放在同一個目錄。`app/windows/CMakeLists.txt` 已經在 DLL 存在時把它 `install()` 到 `${CMAKE_INSTALL_PREFIX}`（exe 旁邊）。請先執行 `bash scripts/build-desktop-lib.sh windows`，再跑 `flutter build windows`，CMake 設定當下才看得到檔案。

Dart 端先開 exe 同目錄的 `click_spirit.dll`，再開目前工作目錄的同名檔。

Windows 的滑鼠事件走 `SendInput`。程式會盡量呼叫 `SetProcessDpiAwarenessContext`（每螢幕 DPI）或 `SetProcessDPIAware`。對完整性較高（以系統管理員身分執行）的視窗，一般權限的行程送不出輸入，這是 UIPI 的限制，不是腳本格式問題。
