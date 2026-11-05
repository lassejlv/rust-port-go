/* SPDX-License-Identifier: MIT OR Apache-2.0
 * Child-only observation/error injection for actual unmodified SDK stdio. */
#define _GNU_SOURCE
#include <errno.h>
#include <stdatomic.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/syscall.h>
static char *script;
static int trace;
static atomic_int cursor;
__attribute__((constructor))static void init(void){const char*s=getenv("FERRUGO_STDIO_SCRIPT");script=strdup(s?s:"");trace=getenv("FERRUGO_STDIO_TRACE")!=NULL;}
ssize_t write(int fd,const void*buffer,size_t size){
 if(fd!=2)return syscall(SYS_write,fd,buffer,size);
 if(trace){const unsigned char*b=buffer;char record[4096];size_t n=0;memcpy(record,"write:",6);n=6;for(size_t i=0;i<size&&n+3<sizeof(record);i++){record[n++]="0123456789abcdef"[b[i]>>4];record[n++]="0123456789abcdef"[b[i]&15];}record[n++]='\n';syscall(SYS_write,1,record,n);}
 int index=atomic_fetch_add(&cursor,1);char*part=script;
 while(index-->0&&part&&*part){part=strchr(part,',');if(part)part++;}
 if(part&&*part){if(*part=='e'){errno=(int)strtol(part+1,NULL,10);return -1;}if(*part=='z')return 0;if(*part=='n'){size_t n=(size_t)strtoul(part+1,NULL,10);if(n<size)size=n;}}
 return syscall(SYS_write,fd,buffer,size);
}
