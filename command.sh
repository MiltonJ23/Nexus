 sudo cat /sys/fs/cgroup/nexus/node-alpine/cgroup.procs
  sudo cat /sys/fs/cgroup/nexus/node-alpine/memory.max
  sudo cat /sys/fs/cgroup/nexus/node-alpine/cpu.weight


  ls -la /proc/$$/ns/

  sudo nsenter -t 454003 -m -u -i -n sh -c 'chroot /proc/self/root /bin/sh'