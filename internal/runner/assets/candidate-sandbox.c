/* MIT. Platform-owned seccomp launcher, compiled into the pinned Linux runtime.
 * The root broker has already applied chroot, namespaces, unprivileged identity,
 * cgroups and an empty descriptor set except stdio. This program only reduces
 * authority further; a failed filter installation never executes the candidate. */
#define _GNU_SOURCE
#include <errno.h>
#include <linux/audit.h>
#include <linux/filter.h>
#include <linux/seccomp.h>
#include <sched.h>
#include <stddef.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/prctl.h>
#include <sys/resource.h>
#include <sys/syscall.h>
#include <unistd.h>

#if !defined(__x86_64__)
#error This reviewed profile is Linux amd64 only; other architectures need a new filter.
#endif
#define DENY(n) BPF_JUMP(BPF_JMP|BPF_JEQ|BPF_K,(n),0,1), BPF_STMT(BPF_RET|BPF_K,SECCOMP_RET_ERRNO|EPERM)

int main(int argc, char **argv) {
    if (argc < 3 || getuid() == 0 || geteuid() == 0) return 120;
    char *end = NULL;
    errno = 0;
    unsigned long long output = strtoull(argv[1], &end, 10);
    if (errno || !end || *end || output == 0 || output > (1ULL<<30)) return 121;
    struct rlimit files = {output, output}, core = {0,0}, descriptors = {128,128};
    if (setrlimit(RLIMIT_FSIZE,&files) || setrlimit(RLIMIT_CORE,&core) || setrlimit(RLIMIT_NOFILE,&descriptors)) return 122;
    if (prctl(PR_SET_NO_NEW_PRIVS,1,0,0,0)) return 123;
    struct sock_filter filter[] = {
        BPF_STMT(BPF_LD|BPF_W|BPF_ABS,offsetof(struct seccomp_data,arch)),
        BPF_JUMP(BPF_JMP|BPF_JEQ|BPF_K,AUDIT_ARCH_X86_64,1,0),
        BPF_STMT(BPF_RET|BPF_K,SECCOMP_RET_KILL_PROCESS),
        BPF_STMT(BPF_LD|BPF_W|BPF_ABS,offsetof(struct seccomp_data,nr)),
        /* Reject the x32 syscall ABI rather than bypass the native deny list. */
        BPF_JUMP(BPF_JMP|BPF_JSET|BPF_K,0x40000000,0,1),
        BPF_STMT(BPF_RET|BPF_K,SECCOMP_RET_KILL_PROCESS),
        DENY(SYS_ptrace), DENY(SYS_process_vm_readv), DENY(SYS_process_vm_writev),
        DENY(SYS_bpf), DENY(SYS_perf_event_open), DENY(SYS_keyctl),
        DENY(SYS_add_key), DENY(SYS_request_key), DENY(SYS_mount), DENY(SYS_umount2),
        DENY(SYS_pivot_root), DENY(SYS_chroot), DENY(SYS_unshare), DENY(SYS_setns),
        DENY(SYS_reboot), DENY(SYS_kexec_load), DENY(SYS_kexec_file_load),
        DENY(SYS_init_module), DENY(SYS_finit_module), DENY(SYS_delete_module),
        DENY(SYS_swapon), DENY(SYS_swapoff), DENY(SYS_acct), DENY(SYS_syslog),
        DENY(SYS_open_by_handle_at), DENY(SYS_name_to_handle_at),
        DENY(SYS_io_uring_setup), DENY(SYS_userfaultfd), DENY(SYS_fanotify_init),
        DENY(SYS_quotactl), DENY(SYS_socket), DENY(SYS_setuid), DENY(SYS_setgid),
        DENY(SYS_setreuid), DENY(SYS_setregid), DENY(SYS_setresuid), DENY(SYS_setresgid),
        DENY(SYS_setfsuid), DENY(SYS_setfsgid), DENY(SYS_setgroups),
        /* libc may probe clone3; ENOSYS permits its ordinary clone fallback. */
        BPF_JUMP(BPF_JMP|BPF_JEQ|BPF_K,SYS_clone3,0,1),
        BPF_STMT(BPF_RET|BPF_K,SECCOMP_RET_ERRNO|ENOSYS),
        BPF_JUMP(BPF_JMP|BPF_JEQ|BPF_K,SYS_clone,0,3),
        BPF_STMT(BPF_LD|BPF_W|BPF_ABS,offsetof(struct seccomp_data,args[0])),
        BPF_JUMP(BPF_JMP|BPF_JSET|BPF_K,CLONE_NEWUSER|CLONE_NEWNS|CLONE_NEWPID|CLONE_NEWNET|CLONE_NEWIPC|CLONE_NEWUTS|CLONE_NEWCGROUP,0,1),
        BPF_STMT(BPF_RET|BPF_K,SECCOMP_RET_ERRNO|EPERM),
        BPF_STMT(BPF_RET|BPF_K,SECCOMP_RET_ALLOW)
    };
    struct sock_fprog program = {(unsigned short)(sizeof(filter)/sizeof(filter[0])),filter};
    if (prctl(PR_SET_SECCOMP,SECCOMP_MODE_FILTER,&program)) return 124;
    if (argv[2][0] != '/') return 125;
    execv(argv[2], &argv[2]);
    perror("candidate exec");
    return 126;
}
