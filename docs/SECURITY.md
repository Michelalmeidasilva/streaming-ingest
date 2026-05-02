# Security

This document describes the security measures in place for `streaming-ingest`.

## SAST (Static Application Security Testing)

### Semgrep
We use [Semgrep](https://semgrep.dev/) to automatically detect security vulnerabilities, injection attacks, XSS, secrets, and other OWASP Top 10 issues.

#### Configuration

**Workflows:**
- `.github/workflows/semgrep.yml` — Dedicated security scanning workflow
- `.github/workflows/ci.yml` — `security-scan` job runs Semgrep on every push and PR

**GitHub Security tab:**
- SARIF upload requires Code scanning to be enabled in the repository settings.
- If Code scanning is disabled, the workflow still runs Semgrep and the upload step is treated as best-effort.

**Rule Sets:**
- `p/security-audit` — General security best practices
- `p/go` — Go-specific security patterns
- `p/owasp-top-ten` — OWASP Top 10 vulnerabilities
- `p/cwe-top-25` — CWE Top 25 Most Dangerous Software Weaknesses
- `p/injection` — SQL, command, LDAP injection patterns
- `p/xss` — Cross-Site Scripting detection
- `p/secrets` — Hardcoded credentials, API keys, tokens

**Custom Rules:**
See `.semgrep.yml` for custom rules targeting:
- Hardcoded credentials (password, secret, apiKey, token)
- SQL injection (string concatenation in queries)
- Command injection (exec with user input)
- Insecure random (math/rand instead of crypto/rand)
- TLS verification disabled
- Debug logging with sensitive data
- Path traversal vulnerabilities
- Unsafe reflection / type assertions
- XXE vulnerabilities (XML deserialization)
- Insecure deserialization

#### Running Semgrep Locally

**Install Semgrep:**
```bash
pip install semgrep
# or using homebrew:
brew install semgrep
```

**Run full security scan:**
```bash
semgrep \
  --config=p/security-audit \
  --config=p/go \
  --config=p/owasp-top-ten \
  --config=p/cwe-top-25 \
  --config=p/injection \
  --config=p/xss \
  --config=p/secrets \
  .
```

**Run with custom rules:**
```bash
semgrep --config=.semgrep.yml .
```

**Run specific rule:**
```bash
semgrep --config=p/secrets . # Find hardcoded secrets
semgrep --config=p/injection . # Find injection vulnerabilities
```

**Generate JSON report:**
```bash
semgrep --config=p/security-audit --json --output=semgrep-report.json .
```

#### CI/CD Integration

Semgrep runs automatically on:
- Every push to `main` or `fix/**` branches
- Every pull request to `main`

**Failure Conditions:**
The pipeline **fails** if Semgrep detects:
- SQL injection
- Command injection
- TLS verification disabled
- Secrets in code
- OWASP Top 10 violations

## Code Coverage

SonarCloud monitors:
- Code coverage (target: 80% for internal/)
- Code duplication (< 3%)
- Maintainability ratings (A)
- Security ratings (A)
- Reliability ratings (A)

## Secrets Management

**Never commit:**
- Database credentials
- API keys / tokens
- Private keys (SSH, TLS)
- AWS/GCP/Azure credentials
- RabbitMQ passwords

**Use instead:**
- GitHub Secrets for CI/CD
- `.env` files (add to .gitignore)
- Secret management services (HashiCorp Vault, AWS Secrets Manager)

## Dependency Security

Go dependencies are locked in `go.sum`. Regular updates are recommended:
```bash
go get -u ./...
go mod tidy
```

Monitor for security advisories:
```bash
go list -json -m all | nancy sleuth
```

## Authentication & Authorization

- No hardcoded credentials
- Environment variables for sensitive config
- JWT tokens validated on every request
- RabbitMQ credentials managed per environment

## Network Security

- TLS verification enabled for all external connections
- RabbitMQ uses AMQP with authentication
- MongoDB connections use credentials
- MinIO/S3 access via IAM/credentials

## Reporting Security Issues

If you discover a security vulnerability:

1. **Do not** open a public GitHub issue
2. Email: michelalmeida.dev@gmail.com with:
   - Description of the vulnerability
   - Steps to reproduce
   - Proposed fix (if you have one)

We take all security reports seriously and will respond within 24 hours.
