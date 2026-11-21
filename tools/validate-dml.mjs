// validate-dml.mjs — 校验 data.sql 的 INSERT 目标表/列都存在于 table.sql
import fs from 'node:fs';

const dml = fs.readFileSync('databases/data.sql', 'utf8');
const ddl = fs.readFileSync('databases/table.sql', 'utf8');

const tables = {};
for (const m of ddl.matchAll(/CREATE TABLE `(\w+)`([\s\S]*?)\n\) ENGINE/g)) {
  tables[m[1]] = new Set([...m[2].matchAll(/^\s+`(\w+)`\s/gm)].map((x) => x[1]));
}

let bad = 0;
for (const m of dml.matchAll(/INSERT INTO `(\w+)` \(([^)]*)\) VALUES/g)) {
  const tbl = m[1];
  const cols = [...m[2].matchAll(/`(\w+)`/g)].map((x) => x[1]);
  if (!tables[tbl]) { console.log(`MISSING TABLE ${tbl}`); bad++; continue; }
  const miss = cols.filter((c) => !tables[tbl].has(c));
  console.log(`${tbl}: ${cols.length} cols -> ${miss.length ? 'MISSING ' + miss.join(',') : 'OK'}`);
  bad += miss.length;
}
const rowCount = (dml.match(/^\(/gm) || []).length;
console.log(`seed rows: ${rowCount}, problems: ${bad}`);
process.exit(bad ? 1 : 0);
