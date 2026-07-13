# pgwd config profiles

Ready-to-use YAML starting points. Copy, adjust URLs and secrets, then:

```bash
pgwd -config /path/to/profile.yml -client YOUR_CLIENT
```

Or merge snippets into `/etc/pgwd/pgwd.conf`. See [SPECIFICATIONS.md](../../SPECIFICATIONS.md) and [contrib/pgwd.conf.example](../pgwd.conf.example).

| Profile | Mode | Use case |
|---------|------|----------|
| [minimal-slack.yml](minimal-slack.yml) | One-shot (`interval: 0`) | Cron + Slack on threshold breach |
| [daemon-loki.yml](daemon-loki.yml) | Daemon | Loki + SQLite + HTTP `/metrics` |
| [kube-prod.yml](kube-prod.yml) | Daemon | Outside cluster: port-forward + Secret password (no `DISCOVER_MY_PASSWORD`) |
| [multi-db.yml](multi-db.yml) | Daemon | Several Postgres targets + Slack |

Replace placeholder URLs and webhook values before production use.
