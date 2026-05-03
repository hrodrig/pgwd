# Helm chart (moved)

The **pgwd** Helm chart is **not** maintained in this repository. Deployment manifests — including Kubernetes Helm — live in **[pgwd-selfhosted](https://github.com/hrodrig/pgwd-selfhosted)**.

- **Chart source:** [`run/kubernetes/helm/pgwd/`](https://github.com/hrodrig/pgwd-selfhosted/tree/main/run/kubernetes/helm/pgwd)
- **Docs:** [pgwd-selfhosted README — Kubernetes Helm](https://github.com/hrodrig/pgwd-selfhosted/blob/main/README.md#kubernetes-helm)

**OCI chart from pgwd releases:** Previously, tagged releases of this repo could publish a chart to `oci://ghcr.io/hrodrig/pgwd/pgwd`. That path is **no longer updated** from **hrodrig/pgwd** releases. Use the chart and release process in **pgwd-selfhosted** (including chart-releaser / GitHub Pages when applicable).

For running pgwd **inside** the cluster with raw YAML, see [contrib/k8s/README.md](k8s/README.md).
