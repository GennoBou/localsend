# LocalSend プロトコル仕様書 (v2.1 準拠 + Go実装拡張)

---

### 言語リンク
- **日本語**
- [English (英語)](protocol_spec.md)

---

本ドキュメントは、LocalSendプロトコル v2.1 の仕様と、本Go実装における独自拡張（複数アプリの同時動作に対応するデバイスキー管理）について定義した日本語の仕様書です。

---

## 1. 概要とデフォルト設定

LocalSendは、ローカルエリアネットワーク（LAN）内で外部サーバーに依存せず、デバイス間でファイルを安全に転送するためのREST-over-HTTPSプロトコルです。

### 1.1 通信仕様
- **HTTP / HTTPS サーバーポート**: デフォルト `53317` (TCP)
- **マルチキャストアドレス**: `224.0.0.167` (UDP)
- **マルチキャストポート**: `53317` (UDP)
- **暗号化**: 原則としてHTTPSを使用。証明書は起動時に各デバイスで自己署名証明書（RSA 2048bit等）を動的に生成（またはキャッシュからロード）します。

### 1.2 フィンガープリント (Fingerprint)
- **HTTPSモード**: デバイスが使用する自己署名証明書（DER形式）の **SHA-256ハッシュの小文字Hex表現** をフィンガープリントとします。
- **HTTPモード**: ランダムに生成された任意の文字列。
- 自分自身の検出防止、および通信相手の証明書の妥当性検証に使用されます。

### 1.3 【独自拡張】IP:Port / IP:Port:Alias によるデバイスキー管理
公式のLocalSend実装では、検出した周辺デバイスを「IPアドレス」のみをキーとしてメモリ上で管理しています。そのため、同一IP上で複数のLocalSendインスタンスが異なるポートで起動した場合、デバイス情報が競合し上書きされてしまいます。
本実装では、この問題を解決するために以下の拡張仕様を採用します。

- **デバイス管理キー**: `IP:Port` 形式の文字列。
  ```text
  192.168.1.50:53317
  192.168.1.50:53318
  ```
  これにより、同一ホスト上で異なるポートを使用して動作する複数のLocalSendアプリ（CLIや公式アプリ）を個別のデバイスとして認識し、同時通信を可能にします。

---

## 2. セキュリティと証明書検証戦略

### 2.1 ハイブリッド mTLS (クライアント証明書要求)
公式のLocalSendクライアントは、HTTPS通信時にクライアント証明書を提示しません（mTLS非対応）。しかし、本実装ではローカル環境のセキュリティ向上のため、以下のハイブリッドなクライアント検証方式をサポートします。

- **`VerifyClientCertIfGiven` モード（デフォルト）**:
  - 受信サーバーはクライアント証明書の要求を `VerifyClientCertIfGiven` に設定します。
  - 公式LocalSendクライアントなど、クライアント証明書を提示しない接続はスキップして接続を許可します。
  - クライアント証明書を提示した接続に対しては検証（フィンガープリント照合）を行い、検証成功した場合はセッション内の認証状態（`ClientVerified = true`）を保持します。
- **`StrictTLS` モード（オプショナル / CLIでは `--tls-strict`）**:
  - クライアント証明書の提示を必須とします。
  - クライアント証明書が提示されない場合、あるいは提示された証明書のハッシュが `/prepare-upload` 時の `fingerprint` と一致しない接続は、即座に TLS レベルまたは HTTP レベルで拒否（403 Forbidden）します。
  - ※ このモードが有効な場合、公式アプリ（mTLS非対応）からのアップロードは遮断されます。

### 2.2 接続元の検証およびデータ保護
mTLSの提示が無い接続（`VerifyClientCertIfGiven` で証明書なしの場合など）において、サーバー側は以下の多層的なセキュリティチェックによって送信クライアントの妥当性を検証します。

1. **セッションIDとトークンのバインド**:
   - `POST /api/localsend/v2/prepare-upload` 時にサーバーが暗号論的に安全な乱数（`crypto/rand` を使用）を用いて `sessionId` および各ファイル用の `token` を生成し、クライアントに返します。
   - クライアントが実際にファイルを送信する `POST /api/localsend/v2/upload` 時、リクエストパラメータに含まれる `sessionId` と `token` がサーバー側の保持するセッションと一致する場合のみ書き込みを許可します。
2. **接続元IPの一致確認**:
   - `/prepare-upload` 時の送信元クライアントのIPアドレスと、実際の `/upload` 時の接続元IPアドレスが一致することを検証し、セッションハイジャックを防止します。
3. **TOFU (Trust-On-First-Use) モデル**:
   - クライアントおよびサーバーは、初回通信時に相手の `fingerprint`（証明書ハッシュ）をメモリまたは永続キャッシュに記録します。
   - 同一の `IP:Port` または `alias` に対して、証明書のフィンガープリントが途中で変更された場合、中間者攻撃（MITM）の可能性があるとして警告または接続拒否を行います。

---

## 3. デバイス検出 (Discovery)

### 3.1 UDPマルチキャストによる検出
アプリの起動時または更新時に、以下のUDPパケットをマルチキャストアドレス（`224.0.0.167:53317`）に向けて送信します。

**アナウンスJSON (送信データ)**
```json
{
  "alias": "Nice Orange",
  "version": "2.0",
  "deviceModel": "Windows",
  "deviceType": "desktop",
  "fingerprint": "a3b2c1...",
  "port": 53317,
  "protocol": "https",
  "download": true,
  "announce": true
}
```
※ `announce` が `true` の場合、これを受信した他デバイスは自動的に以下の登録API (`POST /api/localsend/v2/register`) を用いて、自身のデバイス情報を送信元IPに対して返信します。

### 3.2 登録API (POST `/api/localsend/v2/register`)
マルチキャストの応答、またはマルチキャストが届かない場合のレガシーユニキャストスキャン時に使用します。

**リクエストボディ (JSON)**
```json
{
  "alias": "Secret Banana",
  "version": "2.0",
  "deviceModel": "Samsung",
  "deviceType": "mobile",
  "fingerprint": "f4e5d6...",
  "port": 53317,
  "protocol": "https",
  "download": true
}
```

**レスポンスボディ (JSON)**
ユニキャストスキャン時などの双方向ハンドシェイクのために、受信側も自身のデバイス情報を返します。
```json
{
  "alias": "Nice Orange",
  "version": "2.0",
  "deviceModel": "Windows",
  "deviceType": "desktop",
  "fingerprint": "a3b2c1...",
  "download": true
}
```

---

## 4. ファイルアップロード転送 (Upload API)

送信側（HTTPクライアント）から受信側（HTTPサーバー）へファイルを送るデフォルトのフローです。

### 4.1 送信準備 (POST `/api/localsend/v2/prepare-upload`)
送信側はファイル送信を開始する前に、送信したいファイルのメタデータ一覧を受信側に送信し、承認を求めます。

- クエリパラメータ: `?pin=xxxxxx` (受信側でPINが要求されている場合)
- リクエストボディ (JSON):
```json
{
  "info": {
    "alias": "Nice Orange",
    "version": "2.0",
    "deviceModel": "Windows",
    "deviceType": "desktop",
    "fingerprint": "a3b2c1...",
    "port": 53317,
    "protocol": "https",
    "download": true
  },
  "files": {
    "file_uuid_1": {
      "id": "file_uuid_1",
      "fileName": "document.pdf",
      "size": 1048576,
      "fileType": "application/pdf",
      "sha256": "abcdef...",
      "preview": "...",
      "metadata": {
        "modified": "2026-06-24T16:00:00Z",
        "accessed": "2026-06-24T16:00:00Z"
      }
    }
  }
}
```

- レスポンスボディ (JSON) - 承認された場合:
```json
{
  "sessionId": "upload_session_uuid",
  "files": {
    "file_uuid_1": "file_transfer_token_1"
  }
}
```

- エラーレスポンス:
  - `204`: すでに同ファイルが存在し、転送が不要な場合（送信側はアップロード処理をスキップする）。
  - `401`: PINが必要、またはPINが一致しない。
  - `403`: 受信側ユーザーにより拒否された。
  - `409`: 他のセッションによる転送が実行中のためブロック。

### 4.2 ファイル本体の転送 (POST `/api/localsend/v2/upload`)
準備段階で取得した `sessionId`、`fileId`、およびファイルごとの `token` を指定し、バイナリデータを送信します。

- パスパラメータ: `POST /api/localsend/v2/upload?sessionId=upload_session_uuid&fileId=file_uuid_1&token=file_transfer_token_1`
- リクエストボディ: 送信対象ファイルの生のバイナリデータ。
- レスポンス: HTTPステータス `200 OK` (ボディなし)。

### 4.3 セッションキャンセル (POST `/api/localsend/v2/cancel`)
送信側または受信側が転送を途中でキャンセルする場合に呼び出します。

- パスパラメータ: `POST /api/localsend/v2/cancel?sessionId=upload_session_uuid`
- レスポンス: HTTPステータス `200 OK` (ボディなし)。

---

## 5. リバースファイル転送 (Download API)

LocalSendがインストールされていないデバイス（ブラウザ等）に対してファイルを送信する機能です。
送信側が一時的にHTTPサーバー（自己署名HTTPSはブラウザの警告が出るため、**HTTPのみ**を使用）を起動し、受信側がブラウザでアクセスしてダウンロードします。

### 5.1 ブラウザ向けURL
受信側は以下のURLにアクセスしてWebUIを開き、ファイルをダウンロードします。
```text
http://<送信側のIP>:<送信側のポート>
```

### 5.2 受信準備 (POST `/api/localsend/v2/prepare-download`)
ブラウザが送信側サーバーから送信されるファイルメタデータの一覧を取得します。

- リクエスト: なし
- レスポンスボディ (JSON):
```json
{
  "info": {
    "alias": "Nice Orange",
    "version": "2.0",
    "deviceModel": "Windows",
    "deviceType": "desktop",
    "fingerprint": "a3b2c1...",
    "download": true
  },
  "sessionId": "download_session_uuid",
  "files": {
    "file_uuid_1": {
      "id": "file_uuid_1",
      "fileName": "photo.jpg",
      "size": 204857,
      "fileType": "image/jpeg",
      "sha256": "xyz...",
      "preview": "..."
    }
  }
}
```

### 5.3 ファイル本体のダウンロード (GET `/api/localsend/v2/download`)
- パスパラメータ: `GET /api/localsend/v2/download?sessionId=download_session_uuid&fileId=file_uuid_1`
- レスポンス: ファイルのバイナリデータ。

---

## 6. その他のAPI (Info API)

### 6.1 デバッグ・状態確認 (GET `/api/localsend/v2/info`)
- レスポンスボディ (JSON):
```json
{
  "alias": "Nice Orange",
  "version": "2.0",
  "deviceModel": "Windows",
  "deviceType": "desktop",
  "fingerprint": "a3b2c1...",
  "download": true
}
```
