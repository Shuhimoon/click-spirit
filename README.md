# 點擊精靈（click-spirit）

點擊精靈用一份 JSON 腳本，在指定座標依序點擊。同一套 Flutter 介面涵蓋 Android、Windows、macOS 與 Linux：桌面透過 Go c-shared 函式庫下指令，Android 則透過無障礙服務送出手勢。

這個目錄是 Go module `github.com/click-spirit/click-spirit`。核心與指令列工具不依賴 CGO。Linux 桌面有兩個後端：有 `DISPLAY` 且以 CGO 編譯時走 X11 XTest（可讀游標、可截圖），否則走 uinput（Wayland 路徑，只送事件）。Windows 用 `SendInput`。macOS 要開啟 CGO，走 Core Graphics。

## 架構

```text
click-spirit/
├── core/                 腳本模型與執行器（純 Go）
├── platform/             桌面點擊後端（Windows / macOS / Linux X11 與 uinput / 其他平台的 stub）
├── internal/engine/      事件佇列與執行狀態，ffi 與 mobile 共用
├── mobile/               gomobile 綁定給 Android
├── ffi/                  c-shared 匯出（CS_*），給 Flutter FFI
├── cmd/clicker/          指令列工具
├── app/                  Flutter 介面（Android、Windows、macOS、Linux）
├── scripts/              建置腳本
└── .github/workflows/    CI
```

執行路徑：

- 桌面：Flutter `FfiClickEngine` 載入 `click_spirit.dll`、`libclick_spirit.dylib` 或 `libclick_spirit.so`（Linux 套件在執行檔旁的 `lib/`），每隔 100 毫秒呼叫 `CS_PollEvent`。`CS_Backend` 回報後端名稱，`CS_CaptureScreenPNG` 在支援的後端寫出 PNG。
- Android：Flutter `ChannelClickEngine` 走 MethodChannel `click_spirit/engine` 與 EventChannel `click_spirit/events`，Kotlin 再呼叫 gomobile 產生的 `com.clickspirit.mobile.Mobile`。
- 指令列：`clicker run` 直接使用 `core.Runner`。`-dry-run` 只印出動作，Linux 上也能跑。

## 腳本 JSON

欄位使用 snake_case，與 Go 的 `encoding/json` 標籤一致。`repeat` 為 0 代表無限循環。步驟的 `clicks` 為 0 代表點一下（只接受 0 到 3）。步驟的 `interval_ms` 為 0 時，改用腳本的 `default_interval_ms`。間隔是點擊之後的等待；`loop_interval_ms` 只加在迴圈與迴圈之間。

```json
{
  "version": 1,
  "name": "示範腳本",
  "steps": [
    {
      "x": 100,
      "y": 200,
      "button": "left",
      "clicks": 1,
      "interval_ms": 50,
      "hold_ms": 0
    },
    {
      "x": 300,
      "y": 400,
      "button": "right",
      "clicks": 2,
      "interval_ms": 0,
      "hold_ms": 30
    }
  ],
  "repeat": 1,
  "default_interval_ms": 100,
  "loop_interval_ms": 500,
  "start_delay_ms": 200
}
```

`button` 只能是 `left`、`right` 或 `middle`。座標必須大於等於 0。

指令列：

```bash
go run ./cmd/clicker sample -o sample.json
go run ./cmd/clicker validate sample.json
go run ./cmd/clicker run -dry-run sample.json
go run ./cmd/clicker run sample.json    # 需要桌面後端（Windows、macOS，或 Linux 的 X11 / uinput）
go run ./cmd/clicker pos                # 印出游標；uinput 不支援，會失敗
```

Windows 執行檔：

```bash
bash scripts/build-windows-cli.sh
# 產出 dist/clicker.exe
```

Ctrl+C 會停止執行中的腳本。

## 建置

Go 版本為 1.26。`golang.org/x/sys` v0.49.0 的 `go` 行是 `1.26.0`，所以本模組的 `go` 行也寫成 `1.26.0`，否則 `GOOS=windows go build` 會要求先跑 `go mod tidy`。

### 在 Linux 上檢查 Go

```bash
gofmt -l .
go vet ./...
go test -race ./...
go build ./...
GOOS=windows GOARCH=amd64 go build ./...
GOOS=windows GOARCH=386 go vet ./platform/
GOOS=darwin CGO_ENABLED=0 go build ./...
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build ./core ./mobile
CGO_ENABLED=0 go build ./...
go build -tags nox11 ./...
go build -buildmode=c-shared -o /tmp/libcs.so ./ffi
# 只在私人 Xvfb 上跑，不要對現有的 DISPLAY 送事件：
xvfb-run -a -s "-screen 0 1280x800x24" env CLICK_SPIRIT_X11_TEST=1 go test ./platform -run X11 -v
```

`go test` 預設不會移動游標。X11 整合測試要另外設 `CLICK_SPIRIT_X11_TEST=1`，而且只應該在 `xvfb-run` 底下跑。`CGO_ENABLED=0` 或 `-tags nox11` 不會編進 X11，這時有 `WAYLAND_DISPLAY`、或 X11 沒有編進來，就改走 uinput。Android 請改用 `mobile` 套件。

### Flutter 外殼

四個平台的 Runner 都在 `app/`。若要補齊缺少的平台檔（這個指令不會覆寫已存在的檔案）：

```bash
cd app
flutter create --platforms=android,windows,macos,linux --org com.clickspirit --project-name click_spirit .
```

若平台目錄已經有手寫檔、但還沒有 Gradle、CMake 或 Xcode 專案，用根目錄的 `bash scripts/flutter-scaffold.sh`。它會把不完整的目錄先移開，執行上面的 `flutter create`，再把手寫檔蓋回去，並把 Android `minSdk` 設成 24。AAR 依賴用 `fileTree` 收 `app/android/app/libs/*.aar`，還沒建出 AAR 時 Gradle 仍可設定。

### Windows

```bash
bash scripts/build-desktop-lib.sh windows
```

等同：

```bash
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
  go build -buildmode=c-shared -o app/windows/libs/click_spirit.dll ./ffi
```

在 Windows 本機把 `CC` 設成 `gcc` 即可。DLL 要放在 exe 旁邊。`app/windows/CMakeLists.txt` 會在 `app/windows/libs/click_spirit.dll` 已經存在時，把它安裝到 exe 旁邊。請先建 DLL，再跑 Flutter。細節見 `app/windows/README-native.md`。

```bash
cd app && flutter build windows
```

### macOS

必須在 macOS 上編譯，因為 Core Graphics 需要 CGO 與系統框架。

```bash
bash scripts/build-desktop-lib.sh macos
cd app && flutter build macos
```

腳本會產出 arm64 與 amd64 兩份 dylib，用 `lipo` 合成 `app/macos/libs/libclick_spirit.dylib`，並把 install name 設成 `@rpath/libclick_spirit.dylib`。Sandbox 必須關閉，否則 `CGEventPost` 會失敗。嵌入 Frameworks、輔助使用權限、簽章與公證的步驟見 `app/macos/README-native.md`。

### Linux

在 Linux 上編譯。需要 X11 與 XTest 的開發套件（`libx11-dev`、`libxtst-dev`）、`pkg-config`，以及 Flutter Linux 桌面要的 GTK（`libgtk-3-dev`、`cmake`、`ninja-build`、`clang`）。

```bash
bash scripts/build-desktop-lib.sh linux
cd app && flutter build linux
```

等同：

```bash
CGO_ENABLED=1 go build -buildmode=c-shared -o app/linux/libs/libclick_spirit.so ./ffi
```

`app/linux/CMakeLists.txt` 會在設定當下看得到 `app/linux/libs/libclick_spirit.so` 時，把它安裝到套件的 `lib/`。執行檔用 `Platform.resolvedExecutable` 旁邊的 `lib/libclick_spirit.so` 載入。

後端選擇：

| 條件 | 後端 |
| --- | --- |
| `CLICK_SPIRIT_BACKEND=x11` 或 `uinput` | 強制使用該後端 |
| 沒有強制，且 `DISPLAY` 有設、X11 有編進來 | X11 |
| 否則 `WAYLAND_DISPLAY` 有設，或 X11 沒編進來 | uinput |
| 都沒有 | 錯誤 |

X11 用 `XTestFakeMotionEvent` / `XTestFakeButtonEvent`（左 1、中 2、右 3），游標用 `XQueryPointer`，截圖用根視窗的 `XGetImage`（ZPixmap，依色罩轉成 RGBA）。所有 Xlib 呼叫都有 mutex，行程載入函式庫時會先 `XInitThreads`。

uinput 是 Wayland 路徑。Wayland 不容易查螢幕大小，所以範圍來自環境變數或建構式：

```bash
export CLICK_SPIRIT_SCREEN=1920x1080   # 寬x高，座標 0..寬-1、0..高-1
```

程式裡也可以呼叫 `platform.NewUinput(width, height)`。uinput 沒有 `CursorPos`，也不能截圖。

### Android

需要 Android SDK、NDK，以及 gomobile：

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
gomobile init
bash scripts/build-android-aar.sh
cd app && flutter build apk
```

AAR 輸出到 `app/android/app/libs/clickspirit.aar`。Java 套件是 `com.clickspirit.mobile`。`minSdk` 為 24，因為 `dispatchGesture` 從 API 24 才有。

## 權限

- **Windows**：輸入走 `SendInput`。完整性較高的視窗（例如以系統管理員身分執行的程式）會被 UIPI 擋住，一般權限的行程點不到。多螢幕座標以虛擬桌面（`SM_XVIRTUALSCREEN` 到 `SM_CYVIRTUALSCREEN`）歸一化。程式會盡量開啟 per-monitor DPI awareness。
- **macOS**：需要「輔助使用」權限，`AXIsProcessTrusted` 為假時事件不會送到其他 App。App Sandbox 必須關閉（`com.apple.security.app-sandbox` = false）。
- **Linux / X11**：行程要連得上 `DISPLAY`，X 伺服器要有 XTest。這條路徑會真的移動該顯示器上的游標。請只對自己的 X 伺服器送事件；開發時用 `xvfb-run` 開私人畫面。
- **Linux / Wayland（uinput）**：要能寫 `/dev/uinput`。使用者需加入 `input` 群組，或設定 udev 規則：

  ```
  KERNEL=="uinput", GROUP="input", MODE="0660"
  ```

  規則放進 `/etc/udev/rules.d/99-uinput.rules` 後重新載入 udev，或重新登入讓群組生效。沒有這個權限時，開裝置會失敗。座標範圍要設 `CLICK_SPIRIT_SCREEN=WxH`。在同時有 `DISPLAY` 與 `WAYLAND_DISPLAY` 的 XWayland 環境，預設仍走 X11。
- **Android**：使用者要在系統設定裡啟用「點擊精靈」無障礙服務，且服務宣告 `canPerformGestures`。點擊是 `GestureDescription` 的點下手勢，不區分滑鼠左右鍵。清單裡已預留 `FOREGROUND_SERVICE` 與 `FOREGROUND_SERVICE_MEDIA_PROJECTION`，供之後的截圖使用；目前的 `ScreenCaptureHelper` 只有接上與釋放 MediaProjection 的空殼。

介面是繁體中文。步驟可以新增、編輯、刪除、拖曳排序。桌面的「擷取座標」會倒數 3 秒再讀游標。Android 提供「開啟無障礙設定」與啟用狀態。腳本用系統檔案選擇器開啟與儲存。

## 待決事項

已定案：Android、Windows、macOS、Linux 共用一個 Flutter 介面。Linux 的 X11 後端已能點擊、讀游標與截圖。

還沒有定案的部分：

- 桌面事件要繼續用 FFI 輪詢 `CS_PollEvent`，還是改成 `NativeCallable` 由 Go 直接回呼 Dart。
- 全域快捷鍵要用 Flutter 的 `hotkey_manager`，還是在 Go 端用 Windows `RegisterHotKey` 與 macOS `CGEventTap`。
- Android 座標擷取要用懸浮窗，還是用 MediaProjection 截圖後讓使用者點選。
- DPI 與多螢幕的座標縮放還沒有完整規格。腳本目前要求 X、Y 大於等於 0，虛擬桌面原點為負的螢幕會點不到。
- 程式碼簽章與公證的發佈流程（macOS notarize、Windows 簽章）還沒有自動化。
- 截圖與找圖點擊：Linux X11 已能截全螢幕（`CS_CaptureScreenPNG`）。Windows、macOS、Android 的 `core.ScreenCapturer` 仍未實作。依圖點擊還沒有做。
- Wayland 下的座標與截圖：uinput 只能送出絕對座標，讀不到游標，也不能截圖。之後可走 xdg-desktop-portal 的 RemoteDesktop（輸入）與 ScreenCast（截圖）。
- uinput 絕對座標與多螢幕：裝置範圍來自 `CLICK_SPIRIT_SCREEN` 或 `NewUinput`，尚未對應多螢幕原點與縮放。
