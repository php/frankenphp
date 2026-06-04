#include "wtr/watcher-c.h"
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <poll.h>

void callback(struct wtr_watcher_event event, void* data)
{
  int* count = (int*)data;
  *count += 1;
  printf(
#ifdef JSON
    "{"
    "\"count\":%d,"
    "\"path_name\":\"%s\","
    "\"effect_type\":%d,"
    "\"path_type\":%d,"
    "\"effect_time\":%lld,"
    "\"associated_path_name\":\"%s\""
    "}\n",
#else
    "count: %d, "
    "path name: %s, "
    "effect type: %d "
    "path type: %d, "
    "effect time: %lld, "
    "associated path name: %s\n",
#endif
    *count,
    event.path_name,
    event.effect_type,
    event.path_type,
    event.effect_time,
    event.associated_path_name ? event.associated_path_name : "");
}

void do_poll_read_loop(int read_fd)
{
  printf("Press enter to exit\n");
  struct pollfd fds[2];
  memset(fds, 0, sizeof(fds));
  fds[0].fd = read_fd;
  fds[0].events = POLLIN;
  fds[0].revents = 0;
  fds[1].fd = STDIN_FILENO;
  fds[1].events = POLLIN;
  fds[1].revents = 0;
  while (true) {
    int ret = poll(fds, 2, -1);
    if (ret < 0) {
      perror("poll");
      break;
    }
    if (fds[0].revents & POLLIN) {
      char buf[4096];
      memset(buf, 0, sizeof(buf));
      ssize_t n = read(read_fd, buf, sizeof(buf) - 1);
      if (n < 0) {
        perror("read");
        break;
      }
      if (n == 0) {
        fprintf(stderr, "EOF\n");
        break;
      }
      printf("%s", buf);
    }
    if (fds[1].revents & POLLIN) {
      break;
    }
  }
}

int main(int argc, char** argv)
{
  char path[4096];
  memset(path, 0, sizeof(path));
  bool badargs = argc != 2 && argc != 3;
  bool ishelp = ! badargs && strcmp(argv[1], "-h") == 0;
  if (badargs || ishelp) {
    fprintf(stderr, "Usage: %s [path]\n", argv[0]);
    fprintf(stderr, "  path: path to watch\n");
    return 1;
  }
  if (argc == 3) strncpy(path, argv[2], sizeof(path) - 1);
  int count = 0;
  void* watcher = wtr_watcher_open(path, callback, &count);
  if (! watcher) return 1;
  getchar();
  if (! wtr_watcher_close(watcher)) return 1;
  return 0;
}
