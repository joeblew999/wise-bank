#!/usr/bin/env nu
# Register the "wise" API with restish so every Wise endpoint becomes a CLI command.
# Run via: mise run api:setup  (idempotent; merges into any existing restish config).
#
# restish reads a per-OS user config dir (macOS: ~/Library/Application Support/restish,
# else ~/.config/restish). We point it at the NORMALIZED 3.0 spec (restish's loader,
# like kin-openapi, rejects the raw 3.1) and inject the token from $WISE_API_TOKEN at
# call time (tasks run under `fnox exec`, so the env var is set; restish expands ${...}).

let cfgdir = (if $nu.os-info.name == "macos" {
  [$nu.home-dir "Library/Application Support/restish"] | path join
} else {
  [$nu.home-dir ".config/restish"] | path join
})
mkdir $cfgdir
let cfgfile = ($cfgdir | path join "apis.json")
let spec = ($env.PWD | path join "docs/reference/wise-openapi-3.0.yaml")

let existing = (if ($cfgfile | path exists) { open $cfgfile } else { {"$schema": "https://rest.sh/schemas/apis.json"} })
let wise = {
  base: "https://api.wise.com"
  spec_files: [$spec]
  profiles: { default: { headers: { authorization: "Bearer ${WISE_API_TOKEN}" } } }
}
$existing | upsert wise $wise | save -f $cfgfile

print $"restish 'wise' API registered -> ($cfgfile)"
print $"  spec    : ($spec)"
print $"  base    : https://api.wise.com  \(sandbox: pass --rsh-server https://api.sandbox.transferwise.tech\)"
print "Use it:  mise run api -- --help        # list all ~205 commands"
print "         mise run api -- rate-get --source=USD --target=EUR"
