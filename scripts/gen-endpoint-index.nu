#!/usr/bin/env nu
# Regenerate docs/reference/wise-endpoints.txt from the local OpenAPI spec.
# Run via: mise run wise:spec   (which also re-downloads the spec first)

let spec = (open docs/reference/wise-openapi.yaml)
let methods = [get post put delete patch]

let rows = ($spec.paths | transpose path ops | each {|r|
  $r.ops | columns | where {|c| $c in $methods} | each {|m|
    let s = (($r.ops | get $m).summary? | default "")
    $"(($m | str upcase) | fill -a left -w 6) ($r.path) — ($s)"
  }
} | flatten | sort)

let header = $"# Wise API endpoint index — ($rows | length) operations across ($spec.paths | columns | length) paths(char nl)# Source: docs/reference/wise-openapi.yaml \(official Wise bundle\). Regenerate: mise run wise:spec(char nl)(char nl)"
$"($header)($rows | str join (char nl))(char nl)" | save -f docs/reference/wise-endpoints.txt
print $"generated ($rows | length) operations -> docs/reference/wise-endpoints.txt"
