export async function getLatestTeamBuddyRelease(signal) {
  // Public GitHub requests must not carry the GoTeams session token.
  const response = await fetch(
    "https://api.github.com/repos/zhimaAi/TeamBuddy/releases/latest",
    {
      headers: { Accept: "application/vnd.github+json" },
      credentials: "omit",
      cache: "no-store",
      signal,
    },
  );
  if (!response.ok)
    throw new Error("Unable to load the latest TeamBuddy release");

  const release = await response.json();
  if (
    typeof release?.tag_name !== "string" ||
    !release.tag_name.trim() ||
    !Array.isArray(release.assets) ||
    release.draft ||
    release.prerelease
  ) {
    throw new Error("Invalid TeamBuddy release");
  }

  return {
    version: release.tag_name.replace(/^v/, ""),
    assets: release.assets
      .filter(
        (asset) =>
          typeof asset?.name === "string" &&
          typeof asset.browser_download_url === "string" &&
          asset.browser_download_url.startsWith(
            "https://github.com/zhimaAi/TeamBuddy/releases/download/",
          ),
      )
      .map((asset) => ({ name: asset.name, url: asset.browser_download_url })),
  };
}
