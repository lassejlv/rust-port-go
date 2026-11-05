/* SPDX-License-Identifier: MIT OR Apache-2.0
 * Scoped native child-only libc write error injection. Rust/SDK source bodies
 * stay unchanged. Go uses an explicit equivalent runtime WriteFmt witness.
 */
#define _GNU_SOURCE
#include <errno.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <sys/syscall.h>
static int selected_errno;
static int pass;
__attribute__((constructor)) static void select_error(void) {
    const char *value=getenv("FERRUGO_STDERR_ERRNO");
    selected_errno=value ? (int)strtol(value,0,10) : 9;
    pass=value && strcmp(value,"pass")==0;
}
ssize_t write(int fd,const void *buffer,size_t size) {
    if (fd==2) {
        // Observe only the source eprintln argument writes. Rust's separate
        // runtime-abort diagnostic after the final hook is outside this
        // component's host exit representation, as declared by the manifest.
        int source_argument=(size>=12 && memcmp(buffer,"fatal error:",12)==0)
            || (size==10 && memcmp(buffer,"4294967294",10)==0)
            || (size==7 && memcmp(buffer," bytes\n",7)==0);
        if (source_argument) {
            const unsigned char *bytes=buffer;
            char record[2048];size_t len=(size_t)snprintf(record,sizeof(record),"stderr_write:%zu:",size);
            const char *digits="0123456789abcdef";
            for(size_t i=0;i<size && len+3<sizeof(record);i++){record[len++]=digits[bytes[i]>>4];record[len++]=digits[bytes[i]&15];}
            record[len++]='\n';syscall(SYS_write,1,record,len);
        }
        if (!pass) {errno=selected_errno;return -1;}
    }
    return syscall(SYS_write,fd,buffer,size);
}
