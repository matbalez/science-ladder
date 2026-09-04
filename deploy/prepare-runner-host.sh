#!/bin/bash
# Run only on an explicitly dedicated Linux verification host. This never
# repartitions disks or deletes existing workloads. Retire those separately.
set -euo pipefail
test "$(id -u)" = 0
test "$(uname -m)" = x86_64
test -c /dev/kvm

install -d -m 700 /etc/science-ladder /var/lib/science-ladder
install -d -m 755 /opt/science-ladder /usr/local/lib/science-ladder
if ! getent group science-ladder-guest >/dev/null; then groupadd -g 10010 science-ladder-guest; fi
if ! id science-ladder-guest >/dev/null 2>&1; then
  useradd -u 10010 -g science-ladder-guest -M -d /nonexistent -s /usr/sbin/nologin science-ladder-guest
fi

# Demonstration custody: dedicated wrapping key is root-readable. Official
# operation must replace it with managed key release; do not claim KMS custody.
if [ ! -e /var/lib/science-ladder/spool.luks ]; then
  test ! -e /etc/science-ladder/spool.key
  (umask 077; head -c 64 /dev/urandom > /etc/science-ladder/spool.key)
  (umask 077; truncate -s 8G /var/lib/science-ladder/spool.luks)
  cryptsetup luksFormat --batch-mode --type luks2 --key-file /etc/science-ladder/spool.key /var/lib/science-ladder/spool.luks
  cryptsetup open --key-file /etc/science-ladder/spool.key /var/lib/science-ladder/spool.luks science_ladder_spool
  mkfs.ext4 -q -L sl-result-spool /dev/mapper/science_ladder_spool
fi
cryptsetup isLuks /var/lib/science-ladder/spool.luks

cat > /usr/local/lib/science-ladder/host-controls <<'CONTROLS'
#!/bin/bash
set -euo pipefail
test "$(id -u)" = 0
echo off > /sys/devices/system/cpu/smt/control
echo 0 > /sys/kernel/mm/ksm/run
swapoff -a
install -d -m 700 /run/science-ladder /run/science-ladder-signer /var/lib/science-ladder/spool
if ! mountpoint -q /run/science-ladder; then
  # Jailer creates its private /dev/kvm node here; nodev would prevent KVM use.
  mount -t tmpfs -o size=12G,mode=0700,nosuid tmpfs /run/science-ladder
fi
if [ ! -e /run/netns/science-ladder-guest ]; then ip netns add science-ladder-guest; fi
if [ ! -e /dev/mapper/science_ladder_spool ]; then
  cryptsetup open --key-file /etc/science-ladder/spool.key /var/lib/science-ladder/spool.luks science_ladder_spool
fi
if ! mountpoint -q /var/lib/science-ladder/spool; then
  mount -o nosuid,nodev,noexec /dev/mapper/science_ladder_spool /var/lib/science-ladder/spool
fi
chmod 700 /var/lib/science-ladder/spool
test "$(cat /sys/devices/system/cpu/smt/active)" = 0
test "$(cat /sys/kernel/mm/ksm/run)" = 0
CONTROLS
chmod 700 /usr/local/lib/science-ladder/host-controls
cat > /etc/systemd/system/science-ladder-host-controls.service <<'UNIT'
[Unit]
Description=Science Ladder dedicated verification host controls
After=local-fs.target network.target
Before=science-ladder-runner.service science-ladder-host-signer.service
[Service]
Type=oneshot
ExecStart=/usr/local/lib/science-ladder/host-controls
RemainAfterExit=yes
UMask=0077
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable --now science-ladder-host-controls.service
printf 'Dedicated host controls installed. Raw wrapping-key custody is controlled-demo only.\n'
