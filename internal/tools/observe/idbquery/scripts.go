// Purpose: Generates the JavaScript that enumerates IndexedDB databases and reads object-store rows.
// Why: Separates script templates from dispatch and reply handling in execute.go.
// Docs: docs/features/feature/observe/index.md

package idbquery

import (
	"encoding/json"
	"fmt"
)

func buildEntriesScript(database, store string, limit int) string {
	return fmt.Sprintf(`(() => (async () => {
  const database = %s;
  const store = %s;
  const limit = %d;
  try {
    if (typeof indexedDB === "undefined") {
      return { ok: false, error: "indexeddb_unsupported" };
    }

    const openReq = indexedDB.open(database);
    const db = await new Promise((resolve, reject) => {
      openReq.onsuccess = () => resolve(openReq.result);
      openReq.onerror = () => reject(openReq.error || new Error("indexeddb_open_failed"));
      openReq.onblocked = () => reject(new Error("indexeddb_open_blocked"));
    });

    const objectStores = Array.from(db.objectStoreNames || []);
    if (!objectStores.includes(store)) {
      db.close();
      return { ok: false, error: "store_not_found", message: "Object store not found", object_stores: objectStores };
    }

    const tx = db.transaction(store, "readonly");
    const objectStore = tx.objectStore(store);

    function serialize(value, depth = 0) {
      if (depth > 8) return "[max_depth]";
      if (value === null || value === undefined) return value;
      const t = typeof value;
      if (t === "string" || t === "number" || t === "boolean") return value;
      if (t === "bigint") return value.toString();
      if (t === "symbol") return String(value);
      if (Array.isArray(value)) return value.slice(0, 100).map((v) => serialize(v, depth + 1));
      if (value instanceof Date) return value.toISOString();
      if (ArrayBuffer.isView(value)) return Array.from(value).slice(0, 1000);
      if (value instanceof ArrayBuffer) return { byte_length: value.byteLength };
      if (t === "object") {
        const out = {};
        for (const key of Object.keys(value).slice(0, 100)) {
          try {
            out[key] = serialize(value[key], depth + 1);
          } catch {
            out[key] = "[unserializable]";
          }
        }
        return out;
      }
      return String(value);
    }

    const entries = [];
    const pushEntry = (key, value) => {
      entries.push({ key: serialize(key), value: serialize(value) });
    };

    if (typeof objectStore.getAll === "function" && typeof objectStore.getAllKeys === "function") {
      const [values, keys] = await Promise.all([
        new Promise((resolve, reject) => {
          const req = objectStore.getAll(undefined, limit);
          req.onsuccess = () => resolve(req.result || []);
          req.onerror = () => reject(req.error || new Error("indexeddb_get_all_failed"));
        }),
        new Promise((resolve, reject) => {
          const req = objectStore.getAllKeys(undefined, limit);
          req.onsuccess = () => resolve(req.result || []);
          req.onerror = () => reject(req.error || new Error("indexeddb_get_all_keys_failed"));
        })
      ]);
      for (let i = 0; i < Math.min(keys.length, values.length); i++) {
        pushEntry(keys[i], values[i]);
      }
    } else {
      await new Promise((resolve, reject) => {
        const req = objectStore.openCursor();
        req.onerror = () => reject(req.error || new Error("indexeddb_cursor_failed"));
        req.onsuccess = () => {
          const cursor = req.result;
          if (!cursor || entries.length >= limit) {
            resolve(undefined);
            return;
          }
          pushEntry(cursor.key, cursor.value);
          cursor.continue();
        };
      });
    }

    db.close();

    return {
      ok: true,
      database,
      store,
      object_stores: objectStores,
      entries,
      count: entries.length,
      limit
    };
  } catch (e) {
    return { ok: false, error: String((e && e.message) || e) };
  }
})())()`, jsStringLiteral(database), jsStringLiteral(store), limit)
}

const listingScript = `(() => (async () => {
  try {
    if (typeof indexedDB === "undefined") {
      return { supported: false, databases: [] };
    }
    if (typeof indexedDB.databases !== "function") {
      return { supported: false, databases: [], error: "indexeddb_databases_unavailable" };
    }

    const infos = await indexedDB.databases();
    const databases = [];
    for (const info of infos || []) {
      if (!info || !info.name) continue;
      const name = String(info.name);
      const version = typeof info.version === "number" ? info.version : null;
      let objectStores = [];
      try {
        const req = indexedDB.open(name);
        const db = await new Promise((resolve, reject) => {
          req.onsuccess = () => resolve(req.result);
          req.onerror = () => reject(req.error || new Error("indexeddb_open_failed"));
          req.onblocked = () => reject(new Error("indexeddb_open_blocked"));
        });
        objectStores = Array.from(db.objectStoreNames || []);
        db.close();
      } catch {
        objectStores = [];
      }

      databases.push({
        name,
        version,
        object_stores: objectStores
      });
    }

    return { supported: true, databases };
  } catch (e) {
    return { supported: false, databases: [], error: String((e && e.message) || e) };
  }
})())()`

func jsStringLiteral(v string) string {
	b, _ := json.Marshal(v)
	return string(b)
}
