# Contributions

0. Fork this repository.
1. Checkout the `next` branch.
2. If you change the headers, make the changes on the headers in `devel/include`.
3. Make commits in the form `topic/subtopic: note`, for example: `adapter/linux: syntax`.
4. If you made changes to the headers, run `tool/format && tool/hone --header-content-amalgam > include/watcher/watcher.hpp && git add include && git commit -m 'tool/hone --amalgam'`. This will format the code and create a single-header library for you. (Alternatively, just use a pre-commit tool.)
5. Push your changes and create a pull request.

Thank you.
