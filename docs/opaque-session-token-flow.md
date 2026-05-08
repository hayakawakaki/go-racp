# Opaque Session Token — Login &amp; Authentication
---

## 1. Login — token creation

```mermaid
sequenceDiagram
    autonumber
    participant B as Browser
    participant H as Handler
    participant A as Auth Service
    participant S as Session Service
    participant D as cp_sessions DB

    B->>H: POST /login (username, password)
    H->>A: Authenticate(cmd)
    A-->>H: GetDTO with ID, Username, Email
    H->>S: Create(ctx, userID)
    Note over S: rand.Read 32 bytes -> token<br>sha256(token) -> hash<br>now, now + 24h
    S->>D: INSERT (token_hash, user_id, timestamps)
    D-->>S: ok
    S-->>H: raw token + Session
    H-->>B: 303 redirect, Set-Cookie racp_session
    Note over B: Cookie value is the raw token<br>HttpOnly, SameSite=Lax, Max-Age=86400
```

The raw token only ever lives in the cookie and (briefly) in the service's local variable. The DB sees only `SHA-256(token)`.

---

## 2. Authenticated request — validate &amp; slide

```mermaid
sequenceDiagram
    autonumber
    participant B as Browser
    participant M as Middleware
    participant S as Session Service
    participant D as cp_sessions DB
    participant Hd as Page Handler

    B->>M: GET /profile with cookie
    M->>S: Validate(ctx, raw)
    Note over S: base64url-decode<br>sha256(token) -> hash
    S->>D: SELECT WHERE token_hash = hash
    alt row missing
        D-->>S: ErrNoRows
        S-->>M: ErrSessionNotFound
        M-->>B: clear cookie + 303 to /login
    else row found, expired
        D-->>S: row past expires_at
        S->>D: DELETE WHERE token_hash = hash
        S-->>M: ErrSessionExpired
        M-->>B: clear cookie + 303 to /login
    else row found, valid
        D-->>S: row active
        S->>D: UPDATE last_seen_at and expires_at
        D-->>S: ok
        S-->>M: Session
        M->>Hd: ctx + session
        Hd-->>B: 200 page
    end
```

Every authenticated request bumps `expires_at` by 24h (sliding window). Active users stay logged in; idle 24h logs them out. `WithSession` follows the same flow but lets anonymous requests through unchanged.
