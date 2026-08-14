# DroidForge

> 将 Android 系统镜像构建产物快速锻造成可运行安全研究环境的 macOS 工具链。

DroidForge 是面向 Android Framework 安全研究的 Go CLI。它将 Android SDK 固定在项目目录内，统一完成 Emulator 部署、研究 AVD 生命周期管理，以及 Linux 编译的自定义 AOSP 镜像加载。

## 功能

- 一键部署 Android SDK、ADB、Emulator、API 33 与 Android 13 系统镜像。
- 根据宿主自动选择 Intel `x86_64` 或 Apple Silicon `arm64-v8a` 镜像。
- 创建研究用 AVD（4 核、4 GB RAM、8 GB data）。
- 使用 SwiftShader 软件渲染，规避部分 Intel Mac 的 Emulator 黑屏问题。
- 启动官方系统镜像，支持冷启动、无窗口和可选的数据重置。
- 按目录加载 Linux 导出的 system/vendor/product/ramdisk/kernel/userdata 等自定义镜像。
- 环境诊断与命令行停止模拟器。

## 快速开始

要求：macOS、Go 1.23+、JDK 17+、网络连接和数 GB 可用空间。

```zsh
git clone https://github.com/Patzilla0o7/DroidForge.git
cd DroidForge
go build -o bin/droidforge ./cmd/droidforge

./bin/droidforge setup
./bin/droidforge create-avd
./bin/droidforge start
```

检查环境或停止实例：

```zsh
./bin/droidforge doctor
./bin/droidforge stop
```

## 自定义 AOSP 镜像

Linux 应使用与 Mac ABI 一致的 target：Intel Mac 使用 `aosp_x86_64-eng`，Apple Silicon 使用 `aosp_arm64-eng`。仅将最终镜像同步到本项目，例如：

```text
artifacts/aosp13-build/
├── system.img                 # 必需
├── vendor.img                 # 建议
├── product.img                # 可选
├── ramdisk.img                # 可选
├── kernel-ranchu 或 kernel    # 可选
└── userdata.img               # 可选的初始 data
```

```zsh
./bin/droidforge start --image-dir artifacts/aosp13-build --cold-boot
```

自定义镜像必须与 AVD 的 ABI、API 级别、分区布局、kernel/ramdisk 和 AVB 配置兼容。首次加载一个新构建建议使用 `--cold-boot`；`--wipe-data` 会清除 AVD 中的应用和设置。

## 开发

```zsh
go test ./cmd/droidforge
go vet ./cmd/droidforge
go build ./cmd/droidforge
```

`.android-sdk-macos/`、`artifacts/` 和 `bin/` 均不会提交到 Git。
