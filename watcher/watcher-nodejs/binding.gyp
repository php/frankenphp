{
  "targets": [
    {
      # Cflags appear to be completely ignored.
      # Otherwise, we could build some targets with sanitizers,
      # and others with aggressive optimizations.
      "target_name": "watcher-napi",
      "sources": [
        "../watcher-c/src/watcher-c.cpp",
        "lib/watcher-napi.cpp",
      ],
      "include_dirs": [
        "../watcher-c/include",
      ],
      "conditions": [
        ['OS=="mac"', {
          "link_settings": {
            "libraries": [
              "-Wl,-rpath,/usr/local/lib",
            ]
          }
        }]
      ]
    }
  ]
}

