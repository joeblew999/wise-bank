#!/usr/bin/env nu
# Smoke-test the wise-bank setup end to end. Non-destructive: no writes to Wise, no
# keychain mutation. Exits non-zero if anything fails (works in CI too).
# Run: mise run smoke
# NOTE: the first run reads keychain items and may pop a macOS "allow access" dialog —
# click Always Allow once, then re-run.

mut fails = 0
def chk [ok: bool, label: string] { print $"  (if $ok { '✓' } else { '✗' })  ($label)"; $ok }

print "wise-bank smoke test"
print "===================="

print ""
print "[tools]"
for t in [go node nu restish fnox] {
  if not (chk ((which $t | length) > 0) $"($t)") { $fails = $fails + 1 }
}

print ""
print "[spec]"
for f in ["docs/reference/wise-openapi.yaml" "docs/reference/wise-openapi-3.0.yaml"] {
  if not (chk ($f | path exists) $f) { $fails = $fails + 1 }
}

print ""
print "[restish config]"
let cfgfile = (if $nu.os-info.name == "macos" {
  [$nu.home-dir "Library/Application Support/restish/apis.json"] | path join
} else {
  [$nu.home-dir ".config/restish/apis.json"] | path join
})
let cols = (if ($cfgfile | path exists) { open $cfgfile | columns } else { [] })
for a in [wise wise-sandbox] {
  if not (chk ($a in $cols) $"restish API '($a)' registered") { $fails = $fails + 1 }
}

print ""
print "[live auth]  (rate-get must return a rate; reads keychain — may prompt once)"
def auth [profile: string, api: string] {
  let r = (^fnox exec --profile $profile --no-defaults -- restish $api rate-get --source=USD --target=EUR -o json | complete)
  ($r.stdout | str contains '"rate"')
}
if not (chk (auth "wise" "wise") "production token  (mise run api)") { $fails = $fails + 1 }
if not (chk (auth "sandbox" "wise-sandbox") "sandbox token     (mise run api:sandbox)") { $fails = $fails + 1 }

print ""
if $fails == 0 {
  print "✓ all checks passed — setup is healthy"
} else {
  print $"✗ ($fails) check\(s\) failed — fix with: mise run secrets:status / secrets:set / secrets:set-sandbox / api:setup"
  exit 1
}
