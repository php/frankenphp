#ifndef _THREADLOCALE_H
#define _THREADLOCALE_H

#ifndef _WIN32
/* Makes setlocale() change the locale of the current thread only */
void frankenphp_override_setlocale(void);
#else
/* PHP keeps the locale per thread on Windows */
static inline void frankenphp_override_setlocale(void) {}
#endif
/* Remembers the locale the worker script set before handling requests */
void frankenphp_save_worker_locale(void);
/* Restores the locale of the worker script after a request */
void frankenphp_restore_worker_locale(void);
/* Forgets the locale of the worker script, and switches the thread back to
 * the process locale at the end of a script */
void frankenphp_reset_thread_locale(void);

#endif
