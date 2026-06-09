#!/usr/bin/env nu
# Show which Wise secrets are present in the keychain — WITHOUT revealing values.
# Presence is tested via `fnox get` exit code; stdout (the value) is captured and
# discarded, never printed. Run via: mise run secrets:status

def check [label: string, key: string, profile: string] {
  let r = (do -i { ^fnox get $key --profile $profile --no-defaults } | complete)
  let mark = (if $r.exit_code == 0 { "✓ set    " } else { "✗ missing" })
  $"  ($mark)  ($label)"
}

print "Wise credentials (values stay in the OS keychain):"
print ""
print "REQUIRED — production:"
print (check "WISE_API_TOKEN            (mise run secrets:set)"          "WISE_API_TOKEN" "wise")
print ""
print "SANDBOX — for write/SCA testing:"
print (check "WISE_SANDBOX_API_TOKEN    (mise run secrets:set-sandbox)"  "WISE_API_TOKEN" "sandbox")
print ""
print "OPTIONAL — OAuth (partner / multi-user only):"
print (check "WISE_CLIENT_ID            (mise run secrets:set-oauth)"    "WISE_CLIENT_ID" "oauth")
print (check "WISE_CLIENT_SECRET        (mise run secrets:set-oauth)"    "WISE_CLIENT_SECRET" "oauth")
