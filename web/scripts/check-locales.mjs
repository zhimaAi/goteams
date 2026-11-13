// 校验 locale 文案能被 vue-i18n 编译，防止 @ 等特殊语法在运行时才抛错导致页面白屏。
// 用法：node scripts/check-locales.mjs
import { baseCompile } from "@intlify/message-compiler";
import zhCN from "../src/locales/zh-CN.js";
import enUS from "../src/locales/en-US.js";

const failures = [];
for (const [locale, messages] of [
  ["zh-CN", zhCN],
  ["en-US", enUS],
]) {
  for (const [key, value] of Object.entries(messages)) {
    if (typeof value !== "string") continue;
    let error = null;
    baseCompile(value, {
      onError(err) {
        error = err;
      },
    });
    if (error) failures.push(`${locale}.${key}: ${error.message} -> ${value}`);
  }
}

if (failures.length) {
  console.error(`locale message compile failed (${failures.length}):`);
  failures.forEach((item) => console.error("  " + item));
  process.exit(1);
}
console.log("locale messages ok");
