#!/bin/ksh
# Install pgwd like the OpenBSD port (do-install + @rcscript) without a full /usr/ports tree.
# Use on lab VMs that only have the port skeleton + local distfile.
#
#   doas sh test-install-from-dist.sh /tmp/pgwd_v0.6.6_openbsd_amd64.tar.gz
#
# Optional second arg: directory containing files/pgwd (default: script dir, then files/)

set -e

if [ "$(id -u)" -ne 0 ]; then
	echo "Run as root (doas sh $0 ...)" >&2
	exit 1
fi

dist="${1:?usage: $0 /path/to/pgwd_vVERSION_openbsd_ARCH.tar.gz [port-files-dir]}"
portdir="${2:-$(dirname "$0")}"
files="$portdir/files"
[ -d "$files" ] || files="$portdir"

[ -f "$files/pgwd" ] || { echo "missing port file: $files/pgwd" >&2; exit 1; }

stage="/tmp/pgwd-port-install-$$"
trap 'rm -rf "$stage"' EXIT INT TERM
mkdir -p "$stage"
echo "Extracting $dist ..."
tar xzf "$dist" -C "$stage"

PREFIX=/usr/local
install -d "$PREFIX/bin" "$PREFIX/man/man1" "$PREFIX/share/doc/pgwd" \
	"$PREFIX/share/examples/pgwd" /etc/rc.d

install -m 755 "$stage/pgwd" "$PREFIX/bin/pgwd"

if [ -f "$stage/share/man/man1/pgwd.1" ]; then
	install -m 644 "$stage/share/man/man1/pgwd.1" "$PREFIX/man/man1/pgwd.1"
fi
if [ -f "$stage/share/doc/pgwd/LICENSE" ]; then
	install -m 644 "$stage/share/doc/pgwd/LICENSE" "$PREFIX/share/doc/pgwd/LICENSE"
fi
if [ -f "$stage/etc/pgwd/pgwd.conf.example" ]; then
	install -m 644 "$stage/etc/pgwd/pgwd.conf.example" "$PREFIX/share/examples/pgwd/pgwd.conf.example"
fi

install -m 555 "$files/pgwd" /etc/rc.d/pgwd

echo "Installed (port-equivalent):"
echo "  $PREFIX/bin/pgwd"
echo "  /etc/rc.d/pgwd"
echo ""
echo "Next:"
echo "  mkdir -p /etc/pgwd"
echo "  cp $PREFIX/share/examples/pgwd/pgwd.conf.example /etc/pgwd/pgwd.conf"
echo "  vi /etc/pgwd/pgwd.conf"
echo "  rcctl enable pgwd && rcctl start pgwd"
echo "  rcctl check pgwd"
