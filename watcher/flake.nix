{
  description =
    "Filesystem watcher. Works anywhere. Simple, efficient and friendly.";

  inputs =
    { nixpkgs     = { url = "github:nixos/nixpkgs/nixos-unstable"; };
      flake-utils = { url = "github:numtide/flake-utils"; };
      snitch-repo = { url = "github:cschreib/snitch"; flake = false; };
    };

  outputs =
    { self
    , nixpkgs
    , flake-utils
    , snitch-repo
    , ...
    }:
    (
      flake-utils.lib.eachDefaultSystem(system:
        let
          pkgs = import nixpkgs { inherit system; };

          build_deps = [ pkgs.clang pkgs.cmake ];
          # If on Darwin, we need this to use FSEvents, dispatch, etc.
          maybe_sys_deps = pkgs.lib.optionals pkgs.stdenv.isDarwin [ pkgs.darwin.apple_sdk.frameworks.CoreServices ];

          snitch = pkgs.stdenv.mkDerivation {
            pname = "snitch";
            version = "0.1.0";
            src = snitch-repo;
            nativeBuildInputs = build_deps;
            buildPhase = ''cmake --build .'';
            installPhase = ''cmake --install . --prefix "$out"'';
          };

          mkWatcherDerivation =
            { src
            , pname
            , buildcfg
            , targets
            # We should just make this a regular target
            , installBashScriptName ? ""
            , installBashScript ? ""
            }: pkgs.stdenv.mkDerivation {
              inherit src pname buildcfg targets;
              version = "0.14.5"; # hook: tool/release
              nativeBuildInputs = build_deps ++ maybe_sys_deps ++ [ snitch ];
              env.WTR_WATCHER_USE_SYSTEM_SNITCH = 1;
              buildPhase = ''
                echo "Building in $PWD/$buildcfg"
                cmake \
                  -S .. \
                  -B "$buildcfg" \
                  -DCMAKE_BUILD_TYPE="$buildcfg" \
                  -DCMAKE_INSTALL_PREFIX="$out"
                for target in $targets
                do
                  cmake \
                    --build "$buildcfg" \
                    --config "$buildcfg" \
                    --target "$target" \
                    --parallel "$(nproc)"
                  if echo "$target" | grep -q 'hdr_'
                  then cp ../include/wtr/watcher.hpp "$buildcfg"
                  fi
                done
              '';
              installPhase = ''
                echo "Installing targets (${toString targets}) from $PWD/$buildcfg to $out"
                ls -al "$buildcfg"
                for target in $targets
                do
                  if echo "$target" | grep -q 'hdr_'
                  then to=$out/include/wtr/watcher.hpp
                  else to=$out/bin/$target
                  fi
                  fromfile=$(basename "$to")
                  todir=$(dirname "$to")
                  test -d "$todir" || mkdir -p "$todir"
                  mv "$buildcfg/$fromfile" "$to"
                  echo "$target -> $to"
                  stat "$to"
                done
                if [ -n "${installBashScript}" ]
                then
                  echo "${installBashScript}" | tee -a "$out/bin/${installBashScriptName}"
                  chmod +x "$out/bin/${installBashScriptName}"
                fi
              '';
            };

          watcher = mkWatcherDerivation {
            src        = self;
            pname      = "wtr.watcher";
            buildcfg   = "Release";
            targets    = [ "wtr.watcher"
                           "wtr.hdr_watcher" ];
          };
          tw = mkWatcherDerivation {
            src        = self;
            pname      = "tw";
            buildcfg   = "Release";
            targets    = [ "tw" ];
          };
          watcher-cli = mkWatcherDerivation {
            src        = self;
            pname      = "wtr.watcher";
            buildcfg   = "Release";
            targets    = [ "wtr.watcher" ];
          };
          watcher-hdr = mkWatcherDerivation {
            src        = self;
            pname      = "exit";
            buildcfg   = "Release";
            targets    = [ "wtr.hdr_watcher" ];
          };
          watcher-test = mkWatcherDerivation {
            src        = self;
            pname      = "wtr.test_watcher.allsan";
            buildcfg   = "Debug";
            targets    = [ "wtr.test_watcher"
                           "wtr.test_watcher.asan"
                           "wtr.test_watcher.msan"
                           "wtr.test_watcher.tsan"
                           "wtr.test_watcher.ubsan" ];
            installBashScriptName = "wtr.test_watcher.allsan";
            installBashScript = ''
              #!/usr/bin/env bash
              PATH="$PATH:$out/bin"
              mkdir -p /tmp/watcher-test
              cd /tmp/watcher-test
              if command -v wtr.test_watcher      ; then wtr.test_watcher      ; fi
              if command -v wtr.test_watcher.asan ; then wtr.test_watcher.asan ; fi
              if command -v wtr.test_watcher.msan ; then wtr.test_watcher.msan ; fi
              if command -v wtr.test_watcher.tsan ; then wtr.test_watcher.tsan ; fi
              if command -v wtr.test_watcher.ubsan; then wtr.test_watcher.ubsan; fi
              rm -rf /tmp/watcher-test
            '';
          };

          # Bring watcher's tree and all of its dependencies in via inputsFrom,
          # and pre-build watcher via buildInputs so that we can use it right away.
          watcher-devshell = pkgs.mkShell {
            inputsFrom = [ tw watcher-cli watcher-hdr watcher-test ];
            buildInputs = [ watcher ];
          };

        in {
          packages = { inherit watcher tw watcher-cli watcher-hdr watcher-test; };
          defaultApp = flake-utils.lib.mkApp { drv = watcher-cli; };
          defaultPackage = watcher;
          devShell = watcher-devshell;
        }
    )
  );
}
