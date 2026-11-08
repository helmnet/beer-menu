#!/bin/sh
# Refuses to start nginx as UID 0 or with GID 0 in any group, like the API does: the deployer
# picks the UID/GID, and the image's /etc/nginx is writable by group root.
for id in $(id -u) $(id -G); do
    if [ "$id" = 0 ]; then
        echo "refusing to run as root: $(id); set a non-root UID and GID and no root group" >&2
        exit 1
    fi
done
