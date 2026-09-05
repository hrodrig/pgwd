PORTNAME=	pgwd
PORTVERSION=	1.2.0
DISTVERSIONPREFIX=	v
CATEGORIES=	sysutils
# Local test before release: copy the tarball into DISTDIR (see `make -V DISTDIR`
# from this port directory) using the same name as DISTFILES, or run e.g.
#   make fetch MASTER_SITES=file:///absolute/path/to/dir/
# See README.md "Test with a local distfile".
MASTER_SITES=	https://github.com/hrodrig/pgwd/releases/download/v${PORTVERSION}/
DISTFILES=	pgwd_v${PORTVERSION}_freebsd_${ARCH:S/aarch64/arm64/}.tar.gz
EXTRACT_SUFX=	.tar.gz

MAINTAINER=	hrodrig@usb.ve
COMMENT=	Postgres Watch Dog - monitor PostgreSQL connections
WWW=		https://github.com/hrodrig/pgwd
LICENSE=	MIT

NO_BUILD=	yes
# amd64/aarch64 only: matches GitHub release tarballs (see README.md "Supported architectures").
ONLY_FOR_ARCHS=	amd64 aarch64

do-install:
	${INSTALL_PROGRAM} ${WRKDIR}/pgwd ${STAGEDIR}${PREFIX}/bin/pgwd
	${MKDIR} ${STAGEDIR}${PREFIX}/share/man/man1
	${INSTALL_MAN} ${WRKDIR}/share/man/man1/pgwd.1 ${STAGEDIR}${PREFIX}/share/man/man1/
	${MKDIR} ${STAGEDIR}${PREFIX}/share/doc/pgwd
	${INSTALL_DATA} ${WRKDIR}/share/doc/pgwd/LICENSE ${STAGEDIR}${PREFIX}/share/doc/pgwd/
	${MKDIR} ${STAGEDIR}${PREFIX}/etc/pgwd
	${INSTALL_DATA} ${WRKDIR}/etc/pgwd/pgwd.conf.example ${STAGEDIR}${PREFIX}/etc/pgwd/
	${MKDIR} ${STAGEDIR}${PREFIX}/etc/rc.d
	${INSTALL_SCRIPT} ${.CURDIR}/rc.d/pgwd ${STAGEDIR}${PREFIX}/etc/rc.d/pgwd

.include <bsd.port.mk>

