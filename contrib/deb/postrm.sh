#!/bin/sh
# postrm: on purge, remove config directory so no trace remains.
case "$1" in
    purge)
        rm -rf /etc/pgwd
        ;;
esac
exit 0
