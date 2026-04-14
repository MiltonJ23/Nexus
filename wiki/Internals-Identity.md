# Internals: Identity & Auth

Packages: `internal/identity/service.go`, `internal/identity/emailer.go`, `proto/auth/`

---

## Overview

Two-factor authentication: password + time-limited OTP sent by email. On success a JWT is issued. The auth service runs as a separate gRPC server.

---

## gRPC service

Defined in `proto/auth/` (generated from `auth.proto`):

```protobuf
service AuthService {
    rpc RegisterInit   (RegisterRequest) returns (GenericResponse);
    rpc RegisterVerify (OTPRequest)      returns (TokenResponse);
    rpc LoginInit      (LoginRequest)    returns (GenericResponse);
    rpc LoginVerify    (OTPRequest)      returns (TokenResponse);
    rpc ValidateToken  (TokenRequest)    returns (ValidationResponse);
}
```

---

## Database

SQLite file at `/var/lib/nexus/auth.db`, managed by GORM with `AutoMigrate`.

Three tables:

```go
type User struct {
    gorm.Model
    Username string `gorm:"uniqueIndex"`
    Email    string `gorm:"uniqueIndex"`
    Password string  // bcrypt hash, cost 12
}

type PendingRegistration struct {
    Email     string `gorm:"primaryKey"`
    Username  string
    Password  string    // bcrypt hash (temporary)
    OTP       string
    ExpiresAt time.Time // now + 5 minutes
}

type LoginOTP struct {
    Email     string `gorm:"primaryKey"`
    OTP       string
    ExpiresAt time.Time // now + 5 minutes
}
```

---

## Registration flow

```
Client                AuthServer              DB
  │                       │                   │
  ├─ RegisterInit ────────►│                   │
  │  {username,password,  │                   │
  │   email}              │                   │
  │                       ├─ count users with ─►
  │                       │  same username/email
  │                       ◄─ 0 ───────────────┤
  │                       │                   │
  │                       │ bcrypt(password,12)│
  │                       │ GenerateOTP()      │
  │                       ├─ Save PendingReg ──►
  │                       │                   │
  │                       │ go SendOTPEmail()  │
  ◄─ "OTP sent" ──────────┤                   │
  │                       │                   │
  ├─ RegisterVerify ──────►│                   │
  │  {email, otp}         ├─ Find pending ─────►
  │                       │  WHERE email=? AND│
  │                       │  otp=?            │
  │                       ◄─ PendingReg ───────┤
  │                       │                   │
  │                       │ check ExpiresAt    │
  │                       ├─ Create User ──────►
  │                       ├─ Delete pending ───►
  │                       │                   │
  │                       │ generateJWT(username)
  ◄─ {token, username} ───┤                   │
```

`db.Save(&reg)` is an upsert: if the same email re-registers before verification, the OTP and hashed password are updated.

---

## Login flow

```
LoginInit:
    1. Find User by username.
    2. bcrypt.CompareHashAndPassword → error on mismatch.
    3. GenerateOTP(), save LoginOTP{email, otp, now+5min}.
    4. go SendOTPEmail(email, otp).
    5. Return "OTP sent to <email>".

LoginVerify:
    1. Find LoginOTP WHERE email=? AND otp=?.
    2. Check ExpiresAt.
    3. Find User by email.
    4. Delete LoginOTP row.
    5. generateJWT(username) → return token.
```

---

## JWT

Algorithm: HS256
Secret: `[]byte("apotheose-secret-key-2025")` — **hardcoded, must be replaced via environment variable before any deployment**.
Expiry: 24 hours.

Claims:
```json
{ "sub": "<username>", "exp": <unix timestamp> }
```

`ValidateToken` parses and validates the token, returns `{valid: true, username: "..."}` on success.

---

## OTP generation

`GenerateOTP()` is in `emailer.go`. Expected to return a 6-digit numeric string (implementation details in the emailer package).

---

## Security notes

| Issue | Severity | Status |
|-------|----------|--------|
| JWT secret hardcoded in source | Critical | Open — move to env var |
| bcrypt cost 12 | Acceptable | — |
| OTP 5-minute window | Standard | — |
| No rate limiting on OTP endpoints | Medium | Open |
| No OTP attempt count limit | Medium | Open |
| SQLite without WAL mode | Low | Fine for single-process |
