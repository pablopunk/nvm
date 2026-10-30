import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createDashboardTableUpdates } from './dashboard-table-updates';

test('focused rows and their month update together after focus leaves', () => {
  const updates = createDashboardTableUpdates();
  const table = {};
  let rows = 'October';
  let label = 'October';
  updates.update(table, true, () => { rows = 'November'; label = 'November'; });
  assert.equal(rows, 'October');
  assert.equal(label, 'October');
  updates.flush(table, true);
  assert.equal(label, 'October');
  updates.flush(table, false);
  assert.equal(rows, 'November');
  assert.equal(label, 'November');
});

test('superseded pending rows cannot overwrite a newer selection', () => {
  const updates = createDashboardTableUpdates();
  const table = {};
  let rows = 'October';
  updates.update(table, true, () => { rows = 'November'; });
  updates.cancel(table);
  updates.flush(table, false);
  assert.equal(rows, 'October');
  updates.update(table, false, () => { rows = 'December'; });
  assert.equal(rows, 'December');
});
