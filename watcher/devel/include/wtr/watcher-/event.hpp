#pragma once

#include <array>
#include <charconv>
#include <chrono>
#include <filesystem>
#include <functional>
#include <ios>
#include <limits>
#include <memory>
#include <string>
#include <string_view>
#include <type_traits>

namespace wtr {
inline namespace watcher {

/*  The `event` object is used to carry information about
    filesystem events to the user through the (user-supplied)
    callback given to `watch`.

    The `event` object will contain the:
      - `path_name`: The path to the event.
      - `path_type`: One of:
        - dir
        - file
        - hard_link
        - sym_link
        - watcher
        - other
      - `effect_type`: One of:
        - rename
        - modify
        - create
        - destroy
        - owner
        - other
      - `effect_time`:
        The time of the event in nanoseconds since epoch.

    The `watcher` type is special.
    Events with this type will include messages from
    the watcher. You may recieve error messages or
    important status updates.

    The first event always has a `create` value for the
    `effect_type`, a `watcher` value for the `path_type`,
    and a status message in the `path_name` field; Either
    "s/self/live@{some path}" or "e/self/live@{some path}".

    Similarly, the last event always carries `destroy` and
    `watcher` in the `effect_type` and `path_type` fields.
    The `path_name` field will have the same message as the
    first event, except for "die" instead of "live". */

struct event {

private:
  /*  I like these names. Very human. */
  using Nanos = std::chrono::nanoseconds;
  using Clock = std::chrono::system_clock;
  using TimePoint = std::chrono::time_point<Clock>;

public:
  /*  Ensure the user's callback can receive
      events and will return nothing. */
  using callback = std::function<void(event const&)>;

  /*  Represents "what happened" to a path. */
  enum class effect_type {
    rename,
    modify,
    create,
    destroy,
    owner,
    other,
  };

  /*  The essential types of paths. */
  enum class path_type {
    dir,
    file,
    hard_link,
    sym_link,
    watcher,
    other,
  };

  std::filesystem::path path_name{};

  enum effect_type effect_type{};

  enum path_type path_type{};

  long long effect_time{std::chrono::duration_cast<Nanos>(
                          TimePoint{Clock::now()}.time_since_epoch())
                          .count()};

  std::unique_ptr<event> associated{nullptr};

  inline event() noexcept = default;

  inline event(event const& from) noexcept
      : path_name{from.path_name}
      , effect_type{from.effect_type}
      , path_type{from.path_type}
      , effect_time{from.effect_time}
      , associated{
          from.associated ? std::make_unique<event>(*from.associated)
                          : nullptr} {};

  inline event(
    std::filesystem::path const& path_name,
    enum effect_type effect_type,
    enum path_type path_type) noexcept
      : path_name{path_name}
      , effect_type{effect_type}
      , path_type{path_type} {};

  inline event(event const& base, event&& associated) noexcept
      : path_name{base.path_name}
      , effect_type{base.effect_type}
      , path_type{base.path_type}
      , associated{std::make_unique<event>(std::forward<event>(associated))} {};

  inline ~event() noexcept = default;

  inline auto operator=(event const& from) noexcept -> event const&
  {
    this->path_name = from.path_name;
    this->effect_type = from.effect_type;
    this->path_type = from.path_type;
    this->effect_time = from.effect_time;
    this->associated = from.associated
                     ? std::make_unique<event>(*from.associated)
                     : nullptr;
    return *this;
  };

  /*  An equality comparison for all the fields in this object.
      Includes the `effect_time`, which might not be wanted,
      because the `effect_time` is typically (not always) unique. */
  inline friend auto operator==(event const& l, event const& r) noexcept -> bool
  {
    return l.path_name == r.path_name && l.effect_time == r.effect_time
        && l.path_type == r.path_type && l.effect_type == r.effect_type
        && (l.associated && r.associated ? *l.associated == *r.associated
                                         : ! l.associated && ! r.associated);
  }

  inline friend auto operator!=(event const& l, event const& r) noexcept -> bool
  {
    return ! (l == r);
  }
};

} /*  namespace watcher */

// clang-format off

namespace {

#define wtr_effect_type_to_str_lit(Char, from, Lit) \
  switch (from) { \
    case ::wtr::event::effect_type::rename  : return Lit"rename"; \
    case ::wtr::event::effect_type::modify  : return Lit"modify"; \
    case ::wtr::event::effect_type::create  : return Lit"create"; \
    case ::wtr::event::effect_type::destroy : return Lit"destroy"; \
    case ::wtr::event::effect_type::owner   : return Lit"owner"; \
    case ::wtr::event::effect_type::other   : return Lit"other"; \
    default                                 : return Lit"other"; \
  }

#define wtr_path_type_to_str_lit(Char, from, Lit) \
  switch (from) { \
    case ::wtr::event::path_type::dir       : return Lit"dir"; \
    case ::wtr::event::path_type::file      : return Lit"file"; \
    case ::wtr::event::path_type::hard_link : return Lit"hard_link"; \
    case ::wtr::event::path_type::sym_link  : return Lit"sym_link"; \
    case ::wtr::event::path_type::watcher   : return Lit"watcher"; \
    case ::wtr::event::path_type::other     : return Lit"other"; \
    default                                 : return Lit"other"; \
  }

#define wtr_event_to_str_cls_as_json(Char, from, Lit)                         \
  using Cls = std::basic_string<Char>;                                        \
  auto&& etm = Lit"\"" + to<Cls>(from.effect_time) + Lit"\"";                 \
  auto&& ety = Lit"\"" + to<Cls>(from.effect_type) + Lit"\"";                 \
  auto&& pnm = Lit"\"" + to<Cls>(from.path_name)   + Lit"\"";                 \
  auto&& pty = Lit"\"" + to<Cls>(from.path_type)   + Lit"\"";                 \
  return {                         etm + Lit":{"                              \
         + Lit"\"effect_type\":" + ety + Lit","                               \
         + Lit"\"path_name\":"   + pnm + Lit","                               \
         + Lit"\"path_type\":"   + pty                                        \
         + [&]() -> Cls {                                                     \
              if (! from.associated) return Cls{};                            \
              auto asc = from.associated.get();                               \
              auto ttl = Cls{Lit",\"associated\""};                           \
              auto ety = Lit"\"" + to<Cls>(asc->effect_type) + Lit"\"";       \
              auto pnm = Lit"\"" + to<Cls>(asc->path_name)   + Lit"\"";       \
              auto pty = Lit"\"" + to<Cls>(asc->path_type)   + Lit"\"";       \
              return { ttl                                                    \
                     + Lit":{"                                                \
                     + Lit"\"effect_type\":" + ety + Lit","                   \
                     + Lit"\"path_name\":"   + pnm + Lit","                   \
                     + Lit"\"path_type\":"   + pty + Lit"}"                   \
                     };                                                       \
           }()                                                                \
         + Lit"}"                                                             \
         };

/*  For types larger than char and/or char8_t, we can just cast
    each element in our `char` buffer to an `unsigned char`, and
    then zero-extend the elements to any of `wchar_t`, `char16_t`
    and/or `char32_t`.
    We can use something like `format_to(chararray, "{}", from)`
    when library support is more common.
    If we support C++20 later on, we should parameterize `from`
    as `std::integral auto from` and add `char8_t` to the list
    of allowed narrow character types. */
template<class Char>
inline auto num_to_str(long long from) noexcept -> std::basic_string<Char> {
  static_assert(std::is_integral_v<decltype(from)>);
  static constexpr bool is_sys_sane_narrow_char_size =
      (sizeof(char) == sizeof(signed char))
      && (sizeof(char) == sizeof(unsigned char))
  ;
  static constexpr bool is_narrow_char = (
    is_sys_sane_narrow_char_size
    && (  std::is_same_v<Char, char>
       || std::is_same_v<Char, signed char>
       || std::is_same_v<Char, unsigned char>
    )
  );
  static constexpr bool is_wide_char = (
       std::is_same_v<Char, wchar_t>
    || std::is_same_v<Char, char16_t>
    || std::is_same_v<Char, char32_t>
  );
  static_assert(is_narrow_char || is_wide_char);

  static constexpr auto buflen = std::numeric_limits<decltype(from)>::digits10 + 1;
  auto buf = std::array<char, buflen>{0};
  auto [_, ec] = std::to_chars(buf.data(), buf.data() + buf.size(), from);
  if (ec != std::errc{}) {
    return {};
  }
  else if constexpr (is_narrow_char) {
    return {reinterpret_cast<Char const*>(buf.data()), buflen};
  }
  else if constexpr (is_wide_char) {
    auto xbuf = std::array<Char, buflen>{0};
    for (std::size_t i = 0; i < buflen; ++i)
      xbuf[i] = static_cast<Char>(static_cast<unsigned char>(buf[i]));
    return {xbuf.data(), buflen};
  }
};

} /*  namespace */

/*  We use function templates, as opposed to class templates with an
    overloaded () operator, so that we can avoid the extra () or {}
    when (needlessly) "constructing" the class and to make extending
    the templates once or more for the same type (a-la parameterize the
    return type *and* the arguments) possible.

    The `template<template<class...> class... Ts> auto to(auto) noexcept`
    is just for the user. It allows specialization outside of this file,
    and has no effect within this file (it does not guide deduction here).

    Some downsides:
    `std::hash` takes the class template approach. That's a (well?) known
    api. It's not ideal that we miss out on a user's existing familiarity
    with that pattern.
    We need to "declare" a function template before "defining" it:
    `template<class T> auto to(OurTypeOrConcept) -> T` must come before
    the `template<> auto to(...) -> T { ... }`. I'm not sure why class
    templates can get away without declaring a template like that, or
    why we need to do that at all.
    Another downside to the function templates is that we need to "declare"
    function templates before "defining" them. That's not exactly intuitive
    to me, I'm fuzzy on why that is, and users probably shouldn't need to
    remember that whenever they specialize this template.
    We need to declare a template before defining it:
    `template<class T> auto to(OurTypeOrConcept) -> T` must come before
    the `template<> auto to(...) -> T { ... }`. I'm not sure why class
    templates can get away without declaring a template like that, or
    why we need to do that at all.

    Beware of linker errors when using a concept as an argument type.
    AFAICT This is something that can't be done, or I haven't figured out
    how just yet.

    The upsides to function templates are:
    - We get to a concise api, `to<Type>(from_thing)`
    - The user can further specialize the function templates, i.e. We can
      have a specialization `to<string>(type_in_file_a)` and, somewhere else,
      `to<string>(type_in_file_b)` as well. Class templates cannot do that. */

template<class T, class... Ts> auto to(T) noexcept -> decltype(auto);

template<class T> auto to (long long                                from) noexcept -> T ;
template<class T> auto to (decltype(::wtr::event::effect_time)      from) noexcept -> T ;
template<class T> auto to (enum     ::wtr::event::effect_type       from) noexcept -> T ;
template<class T> auto to (decltype(::wtr::event::path_name) const& from) noexcept -> T ;
template<class T> auto to (enum     ::wtr::event::path_type         from) noexcept -> T ;
template<class T> auto to (         ::wtr::event             const& from) noexcept -> T ;

template<> inline constexpr auto to<std::basic_string_view<char>>   (enum     ::wtr::event::effect_type       from) noexcept -> std::basic_string_view<char>    { wtr_effect_type_to_str_lit(  char,    from,   ""          );  }
template<> inline constexpr auto to<std::basic_string_view<char>>   (enum     ::wtr::event::path_type         from) noexcept -> std::basic_string_view<char>    { wtr_path_type_to_str_lit(    char,    from,   ""          );  }
template<> inline           auto to<std::basic_string     <char>>   (decltype(::wtr::event::path_name) const& from) noexcept -> std::basic_string<char>         { return {                     from.generic_string()        };  }
template<> inline           auto to<std::basic_string     <char>>   (enum     ::wtr::event::path_type         from) noexcept -> std::basic_string<char>         { wtr_path_type_to_str_lit(    char,    from,   ""          );  }
template<> inline           auto to<std::basic_string     <char>>   (decltype(::wtr::event::effect_time)      from) noexcept -> std::basic_string<char>         { return num_to_str           <char>(   from                );  }
template<> inline           auto to<std::basic_string     <char>>   (enum     ::wtr::event::effect_type       from) noexcept -> std::basic_string<char>         { wtr_effect_type_to_str_lit(  char,    from,   ""          );  }
template<> inline           auto to<std::basic_string     <char>>   (         ::wtr::event             const& from) noexcept -> std::basic_string<char>         { wtr_event_to_str_cls_as_json(char,    from,   ""          );  }
template<> inline constexpr auto to<std::basic_string_view<wchar_t>>(enum     ::wtr::event::effect_type       from) noexcept -> std::basic_string_view<wchar_t> { wtr_effect_type_to_str_lit(  wchar_t, from,  L""          );  }
template<> inline constexpr auto to<std::basic_string_view<wchar_t>>(enum     ::wtr::event::path_type         from) noexcept -> std::basic_string_view<wchar_t> { wtr_path_type_to_str_lit(    wchar_t, from,  L""          );  }
template<> inline           auto to<std::basic_string     <wchar_t>>(enum     ::wtr::event::path_type         from) noexcept -> std::basic_string<wchar_t>      { wtr_path_type_to_str_lit(    wchar_t, from,  L""          );  }
template<> inline           auto to<std::basic_string     <wchar_t>>(decltype(::wtr::event::path_name) const& from) noexcept -> std::basic_string<wchar_t>      { return {                     from.generic_wstring()       };  }
template<> inline           auto to<std::basic_string     <wchar_t>>(decltype(::wtr::event::effect_time)      from) noexcept -> std::basic_string<wchar_t>      { return num_to_str           <wchar_t>(from                );  }
template<> inline           auto to<std::basic_string     <wchar_t>>(enum     ::wtr::event::effect_type       from) noexcept -> std::basic_string<wchar_t>      { wtr_effect_type_to_str_lit(  wchar_t, from,  L""          );  }
template<> inline           auto to<std::basic_string     <wchar_t>>(         ::wtr::event             const& from) noexcept -> std::basic_string<wchar_t>      { wtr_event_to_str_cls_as_json(wchar_t, from,  L""          );  }
/*
template<> inline constexpr auto to<std::basic_string_view<char8_t>>(enum     ::wtr::event::effect_type       from) noexcept -> std::basic_string_view<char8_t> { wtr_effect_type_to_str_lit(  char8_t, from, u8""          );  }
template<> inline constexpr auto to<std::basic_string_view<char8_t>>(enum     ::wtr::event::path_type         from) noexcept -> std::basic_string_view<char8_t> { wtr_path_type_to_str_lit(    char8_t, from, u8""          );  }
template<> inline           auto to<std::basic_string     <char8_t>>(enum     ::wtr::event::path_type         from) noexcept -> std::basic_string<char8_t>      { wtr_path_type_to_str_lit(    char8_t, from, u8""          );  }
template<> inline           auto to<std::basic_string     <char8_t>>(decltype(::wtr::event::path_name) const& from) noexcept -> std::basic_string<char8_t>      { return {                     from.u8string()              };  }
template<> inline           auto to<std::basic_string     <char8_t>>(decltype(::wtr::event::effect_time)      from) noexcept -> std::basic_string<char8_t>      { return num_to_str           <char8_t>(from                );  }
template<> inline           auto to<std::basic_string     <char8_t>>(enum     ::wtr::event::effect_type       from) noexcept -> std::basic_string<char8_t>      { wtr_effect_type_to_str_lit(  char8_t, from, u8""          );  }
template<> inline           auto to<std::basic_string     <char8_t>>(         ::wtr::event             const& from) noexcept -> std::basic_string<char8_t>      { wtr_event_to_str_cls_as_json(char8_t, from, u8""          );  }
*/

#undef wtr_effect_type_to_str_lit
#undef wtr_path_type_to_str_lit
#undef wtr_event_to_str_cls_as_json

template<class Char, class Traits>
inline auto operator<<(
  std::basic_ostream<Char, Traits>& into,
  enum ::wtr::event::effect_type from) noexcept
-> std::basic_ostream<Char, Traits>&
{ return into << to<std::basic_string<Char, Traits>>(from); };

template<class Char, class Traits>
inline auto operator<<(
  std::basic_ostream<Char, Traits>& into,
  enum ::wtr::event::path_type from) noexcept
-> std::basic_ostream<Char, Traits>&
{ return into << to<std::basic_string<Char, Traits>>(from); };

/*  Streams out `path_name`, `effect_type` and `path_type`.
    Formats the stream as a json object.
    Looks like this (without line breaks)
      "1678046920675963000":{
       "effect_type":"create",
       "path_name":"/some_file.txt",
       "path_type":"file"
      } */
template<class Char, class Traits>
inline auto operator<<(
  std::basic_ostream<Char, Traits>& into,
  ::wtr::event const& from) noexcept
-> std::basic_ostream<Char, Traits>&
{ return into << to<std::basic_string<Char, Traits>>(from); };

// clang-format on

} /*  namespace wtr   */
