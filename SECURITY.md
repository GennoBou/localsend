# Security Policy

## Supported Versions

Only the latest release line (`1.1.x`) and the `main` branch actively receive security fixes and updates.

| Version | Supported          |
| ------- | ------------------ |
| 1.1.x   | :white_check_mark: |
| <= 1.0.x | :x:               |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them privately using GitHub's **Private Vulnerability Reporting** feature:

1. Go to the **Security** tab of this repository.
2. Click **Report a vulnerability** (or **Advisories** -> **New draft security advisory**).
3. Provide a clear description of the issue, reproduction steps, affected version/commit, and potential impact.

This repository is maintained on a best-effort basis by an individual maintainer. You can typically expect an initial acknowledgment within 7 days.

## Scope and Design Notes

When evaluating or reporting vulnerabilities, please keep the following architectural context in mind:

- **Unofficial Implementation**: This project is an unofficial Go implementation of the LocalSend protocol (v2.1). Vulnerabilities specific to the official LocalSend desktop/mobile apps should be reported to the official LocalSend project.
- **Local Network Assumption**: LocalSend is designed for file transfers on trusted local area networks (LAN). Exposing the server directly to the public internet is outside the intended threat model.
- **Self-Signed TLS & Fingerprint Verification**: In accordance with the LocalSend protocol, peers use dynamically generated self-signed TLS certificates and verify peer identity via certificate SHA-256 fingerprints rather than public CA chains.
- **Debug / Compatibility Options**: The CLI flags `--insecure` (`-k`) and plain HTTP mode explicitly disable certificate verification or transport encryption for local debugging and compatibility with legacy peers; behavior under those flags is intentional.

---

# セキュリティポリシー (日本語)

## サポート対象バージョン

最新のリリース系列（`1.1.x`）および `main` ブランチのみがセキュリティ修正・アップデートの対象です。

| バージョン | サポート状況 |
| ---------- | ------------ |
| 1.1.x      | :white_check_mark: |
| <= 1.0.x   | :x:          |

## 脆弱性の報告方法

**セキュリティ上の脆弱性を公開の GitHub Issue で報告しないでください。**

GitHub の **Private Vulnerability Reporting（非公開の脆弱性報告）** 機能を利用して報告をお願いします。

1. 本リポジトリの **Security** タブを開きます。
2. **Report a vulnerability**（または **Advisories** → **New draft security advisory**）をクリックします。
3. 問題の概要、再現手順、影響を受けるバージョン・コミット、想定される影響を記載して送信してください。

本リポジトリは個人によりベストエフォートで保守されています。初回確認の目安は 7 日以内です。

## スコープおよび設計上の前提

脆弱性の評価・報告にあたっては、以下の設計前提をご考慮ください。

- **非公式実装**: 本プロジェクトは LocalSend プロトコル（v2.1）の非公式 Go 実装です。公式の LocalSend アプリ固有の脆弱性は公式プロジェクトへご報告ください。
- **LAN 内利用の前提**: 本ツールは信頼されたローカルネットワーク（LAN）内でのファイル転送を前提としています。インターネットへ直接公開する構成は想定外です。
- **自己署名証明書とフィンガープリント検証**: LocalSend プロトコルの仕様に基づき、動的生成される自己署名 TLS 証明書と証明書の SHA-256 フィンガープリントによるピア検証を採用しています。
- **デバッグ・互換性オプション**: CLI の `--insecure`（`-k`）フラグや HTTP モードは、ローカルデバッグやレガシー環境との互換性のために意図的に証明書検証や暗号化を無効化する機能です。
