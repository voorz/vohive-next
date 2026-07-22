<div align="center">

# VoHive

**面向高通 4G/LTE/5G 模组的综合管理与代理平台**

[![License: PolyForm Noncommercial 1.0.0](https://img.shields.io/badge/License-PolyForm--Noncommercial--1.0.0-blue.svg)](https://polyformproject.org/licenses/noncommercial/1.0.0)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](go.mod)
[![Vue 3](https://img.shields.io/badge/Vue-3-42b883?logo=vue.js)](web/package.json)
[![Build](https://img.shields.io/github/actions/workflow/status/voorz/vohive-next/binary-release.yml?logo=github&label=build)](https://github.com/voorz/vohive-next/actions)

[English](README.md) | [简体中文](README_zh.md)

</div>

---

## 项目简介

VoHive 将模组热插拔管理、SOCKS5/HTTP 代理编排、短信收发、VoWiFi/IMS 语音通话、eSIM 全生命周期管理整合到单一服务中，配合一套现代化的响应式 Web 管理后台。

面向 Quectel 模组系列（EC20 / EC25 / EC21 / EG25 / EM20 等），可将挂载多张物理 SIM 卡或 eSIM 的主机变成为一个自包含的移动网络节点。

## 核心特性

| 模块 | 说明 |
| :--- | :--- |
| **多模组并发管理** | USB 热插拔自动发现（ttyUSB 等），多设备实时状态监控 |
| **轻量级代理引擎** | 内建 SOCKS5 / HTTP 代理内核，支持多实例并发；基于 `SO_BINDTODEVICE` 按设备网卡严格绑定出站流量 |
| **通信与短信中心** | 统一界面 / API 处理 AT 短信收发、会话与联系人管理、USSD 交互，短信落库可查 |
| **VoWiFi / IMS 语音** | 借助宽带网络建立 IMS 隧道连接，SIP 注册服务器与双路径呼叫路由（VoWiFi 优先，CS 回退），集成 G.711 μ-law 编解码与 ALSA 音频桥接 |
| **eSIM 管理** | 通过 AT 指令通道直接控制 eSIM 芯片 — Profile 下载、启用 / 停用、重命名、删除，Token 化防过期与 SIM 重载兜底 |
| **全渠道通知** | 重要短信及系统告警可推送至 Telegram、Email、PushPlus、Bark、飞书（Lark/Feishu）、QQ 等 |
| **多架构构建** | 原生支持 amd64 / arm64 / armv7 跨平台编译，路由器到边缘节点均可部署 |

## 技术栈

| 层级 | 技术选型 |
| :--- | :--- |
| **后端** | Go 1.26+ · Gin · GORM · Viper · euicc-go |
| **前端** | Vue 3 · Vite · TailwindCSS · Element Plus |
| **数据库** | SQLite |
| **CI/CD** | GitHub Actions — 多架构二进制与 Docker 镜像构建 |

## License

本项目基于 [PolyForm Noncommercial License 1.0.0](LICENSE) 开源。

**仅限非商业用途。** 可自由查看、使用、修改、分发源码用于个人学习、研究、测试。任何形式的商业使用 — 包括销售、提供付费服务、用于盈利性产品 — 均被严格禁止，如需商业授权请联系作者另行协商。
