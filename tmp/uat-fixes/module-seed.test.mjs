import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import assert from 'node:assert/strict';

// Reproduce fresh PostgreSQL identity allocation and Save-by-ID using the actual
// seed definitions. Every key-to-ID used by permission seeding must survive.
test('module seeding preserves identities after a database reset', () => {
    const source = readFileSync(new URL('../../seed/main.go', import.meta.url), 'utf8');
    const definitions = source.split('modules := []models.AppModule{')[1].split('\n\t}')[0];
    const rows = new Map();
    const permissions = new Map();
    let sequence = 0;
    for (const line of definitions.split('\n')) {
        const key = line.match(/ModuleKey:\s*"([^"]+)"/)?.[1];
        if (!key) continue;
        const fixed = line.match(/\bID:\s*(\d+)/)?.[1];
        const id = fixed ? Number(fixed) : ++sequence;
        rows.set(id, key);
        permissions.set(key, id);
        if (fixed) sequence = Math.max(...rows.keys());
    }
    for (const [key, id] of permissions) {
        assert.equal(rows.get(id), key, `${key} was overwritten at ID ${id}`);
    }
    assert.ok([...rows.values()].includes('DashboardPage'));
    assert.ok([...rows.values()].includes('MyPayslipsPage'));
});
