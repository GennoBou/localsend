# LocalSend Protocol Specification (v2.2 Compliant + Go Implementation Extension)

---

### Language Links
- [日本語 (Japanese)](protocol_spec_ja.md)
- **English**

---

This document defines the LocalSend protocol v2.2 specification and the custom extension adopted in this Go implementation to support multiple app instances running on a single host.

---

## 1. Overview & Defaults

LocalSend is a serverless REST-over-HTTPS protocol designed for securely transferring files between devices in a local area network (LAN).

### 1.1 Technical Specs
- **HTTP / HTTPS Port**: Default `53317` (TCP)
- **Multicast Group Address**: `224.0.0.167` (UDP)
- **Multicast Port**: `53317` (UDP)
- **Encryption**: HTTPS is used by default. TLS certificates (RSA 2048-bit) are dynamically generated on the fly (or loaded from cache) when the application starts.

### 1.2 Fingerprint
- **HTTPS Mode**: The fingerprint is the **lowercase hexadecimal SHA-256 hash** of the TLS certificate (DER format).
- **HTTP Mode**: A randomly generated string.
- The fingerprint prevents self-discovery and is used for certificate pinning and validation.

### 1.3 [Extension] IP:Port / IP:Port:Alias Device Key Management
The official LocalSend implementation manages discovered devices in memory using only the "IP Address" as the key. This causes a conflict if multiple LocalSend instances run on different ports on the same host (same IP address), as the last discovered device overrides the previous ones.
To address this limitation, this implementation extends the device key design:

- **Device Key Format**: `IP:Port`
  ```text
  192.168.1.50:53317
  192.168.1.50:53318
  ```
  This allows the system to recognize and communicate with multiple LocalSend instances (such as a CLI tool and the official app) running concurrently on the same machine.

---

## 2. Security and Verification Strategy

### 2.1 Hybrid mTLS (Client Certificate Request)
Standard LocalSend clients do not present a client certificate (mTLS is not used). To enhance security in a pure Go-to-Go network while maintaining compatibility with the official app, this implementation supports a hybrid client certificate verification strategy:

- **`VerifyClientCertIfGiven` Mode (Default)**:
  - The receiving server configures TLS client authentication as `VerifyClientCertIfGiven`.
  - Connections from clients that do not present a certificate (such as official LocalSend apps) bypass verification and are accepted.
  - If a client presents a certificate, the server validates its SHA-256 fingerprint against the one provided during `/prepare-upload`. The result is marked in the session as `ClientVerified = true`.
- **`StrictTLS` Mode (Optional / CLI `--tls-strict`)**:
  - Client certificate verification is strictly enforced.
  - Connections that do not present a client certificate or fail the fingerprint validation are blocked at the TLS or HTTP level (returning `403 Forbidden`).
  - *Note: Enabling this mode disables compatibility with official LocalSend apps (which do not present client certificates).*

### 2.2 Peer Verification and Session Security
For connections without client certificates (e.g., standard official clients under `VerifyClientCertIfGiven` mode), security is ensured through:

1. **Session ID and Token Binding**:
   - During `POST /api/localsend/v2/prepare-upload`, the server generates a cryptographically secure random `sessionId` (using `crypto/rand`) and unique file `tokens`, returning them to the sender.
   - For every file chunk uploaded via `POST /api/localsend/v2/upload`, the sender must supply the correct `sessionId` and file `token`.
2. **Client IP Validation**:
   - The server verifies that the client's source IP address during `/upload` matches the IP address from which `/prepare-upload` was received, protecting against session hijacking.
3. **TOFU (Trust-On-First-Use) Model**:
   - Clients and servers store the peer's `fingerprint` (certificate hash) in memory or cache upon the first handshake.
   - If the fingerprint associated with a specific `IP:Port` or `alias` changes unexpectedly, the app raises a warning or blocks the transfer to mitigate Man-in-the-Middle (MITM) attacks.

---

## 3. Discovery

This Go implementation supports three discovery modes (`--discovery-mode`) to control how devices are advertised and searched on the network:

*   **`hybrid` (Default)**: Runs UDP multicast, mDNS, and HTTP legacy scan concurrently to maximize connectivity.
*   **`multicast`**: Conforms to the standard LocalSend behavior by running UDP multicast and HTTP legacy scan only.
*   **`mdns`**: Runs mDNS/DNS-SD (RFC 6762/6763) only. To avoid unnecessary network scans, HTTP legacy scan is disabled in this mode.

### 3.1 UDP Multicast Discovery
(Active in `hybrid` or `multicast` mode)
Upon startup or refresh, the application broadcasts a UDP announcement packet to the multicast group (`224.0.0.167:53317`).

**Announcement JSON (Request)**
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
*Note: If `announce` is `true`, receiving devices must reply to the sender's IP and Port using the Register API (`POST /api/localsend/v2/register`).*

### 3.2 mDNS (Multicast DNS) / DNS-SD Discovery
(Active in `hybrid` or `mdns` mode)
Uses standard UDP port `5353` and the service type `_localsend._tcp` in the `local` domain to register/browse services.
Device properties are mapped to DNS-SD **TXT records**:

- **TXT Record Fields**:
  - `alias=<Alias>`
  - `version=<ProtocolVersion>` (typically `"2.0"`)
  - `deviceModel=<DeviceModel>`
  - `deviceType=<DeviceType>`
  - `fingerprint=<Fingerprint>`
  - `protocol=<Protocol>`
  - `download=<download (true/false)>`

### 3.3 Register API (POST `/api/localsend/v2/register`)
Used for responding to multicast announcements, and for legacy unicast scanning (active in `hybrid` or `multicast` mode) when multicast is unavailable.

**Request Body (JSON)**
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

**Response Body (JSON)**
To complete the two-way handshake during a unicast scan, the receiver returns its own device information.
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

## 4. File Transfer (Upload API)

This is the default flow where the sender (HTTP client) uploads files to the receiver (HTTP server).

### 4.1 Preparation (POST `/api/localsend/v2/prepare-upload`)
The sender submits file metadata to request transfer approval from the receiver.

- **Query Parameter**: `?pin=xxxxxx` (required if a PIN code is configured on the receiver)
- **Request Body (JSON)**:
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

- **Response Body (JSON)** (upon acceptance):
```json
{
  "sessionId": "upload_session_uuid",
  "files": {
    "file_uuid_1": "file_transfer_token_1"
  }
}
```

- **Errors**:
  - `204`: Finished / Skip (file already exists on the receiver, no upload needed).
  - `401`: PIN required / Invalid PIN.
  - `403`: Request rejected by the receiver.
  - `409`: Transfer blocked by another active session.

### 4.2 Send File (POST `/api/localsend/v2/upload`)
The sender uploads the binary data of a file using the `sessionId`, `fileId`, and the file-specific `token` obtained from `/prepare-upload`.

- **Endpoint**: `POST /api/localsend/v2/upload?sessionId=upload_session_uuid&fileId=file_uuid_1&token=file_transfer_token_1`
- **Request Body**: Binary file contents.
- **Response**: HTTP `200 OK` (empty body).
- **Integrity Verification (v2.2)**: If `sha256` of the file was provided in `/prepare-upload`, the receiver calculates the SHA-256 hash of the received binary data and responds with `422 Unprocessable Entity` on mismatch.
- **Errors**:
  - `400`: Missing parameters.
  - `403`: Invalid token or IP address mismatch.
  - `409`: Blocked by another active session.
  - `422`: Checksum mismatch (`sha256`).
  - `500`: Internal error by receiver.

### 4.3 Cancel (POST `/api/localsend/v2/cancel`)
Initiated when either side wants to abort the active transfer session.

- **Endpoint**: `POST /api/localsend/v2/cancel?sessionId=upload_session_uuid`
- **Response**: HTTP `200 OK` (empty body).

---

## 5. Reverse File Transfer (Download API)

Used when sending files to devices without the LocalSend app installed (e.g., standard web browsers).
The sender hosts a temporary HTTP server (**HTTP only**; self-signed HTTPS is rejected by browsers), and the receiver downloads files via a browser.

### 5.1 Web Browser URL
The receiver downloads the files by visiting this URL:
```text
http://<sender-ip>:<sender-port>
```

### 5.2 Receive Request (POST `/api/localsend/v2/prepare-download`)
The browser fetches the list of available file metadata from the sender.

- **Request**: Empty body
- **Response Body (JSON)**:
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

### 5.3 Receive File (GET `/api/localsend/v2/download`)
- **Endpoint**: `GET /api/localsend/v2/download?sessionId=download_session_uuid&fileId=file_uuid_1`
- **Response**: Binary file contents.

---

## 6. Other APIs

### 6.1 Info (GET `/api/localsend/v2/info`)
- **Response Body (JSON)**:
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
