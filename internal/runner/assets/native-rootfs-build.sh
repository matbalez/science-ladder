#!/bin/sh
# MIT. Separate fixed recipe for the native v2 profile; v1 bytes stay unchanged.
set -eu
umask 022
export LC_ALL=C TZ=UTC SOURCE_DATE_EPOCH=0 E2FSPROGS_FAKE_TIME=0
mkdir -p /work/root
tar --extract --file=/input/base.tar --directory=/work/root --no-same-owner
mkdir -p /work/root/sbin /work/root/proc /work/root/sys /work/root/dev /work/root/tmp
mkdir -p /work/root/sl/validator /work/root/sl/submission /work/root/sl/suite
mkdir -p /work/root/sl/challenge /work/root/sl/config /work/root/sl/work /work/root/sl/output
mkdir -p /work/root/sl/candidate /work/root/sl/broker /work/root/sl/baseline /work/root/sl/assets
cp /input/sl-init /work/root/sbin/sl-init
chmod 0755 /work/root/sbin/sl-init
test -x /work/root/usr/local/bin/sl-candidate-sandbox
printf '127.0.0.1 localhost\n' > /work/root/etc/hosts
: > /work/root/etc/resolv.conf
rm -f /work/root/etc/machine-id /work/root/var/lib/dbus/machine-id
find /work/root -xdev -exec touch -h -d '@0' '{}' +
truncate -s 4G /output/rootfs.ext4
mke2fs -q -F -t ext4 -b 4096 -U 17d26c2c-fd47-4e0a-9987-d757ca687ee5 \
  -O '^has_journal' -E lazy_itable_init=0,lazy_journal_init=0,hash_seed=5d3e6088-904c-4b6e-bd06-e594e8175202 \
  -d /work/root /output/rootfs.ext4
chmod 0400 /output/rootfs.ext4
