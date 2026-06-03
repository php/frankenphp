fn main() {
    let out_dir = std::env::var("OUT_DIR").unwrap();
    let out_dir = std::path::Path::new(&out_dir);
    bindgen::Builder::default()
        .header(format!("watcher-c/include/wtr/watcher-c.h"))
        .parse_callbacks(Box::new(bindgen::CargoCallbacks::new()))
        .generate()
        .unwrap()
        .write_to_file(out_dir.join("watcher_c.rs"))
        .unwrap();
    if cfg!(target_os = "macos") {
        println!("cargo:rustc-link-lib=framework=CoreFoundation");
        println!("cargo:rustc-link-lib=framework=CoreServices");
    }
    cc::Build::new()
        .cpp(true)
        .opt_level(2)
        .files(["watcher-c/src/watcher-c.cpp"].iter())
        .include("watcher-c/include")
        .include("include")
        .flag_if_supported("-std=c++17")
        .flag_if_supported("-fno-exceptions")
        .flag_if_supported("-fno-rtti")
        .flag_if_supported("-fno-omit-frame-pointer")
        .flag_if_supported("-fstrict-enums")
        .flag_if_supported("-fstrict-overflow")
        .flag_if_supported("-fstrict-aliasing")
        .flag_if_supported("-fstack-protector-strong")
        .flag_if_supported("/std:c++17")
        .flag_if_supported("/EHsc")
        .compile("watcher_c");
}
