#include "wtr/watcher.hpp"
#include <chrono>
#include <cmath>
#include <cstring>
#include <filesystem>
#include <iostream>
#include <optional>
#include <string>
#include <string_view>
#include <thread>

using namespace wtr;
using namespace std;
using namespace chrono;
namespace fs = filesystem;

// clang-format off

struct Args {
  static constexpr auto help =
    "wtr.watcher PATH=. [-UNIT TIME]\n"
    "wtr.watcher -h, --help\n"
    "wtr.watcher -v, --version\n"
    "\n"
    "  PATH\n"
    "    Any path. Relative or absolute.\n"
    "    Defaults to the current directory.\n"
    "    If the given path doesn't exist,\n"
    "    the current directory is used.\n"
    "\n"
    "  UNIT\n"
    "    One of:\n"
    "    -nanoseconds,  -ns,\n"
    "    -microseconds, -us,\n"
    "    -milliseconds, -ms,\n"
    "    -seconds,      -s,\n"
    "    -minutes,      -m,\n"
    "    -hours,        -h,\n"
    "    -days,         -d,\n"
    "    -weeks,        -w,\n"
    "    -months,       -mts,\n"
    "    -years,        -y\n"
    "\n"
    "  TIME\n"
    "    Any positive integer, as long as it's\n"
    "    less than ULONG_MAX. Which is large.\n"
    "\n"
    "  -h, --help\n"
    "    Show this help message and exit.\n"
    "\n"
    "  -v, --version\n"
    "    Show version information and exit.\n";
  static constexpr auto version = WTR_WATCHER_VERSION_S;
  bool const is_help;
  bool const is_version;
  optional<fs::path> const path;
  optional<nanoseconds> const time;

  static auto try_parse(int argc, char const* const* const argv) -> optional<Args>
  {
    auto argis = [&](auto const i, char const* a)
    { return argc > i ? strcmp(a, argv[i]) == 0 : false; };

    bool is_help = argis(1, "-h") || argis(1, "--help");
    bool is_version = argis(1, "-v") || argis(1, "--version");

    auto path =
      is_help ? nullopt
      : argc < 2 ? optional(fs::current_path())
      : fs::exists(argv[1]) ? optional(fs::path(argv[1]))
      : nullopt;

    auto time = [&]() -> optional<nanoseconds> {
      auto st = argc > 3 ? argv[3] : "0";
      auto stend = st + strlen(st);
      auto targis = [&](auto a) { return argis(2, a); };
      long long ttons
        = targis("-nanoseconds")  || targis("-ns")  ?                      1e0
        : targis("-microseconds") || targis("-us")  ?                      1e3
        : targis("-milliseconds") || targis("-ms")  ?                      1e6
        : targis("-seconds")      || targis("-s")   ?                      1e9
        : targis("-minutes")      || targis("-m")   ?                 60 * 1e9
        : targis("-hours")        || targis("-h")   ?            60 * 60 * 1e9
        : targis("-days")         || targis("-d")   ?       24 * 60 * 60 * 1e9
        : targis("-weeks")        || targis("-w")   ? 7   * 24 * 60 * 60 * 1e9
        : targis("-months")       || targis("-mts") ? 30  * 24 * 60 * 60 * 1e9
        : targis("-years")        || targis("-y")   ? 365 * 24 * 60 * 60 * 1e9
                                                    :                      1e6;
      auto td = strtod(st, (char**)&stend);
      if (is_help || td == HUGE_VAL || td <= 0)
        return nullopt;
      else
        return nanoseconds(llroundl(td) * ttons);
    }();

    return is_help || is_version || path || time
           ? optional(Args{is_help, is_version, path, time})
           : nullopt;
  }
};

auto json(event const& e) -> string
{
  auto tails = e.associated
               ? json(*e.associated)
               : "null";
  auto s = [](auto a) { return to<string>(a); };
  auto q = [&](auto a) { return '"' + s(a) + '"'; };
  return "{\"path_type\":"   + q(e.path_type)
       + ",\"path_name\":"   + q(e.path_name)
       + ",\"effect_type\":" + q(e.effect_type)
       + ",\"effect_time\":" + s(e.effect_time)
       + ",\"associated\":"  + tails
       + "}";
}

/*  Watch a path for some time.
    Or watch a path forever.
    Show what happens, or show help. */
int main(int argc, char const* const* const argv)
{
  auto cb = [](auto ev) { cout << json(ev) << endl; };
  auto args = Args::try_parse(argc, argv);
  return ! args ? (cerr << Args::help, 1)
       : args->is_help ? (cout << Args::help, 0)
       : args->is_version ? (cout << Args::version << endl, 0)
       : ! args->path ? (cerr << Args::help, 1)
       : [&] { auto w = watch(*args->path, cb);
               if (! args->time) cin.get();
               else this_thread::sleep_for(*args->time);
               return ! w.close();
             }();
};

// clang-format on
