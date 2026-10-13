// Vitest 全局 setup。
//
// 问题：jsdom 当前并未启用 Web Storage，window.localStorage 为 undefined；
// 而 Node 26 又自带一个被禁用（需 --localstorage-file）的原生 localStorage 全局，
// 导致源码中的裸 `localStorage` 引用为 undefined。
//
// 解决：注入一个最小化的内存版 Storage 实现，并挂到 globalThis 与 window，
// 使 auth.ts / request.ts 等源码在测试环境下可用 localStorage。

class MemoryStorage {
  private store = new Map<string, string>()
  get length(): number {
    return this.store.size
  }
  getItem(key: string): string | null {
    return this.store.has(key) ? (this.store.get(key) as string) : null
  }
  setItem(key: string, value: string): void {
    this.store.set(key, String(value))
  }
  removeItem(key: string): void {
    this.store.delete(key)
  }
  clear(): void {
    this.store.clear()
  }
  key(index: number): string | null {
    return Array.from(this.store.keys())[index] ?? null
  }
}

const memoryStorage = new MemoryStorage()

function assignStorage(target: any, prop: string) {
  try {
    Object.defineProperty(target, prop, {
      value: memoryStorage,
      configurable: true,
      writable: true,
    })
  } catch {
    try {
      target[prop] = memoryStorage
    } catch {
      /* 忽略：极少数环境下全局不可写，裸引用将仍不可用 */
    }
  }
}

assignStorage(globalThis, 'localStorage')
try {
  assignStorage(window, 'localStorage')
} catch {
  /* window 可能不存在 */
}
