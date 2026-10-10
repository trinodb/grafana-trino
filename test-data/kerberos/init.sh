#!/bin/bash
# Run by the ghcr.io/trinodb/testing/kdc entrypoint once the KDC starts, from
# /docker/kerberos-init.d. Provisions everything the Kerberized Trino and
# Grafana need into the volume mounted at /etc/trino-kerberos, which both of
# them mount too:
# - trino.keytab for the coordinator's service principal, named after the
#   trino-kerberos container's hostname,
# - grafana.keytab for the client principal the data source logs in as,
# - trino-kerberos.pem, a self-signed certificate and key for the coordinator's
#   HTTPS endpoint, since Trino only accepts Kerberos over HTTPS,
# - ready, created last, which the other containers wait for.

set -euo pipefail

dir=/etc/trino-kerberos
rm -f "$dir"/ready "$dir"/*.keytab "$dir"/*.pem

# The KDC database outlives a container restart, so drop the principals of a
# previous run instead of failing to add them again.
for principal in trino/trino-kerberos grafana; do
    /usr/sbin/kadmin.local -q "delprinc -force $principal@TRINO.TEST" >/dev/null
    /usr/local/bin/create_principal -p "$principal" -k "$dir/${principal%%/*}.keytab"
done

openssl req -x509 -newkey rsa:2048 -nodes -days 30 \
    -subj /CN=trino-kerberos \
    -addext subjectAltName=DNS:trino-kerberos \
    -keyout /tmp/trino-kerberos.key \
    -out /tmp/trino-kerberos.crt
cat /tmp/trino-kerberos.key /tmp/trino-kerberos.crt > "$dir/trino-kerberos.pem"

# Trino and the Grafana plugin run as unprivileged users of their own images.
chmod 644 "$dir"/*
touch "$dir/ready"
