// Not at all intended to be a comprehensive test suite.
// Leave that to other places, especially when testing accuracy and performance.
// This should just ensure our module's api and basic functionality is working.

import fs from 'fs/promises';
import { tmpdir as ostmpdir } from 'os';
import path from 'path';
import * as watcher from 'watcher';

const sleep = (ms) => new Promise(resolve => setTimeout(resolve, ms));

const withTestDir = async (func) => {
  const p = await fs.realpath(ostmpdir())
  const randint = Math.floor(Math.random() * 1000000);
  const d = path.join(p, `${randint}`); // Lower dirs may be reused. These are not.
  await fs.mkdir(d);
  await func(d);
  await fs.rm(d, { recursive: true });
};

const withoutEffectTime = (event) => {
  const { effectTime, ...rest } = event;
  return rest;
}

const mkCbCheckingLifetime = (testDir, checkingEventFunc) => {
  const testDirCreationEvent = (testDir) => ({
    pathName: testDir,
    effectType: watcher.EffectType.create,
    pathType: watcher.PathType.dir,
    associatedPathName: null,
  });
  const inner = (event) => {
    if (event.effectType === watcher.EffectType.create && event.pathType === watcher.PathType.watcher) {
      expect(event.pathName).toStrictEqual(`s/self/live@${testDir}`);
    } else if (event.pathName == testDir) {
      expect(event).toStrictEqual(testDirCreationEvent(testDir));
    } else if (event.effectType !== watcher.EffectType.destroy && event.pathType !== watcher.PathType.watcher) {
      checkingEventFunc(event);
    } else {
      expect(event.pathName).toStrictEqual(`s/self/die@${testDir}`);
    }
  };
  return inner;
};

describe('Watcher', () => {
  test('Module is typed ok', async () => {
    await withTestDir((testDir) => {
      expect(typeof watcher.watch).toBe('function');
      const w = watcher.watch(testDir, (_) => {});
      expect(typeof w).toBe('object');
      expect(typeof w.close).toBe('function');
      w.close();
    });
  }, 200);

  test('Detects creation', async () => {
    await withTestDir(async (testDir) => {
      const testFile = path.join(testDir, 'f');
      const cb = mkCbCheckingLifetime(testDir, (event) => {
        expect(event).toStrictEqual({
          pathName: testFile,
          effectType: watcher.EffectType.create,
          pathType: watcher.PathType.file,
          associatedPathName: null,
        });
        w.close();
      });
      const w = watcher.watch(testDir, (event) => cb(withoutEffectTime(event)));
      setTimeout(() => { fs.open(testFile, 'w'); }, 200);
      setTimeout(w.close, 300);
      await sleep(300);
    });
  }, 400);
});
