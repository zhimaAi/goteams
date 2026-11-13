import {
  readFileSync,
  readdirSync,
  renameSync,
  rmSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { gzipSync } from "node:zlib";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const webDir = resolve(scriptDir, "..");
const distDir = resolve(webDir, "dist");
const archiveFile = resolve(webDir, "dist.tar.gz");

function collectEntries(directory, archiveDirectory = "") {
  const result = [];
  const entries = readdirSync(directory, { withFileTypes: true }).sort(
    (a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0),
  );

  for (const entry of entries) {
    const entryPath = resolve(directory, entry.name);
    const archivePath = archiveDirectory
      ? `${archiveDirectory}/${entry.name}`
      : entry.name;

    if (entry.isDirectory()) {
      result.push({ archivePath: `${archivePath}/`, type: "directory" });
      result.push(...collectEntries(entryPath, archivePath));
    } else if (entry.isFile()) {
      result.push({ archivePath, filePath: entryPath, type: "file" });
    } else {
      throw new Error(`不支持归档此文件类型：${entryPath}`);
    }
  }

  return result;
}

function splitTarPath(archivePath) {
  if (Buffer.byteLength(archivePath) <= 100) {
    return { name: archivePath, prefix: "" };
  }

  for (let index = archivePath.lastIndexOf("/"); index > 0;) {
    const prefix = archivePath.slice(0, index);
    const name = archivePath.slice(index + 1);
    if (Buffer.byteLength(prefix) <= 155 && Buffer.byteLength(name) <= 100) {
      return { name, prefix };
    }
    index = archivePath.lastIndexOf("/", index - 1);
  }

  throw new Error(`归档路径过长：${archivePath}`);
}

function writeString(header, value, offset, length) {
  const source = Buffer.from(value);
  if (source.length > length) {
    throw new Error(`tar 字段内容过长：${value}`);
  }
  source.copy(header, offset);
}

function writeOctal(header, value, offset, length) {
  const octal = value.toString(8).padStart(length - 1, "0");
  if (octal.length >= length) {
    throw new Error(`tar 数值字段溢出：${value}`);
  }
  writeString(header, `${octal}\0`, offset, length);
}

function createTarHeader(entry, size) {
  const header = Buffer.alloc(512);
  const { name, prefix } = splitTarPath(entry.archivePath);

  writeString(header, name, 0, 100);
  writeOctal(header, entry.type === "directory" ? 0o755 : 0o644, 100, 8);
  writeOctal(header, 0, 108, 8);
  writeOctal(header, 0, 116, 8);
  writeOctal(header, size, 124, 12);
  writeOctal(header, 0, 136, 12);
  header.fill(0x20, 148, 156);
  writeString(header, entry.type === "directory" ? "5" : "0", 156, 1);
  writeString(header, "ustar\0", 257, 6);
  writeString(header, "00", 263, 2);
  writeString(header, "root", 265, 32);
  writeString(header, "root", 297, 32);
  writeOctal(header, 0, 329, 8);
  writeOctal(header, 0, 337, 8);
  writeString(header, prefix, 345, 155);

  let checksum = 0;
  for (const byte of header) {
    checksum += byte;
  }
  writeString(header, `${checksum.toString(8).padStart(6, "0")}\0 `, 148, 8);

  return header;
}

function createTarArchive(entries) {
  const blocks = [];

  for (const entry of entries) {
    const content =
      entry.type === "file" ? readFileSync(entry.filePath) : Buffer.alloc(0);
    blocks.push(createTarHeader(entry, content.length));

    if (content.length > 0) {
      blocks.push(content);
      const paddingSize = (512 - (content.length % 512)) % 512;
      if (paddingSize > 0) {
        blocks.push(Buffer.alloc(paddingSize));
      }
    }
  }

  // tar 规范以两个空块表示归档结束。
  blocks.push(Buffer.alloc(1024));
  return Buffer.concat(blocks);
}

try {
  statSync(resolve(distDir, "index.html"));
} catch {
  console.error("构建产物不完整：dist/index.html 不存在，已取消归档。");
  process.exit(1);
}

// 归档失败时保留 dist，便于排查，也避免覆盖上一次可用产物。
const temporaryArchive = `${archiveFile}.tmp`;
rmSync(temporaryArchive, { force: true });

try {
  const tarArchive = createTarArchive(collectEntries(distDir));
  const compressedArchive = gzipSync(tarArchive, { level: 9, mtime: 0 });
  // 固定 gzip 的时间和操作系统字段，确保跨平台构建结果一致。
  compressedArchive.fill(0, 4, 8);
  compressedArchive[9] = 255;
  writeFileSync(temporaryArchive, compressedArchive);
} catch (error) {
  rmSync(temporaryArchive, { force: true });
  console.error(`dist 归档失败：${error.message}`);
  process.exit(1);
}

rmSync(archiveFile, { force: true });
try {
  renameSync(temporaryArchive, archiveFile);
} catch (error) {
  rmSync(temporaryArchive, { force: true });
  console.error(`替换归档文件失败：${error.message}`);
  process.exit(1);
}

const archiveSize = statSync(archiveFile).size;
rmSync(distDir, { recursive: true, force: true });

console.log(
  `Frontend assets archived to dist.tar.gz (${(archiveSize / 1024 / 1024).toFixed(2)} MiB); dist has been removed.`,
);
