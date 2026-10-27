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

沒有 `target` 的舊腳本仍是定點：點步驟的 `x, y`。新欄位都可省略，省略時行為與舊版相同。

### 步驟重複、圓形與找圖

腳本的 `repeat` 仍是整份腳本的迴圈次數，`0` 代表無限。步驟自己的 `repeat` 是這一筆要連續做幾次，`0` 或 `1` 都是一次。`interval_ms` 加在每一次重複之後；`0` 仍改用 `default_interval_ms`。

`target.kind`：

| kind | 行為 |
| --- | --- |
| 省略或 `point` | 點步驟的 `x, y` |
| `circle` | 在圓盤 `(cx, cy, r)` 內均勻抽一個整數像素。極角 `2πu1`，半徑 `R*sqrt(u2)`，四捨五入後小於 0 的座標會夾到 0 |
| `image` | 在截圖裡做灰階零均值正規化互相關（ZNCC），點在命中框的中心再加上 `offset_x`、`offset_y` |

找圖欄位：`template`（PNG 路徑）或 `template_b64`（可含 data URL）。`threshold` 省略或 `0` 時是 `0.9`，範圍 `0..1`。`region` 是 `{x,y,w,h}`，省略或寬高不大於 0 就搜全圖。`on_not_found` 是 `skip`（預設，不點但仍等間隔）、`wait`（在 `timeout_ms` 內每 `poll_ms` 再截一次，逾時失敗；`poll_ms` 省略是 100）或 `stop`（立刻失敗）。沒有注入截圖來源時，找圖步驟會失敗並說明缺少 capturer。

```json
{
  "version": 1,
  "name": "進階示範",
  "emergency_hotkey": "F12",
  "repeat": 1,
  "default_interval_ms": 100,
  "steps": [
    {"x": 10, "y": 20, "button": "left", "clicks": 1, "interval_ms": 50, "repeat": 2},
    {
      "x": 0, "y": 0, "button": "left", "clicks": 1, "interval_ms": 80,
      "target": {"kind": "circle", "cx": 400, "cy": 300, "r": 40}
    },
    {
      "x": 0, "y": 0, "button": "left", "clicks": 1,
      "target": {
        "kind": "image",
        "template": "btn.png",
        "threshold": 0.9,
        "offset_x": 0,
        "offset_y": 0,
        "on_not_found": "wait",
        "timeout_ms": 3000,
        "poll_ms": 200,
        "region": {"x": 0, "y": 0, "w": 800, "h": 600}
      }
    }
  ]
}
```

`emergency_hotkey` 是緊急停止和弦，例如 `F12` 或 `Ctrl+Shift+Esc`。大小寫不拘，修飾鍵可為 Ctrl、Shift、Alt、Super（也接受 Win、Meta、Cmd）。引擎有熱鍵後端時，開始執行會武裝它；觸發後立刻停止，並送出 `emergency_stop`。空字串不改目前的熱鍵。Android 沒有全域熱鍵，介面按鈕呼叫 `EmergencyStop`。

執行中的 `step` 事件帶步驟索引與第幾次重複（0 起算）。重複超過一次時，訊息會加上 `#1`、`#2`。

### 找圖演算法與限制

`core/imagematch` 是純 Go，沒有 OpenCV、也沒有 cgo。影像先轉成灰階（Rec.601），再用積分圖算零均值正規化互相關。分數在 -1 到 1。模板沒有對比時會失敗。畫面夠大時會先做 2 倍縮小的粗搜，再在附近細搜；粗搜分數太低或細搜明顯變差時改回全圖掃描。`FindAll` 用門檻加非最大抑制。

不支援縮放與旋轉。模板要與畫面上的圖一樣大、一樣正。亮度小幅變化與輕微雜訊仍可命中，因為 ZNCC 對亮度平移與縮放不敏感。

### 錄製與回放

`core/record` 收時間戳事件（move、down、up、click）。`platform.NewInputSource` 在 Linux 上接全域鉤子，其他作業系統仍回傳尚未實作。測試也可以用 `Recorder.Add`，或讓引擎的 `StartRecordingWith` 吃一個假的 `InputSource`。

Linux 怎麼選：

| 條件 | 錄製 |
| --- | --- |
| 點擊後端是 X11（`DISPLAY` 有設、X11 有編進來、沒有強制 uinput），而且伺服器有 RECORD | `XRecord` |
| 否則 | evdev，讀 `/dev/input/event*` |

X11 開兩條 Display：控制連線建立 context，資料連線 `XRecordEnableContextAsync`。範圍是 `XRecordAllClients` 的 `ButtonPress`、`ButtonRelease`、`MotionNotify`。座標用 `xEvent.u.keyButtonPointer` 的 rootX/rootY。按鈕 1/2/3 是左/中/右，4–7（滾輪）略過。時間用送達當下的 `time.Now()`。我們自己的 XTest 回放也會被記下來，程式沒有把那些事件濾掉；錄製時若同時在播放，播放的點擊會進腳本。

Wayland 或沒有 RECORD 時走 evdev。指標裝置要有 `BTN_LEFT`。evdev 的滑鼠只有相對位移，起點用 X11 後端的游標（後端不是 X11 或讀不到時是 0,0），再累加 `REL_X`/`REL_Y`，已知螢幕大小就夾在範圍內。觸控板的 `EV_ABS` 用 `EVIOCGABS` 的最小/最大縮放到 `CLICK_SPIRIT_SCREEN`；沒設螢幕大小時，座標只是裝置值減掉最小值。**Wayland 上的座標是近似值。** 讀取被拒（`EACCES`）時，錯誤會包住 `ErrUnsupported`，並說明使用者要加入 `input` 群組。

熱鍵在 Linux 上同樣二選一：X11 連得上就繼續用 `XGrabKey`；否則 evdev 看有 `KEY_F1`–`KEY_F12` 的鍵盤，追蹤左右 Ctrl、Shift、Alt、Super，只在按下（不是連發）且修飾鍵完全符合時觸發。`CGO_ENABLED=0` 或 `-tags nox11` 沒有 X11，熱鍵與錄製都走 evdev。

轉換規則：移動略過。同一鍵的 down 加 up 合成一次點擊，`hold_ms` 是按住的時間，座標用按下的位置。連續點擊若同鍵、起點間隔在雙擊窗內（預設 300 毫秒，負值關閉）、位移在容許範圍（預設 3 像素），會併成 `clicks` 2 或 3。步驟間隔是這次放開到下次按下，再除以速度（`<=0` 當 1），然後不低於最短間隔。雙擊內部的空隙不會保留。回放就是把產生的腳本交給 `Runner`。FFI 的 `CS_StartRecording` 失敗時回傳 `{"error":...}`，成功是 `{"ok":true}`。`CS_StopRecording` 成功時回傳腳本 JSON。

### 緊急停止的平台實作

- Linux，X11 連得上（cgo 且沒有 `-tags nox11`，`DISPLAY` 開得起來）：另一條 Display 連線上 `XGrabKey`，事件迴圈在自己的 goroutine。Caps Lock 與 Num Lock 的組合會一併抓住。
- Linux，X11 不可用：evdev 鍵盤。使用者要在 `input` 群組才能讀 `/dev/input/event*`。
- Windows：`RegisterHotKey` 加鎖定 OS 執行緒的訊息迴圈。
- macOS：尚未實作。需要 `CGEventTap` 與輔助使用權限。
- Android：沒有全域熱鍵。`mobile.EmergencyStop` 給懸浮按鈕用。
- `CGO_ENABLED=0` 或 `-tags nox11` 的 Linux 建置不含 X11，熱鍵改走 evdev。

指令列：

```bash
go run ./cmd/clicker sample -o sample.json
go run ./cmd/clicker validate sample.json
go run ./cmd/clicker run -dry-run sample.json
go run ./cmd/clicker run sample.json    # 需要桌面後端（Windows、macOS，或 Linux 的 X11 / uinput）
go run ./cmd/clicker pos                # 印出游標；uinput 不支援，會失敗
go run ./cmd/clicker match -threshold 0.9 btn.png screen.png
```

`run` 在非 dry-run 且腳本有 `emergency_hotkey` 時會武裝該熱鍵。`match` 在兩張 PNG 上找圖並印 JSON，找到且沒有錯誤時結束碼為 0。

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

`go test` 預設不會移動游標。會送 XTest 的測試（點擊、截圖、`XGrabKey`、`XRecord`）只應該在 `xvfb-run` 底下跑，而且程式會拒絕 `:7`、`:14`、`:15`、`:17`。舊的 X11 點擊與熱鍵測試還要 `CLICK_SPIRIT_X11_TEST=1`。`XRecord` 錄製測試在 `DISPLAY` 為空或沒有 RECORD 時跳過。`CGO_ENABLED=0` 或 `-tags nox11` 不會編進 X11，這時有 `WAYLAND_DISPLAY`、或 X11 沒有編進來，就改走 uinput；錄製與熱鍵改走 evdev。Android 請改用 `mobile` 套件。

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
cd app && flutter build windows --no-tree-shake-icons
```

### macOS

必須在 macOS 上編譯，因為 Core Graphics 需要 CGO 與系統框架。

```bash
bash scripts/build-desktop-lib.sh macos
cd app && flutter build macos --no-tree-shake-icons
```

腳本會產出 arm64 與 amd64 兩份 dylib，用 `lipo` 合成 `app/macos/libs/libclick_spirit.dylib`，並把 install name 設成 `@rpath/libclick_spirit.dylib`。Sandbox 必須關閉，否則 `CGEventPost` 會失敗。嵌入 Frameworks、輔助使用權限、簽章與公證的步驟見 `app/macos/README-native.md`。

### Linux

在 Linux 上編譯。需要 X11 與 XTest 的開發套件（`libx11-dev`、`libxtst-dev`）、`pkg-config`，以及 Flutter Linux 桌面要的 GTK（`libgtk-3-dev`、`cmake`、`ninja-build`、`clang`）。

```bash
bash scripts/build-desktop-lib.sh linux
cd app && flutter build linux --no-tree-shake-icons
```

`--no-tree-shake-icons`：目前 Flutter 版本的圖示字型裁剪會把部分 Material 圖示（緊急停止、錄製、複製等）畫成錯字，所以發行版先關掉裁剪。

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
cd app && flutter build apk --no-tree-shake-icons
```

AAR 輸出到 `app/android/app/libs/clickspirit.aar`。Java 套件是 `com.clickspirit.mobile`。`minSdk` 為 24，因為 `dispatchGesture` 從 API 24 才有。

## 權限

- **Windows**：輸入走 `SendInput`。完整性較高的視窗（例如以系統管理員身分執行的程式）會被 UIPI 擋住，一般權限的行程點不到。多螢幕座標以虛擬桌面（`SM_XVIRTUALSCREEN` 到 `SM_CYVIRTUALSCREEN`）歸一化。程式會盡量開啟 per-monitor DPI awareness。
- **macOS**：需要「輔助使用」權限，`AXIsProcessTrusted` 為假時事件不會送到其他 App。App Sandbox 必須關閉（`com.apple.security.app-sandbox` = false）。
- **Linux / X11**：行程要連得上 `DISPLAY`，X 伺服器要有 XTest。錄製還要 RECORD。這條路徑會真的移動該顯示器上的游標，也聽得到全螢幕的指標事件。請只對自己的 X 伺服器送事件；開發時用 `xvfb-run` 開私人畫面。
- **Linux / Wayland（uinput 與 evdev）**：送出點擊要能寫 `/dev/uinput`。錄製與緊急停止熱鍵要能讀 `/dev/input/event*`。兩者通常都要求使用者加入 `input` 群組。uinput 還可以設定 udev 規則：

  ```
  KERNEL=="uinput", GROUP="input", MODE="0660"
  ```

  規則放進 `/etc/udev/rules.d/99-uinput.rules` 後重新載入 udev，或重新登入讓群組生效。沒有這個權限時，開裝置會失敗，錄製錯誤會說明要加入 `input` 群組。座標範圍要設 `CLICK_SPIRIT_SCREEN=WxH`。evdev 在 Wayland 上的座標是近似值。在同時有 `DISPLAY` 與 `WAYLAND_DISPLAY` 的 XWayland 環境，點擊、錄製與熱鍵預設仍走 X11。
- **Android**：使用者要在系統設定裡啟用「點擊精靈」無障礙服務，且服務宣告 `canPerformGestures`。點擊是 `GestureDescription` 的點下手勢，不區分滑鼠左右鍵。清單裡已預留 `FOREGROUND_SERVICE` 與 `FOREGROUND_SERVICE_MEDIA_PROJECTION`，供之後的截圖使用；目前的 `ScreenCaptureHelper` 只有接上與釋放 MediaProjection 的空殼。

介面是繁體中文。步驟可以新增、編輯、刪除、拖曳排序。桌面的「擷取座標」會倒數 3 秒再讀游標。Android 提供「開啟無障礙設定」與啟用狀態。腳本用系統檔案選擇器開啟與儲存。

## 待決事項

已定案：Android、Windows、macOS、Linux 共用一個 Flutter 介面。Linux 的 X11 後端已能點擊、讀游標與截圖。

還沒有定案的部分：

- 桌面事件要繼續用 FFI 輪詢 `CS_PollEvent`，還是改成 `NativeCallable` 由 Go 直接回呼 Dart。
- macOS 全域熱鍵仍缺 `CGEventTap`（需要輔助使用權限）。Linux 有 X11 時用 `XGrabKey`，否則用 evdev。Windows 用 `RegisterHotKey`，Android 用 `EmergencyStop`。
- Android 座標擷取要用懸浮窗，還是用 MediaProjection 截圖後讓使用者點選。找圖本體已可吃 Kotlin 傳來的 PNG。
- DPI 與多螢幕的座標縮放還沒有完整規格。腳本目前要求 X、Y 大於等於 0，虛擬桌面原點為負的螢幕會點不到。
- 程式碼簽章與公證的發佈流程（macOS notarize、Windows 簽章）還沒有自動化。
- 截圖：Linux X11 已能截全螢幕（`CS_CaptureScreenPNG`）。Windows、macOS、Android 的 `core.ScreenCapturer` 仍未實作。找圖演算法在 `core/imagematch`，有截圖就能跑。不支援縮放與旋轉。
- 錄製在 Linux 已接上（X11 `XRecord`，否則 evdev）。Windows 與 macOS 的全域滑鼠鉤子還沒做。事件轉腳本與回放已可用。
- Wayland 下的座標與截圖：uinput 只能送出絕對座標，讀不到游標，也不能截圖。錄製改讀 evdev，座標是近似值。之後可走 xdg-desktop-portal 的 RemoteDesktop（輸入）與 ScreenCast（截圖）。
- uinput 絕對座標與多螢幕：裝置範圍來自 `CLICK_SPIRIT_SCREEN` 或 `NewUinput`，尚未對應多螢幕原點與縮放。
