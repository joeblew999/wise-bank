#!/usr/bin/env nu
# Canonical map of every Wise URL this project needs — which page gives which thing.
# This is the source of truth: if a task needs you to fetch something from Wise, the
# URL lives here. Run: mise run secrets:urls

print "Wise URLs — what to get, and where"
print "=================================="
print ""
print "PRODUCTION  (real money — reads only via this toolkit)"
print "  API token   : https://wise.com/your-account/integrations-and-tools/api-tokens"
print "  Public keys : https://wise.com/your-account/integrations-and-tools/api-tokens/public-keys"
print "                (PSD2: signing/SCA REMOVED on personal accounts — no statements/funding via API)"
print ""
print "SANDBOX  (fake money — a SEPARATE account, NOT your real login)"
print "  Register    : https://sandbox.transferwise.tech/register"
print "  API token   : https://sandbox.transferwise.tech/your-account/integrations-and-tools/api-tokens"
print "  Public keys : https://sandbox.transferwise.tech/your-account/integrations-and-tools/api-tokens/public-keys"
print ""
print "OAUTH  (optional — partner / multi-user only)"
print "  User tokens : https://docs.wise.com/api-reference/user-tokens"
print "  Auth guide  : https://docs.wise.com/guides/developer/auth-and-security"
print ""
print "DOCS / SPEC"
print "  API reference : https://docs.wise.com/api-reference"
print "  OpenAPI bundle: https://docs.wise.com/_bundle/api-reference/index.yaml  (vendored; mise run spec:fetch)"
print ""
print "Store what you copy:  secrets:set (prod) · secrets:set-sandbox · secrets:set-oauth · secrets:status"
