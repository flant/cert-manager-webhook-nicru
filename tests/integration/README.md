# Integration Tests

## Overview

Integration tests verify real interaction between cert-manager-webhook-nicru and NIC.RU API using actual credentials.

**Location:** `tests/integration/`
**Build Tag:** `integration` (prevents running with regular unit tests)
**Test Count:** 12 tests (7 token operations + 5 DNS operations)

## Test Coverage

### Token Operations (7 tests - Always Run)

1. `TestIntegration_01_RequestTokensSuccess` - Obtain OAuth tokens via password grant
2. `TestIntegration_02_RequestTokensInvalidCredentials` - Handle invalid credentials
3. `TestIntegration_03_ValidateAccessTokenSuccess` - Validate working access token
4. `TestIntegration_04_ValidateAccessTokenInvalid` - Handle invalid access token
5. `TestIntegration_05_RefreshAccessTokenSuccess` - Refresh access token
6. `TestIntegration_06_RefreshAccessTokenInvalid` - Handle invalid refresh token
7. `TestIntegration_07_TokenManagerFullCycle` - Full TokenManager lifecycle (skip - requires refactoring)

### DNS Operations (5 tests - Skip if No Domains)

1. `TestIntegration_DNS_01_GetUserZones` - List user's DNS zones
2. `TestIntegration_DNS_02_GetZoneRecords` - Get zone records
3. `TestIntegration_DNS_03_CreateTXTRecord` - Create test TXT record
4. `TestIntegration_DNS_04_CommitZone` - Commit zone changes
5. `TestIntegration_DNS_05_DeleteTXTRecord` - Delete test record (cleanup)

## Prerequisites

### 1. NIC.RU Account
- Active NIC.RU account
- OAuth 2.0 application registered at https://www.nic.ru/manager/oauth.cgi

### 2. Required Credentials
Environment variables:
- `NICRU_USERNAME` - Your NIC.RU username
- `NICRU_PASSWORD` - Your NIC.RU password
- `NICRU_CLIENT_ID` - OAuth app client ID
- `NICRU_CLIENT_SECRET` - OAuth app client secret

### 3. Required for DNS Tests: Test Zone
For DNS tests (#8-12), you MUST specify a test zone:
- `NICRU_TEST_ZONE` - DNS zone name (e.g., "test.example.com")

**⚠️ IMPORTANT:**
- Tests will CREATE and DELETE TXT records in this zone
- Use a dedicated TEST zone, NOT production!
- The zone must exist in your NIC.RU account
- DNS tests will FAIL if zone not specified or not found

## Setup

### Step 1: Copy credentials file
```bash
cd /path/to/cert-manager-webhook-nicru
cp test-credentials.example.env test-credentials.env
```

### Step 2: Edit with real credentials
```bash
vim test-credentials.env
```

Fill in your actual credentials:
```bash
export NICRU_USERNAME="your-actual-username"
export NICRU_PASSWORD="your-actual-password"
export NICRU_CLIENT_ID="your-oauth-client-id"
export NICRU_CLIENT_SECRET="your-oauth-client-secret"
export NICRU_TEST_ZONE="test.example.com"  # REQUIRED for DNS tests
```

**⚠️ Security Reminder:**
- Use a DEDICATED test zone
- Never use production domains
- Tests create/delete TXT records with prefix `_acme-challenge-integration-test-*`

### Step 3: Load credentials
```bash
source test-credentials.env
```

Verify:
```bash
echo $NICRU_USERNAME  # Should print your username
```

## Running Tests

### All integration tests
```bash
go test -v -tags=integration ./tests/integration/
```

### Token tests only (tests 01-07)
```bash
go test -v -tags=integration -run TestIntegration_0 ./tests/integration/
```

### DNS tests only (tests DNS_01-05)
```bash
go test -v -tags=integration -run TestIntegration_DNS ./tests/integration/
```

### Specific test
```bash
go test -v -tags=integration -run TestIntegration_01_RequestTokensSuccess ./tests/integration/
```

### With timeout (DNS can be slow)
```bash
go test -v -tags=integration -timeout 120s ./tests/integration/
```

### Save output to file
```bash
go test -v -tags=integration ./tests/integration/ 2>&1 | tee integration-test.log
```

## Expected Results

### With credentials, no domains:
```
=== RUN   TestIntegration_01_RequestTokensSuccess
✅ SUCCESS: Obtained tokens: access=abcd...xyz9, refresh=wxyz...1234
--- PASS: TestIntegration_01_RequestTokensSuccess (0.52s)
...
=== RUN   TestIntegration_DNS_01_GetUserZones
⚠️  WARNING: No DNS zones found - DNS tests will be skipped
--- PASS: TestIntegration_DNS_01_GetUserZones (0.28s)
=== RUN   TestIntegration_DNS_02_GetZoneRecords
--- SKIP: TestIntegration_DNS_02_GetZoneRecords (0.00s)
...
PASS
ok      tests/integration       5.234s
```

### With credentials AND domains:
```
=== RUN   TestIntegration_01_RequestTokensSuccess
--- PASS: TestIntegration_01_RequestTokensSuccess (0.48s)
...
=== RUN   TestIntegration_DNS_03_CreateTXTRecord
✅ SUCCESS: Created TXT record: _acme-challenge-test-1234567890 (ID: 12345)
--- PASS: TestIntegration_DNS_03_CreateTXTRecord (0.87s)
...
PASS
ok      tests/integration       12.456s
```

## Troubleshooting

### Tests immediately skip
**Problem:** All tests show SKIP
**Solution:** Credentials not loaded. Run `source test-credentials.env`

### HTTP 401 Unauthorized
**Problem:** Invalid credentials
**Solution:** Verify credentials in test-credentials.env are correct

### Timeout errors
**Problem:** NIC.RU API slow or unreachable
**Solution:** Increase timeout: `-timeout 180s`

### NICRU_TEST_ZONE not set
**Problem:** DNS tests fail with "NICRU_TEST_ZONE environment variable is required"
**Solution:** Set test zone in test-credentials.env:
```bash
export NICRU_TEST_ZONE="your-test-zone.com"
source test-credentials.env
```

### Test zone not found
**Problem:** "Test zone 'xxx' not found in your NIC.RU account"
**Solution:**
1. Check available zones: Error message lists them
2. Verify zone name matches exactly (case-sensitive)
3. Ensure zone exists in NIC.RU manager
4. Update NICRU_TEST_ZONE with correct zone name

### Zone has no service
**Problem:** "Test zone found but has no service assigned"
**Solution:** Zone exists but not properly configured in NIC.RU. Contact support or use different zone.

### Build errors
**Problem:** Cannot find package
**Solution:** Ensure you're using `-tags=integration` flag

## Security

### ⚠️ IMPORTANT
- **NEVER** commit `test-credentials.env` to git
- File is in `.gitignore` - verify with `git status`
- Only commit `test-credentials.example.env`

### Best Practices
- Use dedicated test account if possible
- Use separate OAuth app for testing
- Rotate credentials periodically
- Don't share credentials in logs/screenshots

## Cleanup

Integration tests automatically clean up:
- Test TXT records are deleted after creation (Test #5)
- Uses `t.Cleanup()` to ensure cleanup even if test fails
- No persistent changes to your NIC.RU account
- Safe to run multiple times

## Test Flow

### Token Tests Flow
```
Test 01: Request tokens (password grant)
  ↓
  Saves: sharedTokens
  ↓
Test 03: Validate access token (uses sharedTokens)
  ↓
Test 05: Refresh token (uses sharedTokens.refresh_token)
  ↓
  Updates: sharedTokens.access_token
```

### DNS Tests Flow
```
Test 08: Get user zones
  ↓
  Reads: NICRU_TEST_ZONE (REQUIRED)
  ↓
  If NICRU_TEST_ZONE not set → FAIL
  If specified zone not found → FAIL with available zones list
  ↓
  Saves: sharedZones, testServiceName, testZoneName
  ↓
Test 09: Get zone records (uses testZoneName)
  ↓
Test 10: Create TXT record (uses testZoneName)
  ↓
  Saves: testRecordID
  ↓
Test 11: Commit zone (uses testZoneName)
  ↓
Test 12: Delete record cleanup (uses testZoneName, testRecordID)
  ↓
  Clears: testRecordID
```

## CI/CD Integration

### Not Recommended
These tests require real credentials and make real API calls.  
**NOT suitable** for automated CI/CD pipelines.

### Use Cases
- Manual testing before releases
- Local development verification
- Troubleshooting production issues

## See Also

- Main testing guide: `../../docs/guides/TESTING.md`
- Unit tests: `../../token_manager_test.go`, `../../bootstrap_test.go`
- OAuth setup: `../../docs/features/AUTOMATED_TOKENS.md`
- Example credentials: `../../test-credentials.example.env`

## Files

- `helpers.go` (128 lines) - Shared utilities and helpers
- `integration_test.go` (344 lines) - 7 OAuth token tests
- `dns_operations_test.go` (356 lines) - 5 DNS operation tests
- `README.md` (this file) - Documentation

**Total:** 828 lines of integration test code
