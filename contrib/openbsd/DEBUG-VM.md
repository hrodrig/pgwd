# OpenBSD VM debug checklist (pgwd)

Run on an **OpenBSD test host** as **root**. rc.d model: `daemon="/usr/local/bin/pgwd"`, **`rc_bg=YES`** (rc.subr backgrounds the daemon).

After manual checks pass, re-run platform tests from the **control node** (pgwd repo root):

```bash
make test-platforms LIMIT=pgwd-openbsd
```

Use the host name from **`testing/platforms/inventory/hosts.yml`** (see **`hosts.yml.example`**).

## 0. Binary and config

```sh
which pgwd
pgwd -version
ls -l /usr/local/bin/pgwd
ls -l /etc/pgwd/pgwd.conf
sed -n '1,30p' /etc/pgwd/pgwd.conf
```

## 1. Install rc.d from the repository or tarball

From a clone (or **`scp`** from the build machine):

```sh
install -m 555 /path/to/pgwd/contrib/openbsd/pgwd /etc/rc.d/pgwd
```

From a release tarball:

```sh
tar xzf pgwd_*_openbsd_amd64.tar.gz
install -m 555 share/openbsd/rc.d/pgwd /etc/rc.d/pgwd
```

Verify rc.d:

```sh
grep -E '^(daemon=|rc_bg=|daemon_flags=)' /etc/rc.d/pgwd
```

## 2. Clean stop

```sh
rcctl stop pgwd 2>/dev/null
pkill -x pgwd 2>/dev/null
```

## 3. Start via rc.d (preferred)

OpenBSD hides `rc_start` stderr unless you use **`-d`**:

```sh
/etc/rc.d/pgwd -d start
rcctl enable pgwd
rcctl start pgwd
echo "rcctl start rc=$?"
rcctl check pgwd
pgrep -af pgwd
tail -n 40 /var/log/daemon
```

## 4. CLI without rc.d

```sh
pgwd -config /etc/pgwd/pgwd.conf -dry-run -interval 0
```

## 5. Optional: kube-postgres / kube-loki

Set in **`/etc/rc.conf.local`**:

```sh
pgwd_env="KUBECONFIG=/root/.kube/config"
```

See **`contrib/openbsd/README.md`**.

## 6. Teardown (Ansible)

```bash
cd testing/platforms
ansible-playbook playbooks/teardown.yml --limit pgwd-openbsd
```

Manual cleanup: `rcctl stop pgwd`; `rcctl disable pgwd`; remove `/etc/rc.d/pgwd`, `/usr/local/bin/pgwd`, `/etc/pgwd`.
