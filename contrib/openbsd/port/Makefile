# OpenBSD port for pgwd (Postgres Watch Dog)
# Copy to /usr/ports/sysutils/pgwd/ and run: make makesum
# Submit: send diff to ports@openbsd.org
# Uses pledge() for security.

COMMENT =	postgres connection watchdog with Slack/Loki alerts
DISTNAME =	pgwd_v0.6.8_openbsd_${MACHINE_ARCH:S/aarch64/arm64/}
PKGNAME =	pgwd-0.6.8
CATEGORIES =	sysutils
HOMEPAGE =	https://github.com/hrodrig/pgwd
MAINTAINER =	hrodrig@usb.ve

# MIT
PERMIT_PACKAGE =	Yes
PERMIT_DISTFILES =	Yes

# Local test before release: copy tarball to DISTDIR or use
#   make fetch MASTER_SITES=file:///absolute/path/to/dir/
# See README.md "Test with a local tarball".
MASTER_SITES =	https://github.com/hrodrig/pgwd/releases/download/v0.6.8/
DISTFILES =	pgwd_v0.6.8_openbsd_${MACHINE_ARCH:S/aarch64/arm64/}.tar.gz
EXTRACT_SUFX =	.tar.gz

ONLY_FOR_ARCHS =	amd64 arm64 aarch64
NO_BUILD =	Yes
NO_TEST =	Yes
PKG_ARCH =	*

do-install:
	${INSTALL_PROGRAM} ${WRKDIR}/pgwd ${PREFIX}/bin/pgwd
	${INSTALL_MAN} ${WRKDIR}/share/man/man1/pgwd.1 ${PREFIX}/man/man1/pgwd.1
	${INSTALL_DATA_DIR} ${PREFIX}/share/doc/pgwd
	${INSTALL_DATA} ${WRKDIR}/share/doc/pgwd/LICENSE ${PREFIX}/share/doc/pgwd/
	${INSTALL_DATA_DIR} ${PREFIX}/share/examples/pgwd
	${INSTALL_DATA} ${WRKDIR}/etc/pgwd/pgwd.conf.example ${PREFIX}/share/examples/pgwd/
# rc.d: pkg/pgwd.rc + @rcscript in PLIST — generate-readmes copies *.rc → ${RCDIR}/pgwd.

.include <bsd.port.mk>
