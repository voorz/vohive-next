<div align="center">

# VoHive

**A unified management & proxy platform for Qualcomm 4G/LTE/5G modems**

[![License: PolyForm Noncommercial 1.0.0](https://img.shields.io/badge/License-PolyForm--Noncommercial--1.0.0-blue.svg)](https://polyformproject.org/licenses/noncommercial/1.0.0)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](go.mod)
[![Vue 3](https://img.shields.io/badge/Vue-3-42b883?logo=vue.js)](web/package.json)
[![Build](https://img.shields.io/github/actions/workflow/status/voorz/vohive-next/binary-release.yml?logo=github&label=build)](https://github.com/voorz/vohive-next/actions)

[English](README.md) | [简体中文](README_zh.md)

</div>

---

## Overview

VoHive integrates modem hot-plug management, SOCKS5/HTTP proxy orchestration, SMS messaging, VoWiFi/IMS voice calling, and full-lifecycle eSIM management into a single service — paired with a modern, responsive web dashboard.

## Core Features

| Module | Description |
| :--- | :--- |
| **Multi-Modem Management** | USB hot-plug auto-discovery (ttyUSB, etc.) with real-time multi-device status monitoring |
| **Lightweight Proxy Engine** | Built-in SOCKS5/HTTP proxy core with multi-instance concurrency; strict per-device outbound binding via `SO_BINDTODEVICE` |
| **SMS & Telephony Hub** | Unified UI/API for AT-based SMS send/receive, conversation & contact management, USSD interaction, with persistent SMS storage |
| **VoWiFi / IMS Voice** | IMS tunneling over broadband, SIP registrar server with dual-path call routing (VoWiFi-first, CS-fallback), G.711 μ-law codec & ALSA audio bridging |
| **eSIM Management** | Direct AT-channel eSIM chip control — profile download, enable/disable, rename, delete; token-based expiry protection with SIM reload fallback |
| **Multi-Channel Notifications** | Push critical SMS and system alerts to Telegram, Email, PushPlus, Bark, Lark/Feishu, QQ, and more |
| **Multi-Architecture Builds** | Native cross-compilation for amd64 / arm64 / armv7 — deploy from routers to edge nodes |

## Tech Stack

| Layer | Technologies |
| :--- | :--- |
| **Backend** | Go 1.26+ · Gin · GORM · Viper · euicc-go |
| **Frontend** | Vue 3 · Vite · TailwindCSS · Element Plus |
| **Database** | SQLite (`vohive.db`) |
| **CI/CD** | GitHub Actions — automated multi-arch binary builds & Docker images |

## License

Licensed under the [PolyForm Noncommercial License 1.0.0](LICENSE).  
**Non-commercial use only** — viewing, using, modifying, and distributing source code for personal study, research, and testing is permitted. Commercial use is strictly prohibited without separate authorization.
