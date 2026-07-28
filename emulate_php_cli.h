#ifndef _EMULATE_PHP_CLI_H
#define _EMULATE_PHP_CLI_H

#include <stdbool.h>

typedef struct {
  char *script;
  int argc;
  char **argv;
  bool eval;
} cli_exec_args_t;
extern cli_exec_args_t *cli_args;
void *emulate_script_cli(void *arg);

#endif
