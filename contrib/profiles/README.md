# pgwd config profiles

Ready-to-use YAML starting points. Copy, adjust URLs and secrets, then:

```bash
pgwd -config /path/to/profile.yml -client YOUR_CLIENT
```

Or merge snippets into `/etc/pgwd/pgwd.conf`. See [SPECIFICATIONS.md](../../SPECIFICATIONS.md) and [contrib/pgwd.conf.example](../pgwd.conf.example).

**Which profile?** See **[docs/use-cases.md](../../docs/use-cases.md)** for the full scenario matrix (single/multi DB, in/out of cluster, cron vs daemon).

| Profile | Mode | Use case |
|---------|------|----------|
| [minimal-slack.yml](minimal-slack.yml) | One-shot (`interval: 0`) | **UC-1** — Cron + Slack, one direct URL |
| [daemon-loki.yml](daemon-loki.yml) | Daemon | **UC-2** — One DB, Loki + SQLite + `/metrics` |
| [kube-prod.yml](kube-prod.yml) | Daemon | **UC-4** — Outside cluster: one DB, port-forward + `kube.password_from_secret` — [guide](../../docs/kubernetes-passwords.md#option-1--kubepassword_from_secret-outside-cluster-daemon) |
| [multi-db.yml](multi-db.yml) | Daemon | **UC-5 / UC-6** — N Postgres, **different creds per `databases[].url`** — [use-cases](../../docs/use-cases.md#uc-5--multi-database-in-cluster-n-different-credentials) |

Replace placeholder URLs and webhook values before production use.
