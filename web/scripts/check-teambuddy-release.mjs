import assert from "node:assert/strict";
import { getLatestTeamBuddyRelease } from "../src/api/teambuddy.js";

const originalFetch = globalThis.fetch;
const downloadUrl = (tag) =>
  `https://github.com/zhimaAi/TeamBuddy/releases/download/${tag}/TeamBuddy-windows-x64.exe`;
let tag = "v1.2.3";
const controller = new AbortController();

try {
  globalThis.fetch = async (url, options) => {
    assert.equal(
      url,
      "https://api.github.com/repos/zhimaAi/TeamBuddy/releases/latest",
    );
    assert.equal(options.cache, "no-store");
    assert.equal(options.credentials, "omit");
    assert.equal(options.signal, controller.signal);
    assert.equal(options.headers.Authorization, undefined);
    return {
      ok: true,
      json: async () => ({
        tag_name: tag,
        assets: [
          {
            name: "TeamBuddy-windows-x64.exe",
            browser_download_url: downloadUrl(tag),
          },
          {
            name: "TeamBuddy-windows-arm64.exe",
            browser_download_url: "javascript:alert(1)",
          },
          null,
        ],
      }),
    };
  };
  const first = await getLatestTeamBuddyRelease(controller.signal);
  assert.equal(first.version, "1.2.3");
  assert.deepEqual(first.assets, [
    { name: "TeamBuddy-windows-x64.exe", url: downloadUrl(tag) },
  ]);

  tag = "v2.0.0";
  const next = await getLatestTeamBuddyRelease(controller.signal);
  assert.equal(next.version, "2.0.0");
  assert.equal(next.assets[0].url, downloadUrl(tag));

  globalThis.fetch = async () => ({
    ok: true,
    json: async () => ({ tag_name: tag, assets: [] }),
  });
  assert.deepEqual((await getLatestTeamBuddyRelease()).assets, []);

  for (const payload of [
    null,
    {},
    { tag_name: " ", assets: [] },
    { tag_name: tag, assets: [], draft: true },
    { tag_name: tag, assets: [], prerelease: true },
  ]) {
    globalThis.fetch = async () => ({ ok: true, json: async () => payload });
    await assert.rejects(
      getLatestTeamBuddyRelease(),
      /Invalid TeamBuddy release/,
    );
  }

  globalThis.fetch = async () => ({ ok: false, status: 403 });
  await assert.rejects(getLatestTeamBuddyRelease(), /Unable to load/);
  globalThis.fetch = async () => {
    throw new DOMException("Aborted", "AbortError");
  };
  await assert.rejects(getLatestTeamBuddyRelease(), { name: "AbortError" });
  console.log("TeamBuddy latest release checks passed");
} finally {
  globalThis.fetch = originalFetch;
}
