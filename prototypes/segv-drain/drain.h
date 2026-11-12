#ifndef DRAIN_H
#define DRAIN_H
void start_threads(int n, int crash_idx, int mode, int own_altstack);
void release_crashed(void);
int notify_fd(void);
void fault_in_c(void);
#endif
