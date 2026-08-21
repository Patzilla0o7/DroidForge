# DroidForge

> 将 Android 系统镜像构建产物快速锻造成可运行安全研究环境的 macOS 工具链。

DroidForge 是面向 Android Framework 安全研究的 Go CLI。它支持 macOS 和 Ubuntu，在项目目录中管理 Android SDK、官方系统镜像 Profile、研究 AVD，以及 Linux 构建的自定义 AOSP 镜像 Profile。

## 工作模型

```text
官方系统镜像 ── image install ──> 官方 Profile ── avd create ──> AVD
Linux AOSP 产物 ── build import ─> 自定义 Profile ── start ───────> Emulator
```

- 官方 Profile 描述 SDK 包路径，例如 `system-images;android-33;default;x86_64`。
- 自定义 Profile 只登记镜像目录的绝对路径，**不会复制或改写** Linux 编译产物。
- 一个 AVD 从官方 Profile 创建；启动时可选择自定义 Profile 覆盖 system/vendor/ramdisk/kernel 等镜像。
- Profile 配置保存在本地 `.droidforge/profiles/`，不会提交包含机器路径的研究环境状态。
- macOS 与 Ubuntu 使用各自的项目内 SDK 目录，避免相互混用宿主平台的 Emulator 二进制。

## 安装 DroidForge

要求：macOS 或 Ubuntu、Go 1.22+、JDK 17+、网络连接及数 GB 可用空间。

```zsh
git clone https://github.com/Patzilla0o7/DroidForge.git
cd DroidForge
go build -o bin/droidforge ./cmd/droidforge
```

可选：在任意开发机生成 macOS 与 Linux 的发布二进制：

```zsh
make release
```

产物位于 `dist/`，包括 Darwin/Linux 的 `amd64` 和 `arm64` 二进制。

## Ubuntu 前置条件

在 Ubuntu x86_64 主机上，先安装 JDK、KVM 与 Emulator 常用运行依赖：

```bash
sudo apt update
sudo apt install -y openjdk-17-jdk qemu-kvm libvirt-daemon-system \
  libvirt-clients libnss3 libpulse0 libglu1-mesa
sudo usermod -aG kvm,libvirt "$USER"
```

执行上述用户组变更后需重新登录。然后运行：

```bash
git clone https://github.com/Patzilla0o7/DroidForge.git
cd DroidForge
go build -o bin/droidforge ./cmd/droidforge
./bin/droidforge setup
./bin/droidforge doctor
```

Ubuntu SDK 保存为 `.android-sdk-linux/`；macOS 保存为 `.android-sdk-macos/`。`doctor` 会在 Linux 中检查当前用户能否访问 `/dev/kvm`。KVM 不可用时可以调试，但 x86/x86_64 模拟会非常慢。

## 官方镜像：下载、分类和启动

首次安装一个 Android 13/API 33 的干净 x86_64 镜像（Intel Mac 或 Ubuntu x86_64）：

```zsh
./bin/droidforge image install \
  --api 33 \
  --tag default \
  --abi x86_64 \
  --name aosp13-intel
```

Apple Silicon 或 Ubuntu ARM64 则使用：

```zsh
./bin/droidforge image install \
  --api 33 \
  --tag default \
  --abi arm64-v8a \
  --name aosp13-apple-silicon
```

`--tag` 可使用 SDK Manager 提供的 tag，例如 `default` 或 `google_apis`。所有可选 API、tag、ABI 由 Google SDK 仓库决定；DroidForge 会将下载好的系统镜像按 SDK 标准目录保存，并以你给出的 `--name` 建立可读的 Profile。

查看已登记镜像：

```zsh
./bin/droidforge image list
```

基于官方 Profile 创建独立 AVD 并启动：

```zsh
./bin/droidforge avd create --name research-aosp13 --image aosp13-intel
./bin/droidforge start --avd research-aosp13
```

常用启动选项：

```zsh
# 不读取 Quick Boot 快照
./bin/droidforge start --avd research-aosp13 --cold-boot

# 清除该 AVD 的应用和设置后启动
./bin/droidforge start --avd research-aosp13 --wipe-data

# 无窗口运行（适用于自动化）
./bin/droidforge start --avd research-aosp13 --no-window

./bin/droidforge stop
```

`setup` 只部署基础环境（Command-line Tools、ADB、Emulator 和许可证），不会下载 Android Platform 或系统镜像：

```zsh
./bin/droidforge setup

# 再明确选择需要的官方镜像
./bin/droidforge image install --api 33 --tag default --abi x86_64 --name baseline
./bin/droidforge avd create --name research --image baseline
```

## 自编译 AOSP 镜像：导入与启动

在 Linux 上构建时，ABI 应与测试目标相容：Intel Mac/Ubuntu x86_64 通常使用 `aosp_x86_64-eng`，Apple Silicon/Ubuntu ARM64 通常使用 `aosp_arm64-eng`。将最终产物同步到项目内，例如：

```text
artifacts/aosp13-dev-20260814/
├── system.img                 # 必需
├── vendor.img                 # 建议
├── product.img                # 可选
├── ramdisk.img                # 可选
├── kernel-ranchu 或 kernel    # 可选
├── userdata.img               # 可选初始 data
└── cache.img                  # 可选
```

将目录登记为自定义 Profile：

```zsh
./bin/droidforge build import \
  --name aosp13-dev \
  --dir artifacts/aosp13-dev-20260814 \
  --api 33 \
  --abi x86_64 \
  --base aosp13-intel
```

然后基于现有 AVD 运行该构建：

```zsh
./bin/droidforge start \
  --avd research-aosp13 \
  --image aosp13-dev \
  --cold-boot
```

DroidForge 自动映射存在的镜像为 Emulator 参数：`system.img → -system`、`vendor.img → -vendor`、`product.img → -product`、`ramdisk.img → -ramdisk`、`kernel-ranchu/kernel → -kernel`、`userdata.img → -initdata`。自定义启动会禁用快照并使用临时可写 system overlay；退出 Emulator 后 overlay 会丢弃。

`system.img` 是导入的最低要求，但完整启动通常还要求匹配的 vendor、ramdisk、kernel、分区布局和 AVB 配置。包含单一 `super.img` 或只包含 `boot.img` 的通用设备构建不能保证直接由 Emulator 启动；应导出与 Emulator target 匹配的镜像组合。

## 诊断与开发

```zsh
./bin/droidforge doctor
./bin/droidforge image list
./bin/droidforge avd list

go test ./cmd/droidforge
go vet ./cmd/droidforge
```

`.android-sdk-macos/`、`.android-sdk-linux/`、`artifacts/`、`bin/`、`dist/` 和 `.droidforge/` 均被 Git 忽略。
