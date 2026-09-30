# k8s-api-deprecation-scan

[![ci](https://github.com/moveeeax/k8s-api-deprecation-scan/actions/workflows/ci.yml/badge.svg)](https://github.com/moveeeax/k8s-api-deprecation-scan/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.23%2B-00ADD8)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Scan Kubernetes manifests for `apiVersion`s deprecated or removed at a target cluster version, before the upgrade breaks them.

Every Kubernetes minor release retires a few API versions. The failure mode is quiet: the manifests apply fine on the old cluster, the upgrade runs, and then a Deployment stops reconciling or an Ingress vanishes. This is a static check — point it at a directory of YAML and a version you are about to upgrade to, and it names the files that will not survive.

No cluster connection, no `client-go`, no kubeconfig. It reads files, so it runs in CI with no credentials.

## Usage

```
k8s-api-deprecation-scan --target-version VERSION [flags] PATH...
```

`PATH` is a file, a directory (walked recursively for `.yaml` / `.yml`), or `-` to read a manifest stream from stdin.

```console
$ k8s-api-deprecation-scan --target-version 1.29 examples/manifests
REMOVED    examples/manifests/networking.yaml:0 Ingress/checkout networking.k8s.io/v1beta1 -> networking.k8s.io/v1 (deprecated 1.19, removed 1.22)
REMOVED    examples/manifests/networking.yaml:2 CronJob/reconcile-orders batch/v1beta1 -> batch/v1 (deprecated 1.21, removed 1.25)
REMOVED    examples/manifests/workloads.yaml:1 PodDisruptionBudget/checkout policy/v1beta1 -> policy/v1 (deprecated 1.21, removed 1.25)
REMOVED    examples/manifests/workloads.yaml:2 HorizontalPodAutoscaler/checkout autoscaling/v2beta2 -> autoscaling/v2 (deprecated 1.23, removed 1.26)

4 removed, 0 deprecated at 1.29 (6 documents in 2 files)
$ echo $?
1
```

The number after the colon is the document index inside the file, so multi-document streams point at the exact object.

Rendered charts work the same way:

```console
$ helm template mychart | k8s-api-deprecation-scan --target-version 1.30 -
$ kustomize build overlays/prod | k8s-api-deprecation-scan --target-version 1.30 -
```

JSON for anything that has to consume the result:

```console
$ k8s-api-deprecation-scan --format json --target-version 1.29 examples/manifests | jq '.summary'
{
  "removed": 4,
  "deprecated": 0
}
```

### Flags

| Flag | Meaning |
| --- | --- |
| `--target-version` | Cluster version you are upgrading to: `1.29`, `v1.29`, `1.29.3`. Required. |
| `--format` | `text` (default) or `json`. |
| `--list-rules` | Print the built-in deprecation table as JSON and exit. |
| `--version` | Print the CLI version and exit. |

### Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Nothing removed at the target version. Deprecations may still be listed. |
| `1` | At least one document uses an `apiVersion` the target version no longer serves. |
| `2` | Usage or I/O error. |

## How it works

Each path is decoded as a multi-document YAML stream. Every document that has both `apiVersion` and `kind` is looked up in a table of retiring GroupVersionKinds, compiled in from the upstream deprecated API migration guides. Anything that is not a Kubernetes object — CI config, a bare list, a chart's `values.yaml` — is skipped without a word, because you want to point the tool at a whole repo.

Levels are decided against the target version:

- **REMOVED** — the table has a removal release and the target is at or past it. The manifest will be rejected. Exit code 1.
- **DEPRECATED** — the API is in the table but the target version still serves it.

Note that everything in the table is reported, including APIs whose deprecation release is still ahead of your target. A `policy/v1beta1` PodDisruptionBudget on a 1.20 cluster is technically not deprecated yet, but it is scheduled to die in 1.25 and the manifest is what you carry into the next upgrade — so it comes back as a warning rather than silence.

The table is data, not magic: `--list-rules` prints it.

```console
$ k8s-api-deprecation-scan --list-rules | jq '.[] | select(.removedIn == "1.25")' | head -8
{
  "apiVersion": "batch/v1beta1",
  "kind": "CronJob",
  "deprecatedIn": "1.21",
  "removedIn": "1.25",
  "replacement": "batch/v1"
}
```

## In CI

```yaml
- name: check manifests against the next cluster version
  run: |
    go install github.com/moveeeax/k8s-api-deprecation-scan/cmd/k8s-api-deprecation-scan@latest
    k8s-api-deprecation-scan --target-version 1.30 ./manifests
```

The job fails only on removals, so you can raise `--target-version` ahead of the actual upgrade and watch the warnings turn into failures as the date approaches.

## Install

```console
$ go install github.com/moveeeax/k8s-api-deprecation-scan/cmd/k8s-api-deprecation-scan@latest
```

Or from a clone:

```console
$ go build -o k8s-api-deprecation-scan ./cmd/k8s-api-deprecation-scan
```

## Development

```console
$ go vet ./...
$ go test ./...
```

Fixtures live in `internal/scan/testdata` and `examples/manifests`.

## License

MIT — see [LICENSE](LICENSE).
