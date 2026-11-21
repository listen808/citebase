// 品牌图标的字节级工装：PNG 分块读写、tEXt 溯源戳、ICO 目录的拼装与解析。纯 Node，零依赖。
//
// 为什么单独成一个模块：生成器（scripts/gen-brand-icons.mjs）**写**这些字节，
// 门禁（tests/brand-closure.test.ts）**读**回来核对声明。两边各写一遍 IHDR 偏移量，
// 就会有一天只改一边 —— 本仓的老病正是"判据抄第二份数学"（见 docs/gates/static-gates.md
// 里 glass-contrast 那段同一口径）。
//
// 复用先查过：仓库里唯一现成的 PNG 构造函数在 Go 测试里
// （internal/api/workspace/document_test.go 的 createTestPNG），跨语言、且测试环境拿不到，
// 所以这里自己写；除此之外全仓 0 处 ICO/PNG 字节解析。
import crypto from 'node:crypto';

/** PNG 签名与 tEXt 里"这枚光栅出自哪套几何"那条戳的关键字 */
const PNG_SIGNATURE = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
export const PROVENANCE_KEYWORD = 'BrandSrc';

/* ------------------------------ CRC32（PNG 分块校验用） ------------------------------ */

const CRC_TABLE = (() => {
  const table = new Int32Array(256);
  for (let n = 0; n < 256; n += 1) {
    let c = n;
    for (let k = 0; k < 8; k += 1) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    table[n] = c;
  }
  return table;
})();

/** PNG 用的标准 CRC-32（IEEE）；计算范围含类型段，不含长度段与 CRC 本身 */
export function crc32(buf) {
  let c = 0xffffffff;
  for (const byte of buf) c = CRC_TABLE[(c ^ byte) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}

/* ------------------------------ PNG ------------------------------ */

const pngChunkBuffer = (type, data) => {
  const body = Buffer.concat([Buffer.from(type, 'ascii'), data]);
  const head = Buffer.alloc(4);
  head.writeUInt32BE(data.length, 0);
  const tail = Buffer.alloc(4);
  tail.writeUInt32BE(crc32(body), 0);
  return Buffer.concat([head, body, tail]);
};

/**
 * 拆开一个 PNG。逐块校验 CRC —— 手写分块最容易错在偏移量上，
 * 而错了的症状是"浏览器里图标不见了"，比在这里抛出来难查得多。
 * @returns {{ type: string, data: Buffer }[]}
 */
export function pngChunks(buf, label = 'PNG') {
  if (buf.subarray(0, 8).compare(PNG_SIGNATURE) !== 0) throw new Error(`${label} 不是 PNG（签名不对）`);
  const chunks = [];
  let off = 8;
  while (off < buf.length) {
    const length = buf.readUInt32BE(off);
    const type = buf.toString('ascii', off + 4, off + 8);
    const data = buf.subarray(off + 8, off + 8 + length);
    const crc = buf.readUInt32BE(off + 8 + length);
    if (data.length !== length) throw new Error(`${label}：${type} 块声明 ${length} 字节，实际只剩 ${data.length}`);
    if (crc32(Buffer.concat([Buffer.from(type, 'ascii'), data])) !== crc) throw new Error(`${label}：${type} 块 CRC 校验不过`);
    chunks.push({ type, data });
    off += 12 + length;
  }
  return chunks;
}

/** 真实像素尺寸（IHDR 头，与文件叫什么名字无关） */
export function pngDims(buf, label = 'PNG') {
  const ihdr = pngChunks(buf, label)[0];
  if (!ihdr || ihdr.type !== 'IHDR') throw new Error(`${label}：第一块不是 IHDR`);
  return { width: ihdr.data.readUInt32BE(0), height: ihdr.data.readUInt32BE(4) };
}

/**
 * 在 IHDR 之后插一条 tEXt。选 tEXt 而不是 pHYs/sTER 之类的，是因为它是规范里的
 * "非关键块"：不认识它的解码器一律整块跳过，所以既不会改变显示结果，
 * 又能让门禁在没有浏览器的情况下读出"这枚光栅是哪一版几何生成的"。
 * 位置放在 IHDR 之后、首个 IDAT 之前（规范建议 ancillary 块在数据前）。
 */
export function addPngText(buf, keyword, text, label = 'PNG') {
  const chunks = pngChunks(buf, label);
  const insertAt = 8 + 12 + chunks[0].data.length; // 签名 + IHDR 整块
  const chunk = pngChunkBuffer('tEXt', Buffer.concat([Buffer.from(keyword, 'latin1'), Buffer.from([0]), Buffer.from(text, 'latin1')]));
  return Buffer.concat([buf.subarray(0, insertAt), chunk, buf.subarray(insertAt)]);
}

/** 读一条 tEXt；没有就返回 undefined（调用方决定是警告还是抛） */
export function readPngText(buf, keyword, label = 'PNG') {
  for (const chunk of pngChunks(buf, label)) {
    if (chunk.type !== 'tEXt') continue;
    const sep = chunk.data.indexOf(0);
    if (sep < 0) continue;
    if (chunk.data.toString('latin1', 0, sep) !== keyword) continue;
    return chunk.data.toString('latin1', sep + 1);
  }
  return undefined;
}

/* ------------------------------ ICO ------------------------------ */

/**
 * 多入口 ICO：目录里的宽高按真实尺寸写，载荷直接用 PNG。
 * PNG 载荷进 ICO 是 Vista 起就支持的写法（比 BMP 载荷小得多，且省掉 AND 掩码那套），
 * 而 Windows 的快捷方式/任务栏恰恰只认 .ico —— 正是这次要修的那个面。
 * @param {{ size: number, png: Buffer }[]} entries
 */
export function buildIco(entries) {
  if (!entries.length) throw new Error('ICO 至少要有一条 entry');
  const sizes = entries.map((e) => e.size);
  if (new Set(sizes).size !== sizes.length) throw new Error(`ICO 尺寸重复：${sizes.join(', ')}`);
  const header = Buffer.alloc(6);
  header.writeUInt16LE(0, 0); // 保留段，必须 0
  header.writeUInt16LE(1, 2); // 类型：1 = 图标
  header.writeUInt16LE(entries.length, 4);

  const dirSize = entries.length * 16;
  let dataOffset = 6 + dirSize;
  const dirs = [];
  const blobs = [];
  for (const { size, png } of entries) {
    const dir = Buffer.alloc(16);
    dir.writeUInt8(size >= 256 ? 0 : size, 0); // 256 用 0 表达（规范就这么定的）
    dir.writeUInt8(size >= 256 ? 0 : size, 1);
    dir.writeUInt8(0, 2); // 调色板色数：真彩为 0
    dir.writeUInt8(0, 3);
    dir.writeUInt16LE(1, 4); // 平面数：恒为 1
    dir.writeUInt16LE(32, 6); // 位深：RGBA8
    dir.writeUInt32LE(png.length, 8);
    dir.writeUInt32LE(dataOffset, 12);
    dirs.push(dir);
    blobs.push(png);
    dataOffset += png.length;
  }
  return Buffer.concat([header, ...dirs, ...blobs]);
}

/**
 * 解析 ICO，并把目录声明的宽高与 PNG 载荷 IHDR 里的真实宽高一起交出去 ——
 * "声明 64×64、文件里其实是 32×32"这类谎正是靠这两组数对不上才看得见的。
 * @returns {{ count: number, entries: { declaredWidth: number, declaredHeight: number, bitCount: number, png: Buffer, width: number, height: number, provenance: string | undefined }[] }}
 */
export function parseIco(buf, label = 'ICO') {
  if (buf.length < 6) throw new Error(`${label} 太小，连头都放不下`);
  if (buf.readUInt16LE(0) !== 0) throw new Error(`${label}：保留段不是 0`);
  if (buf.readUInt16LE(2) !== 1) throw new Error(`${label}：类型不是 1（图标）`);
  const count = buf.readUInt16LE(4);
  const entries = [];
  for (let i = 0; i < count; i += 1) {
    const o = 6 + i * 16;
    const declaredWidth = buf.readUInt8(o) || 256;
    const declaredHeight = buf.readUInt8(o + 1) || 256;
    const bitCount = buf.readUInt16LE(o + 6);
    const length = buf.readUInt32LE(o + 8);
    const offset = buf.readUInt32LE(o + 12);
    if (offset + length > buf.length) throw new Error(`${label} 第 ${i + 1} 条越界：偏移 ${offset} + 长度 ${length} > ${buf.length}`);
    const png = buf.subarray(offset, offset + length);
    if (png.subarray(0, 8).compare(PNG_SIGNATURE) !== 0) {
      throw new Error(
        `${label} 第 ${i + 1} 条的载荷不是 PNG —— 本仓的 .ico 载荷一律由 npm run gen:brand-icons 以 PNG 写入` +
          `（实到 ${png.length} 字节，头 4 字节 ${[...png.subarray(0, 4)].map((b) => b.toString(16).padStart(2, '0')).join(' ')}）。` +
          '这通常意味着有人手工塞了一枚旧 BMP 图标进来：重跑生成器即可',
      );
    }
    const { width, height } = pngDims(png, `${label} 第 ${i + 1} 条`);
    entries.push({
      declaredWidth,
      declaredHeight,
      bitCount,
      png: Buffer.from(png),
      width,
      height,
      provenance: readPngText(png, PROVENANCE_KEYWORD, `${label} 第 ${i + 1} 条`),
    });
  }
  return { count, entries };
}

/* ------------------------------ 几何唯一源 ------------------------------ */

/**
 * 从一段文本里取出品牌标记的几何：三条 path 的 d、节点圆的 cx/cy/r、脊的 stroke-width。
 * 同一份实现同时读 ApBrand.vue（组件里的真相源）、favicon.svg、pwa-icon.svg
 * 和生成物上的戳 —— 判据只有一套，才不会"组件改了、门禁还在按旧写法比"。
 *
 * 只取几何、不取颜色与 aria-label：色值与标签由别的判据管
 * （check-contrast 与这台的"图标 aria-label 等于 APP_NAME"）。
 */
export function extractBrandGeometry(text, label) {
  const paths = [...text.matchAll(/\bd="([^"]+)"/g)].map((m) => m[1]);
  const circleTag = /<circle([^>]*)>/.exec(text);
  const attr = (name) => {
    const m = new RegExp(`\\b${name}="([^"]+)"`).exec(circleTag ? circleTag[1] : '');
    return m ? m[1] : undefined;
  };
  const spineWidth = /<path[^>]*\bstroke-width="([^"]+)"/.exec(text)?.[1];
  const cx = attr('cx');
  const cy = attr('cy');
  const r = attr('r');
  // 缺件即抛：正则一旦被改写成"谁都匹配不到"，哈希会三个文件全都算成同一个空串，
  // 门禁于是绿得毫无意义 —— 与本仓"判据空转比不过更糟"那条一致。
  const missing = [
    paths.length === 3 ? '' : `${label}：期望 3 条 path，实到 ${paths.length}`,
    cx && cy && r ? '' : `${label}：节点圆缺 cx/cy/r`,
    spineWidth ? '' : `${label}：中心脊缺 stroke-width`,
  ].filter(Boolean);
  if (missing.length) throw new Error(missing.join('；'));
  return { paths, cx, cy, r, spineWidth };
}

/** 几何的规范化字符串 + sha256。同一套几何在哪个文件里写的、属性顺序如何，哈希都一样 */
export function brandGeometryStamp(text, label) {
  const g = extractBrandGeometry(text, label);
  const canonical = [`paths:${g.paths.join(' | ')}`, `circle:${g.cx},${g.cy},${g.r}`, `spine:${g.spineWidth}`].join('\n');
  return crypto.createHash('sha256').update(canonical, 'utf8').digest('hex');
}
