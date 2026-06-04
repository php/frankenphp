import * as watcher from 'watcher';

const path = process.argv[2] || '.';
var w = watcher.watch(path, (event) => {
  console.log(event);
});

process.stdin.on('data', () => {
  w.close();
  process.exit();
});
