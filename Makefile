# pgwd — BSD Make (FreeBSD) stub: forwards to GNU Make.
# GNU Make reads GNUmakefile before this file on Linux/macOS/CI, so those
# environments never parse this stub.
#
# On FreeBSD: pkg install gmake   then either   make <target>   or   gmake <target>

_CHECK_GMAKE = @command -v gmake >/dev/null 2>&1 || { echo "This project requires GNU make. On FreeBSD: pkg install gmake"; exit 1; }

help:
	${_CHECK_GMAKE}
	@gmake help

%:
	${_CHECK_GMAKE}
	@gmake $@
