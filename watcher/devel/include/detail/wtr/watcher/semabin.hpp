#pragma once

#include <atomic>

#ifdef __linux__
#include <errno.h>
#include <stdint.h>
#include <string.h>
#include <sys/eventfd.h>
#include <unistd.h>
#elif defined(__APPLE__)
#include <dispatch/dispatch.h>
#endif

namespace detail::wtr::watcher {

/*  A semaphore-like construct which can be
    used with the "native" async I/O APIs
    on (currently) Linux and Darwin.

    On Darwin, this is a semaphore which is
    schedulable with the dispatch library.

    On Linux, this is an eventfd in semaphore
    mode. The file descriptor is exposed for
    use with poll and friends.

    On other platforms, this is an atomic flag
    which can be checked in a sleep, wake loop,
    ideally with a generous sleep time.
*/

class semabin {
public:
  enum state { pending, released, error };

private:
  mutable std::atomic<state> is = pending;

public:
#if defined(__linux__)

  int const fd = eventfd(0, EFD_CLOEXEC | EFD_NONBLOCK | EFD_SEMAPHORE);

  inline auto release() noexcept -> state
  {
    auto write_ev = [this]()
    {
      if (eventfd_write(this->fd, 1) == 0)
        return released;
      else
        return error;
    };

    if (this->is == released)
      return released;
    else
      return this->is = write_ev();
  }

  inline auto state() const noexcept -> state
  {
    auto read_ev = [this]()
    {
      uint64_t _ = 0;
      if (eventfd_read(this->fd, &_) == 0)
        return released;
      else if (errno == EAGAIN)
        return pending;
      else
        return error;
    };

    if (this->is == pending)
      return pending;
    else
      return this->is = read_ev();
  }

  inline ~semabin() noexcept { close(this->fd); }

#elif defined(__APPLE__)

  mutable dispatch_semaphore_t sem = dispatch_semaphore_create(0);

  inline auto release() noexcept -> state
  {
    auto exchange_when = pending;
    auto was_exchanged = this->is.compare_exchange_strong(
      exchange_when,
      released,
      std::memory_order_release,
      std::memory_order_acquire);
    if (was_exchanged) dispatch_semaphore_signal(this->sem);
    return released;
  }

  inline auto state() const noexcept -> state
  {
    return this->is.load(std::memory_order_acquire);
  }

  inline ~semabin() noexcept { this->release(); }

#else

  inline auto release() noexcept -> enum state { return this->is = released; }

  inline auto state() const noexcept -> enum state { return this->is; }

#endif
};

} /*  namespace detail::wtr::watcher */
