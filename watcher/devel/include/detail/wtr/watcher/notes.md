## Mutex-esque Synchronization

Mutexes, in C++, can throw. So can the associated lock classes.
We don't use exceptions, so I've drafted an atomic-based locking
utility. I think that using mutexes and their standard locking
classes is best, for now, because it cuts down on LOC and we are
incredibly unlikely to be thrown on. I *think* the only case the
standard will throw in is for an `EOWNERDEAD`-like error. Maybe
therre are others, but it *seems* like the locking strategy of
scoped locks prevents deadlocks and, naturally, API errors of
the underlying kernel's mutex or futex functionality.

Still, an atomic locking utility is here if we need it.

```cpp
#define WTR_WITH_ATOMIC_OWNERSHIP

#include <optional>

#ifdef WTR_WITH_ATOMIC_OWNERSHIP
#include <atomic>
#include <thread>
#else
#include <mutex>
#endif

class Synchronized {
public:
#ifdef WTR_WITH_ATOMIC_OWNERSHIP
  using Primitive = std::atomic<bool>;
#else
  using Primitive = std::mutex;
#endif

private:
  Primitive& primitive;

  inline Synchronized(Primitive& primitive)
      : primitive{primitive}
  {}

#ifdef WTR_WITH_ATOMIC_OWNERSHIP
  inline static auto exchange_when(
    std::atomic<bool>& current_value,
    bool exchange_when_current_is,
    bool want_value) -> bool
  {
    return current_value.compare_exchange_strong(
      exchange_when_current_is,
      want_value,
      std::memory_order_release,
      std::memory_order_acquire);
  }

  inline static auto try_take(std::atomic<bool>& owner_flag) -> bool
  {
    return exchange_when(owner_flag, false, true);
  }

  inline static auto try_leave(std::atomic<bool>& owner_flag) -> bool
  {
    return exchange_when(owner_flag, true, false);
  }

  inline static auto eventually_take(std::atomic<bool>& owner_flag)
  {
    while (! try_take(owner_flag)) { std::this_thread::yield(); }
  }

#else

  inline static auto try_take(std::mutex& mtx) -> bool
  {
    return mtx.try_lock();
  }

  inline static auto try_leave(std::mutex& mtx) { mtx.unlock(); }

  inline static auto eventually_take(std::mutex& mtx) { mtx.lock(); }

#endif

public:
  inline static auto try_from(Primitive& primitive)
    -> std::optional<Synchronized>
  {
    if (try_take(primitive))
      return Synchronized{primitive};
    else
      return std::nullopt;
  }

  inline static auto eventually_from(Primitive& primitive) -> Synchronized
  {
    eventually_take(primitive);
    return Synchronized{primitive};
  }

  inline ~Synchronized() { try_leave(this->primitive); }
};

#endif

```
