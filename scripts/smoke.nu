#!/usr/bin/env nu
# Smoke-test the wise-bank setup.
#   mise run smoke         -> structural only: tools, spec, restish config.
#                             NO keychain reads, NO network, NO OS prompts. Fast.
#   mise run smoke:auth    -> ALSO does live prod+sandbox auth (rate-get). This READS
#                             the keychain, so macOS may prompt for access — opt-in only.

def main [--auth] {
  mut fails = 0
  def chk [ok: bool, label: string] { print $"  (if $ok { '✓' } else { '✗' })  ($label)"; $ok }

  print "wise-bank smoke test"
  print "===================="

  print ""
  print "[tools]"
  for t in [go node nu restish fnox] {
    if not (chk ((which $t | length) > 0) $t) { $fails = $fails + 1 }
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

  if $auth {
    print ""
    print "[live auth]  (reads keychain — macOS may prompt; click Always Allow)"
    def authok [profile: string, api: string] {
      let r = (^fnox exec --profile $profile --no-defaults -- restish $api rate-get --source=USD --target=EUR -o json | complete)
      ($r.stdout | str contains '"rate"')
    }
    if not (chk (authok "wise" "wise") "production token  (mise run api)") { $fails = $fails + 1 }
    if not (chk (authok "sandbox" "wise-sandbox") "sandbox token     (mise run api:sandbox)") { $fails = $fails + 1 }
  } else {
    print ""
    print "  (skipping live auth — run `mise run smoke:auth` to verify tokens; it reads the keychain)"
  }

  print ""
  if $fails == 0 {
    print "✓ all checks passed"
  } else {
    print $"✗ ($fails) check\(s\) failed"
    exit 1
  }
}
