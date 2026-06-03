# Notes

## Character String Conversions

### C++23 `resive_and_overwrite`

> https://cpplang.slack.com/archives/C21PKDHSL/p1672705999558719

```cpp
#include <cstddef>
#include <string>
#include <Windows.h>

std::string wstring_to_string(std::wstring const& in) {
  int out_len = WideCharToMultiByte(CP_UTF8, 0, in.data(), in.size(), nullptr,
                                    0, nullptr, nullptr);
  std::string out;
  if (out_len != 0) {
    out.resize_and_overwrite(out_len, [&in](char* out, std::size_t out_len) {
      return WideCharToMultiByte(CP_UTF8, 0, in.data(), in.size(), out, out_len,
                                 nullptr, nullptr);
    });
  }
  return out;
}
```

### Full set of (expensive) conversions

```cpp
inline std::string wstring_to_string(std::wstring const& in) noexcept
{
  size_t in_len
      = WideCharToMultiByte(CP_UTF8, 0, in.data(), static_cast<int>(in.size()),
                            nullptr, 0, nullptr, nullptr);
  if (in_len < 1)
    return std::string{};
  else {
    std::unique_ptr<char> out(new char[in_len]);
    size_t out_len = WideCharToMultiByte(
        CP_UTF8, 0, in.data(), static_cast<int>(in.size()), out.get(),
        static_cast<int>(in_len), nullptr, nullptr);
    if (out_len < 1)
      return std::string{};
    else
      return std::string{out.get(), in_len};
  }
}

inline std::wstring string_to_wstring(std::string const& in) noexcept
{
  size_t in_len = MultiByteToWideChar(CP_UTF8, 0, in.data(),
                                      static_cast<int>(in.size()), 0, 0);
  if (in_len < 1)
    return std::wstring{};
  else {
    std::unique_ptr<wchar_t> out(new wchar_t[in_len]);
    size_t out_len = MultiByteToWideChar(CP_UTF8, 0, in.data(),
                                         static_cast<int>(in.size()), out.get(),
                                         static_cast<int>(in_len));
    if (out_len < 1)
      return std::wstring{};
    else
      return std::wstring{out.get(), in_len};
  }
}

inline std::wstring cstring_to_wstring(char const* in) noexcept
{
  auto in_len = strlen(in);
  auto out = std::vector<wchar_t>(in_len + 1);
  mbstowcs_s(nullptr, out.data(), in_len + 1, in, in_len);
  return std::wstring(out.data());
}
```
