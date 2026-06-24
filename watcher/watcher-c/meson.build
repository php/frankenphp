# Libwatcher-c depends on ../include/wtr/watcher.hpp, but meson doesn't allow
# that kind of structure. We can just copy the header file around ourselves.
portable_cp = join_paths(meson.source_root(), 'tool', 'portable-cp')
watcher_hpp_src = join_paths(meson.source_root(), 'include', 'wtr', 'watcher.hpp')
watcher_hpp_dst = join_paths(meson.source_root(), 'watcher-c', 'include', 'wtr', 'watcher.hpp')
watcher_hpp = custom_target(
  'watcher_hpp',
  build_by_default : true,
  command : [portable_cp, watcher_hpp_src, watcher_hpp_dst],
  output : 'watcher.hpp',
  install : false,
)
watcher_hpp_dep = declare_dependency(sources : watcher_hpp)

libwatcher_c_name = 'watcher-c'

libwatcher_c_pc_cfgdata = configuration_data()
libwatcher_c_pc_cfgdata.set('PC_LIBWATCHER_C_PREFIX', get_option('prefix'))
libwatcher_c_pc_cfgdata.set('PC_LIBWATCHER_C_LIBDIR', join_paths(get_option('prefix'), get_option('libdir')))
libwatcher_c_pc_cfgdata.set('PC_LIBWATCHER_C_INCLUDEDIR', join_paths(get_option('prefix'), get_option('includedir')))
libwatcher_c_pc_cfgdata.set('PROJECT_DESCRIPTION', 'watcher: a filesystem watcher')
libwatcher_c_pc_cfgdata.set('PROJECT_HOMEPAGE_URL', 'github.com/e-dant/watcher')
libwatcher_c_pc_cfgdata.set('PROJECT_VERSION', meson.project_version())

if target_machine.system() == 'darwin'
  libwatcher_c_deps = [dependency('CoreServices'), dependency('CoreFoundation'), watcher_hpp_dep]
  libwatcher_c_pc_cfgdata.set('PC_LIBWATCHER_C_LIBS_PRIVATE', '-framework CoreFoundation -framework CoreServices')
else
  libwatcher_c_deps = [dependency('threads'), watcher_hpp_dep]
  libwatcher_c_pc_cfgdata.set('PC_LIBWATCHER_C_LIBS_PRIVATE', '-lpthread')
endif

libwatcher_c = library(
  libwatcher_c_name,
  ['src/watcher-c.cpp'],
  include_directories : ['include'],
  dependencies : libwatcher_c_deps,
  build_rpath : '/usr/local/lib',
  install_rpath : '/usr/local/lib',
  install : true,
  version : meson.project_version(),
)

libwatcher_c_pc = configure_file(
  input : 'watcher-c.pc.in',
  output : 'watcher-c.pc',
  configuration : libwatcher_c_pc_cfgdata,
  install : true,
  install_dir : join_paths(get_option('libdir'), 'pkgconfig')
)

test_libwatcher_c = executable(
  'test-watcher-c',
  ['src/test-watcher-c.c'],
  include_directories : ['include'],
  link_with : [libwatcher_c],
  build_rpath : '/usr/local/lib',
  install_rpath : '/usr/local/lib',
  install : false,
)

subdir('include')
