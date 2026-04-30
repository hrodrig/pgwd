# pgwd — BSD Make (FreeBSD) stub: forwards to GNU Make.
# GNU Make reads GNUmakefile before this file on Linux/macOS/CI, so those
# environments never parse this stub.
#
# On FreeBSD: pkg install gmake   then either   make <target>   or   gmake <target>
#
# BSD Make does not apply a lone "%:" to arbitrary targets (unlike GNU Make).
# Use an explicit "all" default and .DEFAULT for every other goal. For .DEFAULT,
# $(.MAKE.CMDGOALS) can be empty when the recipe runs; the target name is in $@.

_CHECK := command -v gmake >/dev/null 2>&1 || { echo "This project requires GNU make. On FreeBSD: pkg install gmake"; exit 1; }

# Plain "make" (no goals) runs the first target.
all:
	@${_CHECK}
	@gmake help

.DEFAULT:
	@${_CHECK}
	@gmake $@
