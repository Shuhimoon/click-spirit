# 點擊精靈（click-spirit）

點擊精靈用一份 JSON 腳本，在指定座標依序點擊。同一套 Flutter 介面涵蓋 Android、Windows 與 macOS：桌面透過 Go c-shared 函式庫下指令，Android 則透過無障礙服務送出手勢。

這個目錄是 Go module `github.com/click-spirit/click-spirit`。核心與指令列工具不依賴 CGO，在 Linux 上可以完整建置與測試。真正移動游標的實作只在 Windows，以及開啟 CGO 的 macOS。

## 架構

```text
click-spirit/
├── core/                 腳本模型與執行器（純 Go）
├── platform/             桌面點擊後端（Windows / macOS / 其他平台的 stub）
├── internal/engine/      事件佇列與執行狀態，ffi 與 mobile 共用
├── mobile/               gomobile 綁定給 Android
├── ffi/                  c-shared 匯出（CS_*），給 Flutter FFI
├── cmd/clicker/          指令列工具
├── app/                  Flutter 介面（Android、Windows、macOS）
├── scripts/              建置腳本
└── .github/workflows/    CI
```

執行路徑：

- 桌面：Flutter `FfiClickEngine` 載入 `click_spirit.dll` 或 `libclick_spirit.dylib`，每隔 100 毫秒呼叫 `CS_PollEvent`。
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
go run ./cmd/clicker run sample.json    # 需要 Windows 或 macOS 後端
go run ./cmd/clicker pos                # 印出游標；不支援時會失敗
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
go build -buildmode=c-shared -o /tmp/libcs.so ./ffi
```

Linux 的 c-shared 建置會走到 platform stub，函式庫可以載入，但 `CS_Start` 會回報此平台不支援。Android 請改用 `mobile` 套件。

### Flutter 外殼

此環境沒有 Flutter CLI。請先在 `app/` 產生各平台 Runner。這個指令不會覆寫已存在的檔案：

```bash
cd app
flutter create --platforms=android,windows,macos --org com.clickspirit --project-name click_spirit .
```

若平台目錄已經有手寫檔、但還沒有 Gradle 或 Xcode 專案，用根目錄的 `bash scripts/flutter-scaffold.sh`。它會把不完整的目錄先移開，執行 `flutter create`，再把手寫檔蓋回去，並把 Android `minSdk` 設成 24、加上 `clickspirit.aar` 依賴。

### Windows

```bash
bash scripts/build-desktop-lib.sh windows
```

等同：

```bash
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
  go build -buildmode=c-shared -o app/windows/libs/click_spirit.dll ./ffi
```

在 Windows 本機把 `CC` 設成 `gcc` 即可。DLL 要放在 exe 旁邊。CMake `install()` / `POST_BUILD` 片段見 `app/windows/README-native.md`。

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
- **Android**：使用者要在系統設定裡啟用「點擊精靈」無障礙服務，且服務宣告 `canPerformGestures`。點擊是 `GestureDescription` 的點下手勢，不區分滑鼠左右鍵。清單裡已預留 `FOREGROUND_SERVICE` 與 `FOREGROUND_SERVICE_MEDIA_PROJECTION`，供之後的截圖使用；目前的 `ScreenCaptureHelper` 只有接上與釋放 MediaProjection 的空殼。

介面是繁體中文。步驟可以新增、編輯、刪除、拖曳排序。桌面的「擷取座標」會倒數 3 秒再讀游標。Android 提供「開啟無障礙設定」與啟用狀態。腳本用系統檔案選擇器開啟與儲存。

## 待決事項

已定案：三種平台共用一個 Flutter 介面。

還沒有定案的部分：

- 桌面事件要繼續用 FFI 輪詢 `CS_PollEvent`，還是改成 `NativeCallable` 由 Go 直接回呼 Dart。
- 全域快捷鍵要用 Flutter 的 `hotkey_manager`，還是在 Go 端用 Windows `RegisterHotKey` 與 macOS `CGEventTap`。
- Android 座標擷取要用懸浮窗，還是用 MediaProjection 截圖後讓使用者點選。
- DPI 與多螢幕的座標縮放還沒有完整規格。腳本目前要求 X、Y 大於等於 0，虛擬桌面原點為負的螢幕會點不到。
- 程式碼簽章與公證的發佈流程（macOS notarize、Windows 簽章）還沒有自動化。
- 截圖與找圖點擊：`core.ScreenCapturer` 已留介面，各平台尚未實作。
