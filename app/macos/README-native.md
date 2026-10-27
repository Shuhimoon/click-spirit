# macOS 原生函式庫

`flutter create` 不會覆寫已經存在的檔案。請先在 `app/` 產生 Runner，再把通用 dylib 放進 App。

## 1. 建置通用 dylib

在 macOS 上，於專案根目錄執行：

```bash
bash scripts/build-desktop-lib.sh macos
```

腳本會分別以 `GOARCH=arm64` 與 `GOARCH=amd64`、`-buildmode=c-shared` 編譯 `./ffi`，再用 `lipo -create` 合成：

`app/macos/libs/libclick_spirit.dylib`

並執行：

```bash
install_name_tool -id @rpath/libclick_spirit.dylib app/macos/libs/libclick_spirit.dylib
```

## 2. 關閉 App Sandbox

Sandbox 會擋下 `CGEventPost`。`app/macos/Runner/DebugProfile.entitlements` 與 `Release.entitlements` 已設定：

```xml
<key>com.apple.security.app-sandbox</key>
<false/>
```

若 `flutter create` 產生了另一份 entitlements，請覆寫成同樣的內容。Debug 設定同時保留 JIT 與本機網路，讓 Flutter 偵錯可以連上 observatory。

## 3. 用 Xcode 把 dylib 放進 Frameworks

1. 打開 `app/macos/Runner.xcworkspace`。
2. 選 Runner target → Build Phases → 按 `+` → New Copy Files Phase。
3. Destination 選 **Frameworks**。
4. 把 `app/macos/libs/libclick_spirit.dylib` 加進去。
5. 勾選 Code Sign On Copy。

成品路徑是 `ClickSpirit.app/Contents/Frameworks/libclick_spirit.dylib`。執行檔的 rpath 需要包含：

```text
@executable_path/../Frameworks
```

若載入失敗，對執行檔補上：

```bash
install_name_tool -add_rpath @executable_path/../Frameworks \
  path/to/click_spirit.app/Contents/MacOS/click_spirit
```

Dart 端會先試 `Contents/Frameworks/libclick_spirit.dylib`，再退回 `libclick_spirit.dylib`。

## 4. 輔助使用權限

到「系統設定 → 隱私權與安全性 → 輔助使用」，允許這個 App。沒有這項權限時，`CGEventPost` 不會真的移動游標。`CS_AccessibilityTrusted` 會回報目前狀態。

## 5. 簽章與公證

發佈前對 App 與 dylib 做 codesign，再送公證（notarize）。Sandbox 已關閉，公證用的 entitlements 必須與上面一致。開發階段可以先用 ad-hoc 簽章確認目錄結構。
